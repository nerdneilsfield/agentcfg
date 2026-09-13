// Package crush emits a Charmbracelet Crush crush.json fragment.
package crush

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

// Target is the Charmbracelet Crush emitter (github.com/charmbracelet/crush).
type Target struct{}

func (Target) ID() string { return "crush" }

var typeName = map[ir.Protocol]string{
	ir.ProtocolOpenAICompletions: "openai-compat",
	ir.ProtocolOpenAIResponses:   "openai",
	ir.ProtocolAnthropicMessages: "anthropic",
}

func (t Target) Validate(cfg ir.Config) []diag.Diagnostic {
	var diags []diag.Diagnostic
	for i, p := range cfg.Providers {
		if hasCrushExpression(p.APIKey.Value) {
			diags = append(diags, diag.TargetErrorf(t.ID(), fmt.Sprintf("providers[%d].api_key", i), "literal API key contains native expression syntax and cannot be represented literally"))
		}
		if hasCrushExpression(p.BaseURL) {
			diags = append(diags, diag.TargetErrorf(t.ID(), fmt.Sprintf("providers[%d].base_url", i), "literal value contains Crush expression syntax and cannot be represented literally"))
		}
	}
	for i, p := range cfg.Providers {
		path := fmt.Sprintf("providers[%d]", i)
		if _, ok := typeName[p.Protocol]; !ok {
			diags = append(diags, diag.TargetErrorf(t.ID(), path+".protocol",
				"crush provider type must be openai-compat, openai, or anthropic; got %q", p.Protocol))
		}
		for name, v := range p.Headers {
			if hasCrushExpression(v.Value) {
				diags = append(diags, diag.TargetErrorf(t.ID(), path+".headers."+name,
					"literal value contains Crush expression syntax and cannot be represented literally"))
			}
			if v.BearerFromEnv != "" && name != "Authorization" {
				diags = append(diags, diag.TargetErrorf(t.ID(), path+".headers."+name,
					"crush extra_headers are flat strings; Bearer ENV:NAME is only representable as Authorization: Bearer ${VAR}"))
			}
		}
		for j, m := range p.Models {
			mpath := fmt.Sprintf("%s.models[%d]", path, j)
			if m.ContextWindow == nil {
				diags = append(diags, diag.TargetErrorf(t.ID(), mpath+".context_window",
					"crush Model.context_window is required"))
			}
			if m.MaxOutputTokens == nil {
				diags = append(diags, diag.TargetErrorf(t.ID(), mpath+".max_output_tokens",
					"crush Model.default_max_tokens is required"))
			}
			for _, mod := range m.Input {
				if mod != ir.ModalityText && mod != ir.ModalityImage {
					diags = append(diags, diag.TargetErrorf(t.ID(), mpath+".input",
						"crush attachments are image-only; %q is not representable", mod))
				}
			}
			for _, mod := range m.Output {
				if mod != ir.ModalityText {
					diags = append(diags, diag.TargetErrorf(t.ID(), mpath+".output",
						"crush has no output-modality field; non-text output %q is not representable", mod))
				}
			}
			if m.ToolCalling != nil && !*m.ToolCalling {
				diags = append(diags, diag.TargetErrorf(t.ID(), mpath+".tool_calling",
					"crush agents always expose tools; tool_calling: false is not representable"))
			}
		}
	}
	for i, srv := range cfg.MCP {
		path := fmt.Sprintf("mcp[%d]", i)
		if srv.CWD != "" {
			diags = append(diags, diag.TargetErrorf(t.ID(), path+".cwd",
				"crush MCP has no cwd field"))
		}
		for j, value := range append(append([]string{}, srv.Command...), srv.URL) {
			if hasCrushExpression(value) {
				diags = append(diags, diag.TargetErrorf(t.ID(), fmt.Sprintf("%s.command[%d]", path, j),
					"literal value contains Crush expression syntax and cannot be represented literally"))
			}
		}
		for name, v := range srv.Env {
			if hasCrushExpression(v.Value) {
				diags = append(diags, diag.TargetErrorf(t.ID(), path+".env."+name,
					"literal value contains Crush expression syntax and cannot be represented literally"))
			}
		}
		for name, v := range srv.Headers {
			if hasCrushExpression(v.Value) {
				diags = append(diags, diag.TargetErrorf(t.ID(), path+".headers."+name,
					"literal value contains Crush expression syntax and cannot be represented literally"))
			}
		}
		if srv.TimeoutMS != nil && *srv.TimeoutMS%1000 != 0 {
			diags = append(diags, diag.TargetErrorf(t.ID(), path+".timeout_ms",
				"crush timeout is whole seconds; %d ms is not divisible by 1000", *srv.TimeoutMS))
		}
	}
	if cfg.Defaults != nil && cfg.Defaults.Model != "" {
		pid, mid, ok := splitRef(cfg.Defaults.Model)
		if !ok {
			diags = append(diags, diag.TargetErrorf(t.ID(), "defaults.model",
				`crush expects defaults.model as "provider/model"; got %q`, cfg.Defaults.Model))
		} else if !hasModel(cfg, pid, mid) {
			diags = append(diags, diag.TargetErrorf(t.ID(), "defaults.model",
				"crush default %q does not resolve to an emitted provider model", cfg.Defaults.Model))
		}
	}
	return diags
}

