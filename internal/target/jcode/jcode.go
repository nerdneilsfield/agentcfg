// Package jcode emits jcode config.toml and mcp.json fragments.
package jcode

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

// Target is the jcode emitter (github.com/1jehuang/jcode).
type Target struct{}

func (Target) ID() string { return "jcode" }

var providerType = map[ir.Protocol]string{
	ir.ProtocolOpenAICompletions: "openai-compatible",
	ir.ProtocolAnthropicMessages: "anthropic-compatible",
}

func (t Target) Validate(cfg ir.Config) []diag.Diagnostic {
	var diags []diag.Diagnostic
	for i, p := range cfg.Providers {
		path := fmt.Sprintf("providers[%d]", i)
		if p.Protocol == ir.ProtocolOpenAIResponses {
			diags = append(diags, diag.TargetErrorf(t.ID(), path+".protocol",
				"jcode custom profiles use chat completions or Anthropic messages; openai-responses is only available on the built-in openai provider"))
		} else if _, ok := providerType[p.Protocol]; !ok {
			diags = append(diags, diag.TargetErrorf(t.ID(), path+".protocol",
				"jcode provider type must be openai-compatible, anthropic-compatible, or openrouter; got %q", p.Protocol))
		}
		for name, v := range p.Headers {
			if v.FromEnv != "" || v.BearerFromEnv != "" {
				diags = append(diags, diag.TargetErrorf(t.ID(), path+".headers."+name,
					"jcode config.toml is literal-only; only constant header values are representable"))
			}
		}
	}
	for i, s := range cfg.MCP {
		path := fmt.Sprintf("mcp[%d]", i)
		if s.Transport == ir.TransportHTTP {
			diags = append(diags, diag.TargetErrorf(t.ID(), path+".transport",
				"jcode mcp.json loads stdio servers only; http/sse entries are recognized but skipped"))
		}
		if s.CWD != "" {
			diags = append(diags, diag.TargetErrorf(t.ID(), path+".cwd",
				"jcode MCP servers have no cwd field"))
		}
		for name, v := range s.Headers {
			if v.FromEnv != "" || v.BearerFromEnv != "" {
				diags = append(diags, diag.TargetErrorf(t.ID(), path+".headers."+name,
					"jcode http MCP headers are parsed but unused; environment references are not representable"))
			}
		}
	}
	if cfg.Defaults != nil && cfg.Defaults.Model != "" {
		pid, mid, ok := splitRef(cfg.Defaults.Model)
		if !ok {
			diags = append(diags, diag.TargetErrorf(t.ID(), "defaults.model",
				`jcode expects defaults.model as "provider/model"; got %q`, cfg.Defaults.Model))
		} else if !hasModel(cfg, pid, mid) {
			diags = append(diags, diag.TargetErrorf(t.ID(), "defaults.model",
				"jcode default %q does not resolve to an emitted provider model", cfg.Defaults.Model))
		}
	}
	return diags
}

