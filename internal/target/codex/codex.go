// Package codex emits a Codex CLI config.toml fragment.
package codex

import (
	"bytes"
	"fmt"
	"sort"

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
			diags = append(diags, diag.TargetWarnf(t.ID(), path+".protocol",
				"codex supports wire_api=responses (openai-responses) only; got %q", p.Protocol))
		}
		for name, v := range p.Headers {
			if v.BearerFromEnv != "" {
				diags = append(diags, diag.TargetWarnf(t.ID(), path+".headers."+name,
					"codex model_providers has no bearer field; use ENV:NAME for env_http_headers"))
			}
		}
	}
	for i, s := range cfg.MCP {
		path := fmt.Sprintf("mcp[%d]", i)
		switch s.Transport {
		case ir.TransportStdio:
			if len(s.Command) == 0 {
				diags = append(diags, diag.TargetWarnf(t.ID(), path+".command",
					"codex stdio MCP server has no command; the entry is skipped"))
			}
			for name, v := range s.Env {
				if v.FromEnv != "" && v.FromEnv != name {
					diags = append(diags, diag.TargetWarnf(t.ID(), path+".env."+name,
						"codex env_vars inherits same-name variables only; renamed refs are rejected"))
				}
			}
		case ir.TransportHTTP:
			for name, v := range s.Headers {
				if v.BearerFromEnv != "" && name != "Authorization" {
					diags = append(diags, diag.TargetWarnf(t.ID(), path+".headers."+name,
						"codex bearer_token_env_var applies to Authorization only; use a literal or ENV:NAME"))
				}
			}
			if s.CWD != "" {
				diags = append(diags, diag.TargetWarnf(t.ID(), path+".cwd",
					"codex cwd applies to stdio servers only"))
			}
		}
	}
	if cfg.Defaults != nil && cfg.Defaults.Model != "" {
		provider, _ := splitRef(cfg.Defaults.Model)
		emitted := false
		for _, p := range cfg.Providers {
			if p.ID == provider && p.Protocol == ir.ProtocolOpenAIResponses {
				emitted = true
			}
		}
		if !emitted {
			diags = append(diags, diag.TargetWarnf(t.ID(), "defaults.model",
				"codex default %q names a provider that is not emitted; model_provider and model are skipped", cfg.Defaults.Model))
		}
	}
	return diags
}

func (t Target) Emit(cfg ir.Config) ([]artifact.Artifact, error) {
	doc := codexConfig{ModelProviders: map[string]codexProvider{}, MCPServers: map[string]codexMCPServer{}}
	emittedProviders := map[string]bool{}
	for _, p := range cfg.Providers {
		if p.Protocol != ir.ProtocolOpenAIResponses {
			// Validate reports the unsupported protocol; codex always speaks
			// wire_api=responses, so the provider cannot be represented.
			continue
		}
		cp := codexProvider{
			Name:               orDefault(p.Name, p.ID),
			BaseURL:            p.BaseURL,
			EnvKey:             p.APIKey.FromEnv,
			BearerToken:        p.APIKey.Value,
			WireAPI:            "responses",
			RequiresOpenAIAuth: false,
		}
		for name, v := range p.Headers {
			if v.FromEnv != "" {
				if cp.EnvHTTPHeaders == nil {
					cp.EnvHTTPHeaders = map[string]string{}
				}
				cp.EnvHTTPHeaders[name] = v.FromEnv
			} else if v.BearerFromEnv == "" {
				if cp.HTTPHeaders == nil {
					cp.HTTPHeaders = map[string]string{}
				}
				cp.HTTPHeaders[name] = v.Value
			}
		}
		doc.ModelProviders[p.ID] = cp
		emittedProviders[p.ID] = true
	}
	if cfg.Defaults != nil && cfg.Defaults.Model != "" {
		if provider, model := splitRef(cfg.Defaults.Model); emittedProviders[provider] {
			doc.ModelProvider, doc.Model = provider, model
		}
	}
	for _, s := range cfg.MCP {
		if s.Transport == ir.TransportStdio && len(s.Command) == 0 {
			continue
		}
		m := codexMCPServer{}
		if s.Enabled != nil && !*s.Enabled {
			m.Enabled = boolPtr(false)
		}
		if s.TimeoutMS != nil {
			m.StartupTimeoutMS = s.TimeoutMS
		}
		if s.Transport == ir.TransportStdio {
			m.Command = s.Command[0]
			m.Args = s.Command[1:]
			m.CWD = s.CWD
			for name, v := range s.Env {
				if v.FromEnv != "" {
					if v.FromEnv == name {
						m.EnvVars = append(m.EnvVars, v.FromEnv)
					}
					continue
				}
				if m.Env == nil {
					m.Env = map[string]string{}
				}
				m.Env[name] = v.Value
			}
			sort.Strings(m.EnvVars)
		} else {
			m.URL = s.URL
			for name, v := range s.Headers {
				switch {
				case v.BearerFromEnv != "" && name == "Authorization":
					m.BearerTokenEnvVar = v.BearerFromEnv
				case v.BearerFromEnv != "":
					// bearer_token_env_var applies to Authorization only; Validate reports it.
				case v.FromEnv != "":
					if m.EnvHTTPHeaders == nil {
						m.EnvHTTPHeaders = map[string]string{}
					}
					m.EnvHTTPHeaders[name] = v.FromEnv
				default:
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
	BearerToken        string            `toml:"experimental_bearer_token,omitempty"`
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
	StartupTimeoutMS  *int64            `toml:"startup_timeout_ms,omitempty"`
}

func splitRef(providerModel string) (string, string) {
	for i := 0; i < len(providerModel); i++ {
		if providerModel[i] == '/' {
			return providerModel[:i], providerModel[i+1:]
		}
	}
	return "", providerModel
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func boolPtr(b bool) *bool { return &b }
