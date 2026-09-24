// Package zcode emits a ZCode config.json fragment.
package zcode

import (
	"bytes"
	"encoding/json"
	"fmt"

	"agentcfg/internal/artifact"
	"agentcfg/internal/diag"
	"agentcfg/internal/ir"
	"agentcfg/internal/target"
)

func init() { target.Register(Target{}) }

// Target is the ZCode emitter.
type Target struct{}

func (Target) ID() string { return "zcode" }

var kindByProtocol = map[ir.Protocol]string{
	ir.ProtocolOpenAICompletions: "openai-compatible",
	ir.ProtocolAnthropicMessages: "anthropic",
}

func (t Target) Validate(cfg ir.Config) []diag.Diagnostic {
	var diags []diag.Diagnostic
	for i, p := range cfg.Providers {
		path := fmt.Sprintf("providers[%d]", i)
		if p.Protocol == ir.ProtocolOpenAIResponses {
			diags = append(diags, diag.TargetWarnf(t.ID(), path+".protocol",
				"zcode user config exposes provider kind only; openai-responses has no native location for custom providers"))
		} else if _, ok := kindByProtocol[p.Protocol]; !ok {
			diags = append(diags, diag.TargetWarnf(t.ID(), path+".protocol",
				"zcode provider kind must be anthropic, openai, or openai-compatible; got %q", p.Protocol))
		}
		if p.APIKey.FromEnv != "" {
			diags = append(diags, diag.TargetWarnf(t.ID(), path+".api_key",
				"zcode stores inline apiKey strings only with no env interpolation; api_key environment references are not representable"))
		}
		for name, v := range p.Headers {
			if v.FromEnv != "" || v.BearerFromEnv != "" {
				diags = append(diags, diag.TargetWarnf(t.ID(), path+".headers."+name,
					"zcode headers are literal strings; only constant header values are representable"))
			}
		}
	}
	for i, s := range cfg.MCP {
		path := fmt.Sprintf("mcp[%d]", i)
		for name, v := range s.Env {
			if v.FromEnv != "" || v.BearerFromEnv != "" {
				diags = append(diags, diag.TargetWarnf(t.ID(), path+".env."+name,
					"zcode mcp.servers env values are literal strings; environment references are not representable"))
			}
		}
		for name, v := range s.Headers {
			if v.FromEnv != "" || v.BearerFromEnv != "" {
				diags = append(diags, diag.TargetWarnf(t.ID(), path+".headers."+name,
					"zcode mcp.servers headers are literal strings; environment references are not representable"))
			}
		}
	}
	if cfg.Defaults != nil && cfg.Defaults.Model != "" {
		pid, mid, ok := splitRef(cfg.Defaults.Model)
		if !ok {
			diags = append(diags, diag.TargetWarnf(t.ID(), "defaults.model",
				`zcode expects defaults.model as "provider/model"; got %q`, cfg.Defaults.Model))
		} else if !emittedModel(cfg, pid, mid) {
			diags = append(diags, diag.TargetWarnf(t.ID(), "defaults.model",
				"zcode default %q does not resolve to an emitted provider model", cfg.Defaults.Model))
		}
	}
	return diags
}

func emittedModel(cfg ir.Config, pid, mid string) bool {
	for _, p := range cfg.Providers {
		if p.ID != pid {
			continue
		}
		if _, ok := kindByProtocol[p.Protocol]; !ok {
			continue // the provider is skipped, so its models are not emitted
		}
		for _, m := range p.Models {
			if m.ID == mid {
				return true
			}
		}
	}
	return false
}

func splitRef(providerModel string) (string, string, bool) {
	for i := 0; i < len(providerModel); i++ {
		if providerModel[i] == '/' {
			return providerModel[:i], providerModel[i+1:], true
		}
	}
	return "", "", false
}

