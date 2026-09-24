package crush

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
	if arts[0].SuggestedPath != "~/.config/crush/crush.json" {
		t.Fatalf("unexpected suggested path: %s", arts[0].SuggestedPath)
	}
	want := `{
  "models": {
    "large": {
      "model": "glm-5.3",
      "provider": "volcengine"
    },
    "small": {
      "model": "glm-5.3",
      "provider": "volcengine"
    }
  },
  "providers": {
    "volcengine": {
      "id": "volcengine",
      "name": "Volcengine",
      "base_url": "https://example.com/v1",
      "type": "openai-compat",
      "discover_models": false,
      "api_key": "$VOLC_API_KEY",
      "extra_headers": {
        "X-Gateway-Key": "$GATEWAY_KEY",
        "X-Tenant": "engineering"
      },
      "models": [
        {
          "id": "glm-5.3",
          "name": "GLM-5.3",
          "cost_per_1m_in": 0,
          "cost_per_1m_out": 0,
          "cost_per_1m_in_cached": 0,
          "cost_per_1m_out_cached": 0,
          "context_window": 128000,
          "default_max_tokens": 8192,
          "can_reason": true,
          "supports_attachments": true
        }
      ]
    }
  },
  "mcp": {
    "context7": {
      "type": "stdio",
      "command": "npx",
      "args": [
        "-y",
        "@upstash/context7-mcp"
      ],
      "env": {
        "CONTEXT7_API_KEY": "$CONTEXT7_API_KEY"
      },
      "timeout": 60
    },
    "github": {
      "type": "http",
      "url": "https://api.githubcopilot.com/mcp/",
      "headers": {
        "Authorization": "Bearer ${GITHUB_TOKEN}"
      }
    }
  }
}
`
	if got := string(arts[0].Content); got != want {
		t.Fatalf("crush JSON mismatch\ngot:\n%s\nwant:\n%s", got, want)
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
	if !strings.Contains(string(arts[0].Content), `"type": "openai"`) {
		t.Fatalf("openai-responses should emit type openai:\n%s", arts[0].Content)
	}
}

func TestSkipsMCPCWD(t *testing.T) {
	cfg := exampleConfig()
	cfg.MCP[0].CWD = "/tmp"
	expectWarning(t, cfg, "cwd")
}

func TestSkipsFractionalTimeout(t *testing.T) {
	cfg := exampleConfig()
	cfg.MCP[0].TimeoutMS = i64(1500)
	expectWarning(t, cfg, "divisible by 1000")
}

func TestSkipsToolCallingFalse(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Models[0].ToolCalling = b(false)
	expectWarning(t, cfg, "tool_calling")
}

func TestSkipsMissingContextWindow(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Models[0].ContextWindow = nil
	expectWarning(t, cfg, "context_window")
}

func TestSkipsMissingMaxTokens(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Models[0].MaxOutputTokens = nil
	expectWarning(t, cfg, "default_max_tokens")
}

func TestSkipsNonAuthBearerHeader(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Headers["X-Token"] = ir.HeaderValue{BearerFromEnv: "TOKEN"}
	expectWarning(t, cfg, "Authorization")
}

func TestOmitsMCPWhenEmpty(t *testing.T) {
	cfg := exampleConfig()
	cfg.MCP = nil
	expectValid(t, cfg)
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if strings.Contains(string(arts[0].Content), `"mcp"`) {
		t.Fatalf("empty MCP must not emit an mcp object:\n%s", arts[0].Content)
	}
}

func TestEmitsReasoningLevels(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Models[0].Variants = []ir.ReasoningEffort{"low", "high", "ultra"}
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	out := string(arts[0].Content)
	if !strings.Contains(out, `"reasoning_levels": [`) || !strings.Contains(out, `"ultra"`) {
		t.Fatalf("missing reasoning levels:\n%s", out)
	}
	if strings.Contains(out, `default_reasoning_effort`) {
		t.Fatalf("must not select default effort:\n%s", out)
	}
}

func TestSkipsLiteralCrushExpressions(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*ir.Config)
	}{
		{"provider base URL", func(c *ir.Config) { c.Providers[0].BaseURL = "https://$HOST/v1" }},
		{"provider header", func(c *ir.Config) { c.Providers[0].Headers["X-Test"] = ir.HeaderValue{Value: "$(whoami)"} }},
		{"MCP argument", func(c *ir.Config) { c.MCP[0].Command[1] = "`whoami`" }},
		{"MCP environment", func(c *ir.Config) { c.MCP[0].Env["X"] = ir.HeaderValue{Value: "$HOME"} }},
		{"MCP URL", func(c *ir.Config) { c.MCP[1].URL = "https://${HOST}/mcp" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := exampleConfig()
			tc.mutate(&cfg)
			expectWarning(t, cfg, "expression syntax")
		})
	}
}

