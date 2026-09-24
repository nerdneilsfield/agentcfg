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

func TestSkipsProviderBearerHeader(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Headers["Authorization"] = ir.HeaderValue{BearerFromEnv: "TOKEN"}
	expectWarning(t, cfg, "bearer-from-env")
}

func TestSkipsNonImageModality(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Models[0].Input = []ir.Modality{ir.ModalityAudio}
	expectWarning(t, cfg, "text and image")
}

func TestEmitsEffortThinkingLevels(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Models[0].Variants = []ir.ReasoningEffort{"low", "high", "max"}
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	out := string(arts[0].Content)
	for _, want := range []string{"thinking:", "mode: effort", "minLevel: low", "maxLevel: max", "supportsReasoningEffort: true", "levels:", "- max"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "defaultLevel") {
		t.Fatalf("must not select default level:\n%s", out)
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
	if !strings.Contains(out, "minLevel: low") || strings.Contains(out, "ultra") {
		t.Fatalf("unsupported effort was not skipped:\n%s", out)
	}
}

func TestSkipsBearerProviderHeaderOutput(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Headers = map[string]ir.HeaderValue{"Authorization": {BearerFromEnv: "TOKEN"}}
	expectWarning(t, cfg, "bearer-from-env")
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	out := string(arts[0].Content)
	if strings.Contains(out, "Authorization") || strings.Contains(out, `""`) {
		t.Fatalf("a flat string cannot carry a bearer reference:\n%s", out)
	}
	if !strings.Contains(out, "volcengine") {
		t.Fatalf("provider entry must survive the omission:\n%s", out)
	}
}

func TestDropsUnsupportedModalities(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Models[0].Input = []ir.Modality{ir.ModalityText, ir.ModalityAudio}
	expectWarning(t, cfg, "text and image")
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	out := string(arts[0].Content)
	if strings.Contains(out, "audio") {
		t.Fatalf("unsupported modality must be dropped:\n%s", out)
	}
	if !strings.Contains(out, "- text") {
		t.Fatalf("supported modality must remain:\n%s", out)
	}
}

func TestSkipsUnmappableProtocolProvider(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Protocol = ir.Protocol("gemini")
	expectWarning(t, cfg, "gajae api must be")
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if out := string(arts[0].Content); strings.Contains(out, "volcengine") {
		t.Fatalf("provider with no representable api must be skipped:\n%s", out)
	}
}

func TestSkipsStdioMCPWithoutCommand(t *testing.T) {
	cfg := exampleConfig()
	cfg.MCP[0].Command = nil
	expectWarning(t, cfg, "needs a command")
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	out := string(arts[2].Content)
	if strings.Contains(out, "context7") {
		t.Fatalf("commandless stdio server must be skipped:\n%s", out)
	}
	if !strings.Contains(out, "github") {
		t.Fatalf("other MCP servers must still emit:\n%s", out)
	}
}
