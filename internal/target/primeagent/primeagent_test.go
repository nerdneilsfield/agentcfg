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

func TestSkipsUnsupportedModelInput(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Models[0].Input = []ir.Modality{ir.ModalityAudio}
	expectWarning(t, cfg, "text and image only")
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

func TestSkipsUnsupportedOptionalFields(t *testing.T) {
	cfg := exampleConfig()
	cfg.MCP[0].TimeoutMS = i64(5000)
	cfg.MCP[0].Env["LITERAL"] = ir.HeaderValue{Value: "skip-me"}
	expectValid(t, cfg)
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	settings := string(arts[1].Content)
	if strings.Contains(settings, "skip-me") || strings.Contains(settings, "LITERAL") {
		t.Fatalf("unsupported optional fields must be skipped:\n%s", settings)
	}
}

func TestSkipsBearerProviderHeader(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Headers = map[string]ir.HeaderValue{"Authorization": {BearerFromEnv: "TOK"}}
	expectWarning(t, cfg, "no bearer-from-env convention")
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	models := string(arts[0].Content)
	if strings.Contains(models, `"Authorization"`) || strings.Contains(models, `"headers"`) {
		t.Fatalf("bearer provider header must be omitted, got:\n%s", models)
	}
	if !strings.Contains(models, `"volcengine"`) {
		t.Fatalf("provider entry must survive the omission:\n%s", models)
	}
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

func TestAuthTypeMappings(t *testing.T) {
	mapped := map[ir.AuthType]bool{}
	for _, authType := range (Target{}).MappedAuthTypes() {
		mapped[authType] = true
	}
	for _, want := range []ir.AuthType{ir.AuthTypeOfficial, ir.AuthTypeBearer, ir.AuthTypeNone} {
		if !mapped[want] {
			t.Errorf("auth_type %s is not mapped", want)
		}
	}
}

func TestOmitsLiteralExpressionAPIKey(t *testing.T) {
	cfg := exampleConfig()
	cfg.MCP = nil
	cfg.Providers[0].APIKey = ir.HeaderValue{Value: "$TOKEN"}
	expectWarning(t, cfg, "native expression syntax")
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	models := string(arts[0].Content)
	if strings.Contains(models, "apiKey") || strings.Contains(models, "$TOKEN") {
		t.Fatalf("unrepresentable literal key must be omitted:\n%s", models)
	}
	if !strings.Contains(models, `"volcengine"`) {
		t.Fatalf("provider entry must survive the omission:\n%s", models)
	}
}

func TestDropsUnsupportedModelModalities(t *testing.T) {
	cfg := exampleConfig()
	cfg.MCP = nil
	cfg.Providers[0].Models[0].Input = []ir.Modality{ir.ModalityText, ir.ModalityAudio, ir.ModalityImage}
	expectWarning(t, cfg, "text and image only")
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	models := string(arts[0].Content)
	if strings.Contains(models, "audio") {
		t.Fatalf("unsupported modality must be dropped:\n%s", models)
	}
	if !strings.Contains(models, `"text"`) || !strings.Contains(models, `"image"`) {
		t.Fatalf("supported modalities must remain:\n%s", models)
	}
}

func TestSkipsMCPHTTPEnvHeader(t *testing.T) {
	cfg := exampleConfig()
	cfg.MCP[1].Headers = map[string]ir.HeaderValue{
		"Authorization": {BearerFromEnv: "GITHUB_TOKEN"},
		"X-Auth":        {FromEnv: "MCP_TOKEN"},
	}
	expectWarning(t, cfg, "environment interpolation is not verified")
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	settings := string(arts[1].Content)
	if strings.Contains(settings, `"X-Auth"`) || strings.Contains(settings, `""`) {
		t.Fatalf("an MCP http env header must be skipped:\n%s", settings)
	}
	if !strings.Contains(settings, `"bearerTokenEnvVar": "GITHUB_TOKEN"`) {
		t.Fatalf("the bearer token field must remain:\n%s", settings)
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
	settings := string(arts[1].Content)
	if strings.Contains(settings, `"context7"`) {
		t.Fatalf("commandless stdio server must be skipped:\n%s", settings)
	}
	if !strings.Contains(settings, `"github"`) {
		t.Fatalf("other MCP servers must still emit:\n%s", settings)
	}
}
