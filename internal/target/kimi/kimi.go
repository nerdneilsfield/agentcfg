// Package kimi emits Kimi Code config.toml and mcp.json fragments.
package kimi

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/BurntSushi/toml"

	"agentcfg/internal/artifact"
	"agentcfg/internal/diag"
	"agentcfg/internal/ir"
	"agentcfg/internal/target"
)

func init() { target.Register(Target{}) }

// Target is the Kimi Code emitter.
type Target struct{}

func (Target) ID() string { return "kimi" }

var providerType = map[ir.Protocol]string{
	ir.ProtocolOpenAICompletions: "openai",
	ir.ProtocolOpenAIResponses:   "openai_responses",
	ir.ProtocolAnthropicMessages: "anthropic",
}

func (t Target) Validate(cfg ir.Config) []diag.Diagnostic {
	var diags []diag.Diagnostic
	seen := map[string]string{}
	for i, p := range cfg.Providers {
		path := fmt.Sprintf("providers[%d]", i)
		if _, ok := providerType[p.Protocol]; !ok {
			diags = append(diags, diag.TargetErrorf(t.ID(), path+".protocol",
				"kimi provider type must be one of kimi, anthropic, openai, openai_responses, google-genai, vertexai; got %q", p.Protocol))
		}
		if p.APIKey.FromEnv != "" {
			diags = append(diags, diag.TargetErrorf(t.ID(), path+".api_key",
				"kimi reads literal api_key values only and has no environment fallback; api_key environment references are not representable"))
		}
		for name, v := range p.Headers {
			if v.FromEnv != "" || v.BearerFromEnv != "" {
				diags = append(diags, diag.TargetErrorf(t.ID(), path+".headers."+name,
					"kimi custom_headers are literal strings with no interpolation; only constant header values are representable"))
			}
		}
		for j, m := range p.Models {
			if m.ContextWindow == nil {
				diags = append(diags, diag.TargetErrorf(t.ID(), fmt.Sprintf("%s.models[%d]", path, j),
					"kimi requires max_context_size for every model; set context_window in the IR"))
			}
			if prev, dup := seen[m.ID]; dup && prev != p.ID {
				diags = append(diags, diag.TargetErrorf(t.ID(), path+".models",
					"kimi model aliases are flat; duplicate model id %q across providers %q and %q is rejected", m.ID, prev, p.ID))
			} else if !dup {
				seen[m.ID] = p.ID
			}
		}
	}
	for i, s := range cfg.MCP {
		path := fmt.Sprintf("mcp[%d]", i)
		if s.Transport == ir.TransportStdio {
			for name, v := range s.Env {
				if v.FromEnv != "" || v.BearerFromEnv != "" {
					diags = append(diags, diag.TargetErrorf(t.ID(), path+".env."+name,
						"kimi mcp.json env values are literal strings; environment references are not representable"))
				}
			}
		} else {
			for name, v := range s.Headers {
				if v.FromEnv != "" {
					diags = append(diags, diag.TargetErrorf(t.ID(), path+".headers."+name,
						"kimi mcp.json header values are literal strings; use Bearer ENV:NAME on Authorization instead"))
				}
				if v.BearerFromEnv != "" && name != "Authorization" {
					diags = append(diags, diag.TargetErrorf(t.ID(), path+".headers."+name,
						"kimi bearerTokenEnvVar applies to Authorization only; use a constant value for other headers"))
				}
			}
		}
	}
	if cfg.Defaults != nil && cfg.Defaults.Model != "" {
		if _, ok := seen[modelKey(cfg.Defaults.Model)]; !ok {
			diags = append(diags, diag.TargetErrorf(t.ID(), "defaults.model",
				"kimi default_model must be a [models] alias; %q does not resolve to an emitted model", cfg.Defaults.Model))
		}
	}
	return diags
}

