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
			ID:        "volcengine",
			Name:      "Volcengine",
			Protocol:  ir.ProtocolOpenAICompletions,
			BaseURL:   "https://example.com/v1",
			APIKeyEnv: "VOLC_API_KEY",
			Headers: map[string]ir.HeaderValue{
				"X-Tenant": {Value: "engineering"},
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
	if got := string(arts[1].Content); got != `GOOSE_PROVIDER: volcengine
GOOSE_MODEL: glm-5.3
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
    uri: https://api.githubcopilot.com/mcp/
    headers:
      Authorization: Bearer ${GITHUB_TOKEN}
` {
		t.Fatalf("goose config.yaml mismatch\ngot:\n%s", got)
	}
}

func TestRejectsOpenAIResponses(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Protocol = ir.ProtocolOpenAIResponses
	expectInvalid(t, cfg, "openai-responses")
}

func TestRejectsEnvDerivedProviderHeaders(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Headers["X-Gateway-Key"] = ir.HeaderValue{FromEnv: "GATEWAY_KEY"}
	expectInvalid(t, cfg, "literal strings")
}

func TestRejectsUnresolvedDefault(t *testing.T) {
	cfg := exampleConfig()
	cfg.Defaults = &ir.Defaults{Model: "volcengine/nope"}
	expectInvalid(t, cfg, "does not resolve")
}
