// Package gajae emits Gajae Code (gjc) models.yml, mcp.json, and a
// config.yml modelRoles fragment.
package gajae

import (
	"bytes"
	"encoding/json"
	"fmt"

	yaml "go.yaml.in/yaml/v3"

	"agentcfg/internal/artifact"
	"agentcfg/internal/diag"
	"agentcfg/internal/ir"
	"agentcfg/internal/target"
)

func init() { target.Register(Target{}) }

// Target is the Gajae Code emitter (github.com/Yeachan-Heo/gajae-code).
type Target struct{}

func (Target) ID() string { return "gajae" }

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
				"gajae api must be openai-completions, openai-responses, or anthropic-messages for custom providers; got %q", p.Protocol))
		}
		for name, v := range p.Headers {
			if v.BearerFromEnv != "" {
				diags = append(diags, diag.TargetErrorf(t.ID(), path+".headers."+name,
					"gajae provider headers are flat strings with no bearer-from-env convention; use ENV:NAME or a constant value"))
			}
		}
		for j, m := range p.Models {
			mpath := fmt.Sprintf("%s.models[%d]", path, j)
			for _, mod := range append(append([]ir.Modality{}, m.Input...), m.Output...) {
				if mod != ir.ModalityText && mod != ir.ModalityImage {
					diags = append(diags, diag.TargetErrorf(t.ID(), mpath,
						"gajae modalities are limited to text and image; %q is not representable", mod))
				}
			}
		}
	}
	return diags
}

func (t Target) Emit(cfg ir.Config) ([]artifact.Artifact, error) {
	// Artifact 1: models.yml
	providers := map[string]gajaeProvider{}
	for _, p := range cfg.Providers {
		gp := gajaeProvider{
			BaseURL:   p.BaseURL,
			API:       apiName[p.Protocol],
			APIKeyEnv: p.APIKey.FromEnv,
			APIKey:    p.APIKey.Value,
		}
		if len(p.Headers) > 0 {
			headers := map[string]string{}
			for name, v := range p.Headers {
				headers[name] = headerValue(v)
			}
			gp.Headers = headers
		}
		for _, m := range p.Models {
			gm := gajaeModel{ID: m.ID}
			if m.Name != "" {
				gm.Name = m.Name
			}
			if m.ContextWindow != nil {
				gm.ContextWindow = m.ContextWindow
			}
			if m.MaxOutputTokens != nil {
				gm.MaxTokens = m.MaxOutputTokens
			}
			if len(m.Input) > 0 {
				in := make([]string, 0, len(m.Input))
				for _, mod := range m.Input {
					in = append(in, string(mod))
				}
				gm.Input = in
			}
			if len(m.Output) > 0 {
				out := make([]string, 0, len(m.Output))
				for _, mod := range m.Output {
					out = append(out, string(mod))
				}
				gm.Output = out
			}
			if m.Reasoning != nil {
				gm.Reasoning = m.Reasoning
			}
			if efforts := gajaeSupportedEfforts(m.Variants); len(efforts) > 0 {
				gm.Thinking = gajaeThinkingFor(efforts)
				gm.Compat = map[string]bool{"supportsReasoningEffort": true}
			}
			gp.Models = append(gp.Models, gm)
		}
		providers[p.ID] = gp
	}
	var ybuf bytes.Buffer
	yenc := yaml.NewEncoder(&ybuf)
	yenc.SetIndent(2)
	if err := yenc.Encode(struct {
		Providers map[string]gajaeProvider `yaml:"providers"`
	}{Providers: providers}); err != nil {
		return nil, fmt.Errorf("encoding gajae models.yml: %w", err)
	}
	if err := yenc.Close(); err != nil {
		return nil, fmt.Errorf("encoding gajae models.yml: %w", err)
	}
	arts := []artifact.Artifact{{
		Target:        t.ID(),
		Name:          "models.yml",
		Format:        "yaml",
		SuggestedPath: "~/.gjc/agent/models.yml",
		Content:       ybuf.Bytes(),
	}}

	// Artifact 2: config.yml modelRoles fragment (defaults)
	if cfg.Defaults != nil && cfg.Defaults.Model != "" {
		var cbuf bytes.Buffer
		cenc := yaml.NewEncoder(&cbuf)
		cenc.SetIndent(2)
		if err := cenc.Encode(struct {
			ModelRoles map[string]string `yaml:"modelRoles"`
		}{ModelRoles: map[string]string{"default": cfg.Defaults.Model}}); err != nil {
			return nil, fmt.Errorf("encoding gajae config.yml: %w", err)
		}
		if err := cenc.Close(); err != nil {
			return nil, fmt.Errorf("encoding gajae config.yml: %w", err)
		}
		arts = append(arts, artifact.Artifact{
			Target:        t.ID(),
			Name:          "config.yml",
			Format:        "yaml",
			SuggestedPath: "~/.gjc/agent/config.yml",
			Content:       cbuf.Bytes(),
		})
	}

	// Artifact 3: mcp.json
	if len(cfg.MCP) > 0 {
		servers := map[string]gajaeMCPServer{}
		for _, s := range cfg.MCP {
			gs := gajaeMCPServer{Enabled: true}
			if s.Enabled != nil {
				gs.Enabled = *s.Enabled
			}
			if s.TimeoutMS != nil {
				gs.Timeout = s.TimeoutMS
			}
			if s.Transport == ir.TransportStdio {
				gs.Type = "stdio"
				gs.Command = s.Command[0]
				if len(s.Command) > 1 {
					gs.Args = s.Command[1:]
				}
				if len(s.Env) > 0 {
					env := map[string]string{}
					for name, v := range s.Env {
						env[name] = envValue(v)
					}
					gs.Env = env
				}
				if s.CWD != "" {
					gs.CWD = s.CWD
				}
			} else {
				gs.Type = "http"
				gs.URL = s.URL
				if len(s.Headers) > 0 {
					headers := map[string]string{}
					for name, v := range s.Headers {
						headers[name] = envValue(v)
					}
					gs.Headers = headers
				}
			}
			servers[s.ID] = gs
		}
		var jbuf bytes.Buffer
		jenc := json.NewEncoder(&jbuf)
		jenc.SetIndent("", "  ")
		if err := jenc.Encode(struct {
			MCPServers map[string]gajaeMCPServer `json:"mcpServers"`
		}{MCPServers: servers}); err != nil {
			return nil, fmt.Errorf("encoding gajae mcp.json: %w", err)
		}
		arts = append(arts, artifact.Artifact{
			Target:        t.ID(),
			Name:          "mcp.json",
			Format:        "json",
			SuggestedPath: "~/.gjc/agent/mcp.json",
			Content:       jbuf.Bytes(),
		})
	}
	return arts, nil
}

