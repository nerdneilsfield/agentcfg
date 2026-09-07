// Package openclaw emits an OpenClaw openclaw.json fragment.
package openclaw

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

// Target is the OpenClaw emitter (github.com/openclaw/openclaw).
type Target struct{}

func (Target) ID() string { return "openclaw" }

var apiName = map[ir.Protocol]string{
	ir.ProtocolOpenAICompletions: "openai-completions",
	ir.ProtocolOpenAIResponses:   "openai-responses",
	ir.ProtocolAnthropicMessages: "anthropic-messages",
}

func (t Target) Validate(cfg ir.Config) []diag.Diagnostic {
	var diags []diag.Diagnostic
	for i, p := range cfg.Providers {
		path := fmt.Sprintf("providers[%d]", i)
		if _, ok := apiName[p.Protocol]; !ok {
			diags = append(diags, diag.TargetErrorf(t.ID(), path+".protocol",
				"openclaw api must be openai-completions, openai-responses, or anthropic-messages for custom providers; got %q", p.Protocol))
		}
	}
	if cfg.Defaults != nil && cfg.Defaults.Model != "" {
		pid, mid, ok := splitRef(cfg.Defaults.Model)
		if !ok {
			diags = append(diags, diag.TargetErrorf(t.ID(), "defaults.model",
				`openclaw expects defaults.model as "provider/model"; got %q`, cfg.Defaults.Model))
		} else if !hasModel(cfg, pid, mid) {
			diags = append(diags, diag.TargetErrorf(t.ID(), "defaults.model",
				"openclaw default %q does not resolve to an emitted provider model", cfg.Defaults.Model))
		}
	}
	return diags
}

func (t Target) Emit(cfg ir.Config) ([]artifact.Artifact, error) {
	providers := map[string]openclawProvider{}
	for _, p := range cfg.Providers {
		op := openclawProvider{
			BaseURL: p.BaseURL,
			API:     apiName[p.Protocol],
		}
		if p.APIKeyEnv != "" {
			op.APIKey = "${" + p.APIKeyEnv + "}"
		}
		if len(p.Headers) > 0 {
			headers := map[string]string{}
			for name, v := range p.Headers {
				headers[name] = envInterp(v)
			}
			op.Headers = headers
		}
		for _, m := range p.Models {
			om := openclawModel{ID: m.ID}
			if m.Name != "" {
				om.Name = m.Name
			}
			if m.ContextWindow != nil {
				om.ContextWindow = m.ContextWindow
			}
			if m.MaxOutputTokens != nil {
				om.MaxTokens = m.MaxOutputTokens
			}
			if len(m.Input) > 0 {
				in := make([]string, 0, len(m.Input))
				for _, mod := range m.Input {
					in = append(in, string(mod))
				}
				om.Input = in
			}
			if m.Reasoning != nil {
				om.Reasoning = m.Reasoning
			}
			if m.ToolCalling != nil {
				om.Compat = map[string]bool{"supportsTools": *m.ToolCalling}
			}
			op.Models = append(op.Models, om)
		}
		providers[p.ID] = op
	}
	doc := openclawConfig{
		Models: openclawModels{Providers: providers},
	}
	if cfg.Defaults != nil && cfg.Defaults.Model != "" {
		doc.Agents = &openclawAgents{Defaults: openclawAgentDefaults{Model: cfg.Defaults.Model}}
	}
	if len(cfg.MCP) > 0 {
		servers := map[string]openclawMCPServer{}
		for _, s := range cfg.MCP {
			osrv := openclawMCPServer{Enabled: true}
			if s.Enabled != nil {
				osrv.Enabled = *s.Enabled
			}
			if s.Transport == ir.TransportStdio {
				osrv.Transport = "stdio"
				osrv.Command = s.Command[0]
				if len(s.Command) > 1 {
					osrv.Args = s.Command[1:]
				}
				if len(s.Env) > 0 {
					env := map[string]string{}
					for name, v := range s.Env {
						env[name] = envInterp(v)
					}
					osrv.Env = env
				}
				if s.CWD != "" {
					osrv.CWD = s.CWD
				}
			} else {
				osrv.Transport = "streamable-http"
				osrv.URL = s.URL
				if len(s.Headers) > 0 {
					headers := map[string]string{}
					for name, v := range s.Headers {
						headers[name] = envInterp(v)
					}
					osrv.Headers = headers
				}
			}
			if s.TimeoutMS != nil {
				osrv.ConnectionTimeoutMs = s.TimeoutMS
				osrv.RequestTimeoutMs = s.TimeoutMS
			}
			servers[s.ID] = osrv
		}
		doc.MCP = &openclawMCP{Servers: servers}
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return nil, fmt.Errorf("encoding openclaw.json: %w", err)
	}
	return []artifact.Artifact{{
		Target:        t.ID(),
		Name:          "openclaw.json",
		Format:        "json",
		SuggestedPath: "~/.openclaw/openclaw.json",
		Content:       buf.Bytes(),
	}}, nil
}

func envInterp(v ir.HeaderValue) string {
	if v.BearerFromEnv != "" {
		return "Bearer ${" + v.BearerFromEnv + "}"
	}
	if v.FromEnv != "" {
		return "${" + v.FromEnv + "}"
	}
	return v.Value
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

type openclawConfig struct {
	Models openclawModels  `json:"models"`
	Agents *openclawAgents `json:"agents,omitempty"`
	MCP    *openclawMCP    `json:"mcp,omitempty"`
}

type openclawModels struct {
	Providers map[string]openclawProvider `json:"providers"`
}

type openclawAgents struct {
	Defaults openclawAgentDefaults `json:"defaults"`
}

type openclawAgentDefaults struct {
	Model string `json:"model"`
}

type openclawMCP struct {
	Servers map[string]openclawMCPServer `json:"servers"`
}

type openclawProvider struct {
	BaseURL string            `json:"baseUrl"`
	API     string            `json:"api"`
	APIKey  string            `json:"apiKey,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Models  []openclawModel   `json:"models"`
}

type openclawModel struct {
	ID            string          `json:"id"`
	Name          string          `json:"name,omitempty"`
	ContextWindow *int64          `json:"contextWindow,omitempty"`
	MaxTokens     *int64          `json:"maxTokens,omitempty"`
	Input         []string        `json:"input,omitempty"`
	Reasoning     *bool           `json:"reasoning,omitempty"`
	Compat        map[string]bool `json:"compat,omitempty"`
}

type openclawMCPServer struct {
	Enabled             bool              `json:"enabled"`
	Transport           string            `json:"transport,omitempty"`
	Command             string            `json:"command,omitempty"`
	Args                []string          `json:"args,omitempty"`
	Env                 map[string]string `json:"env,omitempty"`
	CWD                 string            `json:"cwd,omitempty"`
	URL                 string            `json:"url,omitempty"`
	Headers             map[string]string `json:"headers,omitempty"`
	ConnectionTimeoutMs *int64            `json:"connectionTimeoutMs,omitempty"`
	RequestTimeoutMs    *int64            `json:"requestTimeoutMs,omitempty"`
}