func TestEmitSkipsModelWithMissingLimit(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*ir.Model)
	}{
		{"context_window", func(m *ir.Model) { m.ContextWindow = nil }},
		{"max_output_tokens", func(m *ir.Model) { m.MaxOutputTokens = nil }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := exampleConfig()
			tc.mutate(&cfg.Providers[0].Models[0])
			expectWarning(t, cfg, "required")
			arts, err := (Target{}).Emit(cfg)
			if err != nil {
				t.Fatalf("Emit: %v", err)
			}
			if out := string(arts[0].Content); strings.Contains(out, "glm-5.3") {
				t.Fatalf("model missing a required limit must be skipped:\n%s", out)
			}
		})
	}
}

func TestEmitSkipsDefaultForSkippedModel(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Models[0].ContextWindow = nil
	expectWarning(t, cfg, "does not resolve")
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if out := string(arts[0].Content); strings.Contains(out, `"models"`) {
		t.Fatalf("default for a skipped model must not be emitted:\n%s", out)
	}
}

func TestEmitSkipsUnknownProtocolProvider(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Protocol = ir.Protocol("vertex")
	expectWarning(t, cfg, "provider type must be")
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	out := string(arts[0].Content)
	if strings.Contains(out, "volcengine") {
		t.Fatalf("provider with no native type must be skipped:\n%s", out)
	}
	if strings.Contains(out, `"type": ""`) {
		t.Fatalf("must not emit an empty provider type:\n%s", out)
	}
}

func TestEmitOmitsExpressionLiterals(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].BaseURL = "https://$HOST/v1"
	cfg.Providers[0].APIKey = ir.HeaderValue{Value: "$(token)"}
	cfg.Providers[0].Headers["X-Test"] = ir.HeaderValue{Value: "`whoami`"}
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	out := string(arts[0].Content)
	for _, bad := range []string{"$HOST", "$(token)", "whoami", "X-Test"} {
		if strings.Contains(out, bad) {
			t.Fatalf("expression literal %q must be omitted:\n%s", bad, out)
		}
	}
}

func TestEmitOmitsFractionalTimeout(t *testing.T) {
	cfg := exampleConfig()
	cfg.MCP[0].TimeoutMS = i64(1500)
	expectWarning(t, cfg, "divisible by 1000")
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if out := string(arts[0].Content); strings.Contains(out, `"timeout"`) {
		t.Fatalf("a 1500 ms timeout must not be truncated to whole seconds:\n%s", out)
	}
}

func TestEmitSkipsMCPWithExpressionCommandOrURL(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*ir.Config)
		id     string
	}{
		{"command", func(c *ir.Config) { c.MCP[0].Command[1] = "`whoami`" }, "context7"},
		{"url", func(c *ir.Config) { c.MCP[1].URL = "https://${HOST}/mcp" }, "github"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := exampleConfig()
			tc.mutate(&cfg)
			expectWarning(t, cfg, "expression syntax")
			arts, err := (Target{}).Emit(cfg)
			if err != nil {
				t.Fatalf("Emit: %v", err)
			}
			if out := string(arts[0].Content); strings.Contains(out, tc.id) {
				t.Fatalf("MCP entry with an unrepresentable %s must be skipped:\n%s", tc.name, out)
			}
		})
	}
}

func TestEmitOmitsMCPExpressionFields(t *testing.T) {
	cfg := exampleConfig()
	cfg.MCP[0].Env["BAD"] = ir.HeaderValue{Value: "$HOME"}
	cfg.MCP[1].Headers["X-Bad"] = ir.HeaderValue{Value: "$(x)"}
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	out := string(arts[0].Content)
	for _, bad := range []string{"BAD", "X-Bad"} {
		if strings.Contains(out, bad) {
			t.Fatalf("expression field %q must be omitted:\n%s", bad, out)
		}
	}
	for _, kept := range []string{"context7", "github"} {
		if !strings.Contains(out, kept) {
			t.Fatalf("entry %q must be kept:\n%s", kept, out)
		}
	}
}

func TestReportsMCPURLExpressionOnURLPath(t *testing.T) {
	cfg := exampleConfig()
	cfg.MCP[1].URL = "https://ex.com/$TOKEN"
	diags := (Target{}).Validate(cfg)
	found := false
	for _, d := range diags {
		if strings.Contains(d.Path, ".url") && strings.Contains(d.Message, "expression syntax") {
			found = true
		}
		if strings.Contains(d.Path, ".command") {
			t.Fatalf("URL expression reported as command path: %v", diags)
		}
	}
	if !found {
		t.Fatalf("missing url diagnostic: %v", diags)
	}
}