func (t Target) Emit(cfg ir.Config) ([]artifact.Artifact, error) {
	doc := kimiConfig{
		Providers: map[string]kimiProvider{},
		Models:    map[string]kimiModel{},
	}
	for _, p := range cfg.Providers {
		kp := kimiProvider{Type: providerType[p.Protocol], APIKey: p.APIKey.Value}
		if p.BaseURL != "" {
			kp.BaseURL = p.BaseURL
		}
		if len(p.Headers) > 0 {
			headers := map[string]string{}
			for name, v := range p.Headers {
				headers[name] = v.Value
			}
			if len(headers) > 0 {
				kp.CustomHeaders = headers
			}
		}
		doc.Providers[p.ID] = kp
		for _, m := range p.Models {
			km := kimiModel{
				Provider:       p.ID,
				Model:          m.ID,
				MaxContextSize: *m.ContextWindow,
			}
			if m.Name != "" {
				km.DisplayName = m.Name
			}
			if m.MaxOutputTokens != nil {
				km.MaxOutputSize = m.MaxOutputTokens
			}
			km.Capabilities = capabilities(m)
			if len(m.Variants) > 0 {
				km.SupportEfforts = effortStrings(m.Variants)
			}
			doc.Models[m.ID] = km
		}
	}
	if cfg.Defaults != nil && cfg.Defaults.Model != "" {
		doc.DefaultModel = modelKey(cfg.Defaults.Model)
	}

	var tbuf bytes.Buffer
	if err := toml.NewEncoder(&tbuf).Encode(doc); err != nil {
		return nil, fmt.Errorf("encoding kimi config.toml: %w", err)
	}
	arts := []artifact.Artifact{{
		Target:        t.ID(),
		Name:          "config.toml",
		Format:        "toml",
		SuggestedPath: "~/.kimi-code/config.toml",
		Content:       tbuf.Bytes(),
	}}

	if len(cfg.MCP) > 0 {
		servers := map[string]kimiMCPServer{}
		for _, s := range cfg.MCP {
			entry := kimiMCPServer{Enabled: true}
			if s.Enabled != nil {
				entry.Enabled = *s.Enabled
			}
			if s.Transport == ir.TransportStdio {
				entry.Transport = "stdio"
				entry.Command = s.Command[0]
				if len(s.Command) > 1 {
					entry.Args = s.Command[1:]
				}
				if len(s.Env) > 0 {
					env := map[string]string{}
					for name, v := range s.Env {
						env[name] = v.Value
					}
					entry.Env = env
				}
				if s.CWD != "" {
					entry.CWD = s.CWD
				}
			} else {
				entry.Transport = "http"
				entry.URL = s.URL
				headers := map[string]string{}
				for name, v := range s.Headers {
					if v.BearerFromEnv != "" && name == "Authorization" {
						entry.BearerTokenEnvVar = v.BearerFromEnv
						continue
					}
					headers[name] = v.Value
				}
				if len(headers) > 0 {
					entry.Headers = headers
				}
			}
			servers[s.ID] = entry
		}
		var jbuf bytes.Buffer
		jenc := json.NewEncoder(&jbuf)
		jenc.SetIndent("", "  ")
		if err := jenc.Encode(struct {
			MCPServers map[string]kimiMCPServer `json:"mcpServers"`
		}{MCPServers: servers}); err != nil {
			return nil, fmt.Errorf("encoding kimi mcp.json: %w", err)
		}
		arts = append(arts, artifact.Artifact{
			Target:        t.ID(),
			Name:          "mcp.json",
			Format:        "json",
			SuggestedPath: "~/.kimi-code/mcp.json",
			Content:       jbuf.Bytes(),
		})
	}
	return arts, nil
}

func capabilities(m ir.Model) []string {
	var out []string
	if m.Reasoning != nil && *m.Reasoning {
		out = append(out, "thinking")
	}
	if m.ToolCalling != nil && *m.ToolCalling {
		out = append(out, "tool_use")
	}
	for _, in := range m.Input {
		switch in {
		case ir.ModalityImage:
			out = append(out, "image_in")
		case ir.ModalityVideo:
			out = append(out, "video_in")
		case ir.ModalityAudio:
			out = append(out, "audio_in")
		}
	}
	return out
}

func modelKey(providerModel string) string {
	for i := len(providerModel) - 1; i >= 0; i-- {
		if providerModel[i] == '/' {
			return providerModel[i+1:]
		}
	}
	return providerModel
}

type kimiConfig struct {
	DefaultModel string                  `toml:"default_model,omitempty"`
	Providers    map[string]kimiProvider `toml:"providers"`
	Models       map[string]kimiModel    `toml:"models"`
}

type kimiProvider struct {
	APIKey        string            `toml:"api_key,omitempty"`
	Type          string            `toml:"type"`
	BaseURL       string            `toml:"base_url,omitempty"`
	CustomHeaders map[string]string `toml:"custom_headers,omitempty"`
}

type kimiModel struct {
	Provider       string   `toml:"provider"`
	Model          string   `toml:"model"`
	MaxContextSize int64    `toml:"max_context_size"`
	MaxOutputSize  *int64   `toml:"max_output_size,omitempty"`
	DisplayName    string   `toml:"display_name,omitempty"`
	Capabilities   []string `toml:"capabilities,omitempty"`
	SupportEfforts []string `toml:"support_efforts,omitempty"`
}

// kimiMCPServer is one entry in mcp.json. Field order is deliberate:
// transport first mirrors the kimi-code documentation examples.
type kimiMCPServer struct {
	Transport         string            `json:"transport"`
	Command           string            `json:"command,omitempty"`
	Args              []string          `json:"args,omitempty"`
	URL               string            `json:"url,omitempty"`
	Headers           map[string]string `json:"headers,omitempty"`
	BearerTokenEnvVar string            `json:"bearerTokenEnvVar,omitempty"`
	Env               map[string]string `json:"env,omitempty"`
	CWD               string            `json:"cwd,omitempty"`
	Enabled           bool              `json:"enabled"`
}

func effortStrings(efforts []ir.ReasoningEffort) []string {
	out := make([]string, len(efforts))
	for i, effort := range efforts {
		out[i] = string(effort)
	}
	return out
}