func (t Target) Emit(cfg ir.Config) ([]artifact.Artifact, error) {
	doc := jcodeConfig{
		Providers: map[string]jcodeProvider{},
	}
	defPID, defMID := "", ""
	if cfg.Defaults != nil && cfg.Defaults.Model != "" {
		defPID, defMID, _ = splitRef(cfg.Defaults.Model)
	}
	for _, p := range cfg.Providers {
		jp := jcodeProvider{
			Type:         providerType[p.Protocol],
			BaseURL:      p.BaseURL,
			EnvKey:       p.APIKeyEnv,
			DefaultModel: defaultModelFor(p, defPID, defMID),
		}
		if len(p.Headers) > 0 {
			headers := map[string]string{}
			for name, v := range p.Headers {
				if v.Value != "" {
					headers[name] = v.Value
				}
			}
			if len(headers) > 0 {
				jp.Headers = headers
			}
		}
		for _, m := range p.Models {
			jm := jcodeModel{ID: m.ID}
			if m.ContextWindow != nil {
				jm.ContextWindow = m.ContextWindow
			}
			if m.Reasoning != nil {
				jm.Reasoning = m.Reasoning
			}
			for _, in := range m.Input {
				if in == ir.ModalityImage {
					jm.Input = []string{"image"}
					break
				}
			}
			jp.Models = append(jp.Models, jm)
		}
		doc.Providers[p.ID] = jp
	}
	if defPID != "" {
		doc.Provider = &jcodeDefaultProvider{DefaultProvider: defPID, DefaultModel: defMID}
	}

	var tbuf bytes.Buffer
	if err := toml.NewEncoder(&tbuf).Encode(doc); err != nil {
		return nil, fmt.Errorf("encoding jcode config.toml: %w", err)
	}
	arts := []artifact.Artifact{{
		Target:        t.ID(),
		Name:          "config.toml",
		Format:        "toml",
		SuggestedPath: "~/.jcode/config.toml",
		Content:       tbuf.Bytes(),
	}}

	if len(cfg.MCP) > 0 {
		servers := map[string]jcodeMCPServer{}
		for _, s := range cfg.MCP {
			entry := jcodeMCPServer{
				Command: s.Command[0],
				Enabled: true,
			}
			if s.Enabled != nil {
				entry.Enabled = *s.Enabled
			}
			if len(s.Command) > 1 {
				entry.Args = s.Command[1:]
			}
			if len(s.Env) > 0 {
				env := map[string]string{}
				for name, v := range s.Env {
					env[name] = envLiteral(v)
				}
				entry.Env = env
			}
			if s.TimeoutMS != nil {
				entry.TimeoutSecs = float64(*s.TimeoutMS) / 1000
			}
			servers[s.ID] = entry
		}
		var jbuf bytes.Buffer
		jenc := json.NewEncoder(&jbuf)
		jenc.SetIndent("", "  ")
		if err := jenc.Encode(struct {
			MCPServers map[string]jcodeMCPServer `json:"mcpServers"`
		}{MCPServers: servers}); err != nil {
			return nil, fmt.Errorf("encoding jcode mcp.json: %w", err)
		}
		arts = append(arts, artifact.Artifact{
			Target:        t.ID(),
			Name:          "mcp.json",
			Format:        "json",
			SuggestedPath: "~/.jcode/mcp.json",
			Content:       jbuf.Bytes(),
		})
	}
	return arts, nil
}

// envLiteral renders MCP env values. jcode mcp.json supports ${VAR}
// expansion (Claude Code syntax), so environment references are native.
func envLiteral(v ir.HeaderValue) string {
	if v.FromEnv != "" {
		return "${" + v.FromEnv + "}"
	}
	return v.Value
}

func defaultModelFor(p ir.Provider, defPID, defMID string) string {
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

type jcodeConfig struct {
	Provider  *jcodeDefaultProvider    `toml:"provider,omitempty"`
	Providers map[string]jcodeProvider `toml:"providers"`
}

type jcodeDefaultProvider struct {
	DefaultProvider string `toml:"default_provider"`
	DefaultModel    string `toml:"default_model"`
}

type jcodeProvider struct {
	Type         string            `toml:"type"`
	BaseURL      string            `toml:"base_url"`
	EnvKey       string            `toml:"api_key_env,omitempty"`
	DefaultModel string            `toml:"default_model,omitempty"`
	Headers      map[string]string `toml:"headers,omitempty"`
	Models       []jcodeModel      `toml:"models"`
}

type jcodeModel struct {
	ID            string   `toml:"id"`
	ContextWindow *int64   `toml:"context_window,omitempty"`
	Reasoning     *bool    `toml:"reasoning,omitempty"`
	Input         []string `toml:"input,omitempty"`
}

type jcodeMCPServer struct {
	Command     string            `json:"command"`
	Args        []string          `json:"args,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	TimeoutSecs float64           `json:"timeout_secs,omitempty"`
	Enabled     bool              `json:"enabled"`
}
