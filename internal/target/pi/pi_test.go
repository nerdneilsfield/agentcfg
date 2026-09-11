package pi

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
				ID:            "glm-5.3",
				ContextWindow: i64(128000),
			}},
		}},
	}
}

func TestEmitsProviderHeadersWithEnvSyntax(t *testing.T) {
	expectValid(t, exampleConfig())
	arts, err := (Target{}).Emit(exampleConfig())
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	out := string(arts[0].Content)
	for _, want := range []string{
		`"X-Tenant": "engineering"`,
		`"X-Gateway-Key": "$GATEWAY_KEY"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
}

func TestRejectsBearerFromEnv(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Headers = map[string]ir.HeaderValue{
		"Authorization": {BearerFromEnv: "GITHUB_TOKEN"},
	}
	expectInvalid(t, cfg, "$NAME expression syntax")
}

func TestRejectsMCP(t *testing.T) {
	cfg := exampleConfig()
	cfg.MCP = []ir.MCPServer{{
		ID:        "context7",
		Transport: ir.TransportStdio,
		Command:   []string{"npx"},
	}}
	expectInvalid(t, cfg, "no built-in MCP support")
}