func (t Target) Emit(cfg ir.Config) ([]artifact.Artifact, error) {
	doc := zcodeConfig{
		Provider: map[string]zcodeProvider{},
	}
	servers := map[string]any{}
	emittedProviders := map[string]bool{}
	for _, p := range cfg.Providers {
		kind, ok := kindByProtocol[p.Protocol]
		if !ok {
			continue // openai-responses has no custom-provider kind
		}
		emittedProviders[p.ID] = true
		opts := map[string]any{"baseURL": p.BaseURL}
		if p.APIKey.Value != "" {
			opts["apiKey"] = p.APIKey.Value
		}
		if len(p.Headers) > 0 {
			headers := map[string]string{}
			for name, v := range p.Headers {
				if v.FromEnv != "" || v.BearerFromEnv != "" {
					continue // zcode headers are literal-only
				}
				headers[name] = v.Value
			}
			if len(headers) > 0 {
				opts["headers"] = headers
			}
		}
		zp := zcodeProvider{Kind: kind, Name: p.Name, Options: opts, Models: map[string]zcodeModel{}}
		for _, m := range p.Models {
			zm := zcodeModel{}
			if m.Name != "" {
				zm.Name = m.Name
			}
			if m.ContextWindow != nil || m.MaxOutputTokens != nil {
				zm.Limit = map[string]int64{}
				if m.ContextWindow != nil {
					zm.Limit["context"] = *m.ContextWindow
				}
				if m.MaxOutputTokens != nil {
					zm.Limit["output"] = *m.MaxOutputTokens
				}
			}
			in := m.Input
			if len(in) == 0 {
				in = []ir.Modality{ir.ModalityText}
			}
			out := m.Output
			if len(out) == 0 {
				out = []ir.Modality{ir.ModalityText}
			}
			zm.Modalities = map[string][]ir.Modality{"input": in, "output": out}
			if len(m.Variants) > 0 {
				zm.Reasoning = map[string]any{"enabled": true, "levels": effortStrings(m.Variants)}
			} else if m.Reasoning != nil {
				zm.Reasoning = *m.Reasoning
			}
			if m.ToolCalling != nil {
				zm.ToolCall = m.ToolCalling
			}
			zp.Models[m.ID] = zm
		}
		doc.Provider[p.ID] = zp
	}
	for _, s := range cfg.MCP {
		enabled := true
		if s.Enabled != nil {
			enabled = *s.Enabled
		}
		if s.Transport == ir.TransportStdio {
			env := map[string]string{}
			for name, v := range s.Env {
				if v.FromEnv != "" || v.BearerFromEnv != "" {
					continue // zcode MCP env values are literal-only
				}
				env[name] = v.Value
			}
			entry := map[string]any{
				"type":    "stdio",
				"command": s.Command[0],
				"enabled": enabled,
			}
			if len(s.Command) > 1 {
				entry["args"] = s.Command[1:]
			}
			if len(env) > 0 {
				entry["env"] = env
			}
			if s.CWD != "" {
				entry["cwd"] = s.CWD
			}
			if s.TimeoutMS != nil {
				entry["timeoutMs"] = *s.TimeoutMS
			}
			servers[s.ID] = entry
		} else {
			headers := map[string]string{}
			for name, v := range s.Headers {
				if v.FromEnv != "" || v.BearerFromEnv != "" {
					continue // zcode MCP header values are literal-only
				}
				headers[name] = v.Value
			}
			entry := map[string]any{
				"type":    "http",
				"url":     s.URL,
				"enabled": enabled,
			}
			if len(headers) > 0 {
				entry["headers"] = headers
			}
			if s.TimeoutMS != nil {
				entry["timeoutMs"] = *s.TimeoutMS
			}
			servers[s.ID] = entry
		}
	}
	if cfg.Defaults != nil && cfg.Defaults.Model != "" {
		if pid, _, ok := splitRef(cfg.Defaults.Model); ok && emittedProviders[pid] {
			doc.Model = &zcodeModelSel{Main: cfg.Defaults.Model}
		}
	}
	if len(servers) > 0 {
		doc.MCP = &zcodeMCP{Servers: servers}
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return nil, fmt.Errorf("encoding zcode config: %w", err)
	}
	return []artifact.Artifact{{
		Target:        t.ID(),
		Name:          "config.json",
		Format:        "json",
		SuggestedPath: "~/.zcode/cli/config.json",
		Content:       buf.Bytes(),
	}}, nil
}

type zcodeConfig struct {
	Provider map[string]zcodeProvider `json:"provider"`
	Model    *zcodeModelSel           `json:"model,omitempty"`
	MCP      *zcodeMCP                `json:"mcp,omitempty"`
}

type zcodeModelSel struct {
	Main string `json:"main"`
}

type zcodeMCP struct {
	Servers map[string]any `json:"servers"`
}

type zcodeProvider struct {
	Kind    string                `json:"kind"`
	Name    string                `json:"name,omitempty"`
	Options map[string]any        `json:"options"`
	Models  map[string]zcodeModel `json:"models"`
}

type zcodeModel struct {
	Name       string                   `json:"name,omitempty"`
	Limit      map[string]int64         `json:"limit,omitempty"`
	Modalities map[string][]ir.Modality `json:"modalities"`
	Reasoning  any                      `json:"reasoning,omitempty"`
	ToolCall   *bool                    `json:"tool_call,omitempty"`
}

func effortStrings(efforts []ir.ReasoningEffort) []string {
	out := make([]string, len(efforts))
	for i, effort := range efforts {
		out[i] = string(effort)
	}
	return out
}
