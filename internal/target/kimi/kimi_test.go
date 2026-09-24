package kimi

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
			Protocol: ir.ProtocolOpenAIResponses,
			BaseURL:  "https://example.com/v1",
			Headers: map[string]ir.HeaderValue{
				"X-Tenant": {Value: "engineering"},
			},
			Models: []ir.Model{{
				ID:              "glm-5.3",
				Name:            "GLM-5.3",
				ContextWindow:   i64(128000),
				MaxOutputTokens: i64(8192),
				Input:           []ir.Modality{ir.ModalityText, ir.ModalityImage},
				Reasoning:       b(true),
				ToolCalling:     b(true),
			}},
		}},
		MCP: []ir.MCPServer{{
			ID:        "context7",
			Transport: ir.TransportStdio,
			Command:   []string{"npx", "-y", "@upstash/context7-mcp"},
			Env: map[string]ir.HeaderValue{
				"CONTEXT7_API_KEY": {Value: "literal-key"},
			},
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
	if len(arts) != 2 {
		t.Fatalf("expected 2 artifacts, got %d", len(arts))
	}
	wantTOML := `default_model = "glm-5.3"

[providers]
  [providers.volcengine]
    type = "openai_responses"
    base_url = "https://example.com/v1"
    [providers.volcengine.custom_headers]
      X-Tenant = "engineering"

[models]
  [models."glm-5.3"]
    provider = "volcengine"
    model = "glm-5.3"
    max_context_size = 128000
    max_output_size = 8192
    display_name = "GLM-5.3"
    capabilities = ["thinking", "tool_use", "image_in"]
`
	if got := string(arts[0].Content); got != wantTOML {
		t.Fatalf("kimi TOML mismatch\ngot:\n%s\nwant:\n%s", got, wantTOML)
	}
	wantJSON := `{
  "mcpServers": {
    "context7": {
      "transport": "stdio",
      "command": "npx",
      "args": [
        "-y",
        "@upstash/context7-mcp"
      ],
      "env": {
        "CONTEXT7_API_KEY": "literal-key"
      },
      "enabled": true
    },
    "github": {
      "transport": "http",
      "url": "https://api.githubcopilot.com/mcp/",
      "bearerTokenEnvVar": "GITHUB_TOKEN",
      "enabled": true
    }
  }
}
`
	if got := string(arts[1].Content); got != wantJSON {
		t.Fatalf("kimi mcp.json mismatch\ngot:\n%s\nwant:\n%s", got, wantJSON)
	}
}

func TestEmitsAPIKeyEnv(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].APIKey.FromEnv = "VOLC_API_KEY"
	expectValid(t, cfg)
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	out := string(arts[0].Content)
	if !strings.Contains(out, `api_key_env = "VOLC_API_KEY"`) {
		t.Fatalf("missing api_key_env:\n%s", out)
	}
	if strings.Contains(out, `api_key =`) {
		t.Fatalf("environment reference emitted as literal api_key:\n%s", out)
	}
}

func TestSkipsMissingContextWindow(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Models[0].ContextWindow = nil
	expectWarning(t, cfg, "max_context_size")
	expectWarning(t, cfg, "does not resolve")
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	out := string(arts[0].Content)
	if strings.Contains(out, `[models."glm-5.3"]`) {
		t.Fatalf("a model without context_window must be skipped:\n%s", out)
	}
	if strings.Contains(out, "default_model") {
		t.Fatalf("a default naming a skipped alias must be omitted:\n%s", out)
	}
}

func TestSkipsDuplicateModelID(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers = append(cfg.Providers, ir.Provider{
		ID:       "second",
		Protocol: ir.ProtocolAnthropicMessages,
		BaseURL:  "https://example.com/anthropic",
		Models:   []ir.Model{{ID: "glm-5.3", ContextWindow: i64(64000)}},
	})
	expectWarning(t, cfg, "duplicate model id")
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	out := string(arts[0].Content)
	if n := strings.Count(out, `[models."glm-5.3"]`); n != 1 {
		t.Fatalf("duplicate model id must collapse to one alias, got %d:\n%s", n, out)
	}
	if !strings.Contains(out, `provider = "volcengine"`) {
		t.Fatalf("the first provider's model must win:\n%s", out)
	}
}

