package cometixcode

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
			ID:       "claude-proxy",
			Name:     "Claude Proxy",
			Protocol: ir.ProtocolAnthropicMessages,
			BaseURL:  "https://anthropic-proxy.example.com",
			APIKey:   ir.HeaderValue{Value: "sk-example-not-a-real-key"},
			Headers: map[string]ir.HeaderValue{
				"X-Tenant": {Value: "engineering"},
			},
			Models: []ir.Model{{
				ID:            "glm-5.3",
				Name:          "GLM-5.3",
				ContextWindow: i64(128000),
			}},
		}},
		MCP: []ir.MCPServer{{
			ID:        "context7",
			Transport: ir.TransportStdio,
			Command:   []string{"npx", "-y", "@upstash/context7-mcp"},
			Env: map[string]ir.HeaderValue{
				"CONTEXT7_API_KEY": {FromEnv: "CONTEXT7_API_KEY"},
			},
		}, {
			ID:        "github",
			Transport: ir.TransportHTTP,
			URL:       "https://api.githubcopilot.com/mcp/",
			Headers: map[string]ir.HeaderValue{
				"Authorization": {BearerFromEnv: "GITHUB_TOKEN"},
			},
			Enabled: b(false),
		}},
		Defaults: &ir.Defaults{Model: "claude-proxy/glm-5.3"},
	}
}

