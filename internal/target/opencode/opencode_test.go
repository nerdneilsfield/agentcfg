package opencode

import (
	"strings"
	"testing"

	"agentcfg/internal/diag"
	"agentcfg/internal/ir"
)

func i64(n int64) *int64 { return &n }

func expectValid(t *testing.T, cfg ir.Config) {
	t.Helper()
	if diags := (Target{}).Validate(cfg); diag.HasErrors(diags) {
		t.Fatalf("expected valid config, got %v", diags)
	}
}

func expectInvalid(t *testing.T, cfg ir.Config, substr string) {
	t.Helper()
	diags := (Target{}).Validate(cfg)
	if !diag.HasErrors(diags) {
		t.Fatalf("expected validation error containing %q, got none", substr)
	}
	for _, d := range diags {
		if d.Severity == diag.SeverityError && strings.Contains(d.Message, substr) {
			return
		}
	}
	t.Fatalf("no diagnostic containing %q: %v", substr, diags)
}

func exampleConfig() ir.Config {
	return ir.Config{
		Version: 1,
		Providers: []ir.Provider{{
			ID:       "volcengine",
			Name:     "Volcengine",
			Protocol: ir.ProtocolOpenAICompletions,
			BaseURL:  "https://example.com/v1",
			APIKey:   ir.HeaderValue{FromEnv: "VOLC_API_KEY"},
			Headers: map[string]ir.HeaderValue{
				"X-Tenant":      {Value: "engineering"},
				"X-Gateway-Key": {FromEnv: "GATEWAY_KEY"},
			},
			Models: []ir.Model{{
				ID:              "glm-5.3",
				Name:            "GLM-5.3",
				ContextWindow:   i64(128000),
				MaxOutputTokens: i64(8192),
				Input:           []ir.Modality{ir.ModalityText, ir.ModalityImage},
				Output:          []ir.Modality{ir.ModalityText},
				Reasoning:       boolp(true),
			}},
		}},
		Defaults: &ir.Defaults{Model: "volcengine/glm-5.3"},
	}
}

func boolp(v bool) *bool { return &v }

func TestRejectsResponsesAndAnthropicProviders(t *testing.T) {
	responses := exampleConfig()
	responses.Providers[0].Protocol = ir.ProtocolOpenAIResponses
	expectInvalid(t, responses, "openai-completions providers only")

	anthropic := exampleConfig()
	anthropic.Providers[0].Protocol = ir.ProtocolAnthropicMessages
	expectInvalid(t, anthropic, "openai-completions providers only")
}

func TestEmitsCompletionsProvider(t *testing.T) {
	expectValid(t, exampleConfig())
	arts, err := (Target{}).Emit(exampleConfig())
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if len(arts) != 1 {
		t.Fatalf("expected 1 artifact, got %d", len(arts))
	}
	out := string(arts[0].Content)
	for _, want := range []string{
		`"@ai-sdk/openai-compatible"`,
		`"baseURL": "https://example.com/v1"`,
		`"apiKey": "{env:VOLC_API_KEY}"`,
		`"X-Tenant": "engineering"`,
		`"X-Gateway-Key": "{env:GATEWAY_KEY}"`,
		`"model": "volcengine/glm-5.3"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
}

func TestEmitsModelReasoningVariants(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Models[0].Variants = []ir.ReasoningEffort{
		"low",
		"high",
		"max",
		"ultra",
	}
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	out := string(arts[0].Content)
	for _, want := range []string{
		`"variants": {`,
		`"low": {`, `"reasoningEffort": "low"`,
		`"high": {`, `"reasoningEffort": "high"`,
		`"max": {`, `"reasoningEffort": "max"`,
		`"ultra": {`, `"reasoningEffort": "ultra"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
	for _, forbidden := range []string{`reasoningSummary`, `textVerbosity`} {
		if strings.Contains(out, forbidden) {
			t.Errorf("unexpected %q in output:\n%s", forbidden, out)
		}
	}
}
