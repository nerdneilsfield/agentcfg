package cline

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
	// cline rejects api_key_env and env-derived headers: use a clean config.
	cfg := exampleConfig()
	cfg.Providers[0].APIKey.FromEnv = ""
	delete(cfg.Providers[0].Headers, "X-Gateway-Key")
	cfg.MCP[0].Env["CONTEXT7_API_KEY"] = ir.HeaderValue{Value: "literal-key"}
	cfg.MCP[1].Headers["Authorization"] = ir.HeaderValue{Value: "Bearer literal-token"}
	expectValid(t, cfg)
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if len(arts) != 3 {
		t.Fatalf("expected 3 artifacts, got %d", len(arts))
	}
	names := []string{}
	for _, a := range arts {
		names = append(names, a.Name)
	}
	wantNames := []string{"providers.json", "models.json", "cline_mcp_settings.json"}
	for i := range wantNames {
		if names[i] != wantNames[i] {
			t.Fatalf("artifact %d: want %q, got %q", i, wantNames[i], names[i])
		}
	}
	for i, want := range []string{`{
  "version": 1,
  "lastUsedProvider": "volcengine",
  "providers": {
    "volcengine": {
      "settings": {
        "provider": "volcengine",
        "baseUrl": "https://example.com/v1",
        "protocol": "openai-chat",
        "model": "glm-5.3",
        "headers": {
          "X-Tenant": "engineering"
        }
      },
      "tokenSource": "manual"
    }
  }
}
`, `{
  "version": 1,
  "providers": {
    "volcengine": {
      "provider": {
        "name": "Volcengine",
        "baseUrl": "https://example.com/v1",
        "defaultModelId": "glm-5.3"
      },
      "models": {
        "glm-5.3": {
          "name": "GLM-5.3",
          "contextWindow": 128000,
          "maxTokens": 8192,
          "modalities": {
            "input": [
              "text",
              "image"
            ],
            "output": [
              "text"
            ]
          },
          "capabilities": [
            "images",
            "tools",
            "reasoning"
          ]
        }
      }
    }
  }
}
`, `{
  "mcpServers": {
    "context7": {
      "transport": {
        "type": "stdio",
        "command": "npx",
        "args": [
          "-y",
          "@upstash/context7-mcp"
        ],
        "env": {
          "CONTEXT7_API_KEY": "literal-key"
        }
      },
      "timeout": 60
    },
    "github": {
      "transport": {
        "type": "streamableHttp",
        "url": "https://api.githubcopilot.com/mcp/",
        "headers": {
          "Authorization": "Bearer literal-token"
        }
      }
    }
  }
}
`} {
		if got := string(arts[i].Content); got != want {
			t.Fatalf("cline artifact %d mismatch\ngot:\n%s\nwant:\n%s", i, got, want)
		}
	}
}

func TestRejectsAPIKeyEnv(t *testing.T) {
	expectInvalid(t, exampleConfig(), "literal apiKey")
}

func TestRejectsEnvDerivedProviderHeaders(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].APIKey.FromEnv = ""
	expectInvalid(t, cfg, "literal strings")
}

func TestRejectsMCPEnvRef(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].APIKey.FromEnv = ""
	delete(cfg.Providers[0].Headers, "X-Gateway-Key")
	expectInvalid(t, cfg, "literal strings")
}

func TestRejectsTimeoutOutOfRange(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].APIKey.FromEnv = ""
	delete(cfg.Providers[0].Headers, "X-Gateway-Key")
	cfg.MCP[0].Env["CONTEXT7_API_KEY"] = ir.HeaderValue{Value: "literal-key"}
	cfg.MCP[0].TimeoutMS = i64(10)
	expectInvalid(t, cfg, "out of range")
}

func TestOmitsMCPArtifactWhenEmpty(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].APIKey.FromEnv = ""
	delete(cfg.Providers[0].Headers, "X-Gateway-Key")
	cfg.MCP = nil
	expectValid(t, cfg)
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if len(arts) != 2 {
		t.Fatalf("expected 2 artifacts without MCP, got %d", len(arts))
	}
}

func TestRejectsModelReasoningVariants(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Models[0].Reasoning = b(true)
	cfg.Providers[0].Models[0].Variants = []ir.ReasoningEffort{"low", "ultra"}
	expectInvalid(t, cfg, "no model-level reasoning effort list; settings.reasoning is provider-wide")
}
