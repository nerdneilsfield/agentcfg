package omp

import (
	"strings"
	"testing"

	"agentcfg/internal/diag"
	"agentcfg/internal/ir"
	"agentcfg/internal/target"
)

func i64(n int64) *int64 { return &n }
func boolp(v bool) *bool { return &v }

func exampleConfig() ir.Config {
	return ir.Config{
		Version: 1,
		Providers: []ir.Provider{{
			ID:       "relay",
			Name:     "Relay",
			Protocol: ir.ProtocolAnthropicMessages,
			BaseURL:  "https://relay.example",
			APIKey:   ir.HeaderValue{FromEnv: "RELAY_TOKEN"},
			AuthType: ir.AuthTypeBearer,
			Models: []ir.Model{{
				ID:            "claude-sonnet-4-5",
				ContextWindow: i64(200000),
			}},
		}},
		MCP: []ir.MCPServer{{
			ID:        "github",
			Transport: ir.TransportHTTP,
			URL:       "https://api.githubcopilot.com/mcp/",
			Headers: map[string]ir.HeaderValue{
				"Authorization": {BearerFromEnv: "GITHUB_TOKEN"},
				"X-Region":      {Value: "us-east-1"},
			},
		}},
		Defaults: &ir.Defaults{Model: "relay/claude-sonnet-4-5"},
	}
}

func emit(t *testing.T, cfg ir.Config) map[string]string {
	t.Helper()
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	out := make(map[string]string, len(arts))
	for _, a := range arts {
		out[a.Name] = string(a.Content)
	}
	return out
}

func expectValid(t *testing.T, cfg ir.Config) {
	t.Helper()
	if diags := (Target{}).Validate(cfg); len(diags) != 0 {
		t.Fatalf("expected valid config, got %v", diags)
	}
	if diags := target.AuthTypeDiagnostics(Target{}, cfg); diag.HasErrors(diags) {
		t.Fatalf("expected no auth_type diagnostics, got %v", diags)
	}
}

func expectWarning(t *testing.T, cfg ir.Config, substr string) {
	t.Helper()
	diags := (Target{}).Validate(cfg)
	if len(diags) == 0 {
		t.Fatalf("expected a warning containing %q, got none", substr)
	}
	for _, d := range diags {
		if d.Severity == diag.SeverityWarning && strings.Contains(d.Message, substr) {
			return
		}
	}
	t.Fatalf("no diagnostic containing %q: %v", substr, diags)
}

func TestEmitsModelsDocument(t *testing.T) {
	expectValid(t, exampleConfig())
	arts := emit(t, exampleConfig())
	models, ok := arts["models.yml"]
	if !ok {
		t.Fatalf("missing models.yml: %v", arts)
	}
	for _, want := range []string{
		"api: anthropic-messages",
		"apiKey: RELAY_TOKEN",
		"authHeader: true",
		"baseUrl: https://relay.example",
	} {
		if !strings.Contains(models, want) {
			t.Errorf("missing %q in models.yml:\n%s", want, models)
		}
	}
	if strings.Contains(models, "$RELAY_TOKEN") {
		t.Errorf("Oh My Pi resolves whole values as environment names, not $NAME:\n%s", models)
	}
}

func TestEmitsThinkingEfforts(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Models[0].Reasoning = boolp(true)
	cfg.Providers[0].Models[0].Variants = []ir.ReasoningEffort{"low", "high", "ultra"}
	expectValid(t, cfg)
	models := emit(t, cfg)["models.yml"]
	for _, want := range []string{"thinking:", "mode: effort"} {
		if !strings.Contains(models, want) {
			t.Errorf("missing %q in models.yml:\n%s", want, models)
		}
	}
	if !strings.Contains(models, "- low") || !strings.Contains(models, "- high") {
		t.Errorf("declared efforts missing:\n%s", models)
	}
	if strings.Contains(models, "ultra") {
		t.Errorf("an effort outside the native vocabulary must be skipped:\n%s", models)
	}
}

