// Package opencode emits an OpenCode opencode.json fragment.
package opencode

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

// Target is the OpenCode emitter.
type Target struct{}

func (Target) ID() string { return "opencode" }

func (t Target) Validate(cfg ir.Config) []diag.Diagnostic {
	var diags []diag.Diagnostic
	for i, p := range cfg.Providers {
		if strings.Contains(p.APIKey.Value, "{env:") || strings.Contains(p.APIKey.Value, "{file:") {
			diags = append(diags, diag.TargetErrorf(t.ID(), fmt.Sprintf("providers[%d].api_key", i), "literal API key contains native expression syntax and cannot be represented literally"))
		}
	}
	for i, p := range cfg.Providers {
		if p.Protocol != ir.ProtocolOpenAICompletions {
			diags = append(diags, diag.TargetErrorf(t.ID(), fmt.Sprintf("providers[%d].protocol", i),
				"opencode v1 maps openai-completions providers only; %q needs a verified adapter package and is rejected", p.Protocol))
		}
	}
	return diags
}

func (t Target) Emit(cfg ir.Config) ([]artifact.Artifact, error) {
	doc := opencodeConfig{
		Schema:   "https://opencode.ai/config.json",
		Provider: map[string]opencodeProvider{},
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
				headers[name] = headerRef(v)
			}
			opts["headers"] = headers
		}
		op := opencodeProvider{NPM: "@ai-sdk/openai-compatible", Name: orDefault(p.Name, p.ID), Options: opts}
		op.Models = map[string]opencodeModel{}
		for _, m := range p.Models {
			om := opencodeModel{Name: orDefault(m.Name, m.ID)}
			if m.ContextWindow != nil || m.MaxOutputTokens != nil {
				om.Limit = map[string]int64{}
				if m.ContextWindow != nil {
					om.Limit["context"] = *m.ContextWindow
				}
				if m.MaxOutputTokens != nil {
					om.Limit["output"] = *m.MaxOutputTokens
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
			om.Modalities = map[string][]ir.Modality{"input": in, "output": out}
			if m.Reasoning != nil && *m.Reasoning {
				om.Reasoning = true
			}
			if m.ToolCalling != nil && *m.ToolCalling {
				om.Tools = true
			}
			op.Models[m.ID] = om
		}
		doc.Provider[p.ID] = op
	}
	for _, s := range cfg.MCP {
		enabled := true
		if s.Enabled != nil {
			enabled = *s.Enabled
		}
		switch s.Transport {
		case ir.TransportStdio:
			env := map[string]string{}
			for name, v := range s.Env {
				env[name] = envRef(v)
			}
			local := map[string]any{
				"type":        "local",
				"command":     s.Command,
				"enabled":     enabled,
				"environment": env,
			}
			if s.CWD != "" {
				local["cwd"] = s.CWD
			}
			doc.MCP[s.ID] = local
		case ir.TransportHTTP:
			headers := map[string]string{}
			for name, v := range s.Headers {
				headers[name] = headerRef(v)
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

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return nil, fmt.Errorf("encoding opencode config: %w", err)
	}
	return []artifact.Artifact{{
		Target:        t.ID(),
		Name:          "opencode.json",
		Format:        "json",
		SuggestedPath: "opencode.json",
		Content:       buf.Bytes(),
	}}, nil
}

func headerRef(v ir.HeaderValue) string {
	if v.BearerFromEnv != "" {
		return "Bearer {env:" + v.BearerFromEnv + "}"
	}
	if v.FromEnv != "" {
		return "{env:" + v.FromEnv + "}"
	}
	return v.Value
}

func envRef(v ir.HeaderValue) string {
	if v.FromEnv != "" {
		return "{env:" + v.FromEnv + "}"
	}
	return v.Value
}

type opencodeConfig struct {
	Schema   string                      `json:"$schema"`
	Provider map[string]opencodeProvider `json:"provider"`
	MCP      map[string]any              `json:"mcp,omitempty"`
	Model    string                      `json:"model,omitempty"`
}

type opencodeProvider struct {
	NPM     string                   `json:"npm"`
	Name    string                   `json:"name"`
	Options map[string]any           `json:"options"`
	Models  map[string]opencodeModel `json:"models"`
}

type opencodeModel struct {
	Name       string                   `json:"name"`
	Limit      map[string]int64         `json:"limit,omitempty"`
	Modalities map[string][]ir.Modality `json:"modalities"`
	Reasoning  bool                     `json:"reasoning,omitempty"`
	Tools      bool                     `json:"tools,omitempty"`
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
