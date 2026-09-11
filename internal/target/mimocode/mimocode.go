// Package mimocode emits a MiMo Code mimocode.json fragment.
package mimocode

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"agentcfg/internal/artifact"
	"agentcfg/internal/diag"
	"agentcfg/internal/ir"
	"agentcfg/internal/target"
)

func init() { target.Register(Target{}) }

// Target is the MiMo Code emitter.
type Target struct{}

func (Target) ID() string { return "mimocode" }

var npmByProtocol = map[ir.Protocol]string{
	ir.ProtocolOpenAICompletions: "@ai-sdk/openai-compatible",
	ir.ProtocolAnthropicMessages: "@ai-sdk/anthropic",
}

func (t Target) Validate(cfg ir.Config) []diag.Diagnostic {
	var diags []diag.Diagnostic
	for i, p := range cfg.Providers {
		if strings.Contains(p.APIKey.Value, "{env:") || strings.Contains(p.APIKey.Value, "{file:") {
			diags = append(diags, diag.TargetErrorf(t.ID(), fmt.Sprintf("providers[%d].api_key", i), "literal API key contains native expression syntax and cannot be represented literally"))
		}
	}
	for i, p := range cfg.Providers {
		path := fmt.Sprintf("providers[%d]", i)
		if p.Protocol == ir.ProtocolOpenAIResponses {
			diags = append(diags, diag.TargetErrorf(t.ID(), path+".protocol",
				"mimocode maps custom providers via AI SDK npm packages; openai-responses is not representable for custom providers"))
		} else if _, ok := npmByProtocol[p.Protocol]; !ok {
			diags = append(diags, diag.TargetErrorf(t.ID(), path+".protocol",
				"mimocode has no native mapping for protocol %q", p.Protocol))
		}
	}
	return diags
}

func (t Target) Emit(cfg ir.Config) ([]artifact.Artifact, error) {
	doc := mimoConfig{
		Provider: map[string]mimoProvider{},
		MCP:      map[string]any{},
	}
	for _, p := range cfg.Providers {
		opts := map[string]any{"baseURL": p.BaseURL}
		if p.APIKey.FromEnv != "" {
			opts["apiKey"] = "{env:" + p.APIKey.FromEnv + "}"
		} else if p.APIKey.Value != "" {
			opts["apiKey"] = p.APIKey.Value
		}
		if len(p.Headers) > 0 {
			headers := map[string]string{}
			for name, v := range p.Headers {
				headers[name] = envInterp(v)
			}
			opts["headers"] = headers
		}
		mp := mimoProvider{
			NPM:     npmByProtocol[p.Protocol],
			Name:    orDefault(p.Name, p.ID),
			Options: opts,
			Models:  map[string]mimoModel{},
		}
		for _, m := range p.Models {
			mm := mimoModel{Name: orDefault(m.Name, m.ID)}
			if m.ContextWindow != nil || m.MaxOutputTokens != nil {
				mm.Limit = map[string]int64{}
				if m.ContextWindow != nil {
					mm.Limit["context"] = *m.ContextWindow
				}
				if m.MaxOutputTokens != nil {
					mm.Limit["output"] = *m.MaxOutputTokens
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
			mm.Modalities = map[string][]ir.Modality{"input": in, "output": out}
			if m.Reasoning != nil && *m.Reasoning {
				mm.Reasoning = true
			}
			if m.ToolCalling != nil && *m.ToolCalling {
				mm.ToolCall = true
			}
			mp.Models[m.ID] = mm
		}
		doc.Provider[p.ID] = mp
	}
	for _, s := range cfg.MCP {
		enabled := true
		if s.Enabled != nil {
			enabled = *s.Enabled
		}
		if s.Transport == ir.TransportStdio {
			env := map[string]string{}
			for name, v := range s.Env {
				env[name] = envInterp(v)
			}
			local := map[string]any{
				"type":        "local",
				"command":     s.Command,
				"enabled":     enabled,
				"environment": env,
			}
			doc.MCP[s.ID] = local
		} else {
			headers := map[string]string{}
			for name, v := range s.Headers {
				headers[name] = envInterp(v)
			}
			remote := map[string]any{
				"type":    "remote",
				"url":     s.URL,
				"enabled": enabled,
				"headers": headers,
			}
			doc.MCP[s.ID] = remote
		}
	}
	if cfg.Defaults != nil && cfg.Defaults.Model != "" {
		doc.Model = cfg.Defaults.Model
	}
	if len(doc.MCP) == 0 {
		doc.MCP = nil
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return nil, fmt.Errorf("encoding mimocode config: %w", err)
	}
	return []artifact.Artifact{{
		Target:        t.ID(),
		Name:          "mimocode.json",
		Format:        "json",
		SuggestedPath: "~/.config/mimocode/mimocode.jsonc",
		Content:       buf.Bytes(),
	}}, nil
}

func envInterp(v ir.HeaderValue) string {
	if v.BearerFromEnv != "" {
		return "Bearer {env:" + v.BearerFromEnv + "}"
	}
	if v.FromEnv != "" {
		return "{env:" + v.FromEnv + "}"
	}
	return v.Value
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

type mimoConfig struct {
	Provider map[string]mimoProvider `json:"provider"`
	MCP      map[string]any          `json:"mcp,omitempty"`
	Model    string                  `json:"model,omitempty"`
}

type mimoProvider struct {
	NPM     string               `json:"npm"`
	Name    string               `json:"name"`
	Options map[string]any       `json:"options"`
	Models  map[string]mimoModel `json:"models"`
}

type mimoModel struct {
	Name       string                   `json:"name"`
	Limit      map[string]int64         `json:"limit,omitempty"`
	Modalities map[string][]ir.Modality `json:"modalities"`
	Reasoning  bool                     `json:"reasoning,omitempty"`
	ToolCall   bool                     `json:"tool_call,omitempty"`
}
