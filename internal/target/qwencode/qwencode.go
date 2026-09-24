// Package qwencode emits Qwen Code modelProviders and MCP configuration.
package qwencode

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"agentcfg/internal/artifact"
	"agentcfg/internal/diag"
	"agentcfg/internal/ir"
	"agentcfg/internal/target"
)

type Target struct{}

func init()               { target.Register(Target{}) }
func (Target) ID() string { return "qwen-code" }
func protocol(p ir.Provider) string {
	if p.Protocol == ir.ProtocolAnthropicMessages {
		return "anthropic"
	}
	return "openai"
}

func (t Target) Validate(cfg ir.Config) []diag.Diagnostic {
	var ds []diag.Diagnostic
	warn := func(p, m string) { ds = append(ds, diag.TargetWarnf(t.ID(), p, "%s", m)) }
	seen := map[string]bool{}
	for i, p := range cfg.Providers {
		path := fmt.Sprintf("providers[%d]", i)
		if strings.Contains(p.APIKey.Value, "$") {
			warn(path+".api_key", "literal contains environment substitution syntax; credential is skipped")
		}
		for k, v := range p.Headers {
			if strings.Contains(v.Value, "$") {
				warn(path+".headers."+k, "literal contains environment substitution syntax; header is skipped")
			}
		}
		for j, m := range p.Models {
			mp := fmt.Sprintf("%s.models[%d]", path, j)
			key := string(p.Protocol) + "\x00" + p.BaseURL + "\x00" + m.ID
			if seen[key] {
				warn(mp, "Qwen identifies routes by protocol, model and URL; duplicate route is skipped")
				if cfg.Defaults != nil && cfg.Defaults.Model == p.ID+"/"+m.ID {
					warn("defaults.model", "default refers to a skipped duplicate route; default selection is omitted")
				}
			}
			seen[key] = true
			if m.Reasoning != nil || len(m.Variants) > 0 || m.ToolCalling != nil {
				warn(mp, "reasoning/tool capabilities and variant lists have no direct generationConfig mapping; skipped")
			}
			if len(m.Output) > 0 {
				warn(mp+".output", "Qwen has no output modality declaration; skipped")
			}
			for _, mod := range m.Input {
				if mod == ir.ModalityPDF {
					warn(mp+".input", "Qwen modalities have no PDF flag; PDF is skipped")
				}
			}
		}
	}
	for i, s := range cfg.MCP {
		for _, vs := range []map[string]ir.HeaderValue{s.Env, s.Headers} {
			for k, v := range vs {
				if strings.Contains(v.Value, "$") {
					warn(fmt.Sprintf("mcp[%d].%s", i, k), "literal contains environment substitution syntax; value is skipped")
				}
			}
		}
	}
	return ds
}

func (t Target) Emit(cfg ir.Config) ([]artifact.Artifact, error) {
	providers := map[string]any{}
	protocols := map[string]string{}
	env := map[string]string{}
	doc := map[string]any{}
	seen := map[string]bool{}
	for _, p := range cfg.Providers {
		entries := []map[string]any{}
		for _, m := range p.Models {
			key := string(p.Protocol) + "\x00" + p.BaseURL + "\x00" + m.ID
			if seen[key] {
				continue
			}
			seen[key] = true
			e := map[string]any{"id": m.ID, "baseUrl": p.BaseURL}
			if m.Name != "" {
				e["name"] = m.Name
			}
			if p.Protocol != ir.ProtocolAnthropicMessages {
				wire := "chat-completions"
				if p.Protocol == ir.ProtocolOpenAIResponses {
					wire = "responses"
				}
				e["wireApi"] = wire
			}
			if p.EffectiveAuthType() != ir.AuthTypeNone {
				if p.APIKey.FromEnv != "" {
					e["envKey"] = p.APIKey.FromEnv
				} else if p.APIKey.Value != "" && !strings.Contains(p.APIKey.Value, "$") {
					name := "AGENTCFG_KEY_" + strings.ToUpper(hex.EncodeToString([]byte(p.ID)))
					env[name] = p.APIKey.Value
					e["envKey"] = name
				}
			}
			gen := map[string]any{}
			if m.ContextWindow != nil {
				gen["contextWindowSize"] = *m.ContextWindow
			}
			if m.MaxOutputTokens != nil {
				gen["samplingParams"] = map[string]any{"max_tokens": *m.MaxOutputTokens}
			}
			if h := refs(p.Headers); len(h) > 0 {
				gen["customHeaders"] = h
			}
			if len(m.Input) > 0 {
				modalities := map[string]bool{"image": false, "audio": false, "video": false}
				for _, mod := range m.Input {
					if mod == ir.ModalityImage || mod == ir.ModalityAudio || mod == ir.ModalityVideo {
						modalities[string(mod)] = true
					}
				}
				gen["modalities"] = modalities
			}
			if len(gen) > 0 {
				e["generationConfig"] = gen
			}
			entries = append(entries, e)
			if cfg.Defaults != nil && cfg.Defaults.Model == p.ID+"/"+m.ID {
				auth := protocol(p)
				if p.Protocol == ir.ProtocolOpenAIResponses {
					auth = "openai-responses"
				}
				doc["model"] = map[string]string{"name": m.ID}
				doc["security"] = map[string]any{"auth": map[string]string{"selectedType": auth, "baseUrl": p.BaseURL}}
			}
		}
		if len(entries) > 0 {
			providers[p.ID] = entries
			protocols[p.ID] = protocol(p)
		}
	}
	doc["modelProviders"] = providers
	doc["providerProtocol"] = protocols
	if len(env) > 0 {
		doc["env"] = env
	}
	servers := map[string]any{}
	excluded := []string{}
	for _, s := range cfg.MCP {
		e := map[string]any{}
		if s.Transport == ir.TransportStdio {
			if len(s.Command) == 0 {
				return nil, fmt.Errorf("MCP %s has no command", s.ID)
			}
			e["command"] = s.Command[0]
			if len(s.Command) > 1 {
				e["args"] = s.Command[1:]
			}
			if len(s.Env) > 0 {
				e["env"] = refs(s.Env)
			}
			if s.CWD != "" {
				e["cwd"] = s.CWD
			}
		} else {
			e["httpUrl"] = s.URL
			if len(s.Headers) > 0 {
				e["headers"] = refs(s.Headers)
			}
		}
		if s.TimeoutMS != nil {
			e["timeout"] = *s.TimeoutMS
		}
		if s.Enabled != nil && !*s.Enabled {
			excluded = append(excluded, s.ID)
		}
		servers[s.ID] = e
	}
	if len(servers) > 0 {
		doc["mcpServers"] = servers
	}
	if len(excluded) > 0 {
		doc["mcp"] = map[string]any{"excluded": excluded}
	}
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	return []artifact.Artifact{{Target: t.ID(), Name: "settings.json", Format: "json", SuggestedPath: "~/.qwen/settings.json", Content: append(b, '\n')}}, nil
}

func refs(vs map[string]ir.HeaderValue) map[string]string {
	out := map[string]string{}
	for k, v := range vs {
		if v.BearerFromEnv != "" {
			out[k] = "Bearer ${" + v.BearerFromEnv + "}"
		} else if v.FromEnv != "" {
			out[k] = "${" + v.FromEnv + "}"
		} else if !strings.Contains(v.Value, "$") {
			out[k] = v.Value
		}
	}
	return out
}
