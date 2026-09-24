package goose

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
				Input:         []ir.Modality{ir.ModalityText},
				Output:        []ir.Modality{ir.ModalityText},
				Reasoning:     b(true),
				ToolCalling:   b(true),
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
	if len(arts) != 2 {
		t.Fatalf("expected 2 artifacts, got %d", len(arts))
	}
	if arts[0].Name != "volcengine.json" {
		t.Fatalf("artifact 0 name: %s", arts[0].Name)
	}
	if arts[1].Name != "config.yaml" {
		t.Fatalf("artifact 1 name: %s", arts[1].Name)
	}
	if got := string(arts[0].Content); got != `{
  "name": "volcengine",
  "engine": "openai",
  "display_name": "Volcengine",
  "api_key_env": "VOLC_API_KEY",
  "base_url": "https://example.com/v1",
  "models": [
    {
      "name": "glm-5.3",
      "context_limit": 128000,
      "reasoning": true
    }
  ],
  "headers": {
    "X-Tenant": "engineering"
  },
  "supports_streaming": true,
  "requires_auth": true,
  "dynamic_models": false
}
` {
		t.Fatalf("goose provider JSON mismatch\ngot:\n%s", got)
	}
	if got := string(arts[1].Content); got != `active_provider: volcengine
providers:
  volcengine:
    enabled: true
    model: glm-5.3
    configured: true
extensions:
  context7:
    type: stdio
    name: context7
    enabled: true
    cmd: npx
    args:
      - -y
      - '@upstash/context7-mcp'
    env_keys:
      - CONTEXT7_API_KEY
    timeout: 60
  github:
    type: streamable_http
    name: github
    enabled: true
    env_keys:
      - GITHUB_TOKEN
    uri: https://api.githubcopilot.com/mcp/
    headers:
      Authorization: Bearer ${GITHUB_TOKEN}
` {
		t.Fatalf("goose config.yaml mismatch\ngot:\n%s", got)
	}
}

func TestMapsOpenAIResponses(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Protocol = ir.ProtocolOpenAIResponses
	expectValid(t, cfg)
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if !strings.Contains(string(arts[0].Content), `"base_path": "v1/responses"`) {
		t.Fatalf("openai-responses should emit base_path v1/responses:\n%s", arts[0].Content)
	}
}

func TestSkipsPerModelMaxOutputTokens(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Models[0].MaxOutputTokens = i64(8192)
	expectWarning(t, cfg, "max-tokens")
}

func TestSkipsNonTextModality(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Models[0].Input = append(cfg.Providers[0].Models[0].Input, ir.ModalityAudio)
	expectWarning(t, cfg, "modality")
}

func TestSkipsToolCallingFalse(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Models[0].ToolCalling = b(false)
	expectWarning(t, cfg, "tool_calling")
}

func TestSkipsMCPEnvBearer(t *testing.T) {
	cfg := exampleConfig()
	cfg.MCP[0].Env["AUTH"] = ir.HeaderValue{BearerFromEnv: "AUTH_TOKEN"}
	expectWarning(t, cfg, "Bearer ENV:NAME is not representable on env")
}

func TestSkipsFractionalTimeout(t *testing.T) {
	cfg := exampleConfig()
	cfg.MCP[0].TimeoutMS = i64(1500)
	expectWarning(t, cfg, "divisible by 1000")
}

func TestSkipsEnvDerivedProviderHeaders(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Headers["X-Gateway-Key"] = ir.HeaderValue{FromEnv: "GATEWAY_KEY"}
	expectWarning(t, cfg, "literal strings")
}

func TestSkipsUnresolvedDefault(t *testing.T) {
	cfg := exampleConfig()
	cfg.Defaults = &ir.Defaults{Model: "volcengine/nope"}
	expectWarning(t, cfg, "does not resolve")
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

func TestSkipsRenamedStdioEnvReference(t *testing.T) {
	cfg := exampleConfig()
	cfg.MCP[0].Env["DEST"] = ir.HeaderValue{FromEnv: "SOURCE"}
	expectWarning(t, cfg, "renamed environment references")
}

func TestSkipsNonPositiveTimeout(t *testing.T) {
	for _, timeout := range []int64{0, -1000} {
		cfg := exampleConfig()
		cfg.MCP[0].TimeoutMS = i64(timeout)
		expectWarning(t, cfg, "must be positive")
	}
}

func TestEmitOmitsUnrepresentableTimeout(t *testing.T) {
	for _, timeout := range []int64{0, -1000, 1500} {
		cfg := exampleConfig()
		cfg.MCP[0].TimeoutMS = i64(timeout)
		arts, err := (Target{}).Emit(cfg)
		if err != nil {
			t.Fatalf("Emit: %v", err)
		}
		if out := string(arts[1].Content); strings.Contains(out, "timeout") {
			t.Fatalf("%d ms timeout must be omitted, not truncated or invented:\n%s", timeout, out)
		}
	}
}

func TestEmitOmitsRenamedStdioEnvReference(t *testing.T) {
	cfg := exampleConfig()
	cfg.MCP[0].Env["DEST"] = ir.HeaderValue{FromEnv: "SOURCE"}
	expectWarning(t, cfg, "renamed environment references")
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	yaml := string(arts[1].Content)
	if strings.Contains(yaml, "SOURCE") || strings.Contains(yaml, "DEST") {
		t.Fatalf("renamed env reference must be omitted:\n%s", yaml)
	}
}

func TestEmitOmitsEnvDerivedProviderHeader(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Headers["X-Gateway-Key"] = ir.HeaderValue{FromEnv: "GATEWAY_KEY"}
	expectWarning(t, cfg, "literal strings")
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	out := string(arts[0].Content)
	if strings.Contains(out, "X-Gateway-Key") || strings.Contains(out, "GATEWAY_KEY") {
		t.Fatalf("env-derived provider header must be omitted, not emitted empty:\n%s", out)
	}
	if !strings.Contains(out, "X-Tenant") {
		t.Fatalf("literal header must be kept:\n%s", out)
	}
}

func TestEmitSkipsUnknownEngineProvider(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Protocol = ir.Protocol("gemini")
	expectWarning(t, cfg, "engine must be")
	expectWarning(t, cfg, "does not resolve")
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	configYAML := false
	for _, art := range arts {
		if art.Name == "volcengine.json" {
			t.Fatalf("provider with no native engine must be skipped")
		}
		if strings.Contains(string(art.Content), `"engine": ""`) {
			t.Fatalf("must not emit an empty engine:\n%s", art.Content)
		}
		if art.Name == "config.yaml" {
			configYAML = true
			if strings.Contains(string(art.Content), "active_provider") {
				t.Fatalf("default naming a skipped provider must be omitted:\n%s", art.Content)
			}
		}
	}
	if !configYAML {
		t.Fatal("expected config.yaml for the MCP extensions")
	}
}