// headerValue renders provider header values for models.yml. Gajae
// resolves values with env-name-or-literal semantics (a value matching a
// set environment variable is replaced by it), so an IR ENV:NAME maps to a
// bare environment variable name.
func headerValue(v ir.HeaderValue) string {
	if v.FromEnv != "" {
		return v.FromEnv
	}
	return v.Value
}

// envValue renders MCP values. Gajae mcp.json applies ${VAR} expansion at
// discovery, so environment references are native.
func envValue(v ir.HeaderValue) string {
	if v.BearerFromEnv != "" {
		return "Bearer ${" + v.BearerFromEnv + "}"
	}
	if v.FromEnv != "" {
		return "${" + v.FromEnv + "}"
	}
	return v.Value
}

type gajaeProvider struct {
	APIKey    string            `yaml:"apiKey,omitempty"`
	BaseURL   string            `yaml:"baseUrl"`
	API       string            `yaml:"api,omitempty"`
	APIKeyEnv string            `yaml:"apiKeyEnv,omitempty"`
	Headers   map[string]string `yaml:"headers,omitempty"`
	Models    []gajaeModel      `yaml:"models,omitempty"`
}

type gajaeModel struct {
	ID            string          `yaml:"id"`
	Name          string          `yaml:"name,omitempty"`
	ContextWindow *int64          `yaml:"contextWindow,omitempty"`
	MaxTokens     *int64          `yaml:"maxTokens,omitempty"`
	Input         []string        `yaml:"input,omitempty"`
	Output        []string        `yaml:"output,omitempty"`
	Reasoning     *bool           `yaml:"reasoning,omitempty"`
	Thinking      *gajaeThinking  `yaml:"thinking,omitempty"`
	Compat        map[string]bool `yaml:"compat,omitempty"`
}

type gajaeThinking struct {
	Mode     string   `yaml:"mode"`
	MinLevel string   `yaml:"minLevel"`
	MaxLevel string   `yaml:"maxLevel"`
	Levels   []string `yaml:"levels"`
}

type gajaeMCPServer struct {
	Type    string            `json:"type"`
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	CWD     string            `json:"cwd,omitempty"`
	URL     string            `json:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Timeout *int64            `json:"timeout,omitempty"`
	Enabled bool              `json:"enabled"`
}

func effortStrings(efforts []ir.ReasoningEffort) []string {
	out := make([]string, len(efforts))
	for i, effort := range efforts {
		out[i] = string(effort)
	}
	return out
}

var gajaeEfforts = []ir.ReasoningEffort{"minimal", "low", "medium", "high", "xhigh", "max"}

func gajaeKnownEffort(e ir.ReasoningEffort) bool { return gajaeEffortIndex(e) >= 0 }
func gajaeEffortIndex(e ir.ReasoningEffort) int {
	for i, v := range gajaeEfforts {
		if v == e {
			return i
		}
	}
	return -1
}

func gajaeSupportedEfforts(efforts []ir.ReasoningEffort) []ir.ReasoningEffort {
	out := []ir.ReasoningEffort{}
	for _, e := range efforts {
		if gajaeKnownEffort(e) {
			out = append(out, e)
		}
	}
	return out
}

func gajaeThinkingFor(efforts []ir.ReasoningEffort) *gajaeThinking {
	min, max := efforts[0], efforts[0]
	for _, e := range efforts {
		if gajaeEffortIndex(e) < gajaeEffortIndex(min) {
			min = e
		}
		if gajaeEffortIndex(e) > gajaeEffortIndex(max) {
			max = e
		}
	}
	return &gajaeThinking{Mode: "effort", MinLevel: string(min), MaxLevel: string(max), Levels: effortStrings(efforts)}
}
