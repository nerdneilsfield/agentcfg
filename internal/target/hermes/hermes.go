// Package hermes emits a Hermes Agent config.yaml fragment.
package hermes

import (
	"bytes"
	"fmt"

	yaml "go.yaml.in/yaml/v3"

	"agentcfg/internal/artifact"
	"agentcfg/internal/diag"
	"agentcfg/internal/ir"
	"agentcfg/internal/target"
)

func init() { target.Register(Target{}) }

// Target is the Hermes Agent emitter (github.com/NousResearch/hermes-agent).
type Target struct{}

func (Target) ID() string { return "hermes" }

var apiMode = map[ir.Protocol]string{
	ir.ProtocolOpenAICompletions: "chat_completions",
	ir.ProtocolOpenAIResponses:   "codex_responses",
	ir.ProtocolAnthropicMessages: "anthropic_messages",
}

func (t Target) Validate(cfg ir.Config) []diag.Diagnostic {
	var diags []diag.Diagnostic
	for i, p := range cfg.Providers {
		path := fmt.Sprintf("providers[%d]", i)
		if _, ok := apiMode[p.Protocol]; !ok {
			diags = append(diags, diag.TargetErrorf(t.ID(), path+".protocol",
				"hermes api_mode must be chat_completions, codex_responses, or anthropic_messages; got %q", p.Protocol))
		}
	}
	if cfg.Defaults != nil && cfg.Defaults.Model != "" {
		pid, mid, ok := splitRef(cfg.Defaults.Model)
		if !ok {
			diags = append(diags, diag.TargetErrorf(t.ID(), "defaults.model",
				`hermes expects defaults.model as "provider/model"; got %q`, cfg.Defaults.Model))
		} else if !hasModel(cfg, pid, mid) {
			diags = append(diags, diag.TargetErrorf(t.ID(), "defaults.model",
				"hermes default %q does not resolve to an emitted provider model", cfg.Defaults.Model))
		}
	}
	return diags
}

func (t Target) Emit(cfg ir.Config) ([]artifact.Artifact, error) {
	doc := hermesConfig{
		Providers:  map[string]hermesProvider{},
		MCPServers: map[string]hermesMCPServer{},
	}
	defPID, defMID := "", ""
	if cfg.Defaults != nil && cfg.Defaults.Model != "" {
		defPID, defMID, _ = splitRef(cfg.Defaults.Model)
	}
	for _, p := range cfg.Providers {
		hp := hermesProvider{
			APIKeyEnv: p.APIKeyEnv,
			APIMode:   apiMode[p.Protocol],
		}
		if p.BaseURL != "" {
			hp.BaseURL = p.BaseURL
		}
		if len(p.Headers) > 0 {
			headers := map[string]string{}
			for name, v := range p.Headers {
				headers[name] = envInterp(v)
			}
			hp.ExtraHeaders = headers
		}
		if len(p.Models) > 0 {
			models := map[string]hermesModel{}
			for _, m := range p.Models {
				hm := hermesModel{}
				if m.ContextWindow != nil {
					hm.ContextLength = m.ContextWindow
				}
				models[m.ID] = hm
			}
			hp.Models = models
		}
		doc.Providers[p.ID] = hp
	}
	if defPID != "" {
		doc.Model = &hermesModelSel{Provider: defPID, Default: defMID}
	}
	for _, s := range cfg.MCP {
		hs := hermesMCPServer{Enabled: true}
		if s.Enabled != nil {
			hs.Enabled = *s.Enabled
		}
		if s.Transport == ir.TransportStdio {
			hs.Command = s.Command[0]
			if len(s.Command) > 1 {
				hs.Args = s.Command[1:]
			}
			if len(s.Env) > 0 {
				env := map[string]string{}
				for name, v := range s.Env {
					env[name] = envInterp(v)
				}
				hs.Env = env
			}
			if s.CWD != "" {
				hs.CWD = s.CWD
			}
		} else {
			hs.URL = s.URL
			if len(s.Headers) > 0 {
				headers := map[string]string{}
				for name, v := range s.Headers {
					headers[name] = envInterp(v)
				}
				hs.Headers = headers
			}
		}
		if s.TimeoutMS != nil {
			hs.Timeout = float64(*s.TimeoutMS) / 1000
		}
		doc.MCPServers[s.ID] = hs
	}
	if len(doc.MCPServers) == 0 {
		doc.MCPServers = nil
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return nil, fmt.Errorf("encoding hermes config.yaml: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("encoding hermes config.yaml: %w", err)
	}
	return []artifact.Artifact{{
		Target:        t.ID(),
		Name:          "config.yaml",
		Format:        "yaml",
		SuggestedPath: "~/.hermes/config.yaml",
		Content:       buf.Bytes(),
	}}, nil
}

// envInterp renders hermes values. Hermes expands ${VAR} and ${env:VAR}
// recursively over all config strings at load, so every IR env reference
// form is native.
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

type hermesConfig struct {
	Model      *hermesModelSel            `yaml:"model,omitempty"`
	Providers  map[string]hermesProvider  `yaml:"providers"`
	MCPServers map[string]hermesMCPServer `yaml:"mcp_servers,omitempty"`
}

type hermesModelSel struct {
	Provider string `yaml:"provider"`
	Default  string `yaml:"default"`
}

type hermesProvider struct {
	BaseURL      string                 `yaml:"base_url,omitempty"`
	APIKeyEnv    string                 `yaml:"api_key_env,omitempty"`
	APIMode      string                 `yaml:"api_mode"`
	ExtraHeaders map[string]string      `yaml:"extra_headers,omitempty"`
	Models       map[string]hermesModel `yaml:"models,omitempty"`
}

type hermesModel struct {
	ContextLength *int64 `yaml:"context_length,omitempty"`
}

type hermesMCPServer struct {
	Command string            `yaml:"command,omitempty"`
	Args    []string          `yaml:"args,omitempty"`
	Env     map[string]string `yaml:"env,omitempty"`
	CWD     string            `yaml:"cwd,omitempty"`
	URL     string            `yaml:"url,omitempty"`
	Headers map[string]string `yaml:"headers,omitempty"`
	Timeout float64           `yaml:"timeout,omitempty"`
	Enabled bool              `yaml:"enabled"`
}
