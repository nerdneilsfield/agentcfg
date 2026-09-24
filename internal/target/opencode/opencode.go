// Package opencode emits an OpenCode opencode.json fragment in the native V2
// configuration shape.
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

// Target is the OpenCode V2 emitter.
type Target struct{}

func (Target) ID() string { return "opencode" }

// opencodePackages maps an IR protocol to the native V2 provider runtime
// package. The -compatible spelling is used where the pinned bundle carries it;
// for the other two wires the shipped bundle loads only the native package, and
// both honor settings.baseURL for a custom endpoint.
var opencodePackages = map[ir.Protocol]string{
	ir.ProtocolOpenAICompletions: "@opencode/ai/providers/openai-compatible",
	ir.ProtocolOpenAIResponses:   "@opencode/ai/providers/openai",
	ir.ProtocolAnthropicMessages: "@opencode/ai/providers/anthropic",
}

// MappedAuthTypes reports the auth_type values OpenCode maps. "none" writes no
// credential reference; OpenCode has no bearer override field.
func (t Target) MappedAuthTypes() []ir.AuthType { return []ir.AuthType{ir.AuthTypeNone} }

func (t Target) Validate(cfg ir.Config) []diag.Diagnostic {
	var diags []diag.Diagnostic
	for i, p := range cfg.Providers {
		path := fmt.Sprintf("providers[%d]", i)
		if strings.Contains(p.APIKey.Value, "{env:") || strings.Contains(p.APIKey.Value, "{file:") {
			diags = append(diags, diag.TargetWarnf(t.ID(), path+".api_key", "literal API key contains native expression syntax and cannot be represented literally; the credential is skipped"))
		}
		for j, m := range p.Models {
			if m.Reasoning != nil {
				diags = append(diags, diag.TargetWarnf(t.ID(), fmt.Sprintf("%s.models[%d].reasoning", path, j),
					"V2 has no model reasoning flag; reasoning is expressed through variants, so this flag is skipped"))
			}
		}
	}
	return diags
}

