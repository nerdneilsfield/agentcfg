// Package primeagent emits Prime Agent models.json and settings.json.
package primeagent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"agentcfg/internal/artifact"
	"agentcfg/internal/diag"
	"agentcfg/internal/ir"
	"agentcfg/internal/target"
)

func init() { target.Register(Target{}) }

// Target is the Prime Agent emitter.
type Target struct{}

func (Target) ID() string { return "prime-agent" }

func (t Target) Validate(cfg ir.Config) []diag.Diagnostic {
	var diags []diag.Diagnostic
	for i, p := range cfg.Providers {
		path := fmt.Sprintf("providers[%d]", i)
		if strings.HasPrefix(p.APIKey.Value, "$") || strings.HasPrefix(p.APIKey.Value, "!") {
			diags = append(diags, diag.TargetErrorf(t.ID(), path+".api_key", "literal API key contains native expression syntax and cannot be represented literally"))
		}
		for name, v := range p.Headers {
			if v.BearerFromEnv != "" {
				diags = append(diags, diag.TargetErrorf(t.ID(), path+".headers."+name,
					"prime-agent provider headers are static strings or environment names; Bearer ENV:NAME is not representable"))
			}
		}
	}
	for i, p := range cfg.Providers {
		for j, m := range p.Models {
			for _, mod := range m.Input {
				if mod != ir.ModalityText && mod != ir.ModalityImage {
					diags = append(diags, diag.TargetErrorf(t.ID(), fmt.Sprintf("providers[%d].models[%d].input", i, j), "prime-agent model input supports text and image only"))
				}
			}
		}
	}
	for i, s := range cfg.MCP {
		path := fmt.Sprintf("mcp[%d]", i)
		if s.TimeoutMS != nil {
			diags = append(diags, diag.TargetErrorf(t.ID(), path+".timeout_ms",
				"prime-agent v1 does not map MCP timeout_ms; the native start/call timeout policy is not finalized"))
		}
		if s.Transport == ir.TransportStdio {
			for name, v := range s.Env {
				if v.FromEnv == "" {
					diags = append(diags, diag.TargetErrorf(t.ID(), path+".env."+name,
						"prime-agent stdio env entries are environment references only (prime-agent mcp add --env CHILD=SOURCE)"))
				}
			}
			continue
		}
		for name, v := range s.Headers {
			if v.BearerFromEnv != "" && name != "Authorization" {
				diags = append(diags, diag.TargetErrorf(t.ID(), path+".headers."+name,
					"prime-agent bearerTokenEnvVar applies to Authorization only; use a constant value for other headers"))
			}
			if v.FromEnv != "" {
				diags = append(diags, diag.TargetErrorf(t.ID(), path+".headers."+name,
					"prime-agent http headers are static strings; environment interpolation is not verified for MCP headers"))
			}
		}
	}
	return diags
}

