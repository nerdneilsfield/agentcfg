// Package fastagent emits fast-agent config and home-local model overlays.
package fastagent

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"

	"agentcfg/internal/artifact"
	"agentcfg/internal/diag"
	"agentcfg/internal/ir"
	"agentcfg/internal/target"
)

func init() { target.Register(Target{}) }

type Target struct{}

func (Target) ID() string                     { return "fast-agent" }
func (Target) MappedAuthTypes() []ir.AuthType { return []ir.AuthType{ir.AuthTypeNone} }

var wires = map[ir.Protocol]string{ir.ProtocolOpenAICompletions: "generic", ir.ProtocolOpenAIResponses: "openresponses", ir.ProtocolAnthropicMessages: "anthropic"}

// A hex-encoded qualified reference is reversible, collision-free, and safe as
// both a flat filename and a model token, including slash-bearing model IDs.
func overlayName(ref string) string { return "agentcfg-" + hex.EncodeToString([]byte(ref)) }

func (t Target) Validate(cfg ir.Config) []diag.Diagnostic {
	var ds []diag.Diagnostic
	warn := func(path, message string) { ds = append(ds, diag.TargetWarnf(t.ID(), path, "%s", message)) }
	for i, p := range cfg.Providers {
		path := fmt.Sprintf("providers[%d]", i)
		for key, v := range p.Headers {
			if strings.Contains(v.Value, "${") {
				warn(path+".headers."+key, "literal contains native environment syntax; header is skipped")
			}
		}
		if strings.Contains(p.APIKey.Value, "${") {
			warn(path+".api_key", "literal contains native environment syntax; credential is skipped")
		}
		for j, m := range p.Models {
			mp := fmt.Sprintf("%s.models[%d]", path, j)
			if len(m.Input) > 0 || len(m.Output) > 0 || m.ToolCalling != nil {
				warn(mp, "overlay metadata has no direct modality/tool capability mapping; these fields are skipped")
			}
			if m.Reasoning != nil || len(m.Variants) > 0 {
				warn(mp+".reasoning", "overlay defaults select a reasoning value, not a capability or variant list; reasoning metadata is skipped")
			}
		}
	}
	for i, s := range cfg.MCP {
		path := fmt.Sprintf("mcp[%d]", i)
		if s.Enabled != nil && !*s.Enabled {
			warn(path, "load_on_start is not a disable switch; disabled MCP server is skipped")
		}
		if s.TimeoutMS != nil && *s.TimeoutMS%1000 != 0 {
			warn(path+".timeout_ms", "fast-agent requires whole seconds; fractional-second timeout is skipped")
		}
		for _, values := range []map[string]ir.HeaderValue{s.Env, s.Headers} {
			for key, v := range values {
				if strings.Contains(v.Value, "${") {
					warn(path+"."+key, "literal contains native environment syntax; value is skipped")
				}
			}
		}
	}
	return ds
}

func (t Target) Emit(cfg ir.Config) ([]artifact.Artifact, error) {
	var arts []artifact.Artifact
	add := func(name, path string, doc any) error {
		content, err := yaml.Marshal(doc)
		if err != nil {
			return err
		}
		arts = append(arts, artifact.Artifact{Target: t.ID(), Name: name, Format: "yaml", SuggestedPath: path, Content: content})
		return nil
	}
	secrets := map[string]any{}
	for _, p := range cfg.Providers {
		wire, ok := wires[p.Protocol]
		if !ok {
			return nil, fmt.Errorf("invalid protocol %q", p.Protocol)
		}
		for _, m := range p.Models {
			name := overlayName(p.ID + "/" + m.ID)
			connection := map[string]any{"base_url": p.BaseURL}
			switch {
			case p.EffectiveAuthType() == ir.AuthTypeNone:
				connection["auth"] = "none"
			case p.APIKey.FromEnv != "":
				connection["auth"], connection["api_key_env"] = "env", p.APIKey.FromEnv
			case p.APIKey.Value != "" && !strings.Contains(p.APIKey.Value, "${"):
				connection["auth"], connection["secret_ref"] = "secret_ref", p.ID
				secrets[p.ID] = map[string]string{"api_key": p.APIKey.Value}
			}
			headers := refs(p.Headers)
			if len(headers) > 0 {
				connection["default_headers"] = headers
			}
			doc := map[string]any{"name": name, "provider": wire, "model": m.ID, "connection": connection}
			metadata := map[string]any{}
			if m.ContextWindow != nil {
				metadata["context_window"] = *m.ContextWindow
			}
			if m.MaxOutputTokens != nil {
				metadata["max_output_tokens"] = *m.MaxOutputTokens
				doc["defaults"] = map[string]any{"max_tokens": *m.MaxOutputTokens}
			}
			if len(metadata) > 0 {
				doc["metadata"] = metadata
			}
			label := m.Name
			if label == "" {
				label = p.ID + "/" + m.ID
			}
			doc["picker"] = map[string]string{"label": label}
			// Keep filenames stable and bounded even for long upstream IDs.
			file := fmt.Sprintf("model-overlays/agentcfg-%x.yaml", sha256.Sum256([]byte(name)))
			if err := add(file, ".fast-agent/"+file, doc); err != nil {
				return nil, err
			}
		}
	}
	if len(secrets) > 0 {
		if err := add("model-overlays.secrets.yaml", ".fast-agent/model-overlays.secrets.yaml", secrets); err != nil {
			return nil, err
		}
	}
	servers := map[string]any{}
	for _, s := range cfg.MCP {
		if s.Enabled != nil && !*s.Enabled {
			continue
		}
		entry := map[string]any{"transport": string(s.Transport)}
		if s.Transport == ir.TransportStdio {
			if len(s.Command) == 0 {
				return nil, fmt.Errorf("MCP %s has no command", s.ID)
			}
			entry["command"] = s.Command[0]
			if len(s.Command) > 1 {
				entry["args"] = s.Command[1:]
			}
			if s.CWD != "" {
				entry["cwd"] = s.CWD
			}
			if len(s.Env) > 0 {
				entry["env"] = refs(s.Env)
			}
		} else {
			entry["url"] = s.URL
			if len(s.Headers) > 0 {
				entry["headers"] = refs(s.Headers)
			}
		}
		if s.TimeoutMS != nil && *s.TimeoutMS%1000 == 0 {
			entry["read_timeout_seconds"] = *s.TimeoutMS / 1000
		}
		servers[s.ID] = entry
	}
	doc := map[string]any{}
	if len(servers) > 0 {
		doc["mcp"] = map[string]any{"servers": servers}
	}
	if cfg.Defaults != nil && cfg.Defaults.Model != "" {
		doc["default_model"] = overlayName(cfg.Defaults.Model)
	}
	if err := add("fastagent.config.yaml", "fastagent.config.yaml", doc); err != nil {
		return nil, err
	}
	return arts, nil
}

func refs(values map[string]ir.HeaderValue) map[string]string {
	out := map[string]string{}
	for key, v := range values {
		switch {
		case v.BearerFromEnv != "":
			out[key] = "Bearer ${" + v.BearerFromEnv + "}"
		case v.FromEnv != "":
			out[key] = "${" + v.FromEnv + "}"
		case !strings.Contains(v.Value, "${"):
			out[key] = v.Value
		}
	}
	return out
}
