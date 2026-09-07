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
	}
	for i, s := range cfg.MCP {
		path := fmt.Sprintf("mcp[%d]", i)
		if s.Transport != ir.TransportStdio {
			diags = append(diags, diag.TargetErrorf(t.ID(), path, "codex v1 emitter maps stdio MCP servers only"))
			continue
		}
		for name, v := range s.Env {
			if v.Value != "" && v.FromEnv != "" && v.FromEnv != name {
				diags = append(diags, diag.TargetErrorf(t.ID(), path+".env."+name,
					"codex env_vars inherits same-name variables only; renamed refs are rejected"))
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
		if len(p.Headers) > 0 {
			cp.HTTPHeaders = map[string]string{}
			for name, v := range p.Headers {
				if v.Value != "" {
					cp.HTTPHeaders[name] = v.Value
				}
			}
		}
		doc.ModelProviders[p.ID] = cp
	}
	for _, s := range cfg.MCP {
		m := codexMCPServer{Args: s.Command[1:]}
		m.Command = s.Command[0]
		if s.Enabled != nil && !*s.Enabled {
			m.Enabled = boolPtr(false)
		}
		if len(s.Env) > 0 {
			m.EnvVars = map[string]string{}
			m.Env = map[string]string{}
			for name, v := range s.Env {
				if v.FromEnv != "" {
					m.EnvVars[name] = name
				} else if v.Value != "" {
					m.Env[name] = v.Value
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
}

type codexMCPServer struct {
	Command string            `toml:"command"`
	Args    []string          `toml:"args,omitempty"`
	Enabled *bool             `toml:"enabled,omitempty"`
	EnvVars map[string]string `toml:"env_vars,omitempty"`
	Env     map[string]string `toml:"env,omitempty"`
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func boolPtr(b bool) *bool { return &b }
