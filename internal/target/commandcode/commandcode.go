// Package commandcode emits Command Code providers.json, .mcp.json, and
// config.json fragments.
package commandcode

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

// Target is the Command Code emitter.
type Target struct{}

func (Target) ID() string { return "commandcode" }

// commandcodeEfforts are the reasoning efforts the native model schema accepts,
// in output order. Declared names outside this ladder are skipped, never
// remapped.
var commandcodeEfforts = []ir.ReasoningEffort{"low", "medium", "high", "xhigh", "max"}

// commandcodeReservedIDs are provider ids Command Code keeps for its built-in
// subscription lanes and their reserved aliases. providers.json skips an entry
// that uses one of them. The credential store's own field names are reserved as
// well, but they carry uppercase letters and cannot be a valid IR provider id.
var commandcodeReservedIDs = []string{"anthropic", "copilot", "github-copilot", "codex", "openai", "command-code"}

// MappedAuthTypes reports the auth_type values Command Code maps. "none" is a
// keyless provider; a bearer credential has no native field.
func (t Target) MappedAuthTypes() []ir.AuthType { return []ir.AuthType{ir.AuthTypeNone} }

// reservedProviderID reports whether Command Code keeps an id for a built-in
// subscription lane. providers.json skips such an entry.
func reservedProviderID(id string) bool {
	for _, reserved := range commandcodeReservedIDs {
		if id == reserved {
			return true
		}
	}
	return false
}

func (t Target) Validate(cfg ir.Config) []diag.Diagnostic {
	var diags []diag.Diagnostic
	for i, p := range cfg.Providers {
		path := fmt.Sprintf("providers[%d]", i)
		if reservedProviderID(p.ID) {
			diags = append(diags, diag.TargetWarnf(t.ID(), path+".id",
				"commandcode reserves the provider id %q for a built-in subscription lane, so providers.json would skip it; the entry is skipped (rename it, for example %q, to keep it)", p.ID, p.ID+"-api"))
		}
		if p.APIKey.FromEnv == "" && p.APIKey.Value != "" {
			diags = append(diags, diag.TargetWarnf(t.ID(), path+".api_key",
				`commandcode refuses a raw secret in providers.json, so the credential is skipped; use api_key: "ENV:NAME", or store the key with /connect`))
		}
		var literalOnly []string
		for name, v := range p.Headers {
			if v.FromEnv != "" || v.BearerFromEnv != "" {
				literalOnly = append(literalOnly, name)
			}
		}
		if len(literalOnly) > 0 {
			sort.Strings(literalOnly)
			diags = append(diags, diag.TargetWarnf(t.ID(), path+".headers",
				"commandcode sends provider headers literally and does not interpolate environment references; these headers are skipped: %s", strings.Join(literalOnly, ", ")))
		}
		for j, m := range p.Models {
			mpath := fmt.Sprintf("%s.models[%d]", path, j)
			if len(m.Input) > 0 || len(m.Output) > 0 {
				diags = append(diags, diag.TargetWarnf(t.ID(), mpath+".input",
					"commandcode has no per-model modality field, so input and output modalities are skipped"))
			}
			if m.ToolCalling != nil {
				diags = append(diags, diag.TargetWarnf(t.ID(), mpath+".tool_calling",
					"commandcode has no per-model tool field, so tool_calling is skipped"))
			}
			if dropped := droppedEfforts(m.Variants); len(dropped) > 0 {
				diags = append(diags, diag.TargetWarnf(t.ID(), mpath+".variants",
					"commandcode reasoning efforts are limited to low, medium, high, xhigh, and max; these names are skipped: %s", strings.Join(dropped, ", ")))
			}
		}
	}
	for i, s := range cfg.MCP {
		path := fmt.Sprintf("mcp[%d]", i)
		if s.CWD != "" {
			diags = append(diags, diag.TargetWarnf(t.ID(), path+".cwd",
				"the stored MCP server shape has no working directory, so cwd is skipped"))
		}
		if s.TimeoutMS != nil {
			diags = append(diags, diag.TargetWarnf(t.ID(), path+".timeout_ms",
				"the stored MCP server shape has no timeout field, so timeout_ms is skipped"))
		}
	}
	return diags
}

