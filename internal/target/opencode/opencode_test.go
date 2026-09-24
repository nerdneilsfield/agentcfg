package opencode

import (
	"strings"
	"testing"

	"agentcfg/internal/diag"
	"agentcfg/internal/ir"
)

func i64(n int64) *int64 { return &n }

func boolp(v bool) *bool { return &v }

func expectValid(t *testing.T, cfg ir.Config) {
	t.Helper()
	if diags := (Target{}).Validate(cfg); len(diags) != 0 {
		t.Fatalf("expected no diagnostics, got %v", diags)
	}
}

func expectWarning(t *testing.T, cfg ir.Config, substr string) []diag.Diagnostic {
	t.Helper()
	diags := (Target{}).Validate(cfg)
	for _, d := range diags {
		if d.Severity == diag.SeverityWarning && strings.Contains(d.Message, substr) {
			return diags
		}
	}
	t.Fatalf("no warning containing %q: %v", substr, diags)
	return nil
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
				ID:              "glm-5.3",
				Name:            "GLM-5.3",
				ContextWindow:   i64(128000),
				MaxOutputTokens: i64(8192),
				Input:           []ir.Modality{ir.ModalityText, ir.ModalityImage},
				Output:          []ir.Modality{ir.ModalityText},
				ToolCalling:     boolp(true),
				Variants:        []ir.ReasoningEffort{"low", "high", "ultra"},
			}},
		}},
		Defaults: &ir.Defaults{Model: "volcengine/glm-5.3"},
	}
}

func TestEmitsV2ProviderShape(t *testing.T) {
	expectValid(t, exampleConfig())
	arts, err := Target{}.Emit(exampleConfig())
	if err != nil {
		t.Fatal(err)
	}
	if len(arts) != 1 || arts[0].Name != "opencode.json" || arts[0].SuggestedPath != "opencode.json" {
		t.Fatalf("unexpected artifacts: %v", arts)
	}
	want := `{
  "$schema": "https://opencode.ai/config.json",
  "providers": {
    "volcengine": {
      "name": "Volcengine",
      "env": [
        "VOLC_API_KEY"
      ],
      "package": "@opencode/ai/providers/openai-compatible",
      "settings": {
        "baseURL": "https://example.com/v1"
      },
      "headers": {
        "X-Tenant": "engineering"
      },
      "models": {
        "glm-5.3": {
          "name": "GLM-5.3",
          "capabilities": {
            "tools": true,
            "input": [
              "text",
              "image"
            ],
            "output": [
              "text"
            ]
          },
          "limit": {
            "context": 128000,
            "output": 8192
          },
          "variants": [
            {
              "id": "low",
              "settings": {
                "reasoningEffort": "low"
              }
            },
            {
              "id": "high",
              "settings": {
                "reasoningEffort": "high"
              }
            },
            {
              "id": "ultra",
              "settings": {
                "reasoningEffort": "ultra"
              }
            }
          ]
        }
      }
    }
  },
  "model": "volcengine/glm-5.3"
}
`
	if got := string(arts[0].Content); got != want {
		t.Fatalf("opencode.json mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestMapsEveryProtocolToANativePackage(t *testing.T) {
	for protocol, want := range map[ir.Protocol]string{
		ir.ProtocolOpenAICompletions: "@opencode/ai/providers/openai-compatible",
		ir.ProtocolOpenAIResponses:   "@opencode/ai/providers/openai",
		ir.ProtocolAnthropicMessages: "@opencode/ai/providers/anthropic",
	} {
		t.Run(string(protocol), func(t *testing.T) {
			cfg := exampleConfig()
			cfg.Providers[0].Protocol = protocol
			expectValid(t, cfg)
			arts, err := Target{}.Emit(cfg)
			if err != nil {
				t.Fatal(err)
			}
			if got := string(arts[0].Content); !strings.Contains(got, `"package": "`+want+`"`) {
				t.Fatalf("missing package %q:\n%s", want, got)
			}
		})
	}
}

func TestEmitsMCPV2Shape(t *testing.T) {
	cfg := exampleConfig()
	cfg.MCP = []ir.MCPServer{{
		ID:        "context7",
		Transport: ir.TransportStdio,
		Command:   []string{"npx", "-y", "@upstash/context7-mcp"},
		CWD:       "/var/lib/context7",
		Env: map[string]ir.HeaderValue{
			"CONTEXT7_API_KEY": {FromEnv: "CONTEXT7_API_KEY"},
		},
		TimeoutMS: i64(30000),
	}, {
		ID:        "github",
		Transport: ir.TransportHTTP,
		URL:       "https://api.githubcopilot.com/mcp/",
		Headers: map[string]ir.HeaderValue{
			"Authorization": {BearerFromEnv: "GITHUB_TOKEN"},
		},
		Enabled: boolp(false),
	}}
	expectValid(t, cfg)
	arts, err := Target{}.Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	got := string(arts[0].Content)
	for _, want := range []string{
		`"mcp": {`,
		`"servers": {`,
		`"type": "local"`,
		`"cwd": "/var/lib/context7"`,
		`"CONTEXT7_API_KEY": "{env:CONTEXT7_API_KEY}"`,
		`"timeout": {`,
		`"catalog": 30000`,
		`"execution": 30000`,
		`"type": "remote"`,
		`"Authorization": "Bearer {env:GITHUB_TOKEN}"`,
		`"disabled": true`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("V2 MCP output missing %q:\n%s", want, got)
		}
	}
}

// V2 resolves {env:NAME} in provider header values, so a header keeps its
// reference instead of being dropped.
func TestEmitsEnvDerivedProviderHeaders(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Headers = map[string]ir.HeaderValue{
		"X-Tenant":      {Value: "engineering"},
		"X-Gateway-Key": {FromEnv: "GATEWAY_KEY"},
		"Authorization": {BearerFromEnv: "UPSTREAM_TOKEN"},
	}
	expectValid(t, cfg)
	arts, err := Target{}.Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	got := string(arts[0].Content)
	for _, want := range []string{
		`"X-Tenant": "engineering"`,
		`"X-Gateway-Key": "{env:GATEWAY_KEY}"`,
		`"Authorization": "Bearer {env:UPSTREAM_TOKEN}"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q:\n%s", want, got)
		}
	}
}

func TestSkipsModelReasoningFlag(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Models[0].Reasoning = boolp(true)
	expectWarning(t, cfg, "reasoning is expressed through variants")
	arts, err := Target{}.Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(arts[0].Content); strings.Contains(got, `"reasoning"`) {
		t.Fatalf("V2 has no model reasoning flag:\n%s", got)
	}
}

func TestSkipsLiteralNativeExpressionAPIKey(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].APIKey = ir.HeaderValue{Value: "{env:TOKEN}"}
	diags := expectWarning(t, cfg, "literal API key contains native expression")
	for _, d := range diags {
		if strings.Contains(d.String(), "{env:TOKEN}") {
			t.Fatalf("diagnostic exposed the key: %v", d)
		}
	}
	arts, err := Target{}.Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(arts[0].Content); strings.Contains(got, "apiKey") {
		t.Fatalf("an unrepresentable literal must be skipped:\n%s", got)
	}
}

func TestKeylessAuthTypeWritesNoCredential(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].AuthType = ir.AuthTypeNone
	cfg.Providers[0].APIKey = ir.HeaderValue{}
	expectValid(t, cfg)
	arts, err := Target{}.Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	got := string(arts[0].Content)
	if strings.Contains(got, `"env"`) || strings.Contains(got, `"apiKey"`) {
		t.Fatalf("auth_type none must not reference a credential:\n%s", got)
	}
}
