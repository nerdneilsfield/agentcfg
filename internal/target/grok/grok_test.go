package grok

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
			Protocol: ir.ProtocolOpenAIResponses,
			BaseURL:  "https://example.com/v1",
			APIKey:   ir.HeaderValue{FromEnv: "VOLC_API_KEY"},
			Headers: map[string]ir.HeaderValue{
				"X-Tenant": {Value: "engineering"},
			},
			Models: []ir.Model{{
				ID:              "glm-5.3",
				Name:            "GLM-5.3",
				ContextWindow:   i64(128000),
				MaxOutputTokens: i64(8192),
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
	want := `[models]
  default = "glm-5.3"

[model]
  [model."glm-5.3"]
    model = "glm-5.3"
    base_url = "https://example.com/v1"
    name = "GLM-5.3"
    env_key = "VOLC_API_KEY"
    api_backend = "responses"
    context_window = 128000
    max_completion_tokens = 8192
    [model."glm-5.3".extra_headers]
      X-Tenant = "engineering"

[mcp_servers]
  [mcp_servers.context7]
    command = "npx"
    args = ["-y", "@upstash/context7-mcp"]
    [mcp_servers.context7.env]
      CONTEXT7_API_KEY = "${CONTEXT7_API_KEY}"
  [mcp_servers.github]
    url = "https://api.githubcopilot.com/mcp/"
    bearer_token_env_var = "GITHUB_TOKEN"
`
	if got := string(arts[0].Content); got != want {
		t.Fatalf("grok TOML mismatch\ngot:\n%s\nwant:\n%s", got, want)
	}
	if arts[0].SuggestedPath != "~/.grok/config.toml" {
		t.Fatalf("unexpected suggested path: %s", arts[0].SuggestedPath)
	}
}

func TestEmitsEnvDerivedProviderHeaders(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Headers["X-Gateway-Key"] = ir.HeaderValue{FromEnv: "GATEWAY_KEY"}
	expectValid(t, cfg)
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	out := string(arts[0].Content)
	for _, want := range []string{`[model."glm-5.3".env_http_headers]`, `X-Gateway-Key = "GATEWAY_KEY"`} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, `X-Gateway-Key = ""`) {
		t.Fatalf("env header leaked into extra headers:\n%s", out)
	}
}

func TestRejectsDuplicateModelIDs(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers = append(cfg.Providers, ir.Provider{
		ID:       "other",
		Protocol: ir.ProtocolOpenAICompletions,
		BaseURL:  "https://other.example.com/v1",
		Models:   []ir.Model{{ID: "glm-5.3"}},
	})
	expectInvalid(t, cfg, "duplicate model id")
}

func TestRejectsUnresolvedDefault(t *testing.T) {
	cfg := exampleConfig()
	cfg.Defaults = &ir.Defaults{Model: "volcengine/nope"}
	expectInvalid(t, cfg, "does not resolve")
}

func TestRejectsBearerOnNonAuthorizationHeader(t *testing.T) {
	cfg := exampleConfig()
	cfg.MCP[1].Headers["X-Token"] = ir.HeaderValue{BearerFromEnv: "SOME_TOKEN"}
	expectInvalid(t, cfg, "Authorization only")
}

func TestRejectsMCPTimeout(t *testing.T) {
	cfg := exampleConfig()
	cfg.MCP[0].TimeoutMS = i64(1500)
	expectInvalid(t, cfg, "no timeout field")
}

func TestOmitsEmptyMCPServers(t *testing.T) {
	cfg := exampleConfig()
	cfg.MCP = nil
	expectValid(t, cfg)
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if strings.Contains(string(arts[0].Content), "mcp_servers") {
		t.Fatalf("empty MCP must not emit an [mcp_servers] table:\n%s", arts[0].Content)
	}
}

func TestEmitsSupportedReasoningEfforts(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Models[0].Variants = []ir.ReasoningEffort{"low", "high", "ultra"}
	expectValid(t, cfg)
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	out := string(arts[0].Content)
	for _, want := range []string{`[[model."glm-5.3".reasoning_efforts]]`, `id = "low"`, `value = "high"`} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	for _, forbidden := range []string{`id = "ultra"`, `value = "ultra"`, `supports_reasoning_effort`} {
		if strings.Contains(out, forbidden) {
			t.Errorf("unexpected %q:\n%s", forbidden, out)
		}
	}
}
