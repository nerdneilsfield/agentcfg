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

func TestSkipsResponsesProtocol(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Protocol = ir.ProtocolOpenAIResponses
	expectWarning(t, cfg, "openai-responses")
	expectWarning(t, cfg, "does not resolve")
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	out := string(arts[0].Content)
	if strings.Contains(out, "volcengine") {
		t.Fatalf("an unsupported-protocol provider must be skipped:\n%s", out)
	}
	if strings.Contains(out, "[provider]") {
		t.Fatalf("a default naming a skipped provider must be omitted:\n%s", out)
	}
}

func TestSkipsEnvDerivedProviderHeaders(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Headers["X-Gateway-Key"] = ir.HeaderValue{FromEnv: "GATEWAY_KEY"}
	expectWarning(t, cfg, "literal-only")
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	out := string(arts[0].Content)
	if strings.Contains(out, "X-Gateway-Key") {
		t.Fatalf("an env-derived provider header must be skipped:\n%s", out)
	}
	if !strings.Contains(out, "X-Tenant") {
		t.Fatalf("a literal provider header must be kept:\n%s", out)
	}
}

func TestSkipsHTTPMCP(t *testing.T) {
	cfg := exampleConfig()
	cfg.MCP[0].Transport = ir.TransportHTTP
	cfg.MCP[0].Command = nil
	cfg.MCP[0].URL = "https://example.com/mcp"
	expectWarning(t, cfg, "stdio")
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(arts) != 1 {
		t.Fatalf("an all-http MCP set must not emit mcp.json, got %d artifacts", len(arts))
	}
}

func TestSkipsMCPCWD(t *testing.T) {
	cfg := exampleConfig()
	cfg.MCP[0].CWD = "/work"
	expectWarning(t, cfg, "cwd")
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(arts[1].Content), "cwd") {
		t.Fatalf("jcode has no MCP cwd field:\n%s", arts[1].Content)
	}
}

func TestSkipsUnresolvedDefault(t *testing.T) {
	cfg := exampleConfig()
	cfg.Defaults = &ir.Defaults{Model: "volcengine/nope"}
	expectWarning(t, cfg, "does not resolve")
	cfg.Defaults = &ir.Defaults{Model: "glm-5.3"}
	expectWarning(t, cfg, "provider/model")
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

func TestSkipsFractionalMCPTimeout(t *testing.T) {
	cfg := exampleConfig()
	cfg.MCP[0].TimeoutMS = i64(1500)
	expectWarning(t, cfg, "timeout_secs is an integer")
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(arts[1].Content), "timeout_secs") {
		t.Fatalf("a fractional timeout must not be truncated:\n%s", arts[1].Content)
	}
}
