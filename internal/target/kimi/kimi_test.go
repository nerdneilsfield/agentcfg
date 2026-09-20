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

func TestRejectsAPIKeyEnv(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].APIKey.FromEnv = "VOLC_API_KEY"
	expectInvalid(t, cfg, "api_key")
}

func TestRejectsMissingContextWindow(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Models[0].ContextWindow = nil
	expectInvalid(t, cfg, "max_context_size")
}

func TestRejectsMCPEnvRef(t *testing.T) {
	cfg := exampleConfig()
	cfg.MCP[0].Env["CONTEXT7_API_KEY"] = ir.HeaderValue{FromEnv: "CONTEXT7_API_KEY"}
	expectInvalid(t, cfg, "literal strings")
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