func TestMCPArtifactOnlyWhenDeclared(t *testing.T) {
	if _, ok := emit(t, exampleConfig())["mcp.json"]; !ok {
		t.Fatal("expected mcp.json when the IR declares MCP servers")
	}

	cfg := exampleConfig()
	cfg.MCP = nil
	if _, ok := emit(t, cfg)["mcp.json"]; ok {
		t.Fatal("mcp.json must be omitted when the IR declares no MCP servers")
	}
}

func TestMCPRendersBearerAndTimeout(t *testing.T) {
	cfg := exampleConfig()
	cfg.MCP[0].TimeoutMS = i64(45000)
	expectValid(t, cfg)
	mcp := emit(t, cfg)["mcp.json"]
	for _, want := range []string{
		`"Authorization": "Bearer ${GITHUB_TOKEN}"`,
		`"X-Region": "us-east-1"`,
		`"timeout": 45000`,
		`"type": "http"`,
	} {
		if !strings.Contains(mcp, want) {
			t.Errorf("missing %q in mcp.json:\n%s", want, mcp)
		}
	}
}

func TestRoleArtifactOnlyWhenDefaultSet(t *testing.T) {
	roles := emit(t, exampleConfig())["config.yml"]
	if !strings.Contains(roles, "default: relay/claude-sonnet-4-5") {
		t.Fatalf("missing modelRoles.default:\n%s", roles)
	}

	cfg := exampleConfig()
	cfg.Defaults = nil
	if _, ok := emit(t, cfg)["config.yml"]; ok {
		t.Fatal("config.yml must be omitted when the IR sets no default model")
	}
}

func TestSkipsLiteralExpressionKeys(t *testing.T) {
	for _, key := range []string{"!op read op://x/y", "$TOKEN"} {
		cfg := exampleConfig()
		cfg.Providers[0].APIKey = ir.HeaderValue{Value: key}
		if diags := (Target{}).Validate(cfg); len(diags) == 0 {
			t.Fatalf("expected a diagnostic for literal %q", key)
		}
	}
}

func TestSkipsUnsupportedModelInput(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Models[0].Input = []ir.Modality{ir.ModalityPDF}
	if diags := (Target{}).Validate(cfg); len(diags) == 0 {
		t.Fatal("expected a diagnostic for pdf input")
	}
	models := emit(t, cfg)["models.yml"]
	if strings.Contains(models, "pdf") {
		t.Fatalf("unsupported modality must be dropped:\n%s", models)
	}
	if !strings.Contains(models, "claude-sonnet-4-5") {
		t.Fatalf("model entry must survive the omission:\n%s", models)
	}
}

func TestDropsUnsupportedModelModalities(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Models[0].Input = []ir.Modality{ir.ModalityText, ir.ModalityAudio}
	if diags := (Target{}).Validate(cfg); len(diags) == 0 {
		t.Fatal("expected a diagnostic for audio input")
	}
	models := emit(t, cfg)["models.yml"]
	if strings.Contains(models, "audio") {
		t.Fatalf("unsupported modality must be dropped:\n%s", models)
	}
	if !strings.Contains(models, "text") {
		t.Fatalf("supported modality must remain:\n%s", models)
	}
}

func TestOmitsLiteralExpressionAPIKey(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].APIKey = ir.HeaderValue{Value: "!command"}
	expectWarning(t, cfg, "native expression syntax")
	models := emit(t, cfg)["models.yml"]
	if strings.Contains(models, "apiKey") || strings.Contains(models, "!command") {
		t.Fatalf("unrepresentable literal key must be omitted:\n%s", models)
	}
	if !strings.Contains(models, "relay") {
		t.Fatalf("provider entry must survive the omission:\n%s", models)
	}
}

func TestSkipsStdioMCPWithoutCommand(t *testing.T) {
	cfg := exampleConfig()
	cfg.MCP = append(cfg.MCP, ir.MCPServer{ID: "broken", Transport: ir.TransportStdio})
	expectWarning(t, cfg, "needs a command")
	mcp := emit(t, cfg)["mcp.json"]
	if strings.Contains(mcp, "broken") {
		t.Fatalf("commandless stdio server must be skipped:\n%s", mcp)
	}
	if !strings.Contains(mcp, "github") {
		t.Fatalf("other MCP servers must still emit:\n%s", mcp)
	}
}
