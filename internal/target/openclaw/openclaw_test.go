package openclaw

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
	if arts[0].SuggestedPath != "~/.openclaw/openclaw.json" {
		t.Fatalf("unexpected suggested path: %s", arts[0].SuggestedPath)
	}
	want := `{
  "models": {
    "providers": {
      "volcengine": {
        "baseUrl": "https://example.com/v1",
        "api": "openai-completions",
        "apiKey": "${VOLC_API_KEY}",
        "headers": {
          "X-Gateway-Key": "${GATEWAY_KEY}",
          "X-Tenant": "engineering"
        },
        "models": [
          {
            "id": "glm-5.3",
            "name": "GLM-5.3",
            "contextWindow": 128000,
            "maxTokens": 8192,
            "input": [
              "text",
              "image"
            ],
            "reasoning": true,
            "compat": {
              "supportsTools": true
            }
          }
        ]
      }
    }
  },
  "agents": {
    "defaults": {
      "model": "volcengine/glm-5.3"
    }
  },
  "mcp": {
    "servers": {
      "context7": {
        "enabled": true,
        "transport": "stdio",
        "command": "npx",
        "args": [
          "-y",
          "@upstash/context7-mcp"
        ],
        "env": {
          "CONTEXT7_API_KEY": "${CONTEXT7_API_KEY}"
        },
        "connectionTimeoutMs": 60000,
        "requestTimeoutMs": 60000
      },
      "github": {
        "enabled": true,
        "transport": "streamable-http",
        "url": "https://api.githubcopilot.com/mcp/",
        "headers": {
          "Authorization": "Bearer ${GITHUB_TOKEN}"
        }
      }
    }
  }
}
`
	if got := string(arts[0].Content); got != want {
		t.Fatalf("openclaw JSON mismatch\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestSkipsUnresolvedDefault(t *testing.T) {
	cfg := exampleConfig()
	cfg.Defaults = &ir.Defaults{Model: "volcengine/nope"}
	expectWarning(t, cfg, "does not resolve")
	out := emitContent(t, cfg)
	if strings.Contains(out, `"agents"`) {
		t.Fatalf("unresolved default must be skipped:\n%s", out)
	}
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

func TestEmitsThinkingLevelMap(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Models[0].Variants = []ir.ReasoningEffort{"low", "high", "max"}
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	out := string(arts[0].Content)
	for _, want := range []string{`"thinkingLevelMap"`, `"low": "low"`, `"medium": null`, `"high": "high"`, `"max": "max"`} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
}

func TestSkipsUnsupportedReasoningEffort(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Models[0].Variants = []ir.ReasoningEffort{"low", "ultra"}
	expectValid(t, cfg)
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	out := string(arts[0].Content)
	if !strings.Contains(out, `"low": "low"`) {
		t.Fatalf("missing supported effort:\n%s", out)
	}
	if strings.Contains(out, `"ultra"`) {
		t.Fatalf("unsupported effort must be skipped:\n%s", out)
	}
}

func TestEmitsRequiredModelNameAndRejectsPDF(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Models[0].Name = ""
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(arts[0].Content), `"name": "glm-5.3"`) {
		t.Fatalf("missing fallback name:\n%s", arts[0].Content)
	}
	cfg.Providers[0].Models[0].Input = []ir.Modality{ir.ModalityPDF}
	expectWarning(t, cfg, "does not support pdf")
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
	if !strings.Contains(out, `"api": "openai-completions"`) {
		t.Fatalf("the provider must still be emitted:\n%s", out)
	}
}

func TestFiltersPDFModelInput(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Models[0].Input = []ir.Modality{ir.ModalityText, ir.ModalityPDF, ir.ModalityAudio}
	expectWarning(t, cfg, "does not support pdf")
	out := emitContent(t, cfg)
	if strings.Contains(out, `"pdf"`) {
		t.Fatalf("pdf input must be filtered:\n%s", out)
	}
	for _, want := range []string{`"text"`, `"audio"`} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
}

func TestSkipsUnknownProtocolProvider(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Protocol = "bogus"
	expectWarning(t, cfg, "api must be")
	out := emitContent(t, cfg)
	if strings.Contains(out, `"api": ""`) || strings.Contains(out, "volcengine") {
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
