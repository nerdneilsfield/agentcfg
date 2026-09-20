package primeagent

import (
	"strings"
	"testing"

	"agentcfg/internal/diag"
	"agentcfg/internal/ir"
)

func i64(n int64) *int64 { return &n }

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
			Protocol: ir.ProtocolOpenAICompletions,
			BaseURL:  "https://example.com/v1",
			APIKey:   ir.HeaderValue{FromEnv: "VOLC_API_KEY"},
			Models: []ir.Model{{
				ID:            "glm-5.3",
				ContextWindow: i64(128000),
			}},
		}},
		MCP: []ir.MCPServer{{
			ID:        "context7",
			Transport: ir.TransportStdio,
			Command:   []string{"npx", "-y", "@upstash/context7-mcp"},
			Env: map[string]ir.HeaderValue{
				"CHILD_KEY": {FromEnv: "SOURCE_ENV"},
			},
		}, {
			ID:        "github",
			Transport: ir.TransportHTTP,
			URL:       "https://api.githubcopilot.com/mcp/",
			Headers: map[string]ir.HeaderValue{
				"Authorization": {BearerFromEnv: "GITHUB_TOKEN"},
				"X-Region":      {Value: "us-east-1"},
			},
		}},
	}
}

func TestEmitsMcpServersKey(t *testing.T) {
	expectValid(t, exampleConfig())
	arts, err := (Target{}).Emit(exampleConfig())
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if len(arts) != 2 {
		t.Fatalf("expected 2 artifacts, got %d", len(arts))
	}
	settings := string(arts[1].Content)
	for _, want := range []string{
		`"mcpServers"`,
		`"type": "stdio"`,
		`"CHILD_KEY"`,
		`"env": "SOURCE_ENV"`,
		`"type": "http"`,
		`"bearerTokenEnvVar": "GITHUB_TOKEN"`,
		`"X-Region": "us-east-1"`,
	} {
		if !strings.Contains(settings, want) {
			t.Errorf("missing %q in settings.json:\n%s", want, settings)
		}
	}
	if strings.Contains(settings, `"mcp"`) && !strings.Contains(settings, `"mcpServers"`) {
		t.Errorf("settings.json uses wrong MCP key:\n%s", settings)
	}
}

func TestProviderEnvironmentReferences(t *testing.T) {
	cfg := exampleConfig()
	cfg.MCP = nil
	cfg.Providers[0].Headers = map[string]ir.HeaderValue{"X-Tenant": {FromEnv: "TENANT"}}
	expectValid(t, cfg)
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	out := string(arts[0].Content)
	for _, want := range []string{`"apiKey": "VOLC_API_KEY"`, `"headers"`, `"X-Tenant": "TENANT"`} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
}

func TestRejectsUnsupportedModelInput(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Models[0].Input = []ir.Modality{ir.ModalityAudio}
	expectInvalid(t, cfg, "text and image only")
}

func TestEmitsThinkingLevelMap(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Models[0].Reasoning = boolp(true)
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

func boolp(v bool) *bool { return &v }

func TestSkipsUnsupportedReasoningEffort(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Models[0].Reasoning = boolp(true)
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

func TestRejectsProviderBearerHeader(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Headers = map[string]ir.HeaderValue{"Authorization": {BearerFromEnv: "TOK"}}
	expectInvalid(t, cfg, "Bearer ENV:NAME is not representable")
}

func TestRejectsMCPTimeout(t *testing.T) {
	cfg := exampleConfig()
	cfg.MCP[0].TimeoutMS = i64(5000)
	expectInvalid(t, cfg, "does not map MCP timeout_ms")
}

func TestOmitsAuthorizationHeaderWhenBearerEnvVarIsSet(t *testing.T) {
	expectValid(t, exampleConfig())
	arts, err := (Target{}).Emit(exampleConfig())
	if err != nil {
		t.Fatal(err)
	}
	settings := string(arts[1].Content)
	if !strings.Contains(settings, `"bearerTokenEnvVar": "GITHUB_TOKEN"`) {
		t.Fatalf("missing bearerTokenEnvVar:\n%s", settings)
	}
	if strings.Contains(settings, `"Authorization"`) {
		t.Fatalf("Authorization header must not be emitted alongside bearerTokenEnvVar:\n%s", settings)
	}
}

func TestEmitsAuthHeaderForBearerAuthType(t *testing.T) {
	cfg := exampleConfig()
	cfg.MCP = nil
	cfg.Providers[0].AuthType = ir.AuthTypeBearer
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	out := string(arts[0].Content)
	for _, want := range []string{`"apiKey": "VOLC_API_KEY"`, `"authHeader": true`} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in models.json:\n%s", want, out)
		}
	}
}

func TestOmitsAPIKeyWithoutCredential(t *testing.T) {
	for _, authType := range []ir.AuthType{"", ir.AuthTypeNone} {
		t.Run("auth_type="+string(authType), func(t *testing.T) {
			cfg := exampleConfig()
			cfg.MCP = nil
			cfg.Providers[0].AuthType = authType
			cfg.Providers[0].APIKey = ir.HeaderValue{}
			arts, err := (Target{}).Emit(cfg)
			if err != nil {
				t.Fatal(err)
			}
			if out := string(arts[0].Content); strings.Contains(out, "apiKey") {
				t.Fatalf("keyless provider must not emit apiKey:\n%s", out)
			}
		})
	}
}

func TestAuthTypeXAPIKeyOnlyOnAnthropicMessages(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].AuthType = ir.AuthTypeXAPIKey
	if diags := (Target{}).ValidateAuthTypes(cfg); !diag.HasErrors(diags) {
		t.Fatal("expected a diagnostic for x-api-key on an OpenAI protocol")
	}
	cfg.Providers[0].Protocol = ir.ProtocolAnthropicMessages
	if diags := (Target{}).ValidateAuthTypes(cfg); diag.HasErrors(diags) {
		t.Fatalf("anthropic-messages accepts x-api-key: %v", diags)
	}
}
