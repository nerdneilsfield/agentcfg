package gajae

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
	if len(arts) != 3 {
		t.Fatalf("expected 3 artifacts, got %d", len(arts))
	}
	names := []string{}
	for _, a := range arts {
		names = append(names, a.Name)
	}
	wantNames := []string{"models.yml", "config.yml", "mcp.json"}
	for i := range wantNames {
		if names[i] != wantNames[i] {
			t.Fatalf("artifact %d: want %q, got %q", i, wantNames[i], names[i])
		}
	}
	if got := string(arts[0].Content); got != `providers:
  volcengine:
    baseUrl: https://example.com/v1
    api: openai-completions
    apiKeyEnv: VOLC_API_KEY
    headers:
      X-Gateway-Key: GATEWAY_KEY
      X-Tenant: engineering
    models:
      - id: glm-5.3
        name: GLM-5.3
        contextWindow: 128000
        maxTokens: 8192
        input:
          - text
          - image
        output:
          - text
        reasoning: true
` {
		t.Fatalf("gajae models.yml mismatch\ngot:\n%s", got)
	}
	if got := string(arts[2].Content); got != `{
  "mcpServers": {
    "context7": {
      "type": "stdio",
      "command": "npx",
      "args": [
        "-y",
        "@upstash/context7-mcp"
      ],
      "env": {
        "CONTEXT7_API_KEY": "${CONTEXT7_API_KEY}"
      },
      "timeout": 60000,
      "enabled": true
    },
    "github": {
      "type": "http",
      "url": "https://api.githubcopilot.com/mcp/",
      "headers": {
        "Authorization": "Bearer ${GITHUB_TOKEN}"
      },
      "enabled": true
    }
  }
}
` {
		t.Fatalf("gajae mcp.json mismatch\ngot:\n%s", got)
	}
}

func TestRejectsProviderBearerHeader(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Headers["Authorization"] = ir.HeaderValue{BearerFromEnv: "TOKEN"}
	expectInvalid(t, cfg, "bearer-from-env")
}

func TestRejectsNonImageModality(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Models[0].Input = []ir.Modality{ir.ModalityAudio}
	expectInvalid(t, cfg, "text and image")
}
