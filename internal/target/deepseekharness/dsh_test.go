package deepseekharness

import (
	"strings"
	"testing"

	"agentcfg/internal/diag"
	"agentcfg/internal/ir"
)

func boolPtr(v bool) *bool { return &v }

func variantConfig(efforts ...ir.ReasoningEffort) ir.Config {
	return ir.Config{Version: 1, Providers: []ir.Provider{{
		ID: "provider", Protocol: ir.ProtocolOpenAICompletions, BaseURL: "https://example.com/v1",
		APIKey: ir.HeaderValue{FromEnv: "API_KEY"},
		Models: []ir.Model{{ID: "model", Reasoning: boolPtr(true), Variants: efforts}},
	}}}
}

func TestEmitsReasoningEfforts(t *testing.T) {
	arts, err := (Target{}).Emit(variantConfig("low", "high", "max"))
	if err != nil {
		t.Fatal(err)
	}
	out := string(arts[0].Content)
	for _, want := range []string{"reasoningEfforts:", "low: low", "medium: null", "high: high", "max: max"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
	if strings.Contains(out, "defaultReasoning") {
		t.Fatalf("must not select a default effort:\n%s", out)
	}
}

func TestRejectsUnsupportedReasoningEffort(t *testing.T) {
	diags := (Target{}).Validate(variantConfig("ultra"))
	for _, d := range diags {
		if d.Severity == diag.SeverityError && strings.Contains(d.Message, `does not support reasoning effort "ultra"`) {
			return
		}
	}
	t.Fatalf("missing unsupported effort diagnostic: %v", diags)
}
