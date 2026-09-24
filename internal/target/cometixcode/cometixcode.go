// Package cometixcode emits CometixCode settings.json and .mcp.json fragments.
// CometixCode is a Rust reimplementation of Claude Code, so its native files are
// Claude Code's.
package cometixcode

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"agentcfg/internal/artifact"
	"agentcfg/internal/diag"
	"agentcfg/internal/ir"
	"agentcfg/internal/target"
)

func init() { target.Register(Target{}) }

// Target is the CometixCode emitter.
type Target struct{}

func (Target) ID() string { return "cometixcode" }

// MappedAuthTypes reports the auth_type values CometixCode maps. The official
// and bearer credential injections have one native environment variable each,
// and "none" leaves both unset.
func (t Target) MappedAuthTypes() []ir.AuthType {
	return []ir.AuthType{ir.AuthTypeBearer, ir.AuthTypeNone}
}

func (t Target) Validate(cfg ir.Config) []diag.Diagnostic {
	var diags []diag.Diagnostic
	eligible := eligibleProviders(cfg)
	if len(eligible) > 1 {
		diags = append(diags, diag.TargetWarnf(t.ID(), "providers",
			"CometixCode configures one Anthropic endpoint through ANTHROPIC_BASE_URL, so only the selected provider is emitted; the others are skipped"))
	}
	for i, p := range cfg.Providers {
		path := fmt.Sprintf("providers[%d]", i)
		if p.Protocol != ir.ProtocolAnthropicMessages {
			diags = append(diags, diag.TargetWarnf(t.ID(), path+".protocol",
				"CometixCode speaks the Anthropic Messages API, so a %q provider is skipped", p.Protocol))
			continue
		}
		if p.APIKey.FromEnv != "" {
			diags = append(diags, diag.TargetWarnf(t.ID(), path+".api_key",
				"settings.json env values are literal, so an ENV:NAME credential is skipped; export the variable in the shell or supply a literal API key"))
		}
		var envHeaders []string
		for name, v := range p.Headers {
			if v.FromEnv != "" || v.BearerFromEnv != "" {
				envHeaders = append(envHeaders, name)
			}
		}
		if len(envHeaders) > 0 {
			sort.Strings(envHeaders)
			diags = append(diags, diag.TargetWarnf(t.ID(), path+".headers",
				"ANTHROPIC_CUSTOM_HEADERS is a literal string and does not interpolate, so these headers are skipped: %s", strings.Join(envHeaders, ", ")))
		}
		for j, m := range p.Models {
			var skipped []string
			if m.ContextWindow != nil {
				skipped = append(skipped, "context_window")
			}
			if m.MaxOutputTokens != nil {
				skipped = append(skipped, "max_output_tokens")
			}
			if len(m.Input) > 0 || len(m.Output) > 0 {
				skipped = append(skipped, "input/output")
			}
			if m.Reasoning != nil {
				skipped = append(skipped, "reasoning")
			}
			if len(m.Variants) > 0 {
				skipped = append(skipped, "variants")
			}
			if m.ToolCalling != nil {
				skipped = append(skipped, "tool_calling")
			}
			if len(skipped) > 0 {
				diags = append(diags, diag.TargetWarnf(t.ID(), fmt.Sprintf("%s.models[%d]", path, j),
					"CometixCode has no field for these model facts, so they are skipped: %s", strings.Join(skipped, ", ")))
			}
		}
	}
	for i, s := range cfg.MCP {
		path := fmt.Sprintf("mcp[%d]", i)
		if s.CWD != "" {
			diags = append(diags, diag.TargetWarnf(t.ID(), path+".cwd",
				"a .mcp.json server entry has no working directory, so cwd is skipped"))
		}
		if s.TimeoutMS != nil {
			diags = append(diags, diag.TargetWarnf(t.ID(), path+".timeout_ms",
				"a .mcp.json server entry has no timeout field, so timeout_ms is skipped (MCP_TIMEOUT is a process variable)"))
		}
	}
	if providerID, _, ok := splitDefault(cfg); ok {
		served := false
		for _, p := range eligibleProviders(cfg) {
			if p.ID == providerID {
				served = true
				break
			}
		}
		if !served {
			diags = append(diags, diag.TargetWarnf(t.ID(), "defaults.model",
				"the default model names a provider CometixCode cannot serve, so ANTHROPIC_MODEL is not written"))
		}
	}
	return diags
}

// eligibleProviders returns the providers CometixCode can serve: it speaks the
// Anthropic Messages API only.
func eligibleProviders(cfg ir.Config) []ir.Provider {
	var out []ir.Provider
	for _, p := range cfg.Providers {
		if p.Protocol == ir.ProtocolAnthropicMessages {
			out = append(out, p)
		}
	}
	return out
}

// selectedProvider picks the provider the default model names, or the first one
// that CometixCode can serve.
func selectedProvider(cfg ir.Config) (ir.Provider, string, bool) {
	eligible := eligibleProviders(cfg)
	if len(eligible) == 0 {
		return ir.Provider{}, "", false
	}
	providerID, modelID, ok := splitDefault(cfg)
	if ok {
		for _, p := range eligible {
			if p.ID == providerID {
				return p, modelID, true
			}
		}
	}
	return eligible[0], "", true
}

