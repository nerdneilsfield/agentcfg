// Package deepseekharness emits DeepSeek Harness provider and MCP patches.
package deepseekharness

import (
	"bytes"
	"encoding/json"
	"fmt"

	"go.yaml.in/yaml/v3"

	"agentcfg/internal/artifact"
	"agentcfg/internal/diag"
	"agentcfg/internal/ir"
	"agentcfg/internal/target"
)

func init() { target.Register(Target{}) }

// Target is the DeepSeek Harness emitter.
type Target struct{}

func (Target) ID() string { return "deepseek-harness" }

func (t Target) Validate(cfg ir.Config) []diag.Diagnostic {
	var diags []diag.Diagnostic
	for i, p := range cfg.Providers {
		if p.APIKey.Value != "" {
			diags = append(diags, diag.TargetErrorf(t.ID(), fmt.Sprintf("providers[%d].api_key", i), "deepseek-harness supports apiKeyEnv credential references only; literal API keys are not representable"))
		}
	}
	for i, p := range cfg.Providers {
		for j, m := range p.Models {
			for _, effort := range m.Variants {
				if !dshSupportsEffort(effort) {
					diags = append(diags, diag.TargetErrorf(t.ID(), fmt.Sprintf("providers[%d].models[%d].variants", i, j),
						"deepseek-harness reasoningEfforts does not support reasoning effort %q", effort))
				}
			}
		}
	}
	for i, s := range cfg.MCP {
		path := fmt.Sprintf("mcp[%d]", i)
		if s.Transport != ir.TransportStdio {
			diags = append(diags, diag.TargetErrorf(t.ID(), path,
				"deepseek-harness v1 maps stdio MCP servers through the Cordis dsh-mcp-client patch only; http is rejected"))
			continue
		}
		for name, v := range s.Env {
			if v.FromEnv == "" || v.FromEnv != name {
				diags = append(diags, diag.TargetErrorf(t.ID(), path+".env."+name,
					"deepseek-harness MCP env entries are same-name environment references only"))
			}
		}
	}
	return diags
}

func (t Target) Emit(cfg ir.Config) ([]artifact.Artifact, error) {
	providers := map[string]any{}
	for _, p := range cfg.Providers {
		ms := []any{}
		for _, m := range p.Models {
			mm := map[string]any{
				"id":   m.ID,
				"name": orDefault(m.Name, m.ID),
			}
			mm["input"] = modalStrings(m.Input)
			mm["reasoning"] = m.Reasoning != nil && *m.Reasoning
			if len(m.Variants) > 0 {
				mm["reasoningEfforts"] = dshEffortMap(m.Variants)
			}
			mm["cost"] = map[string]any{"input": 0, "output": 0, "cacheRead": 0, "cacheWrite": 0}
			if m.ContextWindow != nil {
				mm["contextWindow"] = *m.ContextWindow
			}
			if m.MaxOutputTokens != nil {
				mm["maxTokens"] = *m.MaxOutputTokens
			}
			ms = append(ms, mm)
		}
		prov := map[string]any{
			"baseURL": p.BaseURL,
			"api":     string(p.Protocol),
			"models":  ms,
		}
		if p.APIKey.FromEnv != "" {
			prov["apiKeyEnv"] = p.APIKey.FromEnv
		}
		providers[p.ID] = prov
	}
	providerPatch := map[string]any{"llm-pi-ai": map[string]any{"providers": providers}}
	var ybuf bytes.Buffer
	yenc := yaml.NewEncoder(&ybuf)
	yenc.SetIndent(2)
	if err := yenc.Encode(providerPatch); err != nil {
		return nil, fmt.Errorf("encoding deepseek-harness provider patch: %w", err)
	}
	if err := yenc.Close(); err != nil {
		return nil, fmt.Errorf("encoding deepseek-harness provider patch: %w", err)
	}

	mcpServers := map[string]any{}
	for _, s := range cfg.MCP {
		entry := map[string]any{
			"command": s.Command[0],
			"args":    s.Command[1:],
		}
		if s.CWD != "" {
			entry["cwd"] = s.CWD
		}
		if len(s.Env) > 0 {
			env := map[string]string{}
			for name, v := range s.Env {
				env[name] = v.FromEnv
			}
			entry["env"] = env
		}
		mcpServers[s.ID] = entry
	}
	arts := []artifact.Artifact{
		{
			Target:        t.ID(),
			Name:          "providers.patch.yaml",
			Format:        "yaml",
			SuggestedPath: "~/.dsh/settings.yaml (merge llm-pi-ai.providers)",
			Content:       ybuf.Bytes(),
		},
	}
	if len(mcpServers) > 0 {
		var jbuf bytes.Buffer
		jenc := json.NewEncoder(&jbuf)
		jenc.SetIndent("", "  ")
		if err := jenc.Encode(map[string]any{"mcpServers": mcpServers}); err != nil {
			return nil, fmt.Errorf("encoding deepseek-harness MCP patch: %w", err)
		}
		arts = append(arts, artifact.Artifact{
			Target:        t.ID(),
			Name:          "mcp.patch.json",
			Format:        "json",
			SuggestedPath: "~/.dsh/mcp.patch.json (Cordis patch for @deepseek-ai/dsh-mcp-client)",
			Content:       jbuf.Bytes(),
		})
	}
	return arts, nil
}

func modalStrings(in []ir.Modality) []string {
	if len(in) == 0 {
		return []string{string(ir.ModalityText)}
	}
	out := make([]string, 0, len(in))
	for _, m := range in {
		out = append(out, string(m))
	}
	return out
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

var dshEffortLevels = []ir.ReasoningEffort{"off", "minimal", "low", "medium", "high", "xhigh", "max"}

func dshSupportsEffort(effort ir.ReasoningEffort) bool {
	for _, level := range dshEffortLevels {
		if effort == level {
			return true
		}
	}
	return false
}

func dshEffortMap(efforts []ir.ReasoningEffort) map[string]any {
	declared := make(map[string]bool, len(efforts))
	for _, effort := range efforts {
		declared[string(effort)] = true
	}
	out := make(map[string]any, len(dshEffortLevels))
	for _, level := range dshEffortLevels {
		if declared[string(level)] {
			out[string(level)] = string(level)
		} else {
			out[string(level)] = nil
		}
	}
	return out
}
