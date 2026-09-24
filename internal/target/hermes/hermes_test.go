package hermes

import (
	"strings"
	"testing"

	"agentcfg/internal/diag"
	"agentcfg/internal/ir"
)

func i64(n int64) *int64 { return &n }

func b(v bool) *bool { return &v }

func expectValid(t *testing.T, cfg ir.Config) {
	t.Helper()
	if diags := (Target{}).Validate(cfg); len(diags) != 0 {
		t.Fatalf("expected valid config, got %v", diags)
	}
}

func expectWarning(t *testing.T, cfg ir.Config, substr string) {
	t.Helper()
	diags := (Target{}).Validate(cfg)
	if len(diags) == 0 {
		t.Fatalf("expected validation error containing %q, got none", substr)
	}
	for _, d := range diags {
		if d.Severity == diag.SeverityWarning && strings.Contains(d.Message, substr) {
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
				Reasoning:       b(true),
				ToolCalling:     b(true),
			}},
		}},
		MCP: []ir.MCPServer{{
			ID:        "context7",
			Transport: ir.TransportStdio,
			Command:   []string{"npx", "-y", "@upstash/context7-mcp"},
			Env: map[string]ir.HeaderValue{
				"CONTEXT7_API_KEY": {FromEnv: "CONTEXT7_API_KEY"},
			},
			TimeoutMS: i64(60000),
		}, {
			ID:        "github",
			Transport: ir.TransportHTTP,
			URL:       "https://api.githubcopilot.com/mcp/",
			Headers: map[string]ir.HeaderValue{
				"Authorization": {BearerFromEnv: "GITHUB_TOKEN"},
			},
		}},
		Defaults: &ir.Defaults{Model: "volcengine/glm-5.3"},
	}
}

func TestEmitGolden(t *testing.T) {
	expectValid(t, exampleConfig())
	arts, err := (Target{}).Emit(exampleConfig())
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if len(arts) != 1 {
		t.Fatalf("expected 1 artifact, got %d", len(arts))
	}
	if arts[0].SuggestedPath != "~/.hermes/config.yaml" {
		t.Fatalf("unexpected suggested path: %s", arts[0].SuggestedPath)
	}
	want := `model:
  provider: volcengine
  default: glm-5.3
providers:
  volcengine:
    base_url: https://example.com/v1
    api_key_env: VOLC_API_KEY
    api_mode: chat_completions
    extra_headers:
      X-Gateway-Key: ${GATEWAY_KEY}
      X-Tenant: engineering
    models:
      glm-5.3:
        context_length: 128000
mcp_servers:
  context7:
    command: npx
    args:
      - -y
      - '@upstash/context7-mcp'
    env:
      CONTEXT7_API_KEY: ${CONTEXT7_API_KEY}
    timeout: 60
    enabled: true
  github:
    url: https://api.githubcopilot.com/mcp/
    headers:
      Authorization: Bearer ${GITHUB_TOKEN}
    enabled: true
`
	if got := string(arts[0].Content); got != want {
		t.Fatalf("hermes YAML mismatch\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestSkipsUnresolvedDefault(t *testing.T) {
	cfg := exampleConfig()
	cfg.Defaults = &ir.Defaults{Model: "volcengine/nope"}
	expectWarning(t, cfg, "does not resolve")
	out := emitContent(t, cfg)
	if strings.Contains(out, "provider: volcengine") || strings.Contains(out, "default: nope") {
		t.Fatalf("unresolved default must be skipped:\n%s", out)
	}
}

func TestIgnoresModelReasoningVariants(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Models[0].Reasoning = b(true)
	cfg.Providers[0].Models[0].Variants = []ir.ReasoningEffort{"low", "ultra"}
	expectValid(t, cfg)
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	for _, art := range arts {
		if strings.Contains(string(art.Content), `"variants"`) {
			t.Fatalf("variants must be skipped:\n%s", art.Content)
		}
	}
}

func TestSkipsNonPositiveTimeout(t *testing.T) {
	for _, timeout := range []int64{0, -1000} {
		cfg := exampleConfig()
		cfg.MCP[0].TimeoutMS = i64(timeout)
		expectWarning(t, cfg, "must be positive")
		out := emitContent(t, cfg)
		if strings.Contains(out, "timeout:") {
			t.Fatalf("non-positive timeout must be omitted (timeout=%d):\n%s", timeout, out)
		}
	}
}

func emitContent(t *testing.T, cfg ir.Config) string {
	t.Helper()
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if len(arts) != 1 {
		t.Fatalf("expected 1 artifact, got %d", len(arts))
	}
	return string(arts[0].Content)
}

func TestOmitsLiteralAPIKeyWithExpression(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].APIKey = ir.HeaderValue{Value: "${SECRET}"}
	expectWarning(t, cfg, "native expression")
	out := emitContent(t, cfg)
	if strings.Contains(out, "${SECRET}") {
		t.Fatalf("literal key with expression syntax must be skipped:\n%s", out)
	}
	if !strings.Contains(out, "api_mode: chat_completions") {
		t.Fatalf("the provider must still be emitted:\n%s", out)
	}
}

func TestSkipsUnknownProtocolProvider(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Protocol = "bogus"
	expectWarning(t, cfg, "api_mode")
	out := emitContent(t, cfg)
	if strings.Contains(out, "volcengine:") || strings.Contains(out, "provider:") {
		t.Fatalf("provider with an unsupported protocol must be skipped:\n%s", out)
	}
}

func TestSkipsStdioMCPWithoutCommand(t *testing.T) {
	cfg := exampleConfig()
	cfg.MCP[0].Command = nil
	expectWarning(t, cfg, "no command")
	out := emitContent(t, cfg)
	if strings.Contains(out, "context7") {
		t.Fatalf("stdio server without a command must be skipped:\n%s", out)
	}
	if !strings.Contains(out, "github") {
		t.Fatalf("the rest of the document must still generate:\n%s", out)
	}
}
