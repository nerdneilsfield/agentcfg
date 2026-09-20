// Package pifamily builds the custom-provider models document shared by the
// pi-derived agents: pi, Prime Agent, and Oh My Pi. The forks agree on provider
// and model field names but not on the file path, the encoding, or how a value
// references an environment variable, so each target supplies those.
package pifamily

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"

	"agentcfg/internal/diag"
	"agentcfg/internal/ir"
)

// Syntax is how a fork writes an environment reference inside a config value.
type Syntax int

const (
	// DollarEnv is pi's form: "$NAME" and "${NAME}" interpolate, and a value
	// without a leading "$" is a literal.
	DollarEnv Syntax = iota
	// BareEnv is the form Prime Agent and Oh My Pi use: the whole value is an
	// environment variable name, and the literal string is used when unset.
	BareEnv
)

// Options are the per-fork differences the document depends on.
type Options struct {
	// ID is the target id, used in diagnostics.
	ID string
	// Syntax renders environment references for this fork.
	Syntax Syntax
	// Levels lists the thinking levels this fork documents, in output order.
	// A level the model does not declare is written as null.
	Levels []ir.ReasoningEffort
	// Bearer renders an Authorization bearer header value. A nil Bearer means
	// the fork cannot represent one, and its Validate rejects it first.
	Bearer func(name string) string
	// ModelFields adds fork-specific model fields. They run after the shared
	// fields and may override them.
	ModelFields func(m ir.Model) map[string]any
}

// Providers builds the providers object of the models document.
func Providers(cfg ir.Config, opts Options) map[string]any {
	providers := make(map[string]any, len(cfg.Providers))
	for _, p := range cfg.Providers {
		models := make([]any, 0, len(p.Models))
		for _, m := range p.Models {
			models = append(models, model(m, opts))
		}
		entry := map[string]any{
			"baseUrl": p.BaseURL,
			"api":     string(p.Protocol),
			"models":  models,
		}
		if p.EffectiveAuthType() != ir.AuthTypeNone {
			if p.APIKey.FromEnv != "" {
				entry["apiKey"] = envRef(opts.Syntax, p.APIKey.FromEnv)
			} else if p.APIKey.Value != "" {
				entry["apiKey"] = p.APIKey.Value
			}
		}
		if p.EffectiveAuthType() == ir.AuthTypeBearer {
			entry["authHeader"] = true
		}
		if len(p.Headers) > 0 {
			headers := make(map[string]string, len(p.Headers))
			for name, v := range p.Headers {
				switch {
				case v.BearerFromEnv != "" && opts.Bearer != nil:
					headers[name] = opts.Bearer(v.BearerFromEnv)
				case v.FromEnv != "":
					headers[name] = envRef(opts.Syntax, v.FromEnv)
				default:
					headers[name] = v.Value
				}
			}
			entry["headers"] = headers
		}
		providers[p.ID] = entry
	}
	return providers
}

func model(m ir.Model, opts Options) map[string]any {
	out := map[string]any{
		"id":    m.ID,
		"name":  orDefault(m.Name, m.ID),
		"input": modalities(m.Input),
		"cost":  map[string]any{"input": 0, "output": 0, "cacheRead": 0, "cacheWrite": 0},
	}
	out["reasoning"] = m.Reasoning != nil && *m.Reasoning
	if levels := thinkingLevelMap(m.Variants, opts.Levels); levels != nil {
		out["thinkingLevelMap"] = levels
	}
	if m.ContextWindow != nil {
		out["contextWindow"] = *m.ContextWindow
	}
	if m.MaxOutputTokens != nil {
		out["maxTokens"] = *m.MaxOutputTokens
	}
	if opts.ModelFields != nil {
		for key, value := range opts.ModelFields(m) {
			out[key] = value
		}
	}
	return out
}

// envRef renders one environment variable reference in the fork's syntax.
func envRef(syntax Syntax, name string) string {
	if syntax == BareEnv {
		return name
	}
	return "$" + name
}

// EncodeJSON renders a models document the way the forks read it.
func EncodeJSON(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, fmt.Errorf("encoding models document: %w", err)
	}
	return buf.Bytes(), nil
}

// EncodeYAML renders a document for the forks that read YAML.
func EncodeYAML(v any) ([]byte, error) {
	out, err := yaml.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("encoding models document: %w", err)
	}
	return out, nil
}

// ValidateAuthTypes reports auth_type values these forks cannot map: their
// OpenAI transports always send "Authorization: Bearer", so x-api-key is only
// native on the Anthropic Messages wire.
func ValidateAuthTypes(tid string, cfg ir.Config) []diag.Diagnostic {
	var diags []diag.Diagnostic
	for i, p := range cfg.Providers {
		if p.EffectiveAuthType() != ir.AuthTypeXAPIKey || p.Protocol == ir.ProtocolAnthropicMessages {
			continue
		}
		diags = append(diags, diag.TargetErrorf(tid, fmt.Sprintf("providers[%d].auth_type", i),
			"%s authenticates %s with Authorization: Bearer; auth_type: x-api-key is only native on anthropic-messages", tid, p.Protocol))
	}
	return diags
}

// ValidateLiteralAPIKeys rejects literals a fork would read as its own
// expression syntax instead of as a literal token.
func ValidateLiteralAPIKeys(tid string, cfg ir.Config) []diag.Diagnostic {
	var diags []diag.Diagnostic
	for i, p := range cfg.Providers {
		key := p.APIKey.Value
		if !strings.HasPrefix(key, "$") && !strings.HasPrefix(key, "!") {
			continue
		}
		diags = append(diags, diag.TargetErrorf(tid, fmt.Sprintf("providers[%d].api_key", i),
			"literal API key contains native expression syntax and cannot be represented literally"))
	}
	return diags
}

// ValidateModelInput rejects input modalities the forks do not carry.
func ValidateModelInput(tid string, cfg ir.Config) []diag.Diagnostic {
	var diags []diag.Diagnostic
	for i, p := range cfg.Providers {
		for j, m := range p.Models {
			for _, mod := range m.Input {
				if mod == ir.ModalityText || mod == ir.ModalityImage {
					continue
				}
				diags = append(diags, diag.TargetErrorf(tid, fmt.Sprintf("providers[%d].models[%d].input", i, j),
					"%s model input supports text and image only", tid))
			}
		}
	}
	return diags
}

func thinkingLevelMap(efforts []ir.ReasoningEffort, levels []ir.ReasoningEffort) map[string]any {
	out := make(map[string]any, len(levels))
	declared := false
	for _, level := range levels {
		if containsEffort(efforts, level) {
			out[string(level)] = string(level)
			declared = true
			continue
		}
		out[string(level)] = nil
	}
	if !declared {
		return nil
	}
	return out
}

func containsEffort(efforts []ir.ReasoningEffort, wanted ir.ReasoningEffort) bool {
	for _, effort := range efforts {
		if effort == wanted {
			return true
		}
	}
	return false
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

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