func (t Target) Emit(cfg ir.Config) ([]artifact.Artifact, error) {
	providers := map[string]opencodeProvider{}
	for _, p := range cfg.Providers {
		op := opencodeProvider{
			Name:    p.Name,
			Package: opencodePackages[p.Protocol],
			Models:  map[string]opencodeModel{},
		}
		settings := map[string]any{"baseURL": p.BaseURL}
		if p.EffectiveAuthType() != ir.AuthTypeNone {
			switch {
			case p.APIKey.FromEnv != "":
				op.Env = []string{p.APIKey.FromEnv}
			case p.APIKey.Value != "" && !strings.Contains(p.APIKey.Value, "{env:") && !strings.Contains(p.APIKey.Value, "{file:"):
				settings["apiKey"] = p.APIKey.Value
			}
		}
		op.Settings = settings
		if len(p.Headers) > 0 {
			headers := map[string]string{}
			for name, v := range p.Headers {
				headers[name] = headerRef(v)
			}
			op.Headers = headers
		}
		for _, m := range p.Models {
			om := opencodeModel{Name: m.Name}
			caps := opencodeCapabilities{}
			hasCaps := false
			if m.ToolCalling != nil {
				caps.Tools = m.ToolCalling
				hasCaps = true
			}
			if len(m.Input) > 0 {
				caps.Input = modalityNames(m.Input)
				hasCaps = true
			}
			if len(m.Output) > 0 {
				caps.Output = modalityNames(m.Output)
				hasCaps = true
			}
			if hasCaps {
				om.Capabilities = &caps
			}
			if m.ContextWindow != nil || m.MaxOutputTokens != nil {
				om.Limit = map[string]int64{}
				if m.ContextWindow != nil {
					om.Limit["context"] = *m.ContextWindow
				}
				if m.MaxOutputTokens != nil {
					om.Limit["output"] = *m.MaxOutputTokens
				}
			}
			for _, effort := range m.Variants {
				om.Variants = append(om.Variants, opencodeVariant{
					ID:       string(effort),
					Settings: map[string]any{"reasoningEffort": string(effort)},
				})
			}
			op.Models[m.ID] = om
		}
		providers[p.ID] = op
	}

	doc := opencodeConfig{
		Schema:    "https://opencode.ai/config.json",
		Providers: providers,
	}
	for _, s := range cfg.MCP {
		if doc.MCP == nil {
			doc.MCP = &opencodeMCP{Servers: map[string]opencodeMCPServer{}}
		}
		entry := opencodeMCPServer{}
		if s.Transport == ir.TransportStdio {
			entry.Type = "local"
			entry.Command = s.Command
			entry.CWD = s.CWD
			if len(s.Env) > 0 {
				env := map[string]string{}
				for name, v := range s.Env {
					env[name] = envRef(v)
				}
				entry.Environment = env
			}
		} else {
			entry.Type = "remote"
			entry.URL = s.URL
			if len(s.Headers) > 0 {
				headers := map[string]string{}
				for name, v := range s.Headers {
					headers[name] = headerRef(v)
				}
				entry.Headers = headers
			}
		}
		if s.Enabled != nil && !*s.Enabled {
			disabled := true
			entry.Disabled = &disabled
		}
		if s.TimeoutMS != nil {
			entry.Timeout = &opencodeTimeout{Catalog: s.TimeoutMS, Execution: s.TimeoutMS}
		}
		doc.MCP.Servers[s.ID] = entry
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

func modalityNames(mods []ir.Modality) []string {
	out := make([]string, 0, len(mods))
	for _, m := range mods {
		out = append(out, string(m))
	}
	return out
}

// envRef renders an MCP environment value. V2 expands {env:NAME} in stdio
// environment entries and remote headers.
func envRef(v ir.HeaderValue) string {
	if v.FromEnv != "" {
		return "{env:" + v.FromEnv + "}"
	}
	return v.Value
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

type opencodeConfig struct {
	Schema    string                      `json:"$schema"`
	Providers map[string]opencodeProvider `json:"providers"`
	MCP       *opencodeMCP                `json:"mcp,omitempty"`
	Model     string                      `json:"model,omitempty"`
}

type opencodeMCP struct {
	Servers map[string]opencodeMCPServer `json:"servers"`
}

type opencodeProvider struct {
	Name     string                   `json:"name,omitempty"`
	Env      []string                 `json:"env,omitempty"`
	Package  string                   `json:"package"`
	Settings map[string]any           `json:"settings,omitempty"`
	Headers  map[string]string        `json:"headers,omitempty"`
	Models   map[string]opencodeModel `json:"models"`
}

type opencodeModel struct {
	Name         string                `json:"name,omitempty"`
	Capabilities *opencodeCapabilities `json:"capabilities,omitempty"`
	Limit        map[string]int64      `json:"limit,omitempty"`
	Variants     []opencodeVariant     `json:"variants,omitempty"`
}

type opencodeCapabilities struct {
	Tools  *bool    `json:"tools,omitempty"`
	Input  []string `json:"input,omitempty"`
	Output []string `json:"output,omitempty"`
}

type opencodeVariant struct {
	ID       string         `json:"id"`
	Settings map[string]any `json:"settings,omitempty"`
}

type opencodeMCPServer struct {
	Type        string            `json:"type"`
	Command     []string          `json:"command,omitempty"`
	CWD         string            `json:"cwd,omitempty"`
	Environment map[string]string `json:"environment,omitempty"`
	URL         string            `json:"url,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Disabled    *bool             `json:"disabled,omitempty"`
	Timeout     *opencodeTimeout  `json:"timeout,omitempty"`
}

type opencodeTimeout struct {
	Catalog   *int64 `json:"catalog,omitempty"`
	Execution *int64 `json:"execution,omitempty"`
}
