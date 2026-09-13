package jcode

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
				"X-Tenant": {Value: "engineering"},
			},
			Models: []ir.Model{{
				ID:            "glm-5.3",
				Name:          "GLM-5.3",
				ContextWindow: i64(128000),
				Input:         []ir.Modality{ir.ModalityText, ir.ModalityImage},
				Reasoning:     b(true),
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
	if len(arts) != 2 {
		t.Fatalf("expected 2 artifacts, got %d", len(arts))
	}
	wantTOML := `[provider]
  default_provider = "volcengine"
  default_model = "glm-5.3"

[providers]
  [providers.volcengine]
    type = "openai-compatible"
    base_url = "https://example.com/v1"
    api_key_env = "VOLC_API_KEY"
    default_model = "glm-5.3"
    [providers.volcengine.headers]
      X-Tenant = "engineering"

    [[providers.volcengine.models]]
      id = "glm-5.3"
      context_window = 128000
      reasoning = true
      input = ["image"]
`
	if got := string(arts[0].Content); got != wantTOML {
		t.Fatalf("jcode TOML mismatch\ngot:\n%s\nwant:\n%s", got, wantTOML)
	}
	if arts[0].SuggestedPath != "~/.jcode/config.toml" {
		t.Fatalf("unexpected suggested path: %s", arts[0].SuggestedPath)
	}
	wantJSON := `{
  "mcpServers": {
    "context7": {
      "command": "npx",
      "args": [
        "-y",
        "@upstash/context7-mcp"
      ],
      "env": {
        "CONTEXT7_API_KEY": "${CONTEXT7_API_KEY}"
      },
      "timeout_secs": 60,
      "enabled": true
    }
  }
}
`
	if got := string(arts[1].Content); got != wantJSON {
		t.Fatalf("jcode mcp.json mismatch\ngot:\n%s\nwant:\n%s", got, wantJSON)
	}
}

func TestRejectsResponsesProtocol(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Protocol = ir.ProtocolOpenAIResponses
	expectInvalid(t, cfg, "openai-responses")
}

func TestRejectsEnvDerivedProviderHeaders(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Headers["X-Gateway-Key"] = ir.HeaderValue{FromEnv: "GATEWAY_KEY"}
	expectInvalid(t, cfg, "literal-only")
}

func TestRejectsHTTPMCP(t *testing.T) {
	cfg := exampleConfig()
	cfg.MCP[0].Transport = ir.TransportHTTP
	cfg.MCP[0].URL = "https://example.com/mcp"
	expectInvalid(t, cfg, "stdio")
}

func TestRejectsMCPCWD(t *testing.T) {
	cfg := exampleConfig()
	cfg.MCP[0].CWD = "/work"
	expectInvalid(t, cfg, "cwd")
}

func TestRejectsUnresolvedDefault(t *testing.T) {
	cfg := exampleConfig()
	cfg.Defaults = &ir.Defaults{Model: "volcengine/nope"}
	expectInvalid(t, cfg, "does not resolve")
	cfg.Defaults = &ir.Defaults{Model: "glm-5.3"}
	expectInvalid(t, cfg, "provider/model")
}

func TestOmitsMCPArtifactWhenEmpty(t *testing.T) {
	cfg := exampleConfig()
	cfg.MCP = nil
	expectValid(t, cfg)
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if len(arts) != 1 {
		t.Fatalf("expected 1 artifact without MCP, got %d", len(arts))
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
