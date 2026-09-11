// Package goose emits Block Goose custom-provider JSON and a config.yaml
// fragment covering the active provider/model plus MCP extensions.
package goose

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"

	yaml "go.yaml.in/yaml/v3"

	"agentcfg/internal/artifact"
	"agentcfg/internal/diag"
	"agentcfg/internal/ir"
	"agentcfg/internal/target"
)

func init() { target.Register(Target{}) }

// Target is the Block Goose emitter (github.com/block/goose).
type Target struct{}

func (Target) ID() string { return "goose" }

// responsesBasePath forces the OpenAI Responses API for a custom provider.
// An explicit base_path replaces the base_url path entirely, mirroring
// from_declarative_config in crates/goose-providers/src/openai.rs.
const responsesBasePath = "v1/responses"

var engineName = map[ir.Protocol]string{
	ir.ProtocolOpenAICompletions: "openai",
	ir.ProtocolOpenAIResponses:   "openai",
	ir.ProtocolAnthropicMessages: "anthropic",
}

func (t Target) Validate(cfg ir.Config) []diag.Diagnostic {
	var diags []diag.Diagnostic
	for i, p := range cfg.Providers {
		path := fmt.Sprintf("providers[%d]", i)
		if p.APIKey.Value != "" {
			diags = append(diags, diag.TargetErrorf(t.ID(), path+".api_key", "goose does not support literal API keys in the verified native configuration"))
		}
		if _, ok := engineName[p.Protocol]; !ok {
			diags = append(diags, diag.TargetErrorf(t.ID(), path+".protocol",
				"goose custom provider engine must be openai or anthropic; got %q", p.Protocol))
		}
		for name, v := range p.Headers {
			if v.FromEnv != "" || v.BearerFromEnv != "" {
				diags = append(diags, diag.TargetErrorf(t.ID(), path+".headers."+name,
					"goose custom-provider headers are literal strings with no interpolation"))
			}
		}
		for j, m := range p.Models {
			mpath := fmt.Sprintf("%s.models[%d]", path, j)
			if m.MaxOutputTokens != nil {
				diags = append(diags, diag.TargetErrorf(t.ID(), mpath+".max_output_tokens",
					"goose ModelInfo has no per-model max-tokens field (global GOOSE_MAX_TOKENS only); per-model max_output_tokens is not representable"))
			}
			for _, mod := range m.Input {
				if mod != ir.ModalityText {
					diags = append(diags, diag.TargetErrorf(t.ID(), mpath+".input",
						"goose ModelInfo has no modality fields; %q is not representable", mod))
				}
			}
			for _, mod := range m.Output {
				if mod != ir.ModalityText {
					diags = append(diags, diag.TargetErrorf(t.ID(), mpath+".output",
						"goose ModelInfo has no modality fields; %q is not representable", mod))
				}
			}
			if m.ToolCalling != nil && !*m.ToolCalling {
				diags = append(diags, diag.TargetErrorf(t.ID(), mpath+".tool_calling",
					"goose agents always expose tools; tool_calling: false is not representable"))
			}
		}
	}
	for i, s := range cfg.MCP {
		path := fmt.Sprintf("mcp[%d]", i)
		if s.TimeoutMS != nil && *s.TimeoutMS%1000 != 0 {
			diags = append(diags, diag.TargetErrorf(t.ID(), path+".timeout_ms",
				"goose extension timeout is whole seconds; %d ms is not divisible by 1000", *s.TimeoutMS))
		}
		for name, v := range s.Env {
			if v.BearerFromEnv != "" {
				diags = append(diags, diag.TargetErrorf(t.ID(), path+".env."+name,
					"goose extension env values are plain environment names or literals; Bearer ENV:NAME is not representable on env"))
			}
		}
	}
	if cfg.Defaults != nil && cfg.Defaults.Model != "" {
		pid, mid, ok := splitRef(cfg.Defaults.Model)
		if !ok {
			diags = append(diags, diag.TargetErrorf(t.ID(), "defaults.model",
				`goose expects defaults.model as "provider/model"; got %q`, cfg.Defaults.Model))
		} else if !hasModel(cfg, pid, mid) {
			diags = append(diags, diag.TargetErrorf(t.ID(), "defaults.model",
				"goose default %q does not resolve to an emitted provider model", cfg.Defaults.Model))
		}
	}
	return diags
}

