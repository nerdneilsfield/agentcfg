package zcode

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
				"CONTEXT7_API_KEY": {Value: "literal-key"},
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
	want := `{
  "provider": {
    "volcengine": {
      "kind": "openai-compatible",
      "name": "Volcengine",
      "options": {
        "baseURL": "https://example.com/v1",
        "headers": {
          "X-Tenant": "engineering"
        }
      },
      "models": {
        "glm-5.3": {
          "name": "GLM-5.3",
          "limit": {
            "context": 128000,
            "output": 8192
          },
          "modalities": {
            "input": [
              "text"
            ],
            "output": [
              "text"
            ]
          },
          "reasoning": true,
          "tool_call": true
        }
      }
    }
  },
  "model": {
    "main": "volcengine/glm-5.3"
  },
  "mcp": {
    "servers": {
      "context7": {
        "args": [
          "-y",
          "@upstash/context7-mcp"
        ],
        "command": "npx",
        "enabled": true,
        "env": {
          "CONTEXT7_API_KEY": "literal-key"
        },
        "type": "stdio"
      }
    }
  }
}
`
	if got := string(arts[0].Content); got != want {
		t.Fatalf("zcode JSON mismatch\ngot:\n%s\nwant:\n%s", got, want)
	}
	if arts[0].SuggestedPath != "~/.zcode/cli/config.json" {
		t.Fatalf("unexpected suggested path: %s", arts[0].SuggestedPath)
	}
}

func TestRejectsResponsesProtocol(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Protocol = ir.ProtocolOpenAIResponses
	expectInvalid(t, cfg, "no native location")
}

func TestRejectsAPIKeyEnv(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].APIKeyEnv = "VOLC_API_KEY"
	expectInvalid(t, cfg, "api_key_env")
}

func TestRejectsMCPEnvRef(t *testing.T) {
	cfg := exampleConfig()
	cfg.MCP[0].Env["CONTEXT7_API_KEY"] = ir.HeaderValue{FromEnv: "CONTEXT7_API_KEY"}
	expectInvalid(t, cfg, "literal strings")
}

func TestOmitsEmptyMCP(t *testing.T) {
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
