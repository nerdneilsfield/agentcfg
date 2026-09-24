// Package crow emits crow-cli config.yaml fragments.
package crow

import (
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

func (Target) ID() string { return "crow" }

func (t Target) Validate(cfg ir.Config) []diag.Diagnostic {
	var ds []diag.Diagnostic
	warn := func(path, message string) { ds = append(ds, diag.TargetWarnf(t.ID(), path, "%s", message)) }
	for i, p := range cfg.Providers {
		path := fmt.Sprintf("providers[%d]", i)
		if p.Protocol != ir.ProtocolOpenAICompletions {
			warn(path+".protocol", "crow supports only Chat Completions; provider and its models are skipped")
			if cfg.Defaults != nil && strings.HasPrefix(cfg.Defaults.Model, p.ID+"/") {
				warn("defaults.model", "default model belongs to a skipped provider; crow uses the first remaining model")
			}
			continue
		}
		if len(p.Headers) > 0 {
			warn(path+".headers", "crow has no provider headers field; headers are skipped")
		}
		if strings.Contains(p.APIKey.Value, "${") {
			warn(path+".api_key", "literal contains native environment syntax; credential is skipped")
		}
		for j, m := range p.Models {
			mp := fmt.Sprintf("%s.models[%d]", path, j)
			if m.ContextWindow != nil || m.MaxOutputTokens != nil {
				warn(mp, "crow has no per-model context/output limit fields; limits are skipped")
			}
			if m.Reasoning != nil || len(m.Variants) > 0 {
				warn(mp+".reasoning", "crow selects one reasoning effort, not a capability or variant list; reasoning metadata is skipped")
			}
			if m.ToolCalling != nil || len(m.Output) > 0 {
				warn(mp, "crow has no tool capability or output modality fields; these fields are skipped")
			}
			for _, mod := range m.Input {
				if mod == ir.ModalityPDF {
					warn(mp+".input", "crow has no PDF modality; PDF is skipped")
				}
			}
		}
	}
	for i, s := range cfg.MCP {
		path := fmt.Sprintf("mcp[%d]", i)
		if s.Enabled != nil && !*s.Enabled {
			warn(path, "crow has no disabled MCP entry; server is skipped")
		}
		if s.CWD != "" {
			warn(path+".cwd", "crow uses the session working directory; cwd is skipped")
		}
		if s.TimeoutMS != nil {
			warn(path+".timeout_ms", "crow has no MCP timeout field; timeout is skipped")
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
	providers := map[string]any{}
	models := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	var first, rest []*yaml.Node
	for _, p := range cfg.Providers {
		if p.Protocol != ir.ProtocolOpenAICompletions {
			continue
		}
		entry := map[string]any{"base_url": p.BaseURL}
		if p.EffectiveAuthType() != ir.AuthTypeNone && !strings.Contains(p.APIKey.Value, "${") {
			if key := ref(p.APIKey); key != "" {
				entry["api_key"] = key
			}
		}
		providers[p.ID] = entry
		for _, m := range p.Models {
			name := p.ID + "/" + m.ID
			model := map[string]any{"provider": p.ID, "model": m.ID}
			var mods []string
			for _, mod := range m.Input {
				if mod != ir.ModalityPDF {
					mods = append(mods, string(mod))
				}
			}
			if len(mods) > 0 {
				model["modality"] = mods
			}
			var value yaml.Node
			if err := value.Encode(model); err != nil {
				return nil, err
			}
			pair := []*yaml.Node{{Kind: yaml.ScalarNode, Tag: "!!str", Value: name}, &value}
			if cfg.Defaults != nil && cfg.Defaults.Model == name {
				first = pair
			} else {
				rest = append(rest, pair...)
			}
		}
	}
	models.Content = append(first, rest...)
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
			if len(s.Env) > 0 {
				entry["env"] = refs(s.Env)
			}
		} else {
			entry["url"] = s.URL
			if len(s.Headers) > 0 {
				entry["headers"] = refs(s.Headers)
			}
		}
		servers[s.ID] = entry
	}
	doc := map[string]any{"providers": providers, "models": models}
	if len(servers) > 0 {
		doc["mcpServers"] = servers
	}
	content, err := yaml.Marshal(doc)
	if err != nil {
		return nil, err
	}
	return []artifact.Artifact{{Target: t.ID(), Name: "config.yaml", Format: "yaml", SuggestedPath: "~/.agents/crow/config.yaml", Content: content}}, nil
}

func ref(v ir.HeaderValue) string {
	if v.BearerFromEnv != "" {
		return "Bearer ${" + v.BearerFromEnv + "}"
	}
	if v.FromEnv != "" {
		return "${" + v.FromEnv + "}"
	}
	return v.Value
}

func refs(values map[string]ir.HeaderValue) map[string]string {
	out := map[string]string{}
	for k, v := range values {
		if !strings.Contains(v.Value, "${") {
			out[k] = ref(v)
		}
	}
	return out
}
