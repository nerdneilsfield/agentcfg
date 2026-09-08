// Package codex emits a Codex CLI config.toml fragment.
package codex

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

// Target is the Codex CLI emitter.
type Target struct{}

func (Target) ID() string { return "codex" }

func (t Target) Validate(cfg ir.Config) []diag.Diagnostic {
	var diags []diag.Diagnostic
	for i, p := range cfg.Providers {
		path := fmt.Sprintf("providers[%d]", i)
		if p.Protocol != ir.ProtocolOpenAIResponses {
			diags = append(diags, diag.TargetErrorf(t.ID(), path+".protocol",
				"codex supports wire_api=responses (openai-responses) only; got %q", p.Protocol))
		}
		for name, v := range p.Headers {
			if v.BearerFromEnv != "" {
				diags = append(diags, diag.TargetErrorf(t.ID(), path+".headers."+name,
					"codex model_providers has no bearer field; use env_http_headers with from_env"))
			}
		}
	}
	for i, s := range cfg.MCP {
		path := fmt.Sprintf("mcp[%d]", i)
		if s.TimeoutMS != nil && *s.TimeoutMS%1000 != 0 {
			diags = append(diags, diag.TargetErrorf(t.ID(), path+".timeout_ms",
				"codex startup_timeout_sec is whole seconds; timeout_ms must be divisible by 1000"))
		}
		switch s.Transport {
		case ir.TransportStdio:
			for name, v := range s.Env {
				if v.FromEnv != "" && v.FromEnv != name {
					diags = append(diags, diag.TargetErrorf(t.ID(), path+".env."+name,
						"codex env_vars inherits same-name variables only; renamed refs are rejected"))
				}
			}
		case ir.TransportHTTP:
			for name, v := range s.Headers {
				if v.BearerFromEnv != "" && name != "Authorization" {
					diags = append(diags, diag.TargetErrorf(t.ID(), path+".headers."+name,
						"codex bearer_token_env_var applies to Authorization only; use value or from_env"))
				}
			}
			if s.CWD != "" {
				diags = append(diags, diag.TargetErrorf(t.ID(), path+".cwd",
					"codex cwd applies to stdio servers only"))
			}
		}
	}
	return diags
}

func (t Target) Emit(cfg ir.Config) ([]artifact.Artifact, error) {
	doc := codexConfig{ModelProviders: map[string]codexProvider{}, MCPServers: map[string]codexMCPServer{}}
	for _, p := range cfg.Providers {
		cp := codexProvider{
			Name:               orDefault(p.Name, p.ID),
			BaseURL:            p.BaseURL,
			EnvKey:             p.APIKeyEnv,
			WireAPI:            "responses",
			RequiresOpenAIAuth: false,
		}
		for name, v := range p.Headers {
			if v.FromEnv != "" {
				if cp.EnvHTTPHeaders == nil {
					cp.EnvHTTPHeaders = map[string]string{}
				}
				cp.EnvHTTPHeaders[name] = v.FromEnv
			} else if v.Value != "" {
				if cp.HTTPHeaders == nil {
					cp.HTTPHeaders = map[string]string{}
				}
				cp.HTTPHeaders[name] = v.Value
			}
		}
		doc.ModelProviders[p.ID] = cp
	}
	for _, s := range cfg.MCP {
		m := codexMCPServer{}
		if s.Enabled != nil && !*s.Enabled {
			m.Enabled = boolPtr(false)
		}
		if s.TimeoutMS != nil {
			m.StartupTimeoutSec = int64Ptr(*s.TimeoutMS / 1000)
		}
		if s.Transport == ir.TransportStdio {
			m.Command = s.Command[0]
			m.Args = s.Command[1:]
			m.CWD = s.CWD
			for name, v := range s.Env {
				if v.FromEnv != "" {
					m.EnvVars = append(m.EnvVars, v.FromEnv)
				} else if v.Value != "" {
					if m.Env == nil {
						m.Env = map[string]string{}
					}
					m.Env[name] = v.Value
				}
			}
		} else {
			m.URL = s.URL
			for name, v := range s.Headers {
				switch {
				case v.BearerFromEnv != "":
					m.BearerTokenEnvVar = v.BearerFromEnv
				case v.FromEnv != "":
					if m.EnvHTTPHeaders == nil {
						m.EnvHTTPHeaders = map[string]string{}
					}
					m.EnvHTTPHeaders[name] = v.FromEnv
				case v.Value != "":
					if m.HTTPHeaders == nil {
						m.HTTPHeaders = map[string]string{}
					}
					m.HTTPHeaders[name] = v.Value
				}
			}
		}
		doc.MCPServers[s.ID] = m
	}

	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(doc); err != nil {
		return nil, fmt.Errorf("encoding codex config: %w", err)
	}
	return []artifact.Artifact{{
		Target:        t.ID(),
		Name:          "config.toml",
		Format:        "toml",
		SuggestedPath: "~/.codex/config.toml",
		Content:       buf.Bytes(),
	}}, nil
}

type codexConfig struct {
	ModelProvider  string                    `toml:"model_provider,omitempty"`
	Model          string                    `toml:"model,omitempty"`
	ModelProviders map[string]codexProvider  `toml:"model_providers"`
	MCPServers     map[string]codexMCPServer `toml:"mcp_servers,omitempty"`
}

type codexProvider struct {
	Name               string            `toml:"name"`
	BaseURL            string            `toml:"base_url"`
	EnvKey             string            `toml:"env_key,omitempty"`
	WireAPI            string            `toml:"wire_api"`
	RequiresOpenAIAuth bool              `toml:"requires_openai_auth"`
	HTTPHeaders        map[string]string `toml:"http_headers,omitempty"`
	EnvHTTPHeaders     map[string]string `toml:"env_http_headers,omitempty"`
}

type codexMCPServer struct {
	Command           string            `toml:"command,omitempty"`
	Args              []string          `toml:"args,omitempty"`
	URL               string            `toml:"url,omitempty"`
	Enabled           *bool             `toml:"enabled,omitempty"`
	EnvVars           []string          `toml:"env_vars,omitempty"`
	Env               map[string]string `toml:"env,omitempty"`
	CWD               string            `toml:"cwd,omitempty"`
	BearerTokenEnvVar string            `toml:"bearer_token_env_var,omitempty"`
	HTTPHeaders       map[string]string `toml:"http_headers,omitempty"`
	EnvHTTPHeaders    map[string]string `toml:"env_http_headers,omitempty"`
	StartupTimeoutSec *int64            `toml:"startup_timeout_sec,omitempty"`
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func boolPtr(b bool) *bool { return &b }

func int64Ptr(v int64) *int64 { return &v }
