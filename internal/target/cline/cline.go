// Package cline emits Cline CLI providers.json, models.json, and
// cline_mcp_settings.json fragments.
package cline

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

// Target is the Cline CLI emitter (github.com/cline/cline, apps/cli).
type Target struct{}

func (Target) ID() string { return "cline" }

var protocolName = map[ir.Protocol]string{
	ir.ProtocolOpenAICompletions: "openai-chat",
	ir.ProtocolOpenAIResponses:   "openai-responses",
	ir.ProtocolAnthropicMessages: "anthropic",
}

// cline timeout is whole seconds in [1, 3600]; out-of-range values are rejected.
const (
	minTimeoutMS = 1000
	maxTimeoutMS = 3_600_000
)

func (t Target) Validate(cfg ir.Config) []diag.Diagnostic {
	var diags []diag.Diagnostic
	for i, p := range cfg.Providers {
		path := fmt.Sprintf("providers[%d]", i)
		if _, ok := protocolName[p.Protocol]; !ok {
			diags = append(diags, diag.TargetErrorf(t.ID(), path+".protocol",
				"cline protocol must be anthropic, gemini, openai-chat, or openai-responses; got %q", p.Protocol))
		}
		if p.APIKeyEnv != "" {
			diags = append(diags, diag.TargetErrorf(t.ID(), path+".api_key_env",
				"cline stores literal apiKey strings only; the environment fallback exists for built-in provider ids, not custom providers"))
		}
		for name, v := range p.Headers {
			if v.FromEnv != "" || v.BearerFromEnv != "" {
				diags = append(diags, diag.TargetErrorf(t.ID(), path+".headers."+name,
					"cline headers are literal strings with no interpolation; only constant header values are representable"))
			}
		}
	}
	for i, s := range cfg.MCP {
		path := fmt.Sprintf("mcp[%d]", i)
		if s.TimeoutMS != nil && (*s.TimeoutMS < minTimeoutMS || *s.TimeoutMS > maxTimeoutMS) {
			diags = append(diags, diag.TargetErrorf(t.ID(), path+".timeout_ms",
				"cline timeout is whole seconds clamped to 1..3600; %d ms is out of range", *s.TimeoutMS))
		}
		if s.Transport == ir.TransportStdio {
			for name, v := range s.Env {
				if v.FromEnv != "" || v.BearerFromEnv != "" {
					diags = append(diags, diag.TargetErrorf(t.ID(), path+".env."+name,
						"cline MCP env values are literal strings merged over the process environment; environment references are not representable"))
				}
			}
		} else {
			for name, v := range s.Headers {
				if v.FromEnv != "" || v.BearerFromEnv != "" {
					diags = append(diags, diag.TargetErrorf(t.ID(), path+".headers."+name,
						"cline MCP header values are literal strings; environment references are not representable"))
				}
			}
		}
	}
	if cfg.Defaults != nil && cfg.Defaults.Model != "" {
		pid, mid, ok := splitRef(cfg.Defaults.Model)
		if !ok {
			diags = append(diags, diag.TargetErrorf(t.ID(), "defaults.model",
				`cline expects defaults.model as "provider/model"; got %q`, cfg.Defaults.Model))
		} else if !hasModel(cfg, pid, mid) {
			diags = append(diags, diag.TargetErrorf(t.ID(), "defaults.model",
				"cline default %q does not resolve to an emitted provider model", cfg.Defaults.Model))
		}
	}
	return diags
}

