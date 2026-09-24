// Package aider emits Aider model settings, metadata, and default selection.
package aider

import (
	"encoding/json"
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"

	"agentcfg/internal/artifact"
	"agentcfg/internal/diag"
	"agentcfg/internal/ir"
	"agentcfg/internal/target"
)

type Target struct{}

func init()               { target.Register(Target{}) }
func (Target) ID() string { return "aider" }
func (t Target) Validate(cfg ir.Config) []diag.Diagnostic {
	var ds []diag.Diagnostic
	warn := func(p, m string) { ds = append(ds, diag.TargetWarnf(t.ID(), p, "%s", m)) }
	if len(cfg.MCP) > 0 {
		warn("mcp", "Aider has no native MCP client configuration; MCP entries are skipped")
	}
	for i, p := range cfg.Providers {
		path := fmt.Sprintf("providers[%d]", i)
		if strings.HasPrefix(p.APIKey.Value, "os.environ/") {
			warn(path+".api_key", "literal contains LiteLLM secret-reference syntax; credential is skipped")
		}
		if p.Protocol == ir.ProtocolOpenAIResponses {
			warn(path, "Aider completion routing does not explicitly select the Responses wire; provider is skipped")
			continue
		}
		for k, v := range p.Headers {
			if v.FromEnv != "" || v.BearerFromEnv != "" {
				warn(path+".headers."+k, "Aider extra_headers are literal; environment reference is skipped")
			}
		}
		for j, m := range p.Models {
			mp := fmt.Sprintf("%s.models[%d]", path, j)
			if len(m.Input) > 0 || len(m.Output) > 0 || m.Reasoning != nil || m.ToolCalling != nil || len(m.Variants) > 0 {
				warn(mp, "modality, tool and reasoning capabilities/variants have no direct Aider model-settings mapping; skipped")
			}
		}
	}
	if cfg.Defaults != nil {
		for _, p := range cfg.Providers {
			for _, m := range p.Models {
				if cfg.Defaults.Model == p.ID+"/"+m.ID && p.Protocol == ir.ProtocolOpenAIResponses {
					warn("defaults.model", "default belongs to a skipped Responses provider; default selection is omitted")
				}
			}
		}
	}
	return ds
}

func (t Target) Emit(cfg ir.Config) ([]artifact.Artifact, error) {
	settings := []map[string]any{}
	metadata := map[string]any{}
	config := map[string]any{}
	for _, p := range cfg.Providers {
		if p.Protocol == ir.ProtocolOpenAIResponses {
			continue
		}
		wire := "openai"
		if p.Protocol == ir.ProtocolAnthropicMessages {
			wire = "anthropic"
		}
		for _, m := range p.Models {
			name := p.ID + "/" + m.ID
			params := map[string]any{"model": wire + "/" + m.ID, "api_base": p.BaseURL}
			if p.EffectiveAuthType() != ir.AuthTypeNone {
				if p.APIKey.FromEnv != "" {
					params["api_key"] = "os.environ/" + p.APIKey.FromEnv
				} else if p.APIKey.Value != "" && !strings.HasPrefix(p.APIKey.Value, "os.environ/") {
					params["api_key"] = p.APIKey.Value
				}
			}
			headers := map[string]string{}
			for k, v := range p.Headers {
				if v.FromEnv == "" && v.BearerFromEnv == "" {
					headers[k] = v.Value
				}
			}
			if len(headers) > 0 {
				params["extra_headers"] = headers
			}
			if m.MaxOutputTokens != nil {
				params["max_tokens"] = *m.MaxOutputTokens
			}
			settings = append(settings, map[string]any{"name": name, "extra_params": params})
			info := map[string]any{"litellm_provider": wire, "mode": "chat"}
			if m.ContextWindow != nil {
				info["max_input_tokens"] = *m.ContextWindow
			}
			if m.MaxOutputTokens != nil {
				info["max_output_tokens"] = *m.MaxOutputTokens
				info["max_tokens"] = *m.MaxOutputTokens
			}
			metadata[name] = info
			if cfg.Defaults != nil && cfg.Defaults.Model == name {
				config["model"] = name
			}
		}
	}
	var arts []artifact.Artifact
	for _, doc := range []struct {
		name  string
		value any
	}{{".aider.conf.yml", config}, {".aider.model.settings.yml", settings}} {
		b, err := yaml.Marshal(doc.value)
		if err != nil {
			return nil, err
		}
		arts = append(arts, artifact.Artifact{Target: t.ID(), Name: doc.name, Format: "yaml", SuggestedPath: doc.name, Content: b})
	}
	b, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return nil, err
	}
	arts = append(arts, artifact.Artifact{Target: t.ID(), Name: ".aider.model.metadata.json", Format: "json", SuggestedPath: ".aider.model.metadata.json", Content: append(b, '\n')})
	return arts, nil
}
