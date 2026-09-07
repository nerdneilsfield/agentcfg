// Package grok emits a Grok Build config.toml fragment.
package grok

import (
	"bytes"
	"fmt"

	"github.com/BurntSushi/toml"

	"agentcfg/internal/artifact"
	"agentcfg/internal/diag"
	"agentcfg/internal/ir"
	"agentcfg/internal/target"
)

func init() { target.Register(Target{}) }

// Target is the Grok Build emitter.
type Target struct{}

func (Target) ID() string { return "grok" }

var apiBackend = map[ir.Protocol]string{
	ir.ProtocolOpenAICompletions: "chat_completions",
	ir.ProtocolOpenAIResponses:   "responses",
	ir.ProtocolAnthropicMessages: "messages",
}

func (t Target) Validate(cfg ir.Config) []diag.Diagnostic {
	var diags []diag.Diagnostic
	seen := map[string]string{}
	for i, p := range cfg.Providers {
		path := fmt.Sprintf("providers[%d]", i)
		if _, ok := apiBackend[p.Protocol]; !ok {
			diags = append(diags, diag.TargetErrorf(t.ID(), path+".protocol",
				"grok api_backend must be chat_completions, responses, or messages; got %q", p.Protocol))
		}
		for name, v := range p.Headers {
			if v.FromEnv != "" || v.BearerFromEnv != "" {
				diags = append(diags, diag.TargetErrorf(t.ID(), path+".headers."+name,
					"grok extra_headers has no documented env expansion; only constant header values are representable"))
			}
		}
		for _, m := range p.Models {
			if prev, dup := seen[m.ID]; dup && prev != p.ID {
				diags = append(diags, diag.TargetErrorf(t.ID(), path+".models",
					"grok has no provider table; duplicate model id %q across providers %q and %q is rejected", m.ID, prev, p.ID))
			} else if !dup {
				seen[m.ID] = p.ID
			}
		}
	}
	for i, srv := range cfg.MCP {
		path := fmt.Sprintf("mcp[%d]", i)
		if srv.Transport == ir.TransportHTTP {
			for name, v := range srv.Headers {
				if v.BearerFromEnv != "" && name != "Authorization" {
					diags = append(diags, diag.TargetErrorf(t.ID(), path+".headers."+name,
						"grok bearer_token_env_var applies to Authorization only; use a constant value or from_env for other headers"))
				}
			}
		}
	}
	if cfg.Defaults != nil && cfg.Defaults.Model != "" {
		id := modelKey(cfg.Defaults.Model)
		if _, ok := seen[id]; !ok {
			diags = append(diags, diag.TargetErrorf(t.ID(), "defaults.model",
				"grok [models] default must be a [model.*] table key; %q does not resolve to an emitted model", cfg.Defaults.Model))
		}
	}
	return diags
}

func (t Target) Emit(cfg ir.Config) ([]artifact.Artifact, error) {
	doc := grokConfig{
		Model:      map[string]grokModel{},
		MCPServers: map[string]grokMCPServer{},
	}
	for _, p := range cfg.Providers {
		backend := apiBackend[p.Protocol]
		headers := map[string]string{}
		for name, v := range p.Headers {
			if v.Value != "" {
				headers[name] = v.Value
			}
		}
		for _, m := range p.Models {
			gm := grokModel{
				Model:      m.ID,
				BaseURL:    p.BaseURL,
				EnvKey:     p.APIKeyEnv,
				APIBackend: backend,
			}
			if m.Name != "" {
				gm.Name = m.Name
			}
			if m.ContextWindow != nil {
				gm.ContextWindow = m.ContextWindow
			}
			if m.MaxOutputTokens != nil {
				gm.MaxCompletionTokens = m.MaxOutputTokens
			}
			if len(headers) > 0 {
				gm.ExtraHeaders = headers
			}
			doc.Model[m.ID] = gm
		}
	}
	if cfg.Defaults != nil && cfg.Defaults.Model != "" {
		doc.Models = &grokModels{Default: modelKey(cfg.Defaults.Model)}
	}
	for _, s := range cfg.MCP {
		gs := grokMCPServer{}
		if s.Enabled != nil && !*s.Enabled {
			gs.Enabled = boolPtr(false)
		}
		if s.Transport == ir.TransportStdio {
			gs.Command = s.Command[0]
			if len(s.Command) > 1 {
				gs.Args = s.Command[1:]
			}
			if len(s.Env) > 0 {
				env := map[string]string{}
				for name, v := range s.Env {
					env[name] = envLiteral(v)
				}
				gs.Env = env
			}
			if s.CWD != "" {
				gs.CWD = s.CWD
			}
		} else {
			gs.URL = s.URL
			headers := map[string]string{}
			for name, v := range s.Headers {
				if v.BearerFromEnv != "" && name == "Authorization" {
					gs.BearerTokenEnvVar = v.BearerFromEnv
					continue
				}
				headers[name] = envLiteral(v)
			}
			if len(headers) > 0 {
				gs.Headers = headers
			}
		}
		doc.MCPServers[s.ID] = gs
	}

	if len(doc.MCPServers) == 0 {
		doc.MCPServers = nil
	}
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(doc); err != nil {
		return nil, fmt.Errorf("encoding grok config: %w", err)
	}
	return []artifact.Artifact{{
		Target:        t.ID(),
		Name:          "config.toml",
		Format:        "toml",
		SuggestedPath: "~/.grok/config.toml",
		Content:       buf.Bytes(),
	}}, nil
}

// envLiteral renders a literal or documented ${VAR} expansion for grok
// MCP fields. MCP string fields support ${VAR} expansion at load time.
func envLiteral(v ir.HeaderValue) string {
	if v.FromEnv != "" {
		return "${" + v.FromEnv + "}"
	}
	return v.Value
}

func modelKey(providerModel string) string {
	for i := len(providerModel) - 1; i >= 0; i-- {
		if providerModel[i] == '/' {
			return providerModel[i+1:]
		}
	}
	return providerModel
}

type grokConfig struct {
	Models     *grokModels              `toml:"models,omitempty"`
	Model      map[string]grokModel     `toml:"model"`
	MCPServers map[string]grokMCPServer `toml:"mcp_servers,omitempty"`
}

type grokModels struct {
	Default string `toml:"default"`
}

type grokModel struct {
	Model               string            `toml:"model"`
	BaseURL             string            `toml:"base_url"`
	Name                string            `toml:"name,omitempty"`
	EnvKey              string            `toml:"env_key,omitempty"`
	APIBackend          string            `toml:"api_backend"`
	ContextWindow       *int64            `toml:"context_window,omitempty"`
	MaxCompletionTokens *int64            `toml:"max_completion_tokens,omitempty"`
	ExtraHeaders        map[string]string `toml:"extra_headers,omitempty"`
}

type grokMCPServer struct {
	Command           string            `toml:"command,omitempty"`
	Args              []string          `toml:"args,omitempty"`
	Env               map[string]string `toml:"env,omitempty"`
	CWD               string            `toml:"cwd,omitempty"`
	URL               string            `toml:"url,omitempty"`
	Headers           map[string]string `toml:"headers,omitempty"`
	BearerTokenEnvVar string            `toml:"bearer_token_env_var,omitempty"`
	Enabled           *bool             `toml:"enabled,omitempty"`
}

func boolPtr(b bool) *bool { return &b }