func (t Target) Emit(cfg ir.Config) ([]artifact.Artifact, error) {
	defPID, defMID := "", ""
	if cfg.Defaults != nil && cfg.Defaults.Model != "" {
		defPID, defMID, _ = splitRef(cfg.Defaults.Model)
	}

	// Artifact 1: providers.json
	providers := map[string]clineProviderEntry{}
	for _, p := range cfg.Providers {
		settings := clineSettings{
			Provider: p.ID,
			BaseURL:  p.BaseURL,
			Protocol: protocolName[p.Protocol],
			Model:    firstModel(p, defPID, defMID),
		}
		if len(p.Headers) > 0 {
			headers := map[string]string{}
			for name, v := range p.Headers {
				if v.Value != "" {
					headers[name] = v.Value
				}
			}
			if len(headers) > 0 {
				settings.Headers = headers
			}
		}
		providers[p.ID] = clineProviderEntry{
			Settings:    settings,
			TokenSource: "manual",
		}
	}
	providersDoc := clineProvidersDoc{
		Version:   1,
		Providers: providers,
	}
	if defPID != "" {
		providersDoc.LastUsedProvider = defPID
	}
	var pbuf bytes.Buffer
	penc := json.NewEncoder(&pbuf)
	penc.SetIndent("", "  ")
	if err := penc.Encode(providersDoc); err != nil {
		return nil, fmt.Errorf("encoding cline providers.json: %w", err)
	}
	arts := []artifact.Artifact{{
		Target:        t.ID(),
		Name:          "providers.json",
		Format:        "json",
		SuggestedPath: "~/.cline/data/settings/providers.json",
		Content:       pbuf.Bytes(),
	}}

	// Artifact 2: models.json
	modelProviders := map[string]clineModelProvider{}
	for _, p := range cfg.Providers {
		mp := clineModelProvider{
			Provider: clineModelProviderInfo{
				Name:    orDefault(p.Name, p.ID),
				BaseURL: p.BaseURL,
			},
			Models: map[string]clineModel{},
		}
		if len(p.Models) > 0 {
			mp.Provider.DefaultModelID = firstModel(p, defPID, defMID)
		}
		for _, m := range p.Models {
			cm := clineModel{}
			if m.Name != "" {
				cm.Name = m.Name
			}
			if m.ContextWindow != nil {
				cm.ContextWindow = m.ContextWindow
			}
			if m.MaxOutputTokens != nil {
				cm.MaxTokens = m.MaxOutputTokens
			}
			in := m.Input
			if len(in) == 0 {
				in = []ir.Modality{ir.ModalityText}
			}
			out := m.Output
			if len(out) == 0 {
				out = []ir.Modality{ir.ModalityText}
			}
			cm.Modalities = map[string][]ir.Modality{"input": in, "output": out}
			cm.Capabilities = capabilities(m)
			mp.Models[m.ID] = cm
		}
		modelProviders[p.ID] = mp
	}
	var mbuf bytes.Buffer
	menc := json.NewEncoder(&mbuf)
	menc.SetIndent("", "  ")
	if err := menc.Encode(clineModelsDoc{
		Version:   1,
		Providers: modelProviders,
	}); err != nil {
		return nil, fmt.Errorf("encoding cline models.json: %w", err)
	}
	arts = append(arts, artifact.Artifact{
		Target:        t.ID(),
		Name:          "models.json",
		Format:        "json",
		SuggestedPath: "~/.cline/data/settings/models.json",
		Content:       mbuf.Bytes(),
	})

	// Artifact 3: cline_mcp_settings.json
	if len(cfg.MCP) > 0 {
		servers := map[string]clineMCPServer{}
		for _, s := range cfg.MCP {
			entry := clineMCPServer{}
			if s.Enabled != nil && !*s.Enabled {
				entry.Disabled = boolPtr(true)
			}
			if s.TimeoutMS != nil {
				entry.Timeout = int64(*s.TimeoutMS / 1000)
			}
			if s.Transport == ir.TransportStdio {
				tr := clineTransport{
					Type:    "stdio",
					Command: s.Command[0],
				}
				if len(s.Command) > 1 {
					tr.Args = s.Command[1:]
				}
				if s.CWD != "" {
					tr.CWD = s.CWD
				}
				if len(s.Env) > 0 {
					env := map[string]string{}
					for name, v := range s.Env {
						env[name] = v.Value
					}
					tr.Env = env
				}
				entry.Transport = tr
			} else {
				tr := clineTransport{
					Type: "streamableHttp",
					URL:  s.URL,
				}
				if len(s.Headers) > 0 {
					headers := map[string]string{}
					for name, v := range s.Headers {
						headers[name] = v.Value
					}
					tr.Headers = headers
				}
				entry.Transport = tr
			}
			servers[s.ID] = entry
		}
		var jbuf bytes.Buffer
		jenc := json.NewEncoder(&jbuf)
		jenc.SetIndent("", "  ")
		if err := jenc.Encode(struct {
			MCPServers map[string]clineMCPServer `json:"mcpServers"`
		}{MCPServers: servers}); err != nil {
			return nil, fmt.Errorf("encoding cline cline_mcp_settings.json: %w", err)
		}
		arts = append(arts, artifact.Artifact{
			Target:        t.ID(),
			Name:          "cline_mcp_settings.json",
			Format:        "json",
			SuggestedPath: "~/.cline/data/settings/cline_mcp_settings.json",
			Content:       jbuf.Bytes(),
		})
	}
	return arts, nil
}

func capabilities(m ir.Model) []string {
	var out []string
	for _, in := range m.Input {
		switch in {
		case ir.ModalityImage:
			out = append(out, "images")
		case ir.ModalityVideo:
			out = append(out, "video")
		}
	}
	if m.ToolCalling != nil && *m.ToolCalling {
		out = append(out, "tools")
	}
	if m.Reasoning != nil && *m.Reasoning {
		out = append(out, "reasoning")
	}
	return out
}

func firstModel(p ir.Provider, defPID, defMID string) string {
	if p.ID == defPID && defMID != "" {
		return defMID
	}
	if len(p.Models) > 0 {
		return p.Models[0].ID
	}
	return ""
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

type clineProvidersDoc struct {
	Version          int                           `json:"version"`
	LastUsedProvider string                        `json:"lastUsedProvider,omitempty"`
	Providers        map[string]clineProviderEntry `json:"providers"`
}

type clineProviderEntry struct {
	Settings    clineSettings `json:"settings"`
	TokenSource string        `json:"tokenSource"`
}

type clineSettings struct {
	Provider string            `json:"provider"`
	BaseURL  string            `json:"baseUrl,omitempty"`
	Protocol string            `json:"protocol"`
	Model    string            `json:"model,omitempty"`
	Headers  map[string]string `json:"headers,omitempty"`
}

type clineModelsDoc struct {
	Version   int                           `json:"version"`
	Providers map[string]clineModelProvider `json:"providers"`
}

type clineModelProvider struct {
	Provider clineModelProviderInfo `json:"provider"`
	Models   map[string]clineModel  `json:"models"`
}

type clineModelProviderInfo struct {
	Name           string `json:"name"`
	BaseURL        string `json:"baseUrl"`
	DefaultModelID string `json:"defaultModelId,omitempty"`
}

type clineModel struct {
	Name          string                   `json:"name,omitempty"`
	ContextWindow *int64                   `json:"contextWindow,omitempty"`
	MaxTokens     *int64                   `json:"maxTokens,omitempty"`
	Modalities    map[string][]ir.Modality `json:"modalities"`
	Capabilities  []string                 `json:"capabilities,omitempty"`
}

type clineMCPServer struct {
	Transport clineTransport `json:"transport"`
	Disabled  *bool          `json:"disabled,omitempty"`
	Timeout   int64          `json:"timeout,omitempty"`
}

type clineTransport struct {
	Type    string            `json:"type"`
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	CWD     string            `json:"cwd,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	URL     string            `json:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

func boolPtr(b bool) *bool { return &b }