func (t Target) Emit(cfg ir.Config) ([]artifact.Artifact, error) {
	providers := map[string]crushProvider{}
	for _, p := range cfg.Providers {
		cp := crushProvider{
			ID:             p.ID,
			Name:           orDefault(p.Name, p.ID),
			BaseURL:        p.BaseURL,
			Type:           typeName[p.Protocol],
			DiscoverModels: false,
		}
		if p.APIKey.FromEnv != "" {
			cp.APIKey = "$" + p.APIKey.FromEnv
		} else if p.APIKey.Value != "" {
			cp.APIKey = p.APIKey.Value
		}
		if len(p.Headers) > 0 {
			headers := map[string]string{}
			for name, v := range p.Headers {
				headers[name] = envInterp(v)
			}
			cp.ExtraHeaders = headers
		}
		for _, m := range p.Models {
			cm := crushModel{
				ID:               m.ID,
				Name:             orDefault(m.Name, m.ID),
				ContextWindow:    *m.ContextWindow,
				DefaultMaxTokens: *m.MaxOutputTokens,
			}
			if m.Reasoning != nil {
				cm.CanReason = *m.Reasoning
			}
			if len(m.Variants) > 0 {
				cm.ReasoningLevels = effortStrings(m.Variants)
			}
			cm.SupportsAttachments = hasImage(m)
			cp.Models = append(cp.Models, cm)
		}
		providers[p.ID] = cp
	}
	doc := crushConfig{Providers: providers}
	if cfg.Defaults != nil && cfg.Defaults.Model != "" {
		pid, mid, _ := splitRef(cfg.Defaults.Model)
		doc.Models = map[string]crushSelected{
			"large": {Provider: pid, Model: mid},
			"small": {Provider: pid, Model: mid},
		}
	}
	if len(cfg.MCP) > 0 {
		servers := map[string]crushMCP{}
		for _, s := range cfg.MCP {
			entry := crushMCP{}
			if s.Enabled != nil && !*s.Enabled {
				entry.Disabled = true
			}
			if s.TimeoutMS != nil {
				entry.Timeout = int64(*s.TimeoutMS / 1000)
			}
			if s.Transport == ir.TransportStdio {
				entry.Type = "stdio"
				entry.Command = s.Command[0]
				if len(s.Command) > 1 {
					entry.Args = s.Command[1:]
				}
				if len(s.Env) > 0 {
					env := map[string]string{}
					for name, v := range s.Env {
						env[name] = envInterp(v)
					}
					entry.Env = env
				}
			} else {
				entry.Type = "http"
				entry.URL = s.URL
				if len(s.Headers) > 0 {
					headers := map[string]string{}
					for name, v := range s.Headers {
						headers[name] = envInterp(v)
					}
					entry.Headers = headers
				}
			}
			servers[s.ID] = entry
		}
		doc.MCP = servers
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return nil, fmt.Errorf("encoding crush.json: %w", err)
	}
	return []artifact.Artifact{{
		Target:        t.ID(),
		Name:          "crush.json",
		Format:        "json",
		SuggestedPath: "~/.config/crush/crush.json",
		Content:       buf.Bytes(),
	}}, nil
}

func hasCrushExpression(value string) bool { return strings.ContainsAny(value, "$`") }

func envInterp(v ir.HeaderValue) string {
	if v.BearerFromEnv != "" {
		return "Bearer ${" + v.BearerFromEnv + "}"
	}
	if v.FromEnv != "" {
		return "$" + v.FromEnv
	}
	return v.Value
}

func hasImage(m ir.Model) bool {
	for _, in := range m.Input {
		if in == ir.ModalityImage {
			return true
		}
	}
	return false
}

func hasModel(cfg ir.Config, pid, mid string) bool {
	for _, p := range cfg.Providers {
		if p.ID != pid {
			continue
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

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

type crushConfig struct {
	Models    map[string]crushSelected `json:"models,omitempty"`
	Providers map[string]crushProvider `json:"providers"`
	MCP       map[string]crushMCP      `json:"mcp,omitempty"`
}

type crushSelected struct {
	Model    string `json:"model"`
	Provider string `json:"provider"`
}

type crushProvider struct {
	ID             string            `json:"id,omitempty"`
	Name           string            `json:"name,omitempty"`
	BaseURL        string            `json:"base_url,omitempty"`
	Type           string            `json:"type"`
	DiscoverModels bool              `json:"discover_models"`
	APIKey         string            `json:"api_key,omitempty"`
	ExtraHeaders   map[string]string `json:"extra_headers,omitempty"`
	Models         []crushModel      `json:"models,omitempty"`
}

type crushModel struct {
	ID                  string   `json:"id"`
	Name                string   `json:"name"`
	CostPer1mIn         int      `json:"cost_per_1m_in"`
	CostPer1mOut        int      `json:"cost_per_1m_out"`
	CostPer1mInCached   int      `json:"cost_per_1m_in_cached"`
	CostPer1mOutCached  int      `json:"cost_per_1m_out_cached"`
	ContextWindow       int64    `json:"context_window"`
	DefaultMaxTokens    int64    `json:"default_max_tokens"`
	CanReason           bool     `json:"can_reason"`
	ReasoningLevels     []string `json:"reasoning_levels,omitempty"`
	SupportsAttachments bool     `json:"supports_attachments"`
}

type crushMCP struct {
	Type     string            `json:"type"`
	Command  string            `json:"command,omitempty"`
	Args     []string          `json:"args,omitempty"`
	Env      map[string]string `json:"env,omitempty"`
	URL      string            `json:"url,omitempty"`
	Headers  map[string]string `json:"headers,omitempty"`
	Disabled bool              `json:"disabled,omitempty"`
	Timeout  int64             `json:"timeout,omitempty"`
}

func effortStrings(efforts []ir.ReasoningEffort) []string {
	out := make([]string, len(efforts))
	for i, effort := range efforts {
		out[i] = string(effort)
	}
	return out
}