func (t Target) Emit(cfg ir.Config) ([]artifact.Artifact, error) {
	models := map[string]any{
		"providers": map[string]any{},
	}
	providers := models["providers"].(map[string]any)
	for _, p := range cfg.Providers {
		ms := []any{}
		for _, m := range p.Models {
			mm := map[string]any{
				"id":    m.ID,
				"name":  orDefault(m.Name, m.ID),
				"input": modalities(m.Input),
				"cost":  map[string]any{"input": 0, "output": 0, "cacheRead": 0, "cacheWrite": 0},
			}
			mm["reasoning"] = m.Reasoning != nil && *m.Reasoning
			if hasPrimeThinkingLevels(m.Variants) {
				mm["thinkingLevelMap"] = primeThinkingLevelMap(m.Variants)
			}
			if m.ContextWindow != nil {
				mm["contextWindow"] = *m.ContextWindow
			}
			if m.MaxOutputTokens != nil {
				mm["maxTokens"] = *m.MaxOutputTokens
			}
			ms = append(ms, mm)
		}
		prov := map[string]any{
			"baseUrl": p.BaseURL,
			"api":     string(p.Protocol),
			"models":  ms,
		}
		if p.APIKey.FromEnv != "" {
			prov["apiKey"] = p.APIKey.FromEnv
		} else if p.APIKey.Value != "" {
			prov["apiKey"] = p.APIKey.Value
		} else {
			prov["apiKey"] = ""
		}
		if len(p.Headers) > 0 {
			headers := map[string]string{}
			for name, v := range p.Headers {
				if v.FromEnv != "" {
					headers[name] = v.FromEnv
				} else {
					headers[name] = v.Value
				}
			}
			prov["headers"] = headers
		}
		providers[p.ID] = prov
	}

	settings := map[string]any{}
	if cfg.Defaults != nil && cfg.Defaults.Model != "" {
		if providerID, modelID, ok := ir.SplitModelRef(cfg.Defaults.Model); ok {
			settings["defaultProvider"] = providerID
			settings["defaultModel"] = modelID
		}
	}
	if len(cfg.MCP) > 0 {
		mcp := map[string]any{}
		for _, s := range cfg.MCP {
			switch s.Transport {
			case ir.TransportStdio:
				entry := map[string]any{
					"type":    "stdio",
					"command": s.Command[0],
					"args":    s.Command[1:],
				}
				if s.CWD != "" {
					entry["cwd"] = s.CWD
				}
				if len(s.Env) > 0 {
					env := map[string]any{}
					for name, v := range s.Env {
						if v.FromEnv != "" {
							env[name] = map[string]string{"env": v.FromEnv}
						} else {
							env[name] = map[string]string{"value": v.Value}
						}
					}
					entry["env"] = env
				}
				if s.Enabled != nil && !*s.Enabled {
					entry["enabled"] = false
				}
				mcp[s.ID] = entry
			case ir.TransportHTTP:
				entry := map[string]any{
					"type": "http",
					"url":  s.URL,
				}
				headers := map[string]string{}
				for name, v := range s.Headers {
					if v.BearerFromEnv != "" {
						continue
					}
					headers[name] = v.Value
				}
				if len(headers) > 0 {
					entry["headers"] = headers
				}
				if auth, ok := s.Headers["Authorization"]; ok && auth.BearerFromEnv != "" {
					entry["bearerTokenEnvVar"] = auth.BearerFromEnv
				}
				if s.Enabled != nil && !*s.Enabled {
					entry["enabled"] = false
				}
				mcp[s.ID] = entry
			}
		}
		settings["mcpServers"] = mcp
	}

	modelsJSON, err := marshal(models)
	if err != nil {
		return nil, err
	}
	settingsJSON, err := marshal(settings)
	if err != nil {
		return nil, err
	}
	return []artifact.Artifact{
		{
			Target:        t.ID(),
			Name:          "models.json",
			Format:        "json",
			SuggestedPath: "~/.prime/agent/models.json",
			Content:       modelsJSON,
		},
		{
			Target:        t.ID(),
			Name:          "settings.json",
			Format:        "json",
			SuggestedPath: "~/.prime/agent/settings.json",
			Content:       settingsJSON,
		},
	}, nil
}

func modalities(in []ir.Modality) []string {
	if len(in) == 0 {
		return []string{string(ir.ModalityText)}
	}
	out := make([]string, 0, len(in))
	for _, m := range in {
		out = append(out, string(m))
	}
	return out
}

func marshal(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, fmt.Errorf("encoding prime-agent config: %w", err)
	}
	return buf.Bytes(), nil
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

var primeThinkingLevels = []ir.ReasoningEffort{"off", "minimal", "low", "medium", "high", "xhigh", "max"}

func primeThinkingLevel(effort ir.ReasoningEffort) bool {
	return primeContainsEffort(primeThinkingLevels, effort)
}

func hasPrimeThinkingLevels(efforts []ir.ReasoningEffort) bool {
	for _, effort := range efforts {
		if primeThinkingLevel(effort) {
			return true
		}
	}
	return false
}

func primeThinkingLevelMap(efforts []ir.ReasoningEffort) map[string]any {
	levels := make(map[string]any, len(primeThinkingLevels))
	for _, level := range primeThinkingLevels {
		if primeContainsEffort(efforts, level) {
			levels[string(level)] = string(level)
		} else {
			levels[string(level)] = nil
		}
	}
	return levels
}

func primeContainsEffort(efforts []ir.ReasoningEffort, wanted ir.ReasoningEffort) bool {
	for _, effort := range efforts {
		if effort == wanted {
			return true
		}
	}
	return false
}