func splitDefault(cfg ir.Config) (string, string, bool) {
	if cfg.Defaults == nil || cfg.Defaults.Model == "" {
		return "", "", false
	}
	providerID, modelID, ok := ir.SplitModelRef(cfg.Defaults.Model)
	return providerID, modelID, ok
}

func (t Target) Emit(cfg ir.Config) ([]artifact.Artifact, error) {
	env := map[string]string{}
	if p, modelID, ok := selectedProvider(cfg); ok {
		env["ANTHROPIC_BASE_URL"] = p.BaseURL
		if p.APIKey.Value != "" {
			if p.EffectiveAuthType() == ir.AuthTypeBearer {
				env["ANTHROPIC_AUTH_TOKEN"] = p.APIKey.Value
			} else {
				env["ANTHROPIC_API_KEY"] = p.APIKey.Value
			}
		}
		if headers := customHeaders(p.Headers); headers != "" {
			env["ANTHROPIC_CUSTOM_HEADERS"] = headers
		}
		if modelID != "" {
			env["ANTHROPIC_MODEL"] = modelID
		}
	}

	settings := cometixSettings{Schema: "https://json.schemastore.org/claude-code-settings.json", Env: env}
	for _, s := range cfg.MCP {
		if s.Enabled != nil && !*s.Enabled {
			settings.DisabledMcpjsonServers = append(settings.DisabledMcpjsonServers, s.ID)
		}
	}
	sort.Strings(settings.DisabledMcpjsonServers)

	settingsJSON, err := encodeJSON(settings)
	if err != nil {
		return nil, fmt.Errorf("encoding cometixcode settings.json: %w", err)
	}
	arts := []artifact.Artifact{{
		Target:        t.ID(),
		Name:          "settings.json",
		Format:        "json",
		SuggestedPath: "~/.claude/settings.json",
		Content:       settingsJSON,
	}}

	if len(cfg.MCP) > 0 {
		mcpJSON, err := encodeJSON(map[string]any{"mcpServers": mcpServers(cfg)})
		if err != nil {
			return nil, fmt.Errorf("encoding cometixcode .mcp.json: %w", err)
		}
		arts = append(arts, artifact.Artifact{
			Target:        t.ID(),
			Name:          ".mcp.json",
			Format:        "json",
			SuggestedPath: ".mcp.json",
			Content:       mcpJSON,
		})
	}

	return arts, nil
}

// customHeaders renders the newline-separated "Name: Value" string
// ANTHROPIC_CUSTOM_HEADERS takes. Environment-derived headers are skipped;
// Validate reports them.
func customHeaders(headers map[string]ir.HeaderValue) string {
	names := make([]string, 0, len(headers))
	for name, v := range headers {
		if v.FromEnv != "" || v.BearerFromEnv != "" {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	lines := make([]string, 0, len(names))
	for _, name := range names {
		lines = append(lines, name+": "+headers[name].Value)
	}
	return strings.Join(lines, "\n")
}

// mcpServers renders the mcpServers map. A stdio entry is identified by its
// command; a remote entry needs an explicit type. Both interpolate ${NAME} in
// environment and header values.
func mcpServers(cfg ir.Config) map[string]cometixMCPServer {
	servers := make(map[string]cometixMCPServer, len(cfg.MCP))
	for _, s := range cfg.MCP {
		entry := cometixMCPServer{}
		if s.Transport == ir.TransportStdio {
			entry.Command = s.Command[0]
			if len(s.Command) > 1 {
				entry.Args = s.Command[1:]
			}
			if len(s.Env) > 0 {
				env := map[string]string{}
				for name, v := range s.Env {
					env[name] = envRef(v)
				}
				entry.Env = env
			}
		} else {
			entry.Type = "http"
			entry.URL = s.URL
			if len(s.Headers) > 0 {
				headers := map[string]string{}
				for name, v := range s.Headers {
					headers[name] = headerRef(v)
				}
				entry.Headers = headers
			}
		}
		servers[s.ID] = entry
	}
	return servers
}

func envRef(v ir.HeaderValue) string {
	if v.FromEnv != "" {
		return "${" + v.FromEnv + "}"
	}
	return v.Value
}

func headerRef(v ir.HeaderValue) string {
	if v.BearerFromEnv != "" {
		return "Bearer ${" + v.BearerFromEnv + "}"
	}
	if v.FromEnv != "" {
		return "${" + v.FromEnv + "}"
	}
	return v.Value
}

func encodeJSON(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type cometixSettings struct {
	Schema                 string            `json:"$schema"`
	Env                    map[string]string `json:"env,omitempty"`
	DisabledMcpjsonServers []string          `json:"disabledMcpjsonServers,omitempty"`
}

type cometixMCPServer struct {
	Type    string            `json:"type,omitempty"`
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	URL     string            `json:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}
