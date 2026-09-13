// Package deepseekharness emits DeepSeek Harness provider and MCP patches.
package deepseekharness

import (
	"bytes"
	"fmt"
	"strings"

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
		path := fmt.Sprintf("providers[%d]", i)
		if p.APIKey.Value != "" {
			diags = append(diags, diag.TargetErrorf(t.ID(), path+".api_key", "deepseek-harness supports apiKeyEnv credential references only; literal API keys are not representable"))
		}
		for j, m := range p.Models {
			for _, mod := range m.Input {
				if mod != ir.ModalityText && mod != ir.ModalityImage {
					diags = append(diags, diag.TargetErrorf(t.ID(), fmt.Sprintf("%s.models[%d].input", path, j), "deepseek-harness model input supports text and image only"))
				}
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
			if hasDSHEfforts(m.Variants) {
				mm["reasoningEfforts"] = dshEffortMap(m.Variants)
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
			"baseURL": p.BaseURL,
			"api":     string(p.Protocol),
			"models":  ms,
		}
		if p.Name != "" {
			prov["displayName"] = p.Name
		}
		if len(p.Headers) > 0 {
			headers := map[string]string{}
			for name, v := range p.Headers {
				if v.FromEnv == "" && v.BearerFromEnv == "" {
					headers[name] = v.Value
				}
			}
			prov["headers"] = headers
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

	arts := []artifact.Artifact{{
		Target: t.ID(), Name: "providers.patch.yaml", Format: "yaml",
		SuggestedPath: "$DSH_HOME/settings.yaml (merge llm-pi-ai.providers)", Content: ybuf.Bytes(),
	}}

	patchRows := []any{}
	if cfg.Defaults != nil && cfg.Defaults.Model != "" {
		provider, model, ok := ir.SplitModelRef(cfg.Defaults.Model)
		if ok {
			patchRows = append(patchRows, map[string]any{
				"id": "agent-default-model", "name": "@deepseek-ai/dsh-agent-default-model",
				"config": map[string]string{"provider": provider, "model": model},
			})
		}
	}
	for _, s := range cfg.MCP {
		config := map[string]any{"serverName": s.ID}
		if s.TimeoutMS != nil {
			config["toolCallTimeoutMs"] = *s.TimeoutMS
		}
		if s.Transport == ir.TransportStdio {
			config["transport"] = "stdio"
			config["command"] = s.Command[0]
			config["args"] = s.Command[1:]
			if s.CWD != "" {
				config["cwd"] = s.CWD
			}
			if len(s.Env) > 0 {
				env := map[string]any{}
				for name, v := range s.Env {
					env[name] = dshValue(v)
				}
				config["env"] = env
			}
		} else {
			config["transport"] = "streamable-http"
			config["url"] = s.URL
			if len(s.Headers) > 0 {
				headers := map[string]any{}
				for name, v := range s.Headers {
					headers[name] = dshValue(v)
				}
				config["headers"] = headers
			}
		}
		row := map[string]any{"id": "mcp-" + s.ID, "name": "@deepseek-ai/dsh-mcp-client", "config": config}
		if s.Enabled != nil && !*s.Enabled {
			row["disabled"] = true
		}
		patchRows = append(patchRows, row)
	}
	if len(patchRows) > 0 {
		var pbuf bytes.Buffer
		penc := yaml.NewEncoder(&pbuf)
		penc.SetIndent(2)
		if err := penc.Encode([]any{map[string]any{"insert": patchRows}}); err != nil {
			return nil, fmt.Errorf("encoding deepseek-harness Cordis patch: %w", err)
		}
		if err := penc.Close(); err != nil {
			return nil, fmt.Errorf("encoding deepseek-harness Cordis patch: %w", err)
		}
		arts = append(arts, artifact.Artifact{Target: t.ID(), Name: "cordis.patch.yml", Format: "yaml", SuggestedPath: "$DSH_HOME/cordis.patch.yml", Content: []byte(dshExpressions(pbuf.String()))})
	}
	return arts, nil
}

func dshValue(v ir.HeaderValue) string {
	if v.BearerFromEnv != "" {
		return "__DSH_JS__`Bearer ${process.env." + v.BearerFromEnv + "}`"
	}
	if v.FromEnv != "" {
		return "__DSH_JS__process.env." + v.FromEnv
	}
	return v.Value
}

func dshExpressions(doc string) string {
	lines := strings.Split(doc, "\n")
	for i, line := range lines {
		if pos := strings.Index(line, "__DSH_JS__"); pos >= 0 {
			prefix := line[:pos]
			expr := strings.TrimSuffix(line[pos+len("__DSH_JS__"):], "\"")
			lines[i] = prefix + "!!js " + expr
		}
	}
	return strings.Join(lines, "\n")
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

func hasDSHEfforts(efforts []ir.ReasoningEffort) bool {
	for _, effort := range efforts {
		if dshSupportsEffort(effort) {
			return true
		}
	}
	return false
}

func dshEffortMap(efforts []ir.ReasoningEffort) map[string]any {
	out := map[string]any{}
	for _, effort := range efforts {
		if !dshSupportsEffort(effort) {
			continue
		}
		if effort == "off" {
			out[string(effort)] = nil
		} else {
			out[string(effort)] = string(effort)
		}
	}
	return out
}