func (t Target) Emit(cfg ir.Config) ([]artifact.Artifact, error) {
	arts := []artifact.Artifact{}
	for _, p := range cfg.Providers {
		gp := gooseProvider{
			Name:              p.ID,
			Engine:            engineName[p.Protocol],
			DisplayName:       orDefault(p.Name, p.ID),
			APIKeyEnv:         p.APIKey.FromEnv,
			BaseURL:           p.BaseURL,
			SupportsStreaming: true,
			RequiresAuth:      p.APIKey.FromEnv != "",
			DynamicModels:     false,
		}
		if p.Protocol == ir.ProtocolOpenAIResponses {
			gp.BasePath = responsesBasePath
		}
		if len(p.Headers) > 0 {
			headers := map[string]string{}
			for name, v := range p.Headers {
				headers[name] = v.Value
			}
			if len(headers) > 0 {
				gp.Headers = headers
			}
		}
		for _, m := range p.Models {
			gm := gooseModel{Name: m.ID}
			if m.ContextWindow != nil {
				gm.ContextLimit = m.ContextWindow
			}
			if m.Reasoning != nil {
				gm.Reasoning = *m.Reasoning
			}
			gp.Models = append(gp.Models, gm)
		}
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetIndent("", "  ")
		if err := enc.Encode(gp); err != nil {
			return nil, fmt.Errorf("encoding goose custom provider %s: %w", p.ID, err)
		}
		arts = append(arts, artifact.Artifact{
			Target:        t.ID(),
			Name:          p.ID + ".json",
			Format:        "json",
			SuggestedPath: "~/.config/goose/custom_providers/" + p.ID + ".json",
			Content:       buf.Bytes(),
		})
	}

	cfgDoc := gooseConfig{Extensions: map[string]gooseExt{}}
	if cfg.Defaults != nil && cfg.Defaults.Model != "" {
		pid, mid, _ := splitRef(cfg.Defaults.Model)
		cfgDoc.ActiveProvider = pid
		cfgDoc.Providers = map[string]gooseProviderEntry{
			pid: {Enabled: true, Model: mid, Configured: true},
		}
	}
	for _, s := range cfg.MCP {
		ext := gooseExt{
			Name:    s.ID,
			Enabled: true,
		}
		if s.Enabled != nil {
			ext.Enabled = *s.Enabled
		}
		if s.TimeoutMS != nil {
			ext.Timeout = int64(*s.TimeoutMS / 1000)
		}
		if s.Transport == ir.TransportStdio {
			ext.Type = "stdio"
			ext.Cmd = s.Command[0]
			if len(s.Command) > 1 {
				ext.Args = s.Command[1:]
			}
			if s.CWD != "" {
				ext.CWD = s.CWD
			}
			if len(s.Env) > 0 {
				env := map[string]string{}
				keys := []string{}
				for name, v := range s.Env {
					if v.FromEnv != "" {
						keys = append(keys, v.FromEnv)
					} else {
						env[name] = v.Value
					}
				}
				sort.Strings(keys)
				if len(env) > 0 {
					ext.Envs = env
				}
				if len(keys) > 0 {
					ext.EnvKeys = keys
				}
			}
		} else {
			ext.Type = "streamable_http"
			ext.URI = s.URL
			if len(s.Headers) > 0 {
				headers := map[string]string{}
				keys := []string{}
				for name, v := range s.Headers {
					headers[name] = envInterp(v)
					if v.FromEnv != "" {
						keys = append(keys, v.FromEnv)
					} else if v.BearerFromEnv != "" {
						keys = append(keys, v.BearerFromEnv)
					}
				}
				ext.Headers = headers
				if len(keys) > 0 {
					sort.Strings(keys)
					ext.EnvKeys = keys
				}
			}
		}
		cfgDoc.Extensions[s.ID] = ext
	}
	if len(cfgDoc.Extensions) == 0 {
		cfgDoc.Extensions = nil
	}
	if cfgDoc.ActiveProvider != "" || cfgDoc.Extensions != nil {
		var ybuf bytes.Buffer
		yenc := yaml.NewEncoder(&ybuf)
		yenc.SetIndent(2)
		if err := yenc.Encode(cfgDoc); err != nil {
			return nil, fmt.Errorf("encoding goose config.yaml: %w", err)
		}
		if err := yenc.Close(); err != nil {
			return nil, fmt.Errorf("encoding goose config.yaml: %w", err)
		}
		arts = append(arts, artifact.Artifact{
			Target:        t.ID(),
			Name:          "config.yaml",
			Format:        "yaml",
			SuggestedPath: "~/.config/goose/config.yaml",
			Content:       ybuf.Bytes(),
		})
	}
	return arts, nil
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

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

type gooseProvider struct {
	Name              string            `json:"name"`
	Engine            string            `json:"engine"`
	DisplayName       string            `json:"display_name"`
	APIKeyEnv         string            `json:"api_key_env,omitempty"`
	BaseURL           string            `json:"base_url"`
	BasePath          string            `json:"base_path,omitempty"`
	Models            []gooseModel      `json:"models"`
	Headers           map[string]string `json:"headers,omitempty"`
	SupportsStreaming bool              `json:"supports_streaming"`
	RequiresAuth      bool              `json:"requires_auth"`
	DynamicModels     bool              `json:"dynamic_models"`
}

type gooseModel struct {
	Name         string `json:"name"`
	ContextLimit *int64 `json:"context_limit,omitempty"`
	Reasoning    bool   `json:"reasoning,omitempty"`
}

type gooseConfig struct {
	ActiveProvider string                        `yaml:"active_provider,omitempty"`
	Providers      map[string]gooseProviderEntry `yaml:"providers,omitempty"`
	Extensions     map[string]gooseExt           `yaml:"extensions,omitempty"`
}

type gooseProviderEntry struct {
	Enabled    bool   `yaml:"enabled"`
	Model      string `yaml:"model"`
	Configured bool   `yaml:"configured"`
}

type gooseExt struct {
	Type    string            `yaml:"type"`
	Name    string            `yaml:"name"`
	Enabled bool              `yaml:"enabled"`
	Cmd     string            `yaml:"cmd,omitempty"`
	Args    []string          `yaml:"args,omitempty"`
	Envs    map[string]string `yaml:"envs,omitempty"`
	EnvKeys []string          `yaml:"env_keys,omitempty"`
	CWD     string            `yaml:"cwd,omitempty"`
	URI     string            `yaml:"uri,omitempty"`
	Headers map[string]string `yaml:"headers,omitempty"`
	Timeout int64             `yaml:"timeout,omitempty"`
}
