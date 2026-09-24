// Package continueagent emits Continue's native YAML configuration.
package continueagent

import (
	"fmt"
	"strings"

	"agentcfg/internal/artifact"
	"agentcfg/internal/diag"
	"agentcfg/internal/ir"
	"agentcfg/internal/target"

	"go.yaml.in/yaml/v3"
)

type Target struct{}

func init()               { target.Register(Target{}) }
func (Target) ID() string { return "continue" }
func (t Target) Validate(cfg ir.Config) []diag.Diagnostic {
	var ds []diag.Diagnostic
	warn := func(p, m string) { ds = append(ds, diag.TargetWarnf(t.ID(), p, "%s", m)) }
	if cfg.Defaults != nil && cfg.Defaults.Model != "" {
		warn("defaults.model", "Continue keeps model selection in client state; default selection is skipped")
	}
	for i, p := range cfg.Providers {
		path := fmt.Sprintf("providers[%d]", i)
		if p.Protocol == ir.ProtocolOpenAIResponses {
			warn(path+".protocol", "published Continue config-yaml 1.42.0 drops useResponsesApi; Responses provider is skipped")
			continue
		}
		if strings.Contains(p.APIKey.Value, "${{") {
			warn(path+".api_key", "literal contains Continue template syntax; credential is skipped")
		}
		for k, v := range p.Headers {
			if strings.Contains(v.Value, "${{") {
				warn(path+".headers."+k, "literal contains Continue template syntax; header is skipped")
			}
		}
		for j, m := range p.Models {
			mp := fmt.Sprintf("%s.models[%d]", path, j)
			if m.Reasoning != nil || len(m.Variants) > 0 {
				warn(mp+".reasoning", "reasoning capability/variant lists are not request defaults; skipped")
			}
			if m.ToolCalling != nil || len(m.Input) > 0 || len(m.Output) > 0 {
				warn(mp, "Continue capabilities are additive and cannot preserve the full IR capability declaration; modalities and tool flag are skipped")
			}
		}
	}
	for i, s := range cfg.MCP {
		path := fmt.Sprintf("mcp[%d]", i)
		if s.Enabled != nil && !*s.Enabled {
			warn(path, "Continue has no disabled MCP entry; server is skipped")
		}
		if s.TimeoutMS != nil {
			warn(path+".timeout_ms", "Continue connectionTimeout is a startup limit, not a tool timeout; timeout is skipped")
		}
		for _, vs := range []map[string]ir.HeaderValue{s.Env, s.Headers} {
			for k, v := range vs {
				if strings.Contains(v.Value, "${{") {
					warn(path+"."+k, "literal contains Continue template syntax; value is skipped")
				}
			}
		}
	}
	return ds
}

func (t Target) Emit(cfg ir.Config) ([]artifact.Artifact, error) {
	models := []map[string]any{}
	for _, p := range cfg.Providers {
		if p.Protocol == ir.ProtocolOpenAIResponses {
			continue
		}
		for _, m := range p.Models {
			wire := "openai"
			if p.Protocol == ir.ProtocolAnthropicMessages {
				wire = "anthropic"
			}
			entry := map[string]any{"name": p.ID + "/" + m.ID, "provider": wire, "model": m.ID, "apiBase": p.BaseURL}
			if p.EffectiveAuthType() != ir.AuthTypeNone && !strings.Contains(p.APIKey.Value, "${{") {
				if key := ref(p.APIKey); key != "" {
					entry["apiKey"] = key
				}
			}
			if h := refs(p.Headers); len(h) > 0 {
				entry["requestOptions"] = map[string]any{"headers": h}
			}
			options := map[string]any{}
			if m.ContextWindow != nil {
				options["contextLength"] = *m.ContextWindow
			}
			if m.MaxOutputTokens != nil {
				options["maxTokens"] = *m.MaxOutputTokens
			}
			if len(options) > 0 {
				entry["defaultCompletionOptions"] = options
			}
			models = append(models, entry)
		}
	}
	servers := []map[string]any{}
	for _, s := range cfg.MCP {
		if s.Enabled != nil && !*s.Enabled {
			continue
		}
		e := map[string]any{"name": s.ID}
		if s.Transport == ir.TransportStdio {
			if len(s.Command) == 0 {
				return nil, fmt.Errorf("MCP %s has no command", s.ID)
			}
			e["type"] = "stdio"
			e["command"] = s.Command[0]
			if len(s.Command) > 1 {
				e["args"] = s.Command[1:]
			}
			if s.CWD != "" {
				e["cwd"] = s.CWD
			}
			if len(s.Env) > 0 {
				e["env"] = refs(s.Env)
			}
		} else {
			e["type"] = "streamable-http"
			e["url"] = s.URL
			if h := refs(s.Headers); len(h) > 0 {
				e["requestOptions"] = map[string]any{"headers": h}
			}
		}
		servers = append(servers, e)
	}
	doc := map[string]any{"name": "agentcfg", "version": "1.0.0", "schema": "v1", "models": models}
	if len(servers) > 0 {
		doc["mcpServers"] = servers
	}
	b, err := yaml.Marshal(doc)
	if err != nil {
		return nil, err
	}
	return []artifact.Artifact{{Target: t.ID(), Name: "config.yaml", Format: "yaml", SuggestedPath: "~/.continue/config.yaml", Content: b}}, nil
}

func ref(v ir.HeaderValue) string {
	if v.BearerFromEnv != "" {
		return "Bearer ${{ secrets." + v.BearerFromEnv + " }}"
	}
	if v.FromEnv != "" {
		return "${{ secrets." + v.FromEnv + " }}"
	}
	return v.Value
}

func refs(vs map[string]ir.HeaderValue) map[string]string {
	out := map[string]string{}
	for k, v := range vs {
		if !strings.Contains(v.Value, "${{") {
			out[k] = ref(v)
		}
	}
	return out
}
