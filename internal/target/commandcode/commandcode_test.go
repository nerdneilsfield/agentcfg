package commandcode

import (
	"strings"
	"testing"

	"agentcfg/internal/artifact"
	"agentcfg/internal/diag"
	"agentcfg/internal/ir"
)

func i64(n int64) *int64 { return &n }

func b(v bool) *bool { return &v }

// Unsupported fields are skipped, never fatal: generation continues and the
// diagnostic is a warning. The fixture deliberately carries skipped fields, so
// this asserts only that nothing fails.
func expectNoErrors(t *testing.T, cfg ir.Config) {
	t.Helper()
	if diags := (Target{}).Validate(cfg); diag.HasErrors(diags) {
		t.Fatalf("expected no errors, got %v", diags)
	}
}

func expectWarning(t *testing.T, cfg ir.Config, substr string) {
	t.Helper()
	for _, d := range (Target{}).Validate(cfg) {
		if d.Severity == diag.SeverityWarning && strings.Contains(d.Message, substr) {
			return
		}
	}
	t.Fatalf("no warning containing %q: %v", substr, (Target{}).Validate(cfg))
}

func artifactByName(t *testing.T, arts []artifact.Artifact, name string) artifact.Artifact {
	t.Helper()
	for _, a := range arts {
		if a.Name == name {
			return a
		}
	}
	t.Fatalf("no %q artifact in %v", name, arts)
	return artifact.Artifact{}
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
				Reasoning:       b(true),
				Variants:        []ir.ReasoningEffort{"low", "ultra", "high"},
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
			CWD:       "/var/lib/context7",
			TimeoutMS: i64(30000),
		}, {
			ID:        "github",
			Transport: ir.TransportHTTP,
			URL:       "https://api.githubcopilot.com/mcp/",
			Headers: map[string]ir.HeaderValue{
				"Authorization": {BearerFromEnv: "GITHUB_TOKEN"},
			},
			Enabled: b(false),
		}},
		Defaults: &ir.Defaults{Model: "volcengine/glm-5.3"},
	}
}