func TestSkipsEnvDerivedProviderHeader(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Headers["X-Gateway-Key"] = ir.HeaderValue{FromEnv: "GATEWAY_KEY"}
	expectWarning(t, cfg, "literal strings")
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

func TestSkipsMCPEnvRef(t *testing.T) {
	cfg := exampleConfig()
	cfg.MCP[0].Env["CONTEXT7_API_KEY"] = ir.HeaderValue{FromEnv: "CONTEXT7_API_KEY"}
	expectWarning(t, cfg, "literal strings")
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	out := string(arts[1].Content)
	if strings.Contains(out, "CONTEXT7_API_KEY") {
		t.Fatalf("an env-derived MCP env value must be skipped:\n%s", out)
	}
	if !strings.Contains(out, `"context7"`) {
		t.Fatalf("the MCP entry itself must be kept:\n%s", out)
	}
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

func TestSkipsBearerOnNonAuthorizationHeader(t *testing.T) {
	cfg := exampleConfig()
	cfg.MCP[1].Headers["X-Token"] = ir.HeaderValue{BearerFromEnv: "SOME_TOKEN"}
	expectWarning(t, cfg, "Authorization only")
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	out := string(arts[1].Content)
	if strings.Contains(out, "X-Token") {
		t.Fatalf("a bearer reference on a non-Authorization header must be skipped:\n%s", out)
	}
	if !strings.Contains(out, `"bearerTokenEnvVar": "GITHUB_TOKEN"`) {
		t.Fatalf("the Authorization bearer reference must be preserved:\n%s", out)
	}
}

func TestSkipsEnvDerivedMCPHeader(t *testing.T) {
	cfg := exampleConfig()
	cfg.MCP[1].Headers["X-Plain-Env"] = ir.HeaderValue{FromEnv: "SOME_VAR"}
	expectWarning(t, cfg, "literal strings")
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	out := string(arts[1].Content)
	if strings.Contains(out, "X-Plain-Env") || strings.Contains(out, `"": `) {
		t.Fatalf("an env-derived MCP header must be skipped:\n%s", out)
	}
}

func TestEmitsMCPStartupTimeout(t *testing.T) {
	cfg := exampleConfig()
	cfg.MCP[0].TimeoutMS = i64(1500)
	expectValid(t, cfg)
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	out := string(arts[1].Content)
	if !strings.Contains(out, `"startupTimeoutMs": 1500`) {
		t.Fatalf("missing startupTimeoutMs:\n%s", out)
	}
	if strings.Contains(out, "toolTimeoutMs") {
		t.Fatalf("must not emit toolTimeoutMs:\n%s", out)
	}
}

func TestSkipsOutOfRangeMCPTimeout(t *testing.T) {
	cfg := exampleConfig()
	cfg.MCP[0].TimeoutMS = i64(0)
	expectWarning(t, cfg, "startupTimeoutMs")
	cfg.MCP[0].TimeoutMS = i64(2147483648)
	expectWarning(t, cfg, "startupTimeoutMs")
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(arts[1].Content), "startupTimeoutMs") {
		t.Fatalf("an out-of-range timeout must be omitted:\n%s", arts[1].Content)
	}
}

func TestEmitsSupportEfforts(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Models[0].Variants = []ir.ReasoningEffort{"low", "high", "max"}
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	out := string(arts[0].Content)
	if !strings.Contains(out, `support_efforts = ["low", "high", "max"]`) {
		t.Fatalf("missing support efforts:\n%s", out)
	}
	if strings.Contains(out, `default_effort`) {
		t.Fatalf("must not select a default effort:\n%s", out)
	}
}

func TestPreservesProviderReasoningEffortNames(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Models[0].Variants = []ir.ReasoningEffort{"low", "ultra"}
	expectValid(t, cfg)
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	out := string(arts[0].Content)
	if !strings.Contains(out, `support_efforts = ["low", "ultra"]`) {
		t.Fatalf("missing provider effort names:\n%s", out)
	}
}