func TestEmitsSettingsFragment(t *testing.T) {
	expectNoErrors(t, exampleConfig())
	arts, err := Target{}.Emit(exampleConfig())
	if err != nil {
		t.Fatal(err)
	}
	settings := artifactByName(t, arts, "settings.json")
	if settings.Format != "json" || settings.SuggestedPath != "~/.claude/settings.json" {
		t.Fatalf("unexpected artifact metadata: %+v", settings)
	}
	want := `{
  "$schema": "https://json.schemastore.org/claude-code-settings.json",
  "env": {
    "ANTHROPIC_API_KEY": "sk-example-not-a-real-key",
    "ANTHROPIC_BASE_URL": "https://anthropic-proxy.example.com",
    "ANTHROPIC_CUSTOM_HEADERS": "X-Tenant: engineering",
    "ANTHROPIC_MODEL": "glm-5.3"
  },
  "disabledMcpjsonServers": [
    "github"
  ]
}
`
	if got := string(settings.Content); got != want {
		t.Fatalf("settings.json mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestMapsCredentialPerAuthType(t *testing.T) {
	for authType, want := range map[ir.AuthType]string{
		"":                "ANTHROPIC_API_KEY",
		ir.AuthTypeBearer: "ANTHROPIC_AUTH_TOKEN",
	} {
		t.Run(string(authType)+"|"+want, func(t *testing.T) {
			cfg := exampleConfig()
			cfg.Providers[0].AuthType = authType
			expectNoErrors(t, cfg)
			arts, err := Target{}.Emit(cfg)
			if err != nil {
				t.Fatal(err)
			}
			got := string(artifactByName(t, arts, "settings.json").Content)
			if !strings.Contains(got, `"`+want+`": "sk-example-not-a-real-key"`) {
				t.Fatalf("expected %s to carry the credential: %s", want, got)
			}
			other := "ANTHROPIC_API_KEY"
			if want == other {
				other = "ANTHROPIC_AUTH_TOKEN"
			}
			if strings.Contains(got, other) {
				t.Fatalf("%s must not be written as well: %s", other, got)
			}
		})
	}
}

func TestKeylessAuthTypeWritesNoCredential(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].AuthType = ir.AuthTypeNone
	cfg.Providers[0].APIKey = ir.HeaderValue{}
	expectNoErrors(t, cfg)
	arts, err := Target{}.Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	got := string(artifactByName(t, arts, "settings.json").Content)
	if strings.Contains(got, "ANTHROPIC_API_KEY") || strings.Contains(got, "ANTHROPIC_AUTH_TOKEN") {
		t.Fatalf("auth_type none must not write a credential: %s", got)
	}
}

func TestSelectsTheProviderTheDefaultModelNames(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers = append([]ir.Provider{{
		ID:       "other-proxy",
		Protocol: ir.ProtocolAnthropicMessages,
		BaseURL:  "https://other.example.com",
		APIKey:   ir.HeaderValue{Value: "sk-other"},
		Models:   []ir.Model{{ID: "other-model"}},
	}}, cfg.Providers...)
	expectWarning(t, cfg, "only the selected provider is emitted")
	arts, err := Target{}.Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	got := string(artifactByName(t, arts, "settings.json").Content)
	if !strings.Contains(got, `"ANTHROPIC_BASE_URL": "https://anthropic-proxy.example.com"`) {
		t.Fatalf("defaults.model must select the provider: %s", got)
	}
	if strings.Contains(got, "other.example.com") || strings.Contains(got, "sk-other") {
		t.Fatalf("the unselected provider must be skipped: %s", got)
	}
}

func TestSkipsProvidersOnAnotherProtocol(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers = append(cfg.Providers, ir.Provider{
		ID:       "volcengine",
		Protocol: ir.ProtocolOpenAICompletions,
		BaseURL:  "https://example.com/v1",
		APIKey:   ir.HeaderValue{Value: "sk-volc"},
		Models:   []ir.Model{{ID: "glm-5.3"}},
	})
	expectWarning(t, cfg, "CometixCode speaks the Anthropic Messages API")
	arts, err := Target{}.Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	got := string(artifactByName(t, arts, "settings.json").Content)
	if strings.Contains(got, "example.com/v1") || strings.Contains(got, "sk-volc") {
		t.Fatalf("a non-Anthropic provider must be skipped: %s", got)
	}
}

func TestEmitsMCPFragment(t *testing.T) {
	arts, err := Target{}.Emit(exampleConfig())
	if err != nil {
		t.Fatal(err)
	}
	mcp := artifactByName(t, arts, ".mcp.json")
	if mcp.SuggestedPath != ".mcp.json" {
		t.Fatalf("unexpected suggested path: %q", mcp.SuggestedPath)
	}
	want := `{
  "mcpServers": {
    "context7": {
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
      "type": "http",
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

func TestSkipsEnvDerivedCredentialAndHeaders(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].APIKey = ir.HeaderValue{FromEnv: "PROXY_TOKEN"}
	cfg.Providers[0].Headers = map[string]ir.HeaderValue{
		"X-Tenant":      {Value: "engineering"},
		"X-Gateway-Key": {FromEnv: "GATEWAY_KEY"},
	}
	expectWarning(t, cfg, "settings.json env values are literal")
	expectWarning(t, cfg, "X-Gateway-Key")
	arts, err := Target{}.Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	got := string(artifactByName(t, arts, "settings.json").Content)
	if strings.Contains(got, "PROXY_TOKEN") || strings.Contains(got, "GATEWAY_KEY") {
		t.Fatalf("env references cannot be interpolated here: %s", got)
	}
	if !strings.Contains(got, `"ANTHROPIC_CUSTOM_HEADERS": "X-Tenant: engineering"`) {
		t.Fatalf("literal headers must survive: %s", got)
	}
}

func TestWarnsForFieldsWithoutANativeHome(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Models[0].Reasoning = b(true)
	cfg.Providers[0].Models[0].Variants = []ir.ReasoningEffort{"low", "high"}
	cfg.Providers[0].Models[0].ToolCalling = b(true)
	cfg.Providers[0].Models[0].Input = []ir.Modality{ir.ModalityText}
	cfg.MCP[0].CWD = "/var/lib/context7"
	cfg.MCP[0].TimeoutMS = i64(30000)
	expectWarning(t, cfg, "they are skipped: context_window, input/output, reasoning, variants, tool_calling")
	expectWarning(t, cfg, "cwd is skipped")
	expectWarning(t, cfg, "timeout_ms is skipped")
}