func TestEmitsProviderFragment(t *testing.T) {
	expectNoErrors(t, exampleConfig())
	arts, err := Target{}.Emit(exampleConfig())
	if err != nil {
		t.Fatal(err)
	}
	providers := artifactByName(t, arts, "providers.json")
	if providers.Format != "json" || providers.SuggestedPath != "~/.commandcode/providers.json" {
		t.Fatalf("unexpected artifact metadata: %+v", providers)
	}
	want := `{
  "provider": {
    "volcengine": {
      "name": "Volcengine",
      "baseURL": "https://example.com/v1",
      "apiKey": "$VOLC_API_KEY",
      "headers": {
        "X-Tenant": "engineering"
      },
      "models": {
        "glm-5.3": {
          "name": "GLM-5.3",
          "contextWindow": 128000,
          "maxOutput": 8192,
          "reasoning": true,
          "reasoningEfforts": [
            "low",
            "high"
          ]
        }
      }
    }
  }
}
`
	if got := string(providers.Content); got != want {
		t.Fatalf("providers.json mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestEmitsMCPFragment(t *testing.T) {
	arts, err := Target{}.Emit(exampleConfig())
	if err != nil {
		t.Fatal(err)
	}
	mcp := artifactByName(t, arts, ".mcp.json")
	if mcp.Format != "json" || mcp.SuggestedPath != ".mcp.json" {
		t.Fatalf("unexpected artifact metadata: %+v", mcp)
	}
	want := `{
  "mcpServers": {
    "context7": {
      "transport": "stdio",
      "enabled": true,
      "command": "npx",
      "args": [
        "-y",
        "@upstash/context7-mcp"
      ],
      "env": {
        "CONTEXT7_API_KEY": "${CONTEXT7_API_KEY}"
      }
    },
    "github": {
      "transport": "http",
      "enabled": false,
      "url": "https://api.githubcopilot.com/mcp/",
      "headers": {
        "Authorization": "Bearer ${GITHUB_TOKEN}"
      }
    }
  }
}
`
	if got := string(mcp.Content); got != want {
		t.Fatalf(".mcp.json mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// The native settings writer persists one qualified reference, and the reader
// takes that value verbatim.
func TestEmitsDefaultModel(t *testing.T) {
	arts, err := Target{}.Emit(exampleConfig())
	if err != nil {
		t.Fatal(err)
	}
	settings := artifactByName(t, arts, "config.json")
	if settings.Format != "json" || settings.SuggestedPath != "~/.commandcode/config.json" {
		t.Fatalf("unexpected artifact metadata: %+v", settings)
	}
	want := "{\n  \"model\": \"volcengine/glm-5.3\"\n}\n"
	if got := string(settings.Content); got != want {
		t.Fatalf("config.json mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestOmitsOptionalArtifacts(t *testing.T) {
	cfg := exampleConfig()
	cfg.MCP = nil
	cfg.Defaults = nil
	arts, err := Target{}.Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(arts) != 1 || arts[0].Name != "providers.json" {
		t.Fatalf("expected only the provider fragment, got %v", arts)
	}
}

func TestSkipsLiteralAPIKey(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].APIKey = ir.HeaderValue{Value: "sk-example-not-a-real-key"}
	expectWarning(t, cfg, "refuses a raw secret")
	arts, err := Target{}.Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	got := string(artifactByName(t, arts, "providers.json").Content)
	if strings.Contains(got, "sk-example-not-a-real-key") || strings.Contains(got, "apiKey") {
		t.Fatalf("a raw secret must not reach providers.json: %s", got)
	}
	if !strings.Contains(got, `"baseURL": "https://example.com/v1"`) {
		t.Fatalf("the rest of the provider must survive the skip: %s", got)
	}
}

func TestSkipsReservedProviderID(t *testing.T) {
	for _, id := range []string{"anthropic", "copilot", "github-copilot", "codex", "openai", "command-code"} {
		cfg := exampleConfig()
		cfg.Providers[0].ID = id
		expectWarning(t, cfg, "built-in subscription lane")
		arts, err := Target{}.Emit(cfg)
		if err != nil {
			t.Fatal(err)
		}
		got := string(artifactByName(t, arts, "providers.json").Content)
		if strings.Contains(got, id+`"`) {
			t.Fatalf("reserved provider %q must be skipped: %s", id, got)
		}
	}
}

func TestSkipsEnvDerivedProviderHeaders(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Headers = map[string]ir.HeaderValue{
		"X-Tenant":      {Value: "engineering"},
		"X-Gateway-Key": {FromEnv: "GATEWAY_KEY"},
	}
	expectWarning(t, cfg, "X-Gateway-Key")
	arts, err := Target{}.Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	got := string(artifactByName(t, arts, "providers.json").Content)
	if strings.Contains(got, "GATEWAY_KEY") {
		t.Fatalf("an env-derived header cannot be interpolated and must be skipped: %s", got)
	}
	if !strings.Contains(got, `"X-Tenant": "engineering"`) {
		t.Fatalf("literal headers must survive: %s", got)
	}
}

func TestEmitsKeylessProviderForAuthTypeNone(t *testing.T) {
	cfg := exampleConfig()
	disabled := false
	cfg.Providers[0].AuthType = ir.AuthTypeNone
	cfg.Providers[0].APIKey = ir.HeaderValue{}
	cfg.Providers[0].Models[0].Reasoning = &disabled
	expectNoErrors(t, cfg)
	arts, err := Target{}.Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	got := string(artifactByName(t, arts, "providers.json").Content)
	if !strings.Contains(got, `"apiKey": false`) {
		t.Fatalf("auth_type none must render a keyless provider: %s", got)
	}
	if !strings.Contains(got, `"reasoning": false`) {
		t.Fatalf("explicit reasoning false must survive: %s", got)
	}
}

func TestSkipsEffortsOutsideTheNativeLadder(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Models[0].Variants = []ir.ReasoningEffort{"ultra"}
	expectNoErrors(t, cfg)
	expectWarning(t, cfg, "these names are skipped: ultra")
	arts, err := Target{}.Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	got := string(artifactByName(t, arts, "providers.json").Content)
	if strings.Contains(got, "reasoningEfforts") || strings.Contains(got, "ultra") {
		t.Fatalf("out-of-ladder effort must be skipped, not remapped: %s", got)
	}
	if !strings.Contains(got, `"reasoning": true`) {
		t.Fatalf("the model's reasoning flag must survive: %s", got)
	}
}

func TestWarnsForFieldsWithoutANativeHome(t *testing.T) {
	for _, want := range []string{
		"input and output modalities are skipped",
		"tool_calling is skipped",
		"cwd is skipped",
		"timeout_ms is skipped",
	} {
		expectWarning(t, exampleConfig(), want)
	}
}

func TestKeepsAnthropicWireExplicit(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Protocol = ir.ProtocolAnthropicMessages
	expectNoErrors(t, cfg)
	arts, err := Target{}.Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	got := string(artifactByName(t, arts, "providers.json").Content)
	if !strings.Contains(got, `"api": "anthropic-messages"`) {
		t.Fatalf("non-default wire must be explicit: %s", got)
	}
}
