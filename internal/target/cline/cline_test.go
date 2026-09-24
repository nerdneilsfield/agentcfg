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
      "updatedAt": "1970-01-01T00:00:00Z",
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

func TestSkipsAPIKeyEnv(t *testing.T) {
	expectWarning(t, exampleConfig(), "literal apiKey")
	arts, err := (Target{}).Emit(exampleConfig())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(arts[0].Content), `"apiKey"`) {
		t.Fatalf("an env-derived api_key must be skipped:\n%s", arts[0].Content)
	}
}

func TestSkipsEnvDerivedProviderHeaders(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].APIKey.FromEnv = ""
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
	cfg.Providers[0].APIKey.FromEnv = ""
	delete(cfg.Providers[0].Headers, "X-Gateway-Key")
	expectWarning(t, cfg, "literal strings")
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	out := string(arts[2].Content)
	if strings.Contains(out, "CONTEXT7_API_KEY") {
		t.Fatalf("an env-derived MCP env value must be skipped:\n%s", out)
	}
	if !strings.Contains(out, `"context7"`) {
		t.Fatalf("the MCP entry itself must be kept:\n%s", out)
	}
}

func TestSkipsMCPHeaderRef(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].APIKey.FromEnv = ""
	delete(cfg.Providers[0].Headers, "X-Gateway-Key")
	cfg.MCP[1].Headers["X-Gateway-Token"] = ir.HeaderValue{FromEnv: "GW_TOKEN"}
	expectWarning(t, cfg, "literal strings")
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	out := string(arts[2].Content)
	if strings.Contains(out, "X-Gateway-Token") {
		t.Fatalf("an env-derived MCP header must be skipped:\n%s", out)
	}
}

func TestPreservesFractionalMCPTimeout(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].APIKey.FromEnv = ""
	delete(cfg.Providers[0].Headers, "X-Gateway-Key")
	cfg.MCP[0].Env["CONTEXT7_API_KEY"] = ir.HeaderValue{Value: "literal-key"}
	cfg.MCP[1].Headers["Authorization"] = ir.HeaderValue{Value: "Bearer literal-token"}
	cfg.MCP[0].TimeoutMS = i64(1500)
	expectValid(t, cfg)
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(arts[2].Content), `"timeout": 1.5`) {
		t.Fatalf("fractional timeout lost:\n%s", arts[2].Content)
	}
}

func TestSkipsToolCallingFalse(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].APIKey.FromEnv = ""
	delete(cfg.Providers[0].Headers, "X-Gateway-Key")
	cfg.Providers[0].Models[0].ToolCalling = b(false)
	expectWarning(t, cfg, "cannot disable tools")
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	out := string(arts[1].Content)
	if strings.Contains(out, `"tools"`) {
		t.Fatalf("tool_calling: false must not be reported as a tools capability:\n%s", out)
	}
	if !strings.Contains(out, `"glm-5.3"`) {
		t.Fatalf("the model entry itself must be kept:\n%s", out)
	}
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

func TestIgnoresModelReasoningVariants(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].APIKey = ir.HeaderValue{Value: "literal"}
	cfg.Providers[0].Headers = map[string]ir.HeaderValue{"X-Tenant": {Value: "literal"}}
	cfg.MCP = nil
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
