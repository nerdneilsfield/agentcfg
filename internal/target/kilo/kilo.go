// Package kilo emits Kilo CLI's OpenCode-derived configuration format.
package kilo

import (
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
func (Target) ID() string { return "kilo" }
func (t Target) Validate(cfg ir.Config) []diag.Diagnostic {
	var ds []diag.Diagnostic
	warn := func(p, m string) { ds = append(ds, diag.TargetWarnf(t.ID(), p, "%s", m)) }
	for i, p := range cfg.Providers {
		path := fmt.Sprintf("providers[%d]", i)
		if native(p.APIKey.Value) {
			warn(path+".api_key", "literal contains native expression syntax; credential is skipped")
		}
		for k, v := range p.Headers {
			if native(v.Value) {
				warn(path+".headers."+k, "literal contains native expression syntax; header is skipped")
			}
		}
		for j, m := range p.Models {
			if (m.ContextWindow == nil) != (m.MaxOutputTokens == nil) {
				warn(fmt.Sprintf("%s.models[%d]", path, j), "Kilo limit requires both context and output; incomplete limit is skipped")
			}
			if len(m.Variants) > 0 {
				warn(fmt.Sprintf("%s.models[%d].variants", path, j), "reasoning variants require provider-specific request settings; variant list is skipped")
			}
		}
	}
	for i, s := range cfg.MCP {
		path := fmt.Sprintf("mcp[%d]", i)
		if s.CWD != "" {
			warn(path+".cwd", "Kilo MCP schema has no cwd; field is skipped")
		}
		for _, vs := range []map[string]ir.HeaderValue{s.Env, s.Headers} {
			for k, v := range vs {
				if native(v.Value) {
					warn(path+"."+k, "literal contains native expression syntax; value is skipped")
				}
			}
		}
	}
	return ds
}

func (t Target) Emit(cfg ir.Config) ([]artifact.Artifact, error) {
	providers := map[string]any{}
	for _, p := range cfg.Providers {
		npm := "@ai-sdk/openai-compatible"
		switch p.Protocol {
		case ir.ProtocolOpenAIResponses:
			npm = "@ai-sdk/openai"
		case ir.ProtocolAnthropicMessages:
			npm = "@ai-sdk/anthropic"
		}
		opts := map[string]any{"baseURL": p.BaseURL}
		if p.EffectiveAuthType() != ir.AuthTypeNone && !native(p.APIKey.Value) {
			if key := ref(p.APIKey); key != "" {
				opts["apiKey"] = key
			}
		}
		if h := refs(p.Headers); len(h) > 0 {
			opts["headers"] = h
		}
		models := map[string]any{}
		for _, m := range p.Models {
			e := map[string]any{}
			if m.Name != "" {
				e["name"] = m.Name
			}
			if m.ContextWindow != nil && m.MaxOutputTokens != nil {
				e["limit"] = map[string]int64{"context": *m.ContextWindow, "output": *m.MaxOutputTokens}
			}
			if m.ToolCalling != nil {
				e["tool_call"] = *m.ToolCalling
			}
			if m.Reasoning != nil {
				e["reasoning"] = *m.Reasoning
			}
			if len(m.Input) > 0 || len(m.Output) > 0 {
				e["modalities"] = map[string]any{"input": mods(m.Input), "output": mods(m.Output)}
			}
			models[m.ID] = e
		}
		e := map[string]any{"npm": npm, "options": opts, "models": models}
		if p.Name != "" {
			e["name"] = p.Name
		}
		providers[p.ID] = e
	}
	servers := map[string]any{}
	for _, s := range cfg.MCP {
		e := map[string]any{}
		if s.Transport == ir.TransportStdio {
			e["type"] = "local"
			e["command"] = s.Command
			if len(s.Env) > 0 {
				e["environment"] = refs(s.Env)
			}
		} else {
			e["type"] = "remote"
			e["url"] = s.URL
			if len(s.Headers) > 0 {
				e["headers"] = refs(s.Headers)
			}
		}
		if s.Enabled != nil {
			e["enabled"] = *s.Enabled
		}
		if s.TimeoutMS != nil {
			e["timeout"] = *s.TimeoutMS
		}
		servers[s.ID] = e
	}
	doc := map[string]any{"$schema": "https://kilo.ai/config.json", "provider": providers}
	if len(servers) > 0 {
		doc["mcp"] = servers
	}
	if cfg.Defaults != nil && cfg.Defaults.Model != "" {
		doc["model"] = cfg.Defaults.Model
	}
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	return []artifact.Artifact{{Target: t.ID(), Name: "opencode.json", Format: "json", SuggestedPath: "~/.config/kilo/opencode.json", Content: append(b, '\n')}}, nil
}
func native(s string) bool { return strings.Contains(s, "{env:") || strings.Contains(s, "{file:") }
func ref(v ir.HeaderValue) string {
	if v.BearerFromEnv != "" {
		return "Bearer {env:" + v.BearerFromEnv + "}"
	}
	if v.FromEnv != "" {
		return "{env:" + v.FromEnv + "}"
	}
	return v.Value
}

func refs(vs map[string]ir.HeaderValue) map[string]string {
	out := map[string]string{}
	for k, v := range vs {
		if !native(v.Value) {
			out[k] = ref(v)
		}
	}
	return out
}

func mods(ms []ir.Modality) []string {
	out := []string{}
	for _, m := range ms {
		out = append(out, string(m))
	}
	return out
}