func (t Target) Emit(cfg ir.Config) ([]artifact.Artifact, error) {
	providers := map[string]commandcodeProvider{}
	for _, p := range cfg.Providers {
		if reservedProviderID(p.ID) {
			continue
		}
		cp := commandcodeProvider{
			Name:    p.Name,
			API:     wireName(p.Protocol),
			BaseURL: p.BaseURL,
			Models:  map[string]commandcodeModel{},
		}
		switch {
		case p.EffectiveAuthType() == ir.AuthTypeNone:
			cp.APIKey = false
		case p.APIKey.FromEnv != "":
			cp.APIKey = "$" + p.APIKey.FromEnv
		}
		// A literal api_key is skipped: Command Code refuses a raw secret in
		// providers.json, and Validate reports the skip.
		if len(p.Headers) > 0 {
			headers := map[string]string{}
			for name, v := range p.Headers {
				if v.FromEnv != "" || v.BearerFromEnv != "" {
					continue
				}
				headers[name] = v.Value
			}
			if len(headers) > 0 {
				cp.Headers = headers
			}
		}
		for _, m := range p.Models {
			cm := commandcodeModel{
				Name:          m.Name,
				ContextWindow: m.ContextWindow,
				MaxOutput:     m.MaxOutputTokens,
				Reasoning:     m.Reasoning,
			}
			cm.ReasoningEfforts = effortsInLadder(m.Variants)
			cp.Models[m.ID] = cm
		}
		providers[p.ID] = cp
	}

	providersJSON, err := encodeJSON(map[string]any{"provider": providers})
	if err != nil {
		return nil, fmt.Errorf("encoding commandcode providers.json: %w", err)
	}
	arts := []artifact.Artifact{{
		Target:        t.ID(),
		Name:          "providers.json",
		Format:        "json",
		SuggestedPath: "~/.commandcode/providers.json",
		Content:       providersJSON,
	}}

	if len(cfg.MCP) > 0 {
		mcpJSON, err := encodeJSON(map[string]any{"mcpServers": mcpServers(cfg)})
		if err != nil {
			return nil, fmt.Errorf("encoding commandcode .mcp.json: %w", err)
		}
		arts = append(arts, artifact.Artifact{
			Target:        t.ID(),
			Name:          ".mcp.json",
			Format:        "json",
			SuggestedPath: ".mcp.json",
			Content:       mcpJSON,
		})
	}

	if cfg.Defaults != nil && cfg.Defaults.Model != "" {
		settings, err := defaultModelArtifact(t, cfg.Defaults.Model)
		if err != nil {
			return nil, err
		}
		arts = append(arts, settings)
	}

	return arts, nil
}

// defaultModelArtifact renders the default-model selection. Command Code's own
// settings writer persists one qualified `provider/model` string in `model`,
// and the reader takes that value verbatim.
func defaultModelArtifact(t Target, ref string) (artifact.Artifact, error) {
	content, err := encodeJSON(commandcodeSettings{Model: ref})
	if err != nil {
		return artifact.Artifact{}, fmt.Errorf("encoding commandcode config.json: %w", err)
	}
	return artifact.Artifact{
		Target:        t.ID(),
		Name:          "config.json",
		Format:        "json",
		SuggestedPath: "~/.commandcode/config.json",
		Content:       content,
	}, nil
}

// wireName renders the native `api` field. OpenAI-compatible is the native
// default, and Command Code's own provider writer omits it.
func wireName(p ir.Protocol) string {
	if p == ir.ProtocolOpenAICompletions {
		return ""
	}
	return string(p)
}

// effortsInLadder keeps the declared effort names the native schema accepts, in
// native order. An empty result leaves the field out.
func effortsInLadder(declared []ir.ReasoningEffort) []string {
	if len(declared) == 0 {
		return nil
	}
	has := make(map[ir.ReasoningEffort]bool, len(declared))
	for _, effort := range declared {
		has[effort] = true
	}
	var efforts []string
	for _, effort := range commandcodeEfforts {
		if has[effort] {
			efforts = append(efforts, string(effort))
		}
	}
	return efforts
}

// droppedEfforts reports the declared names outside the native ladder, in
// declared order, so a skip is never silent.
func droppedEfforts(declared []ir.ReasoningEffort) []string {
	var dropped []string
	for _, effort := range declared {
		inLadder := false
		for _, known := range commandcodeEfforts {
			if effort == known {
				inLadder = true
				break
			}
		}
		if !inLadder {
			dropped = append(dropped, string(effort))
		}
	}
	return dropped
}

// mcpServers renders the mcpServers map. Command Code stores stdio commands as
// an executable plus an argument array and interpolates ${NAME} in env and
// header values.
func mcpServers(cfg ir.Config) map[string]commandcodeMCPServer {
	servers := make(map[string]commandcodeMCPServer, len(cfg.MCP))
	for _, s := range cfg.MCP {
		enabled := true
		if s.Enabled != nil {
			enabled = *s.Enabled
		}
		entry := commandcodeMCPServer{Transport: string(s.Transport), Enabled: enabled}
		switch s.Transport {
		case ir.TransportStdio:
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
		case ir.TransportHTTP:
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

func headerRef(v ir.HeaderValue) string {
	if v.BearerFromEnv != "" {
		return "Bearer ${" + v.BearerFromEnv + "}"
	}
	if v.FromEnv != "" {
		return "${" + v.FromEnv + "}"
	}
	return v.Value
}

func envRef(v ir.HeaderValue) string {
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

type commandcodeProvider struct {
	Name    string                      `json:"name,omitempty"`
	API     string                      `json:"api,omitempty"`
	BaseURL string                      `json:"baseURL"`
	APIKey  any                         `json:"apiKey,omitempty"`
	Headers map[string]string           `json:"headers,omitempty"`
	Models  map[string]commandcodeModel `json:"models"`
}

type commandcodeModel struct {
	Name             string   `json:"name,omitempty"`
	ContextWindow    *int64   `json:"contextWindow,omitempty"`
	MaxOutput        *int64   `json:"maxOutput,omitempty"`
	Reasoning        *bool    `json:"reasoning,omitempty"`
	ReasoningEfforts []string `json:"reasoningEfforts,omitempty"`
}

type commandcodeMCPServer struct {
	Transport string            `json:"transport"`
	Enabled   bool              `json:"enabled"`
	Command   string            `json:"command,omitempty"`
	Args      []string          `json:"args,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	URL       string            `json:"url,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"`
}

type commandcodeSettings struct {
	Model string `json:"model"`
}
