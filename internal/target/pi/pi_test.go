package pi

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
			Name:     "Volcengine",
			Protocol: ir.ProtocolOpenAICompletions,
			BaseURL:  "https://example.com/v1",
			APIKey:   ir.HeaderValue{FromEnv: "VOLC_API_KEY"},
			Headers: map[string]ir.HeaderValue{
				"X-Tenant":      {Value: "engineering"},
				"X-Gateway-Key": {FromEnv: "GATEWAY_KEY"},
			},
			Models: []ir.Model{{
				ID:            "glm-5.3",
				ContextWindow: i64(128000),
			}},
		}},
	}
}

func TestEmitsProviderHeadersWithEnvSyntax(t *testing.T) {
	expectValid(t, exampleConfig())
	arts, err := (Target{}).Emit(exampleConfig())
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	out := string(arts[0].Content)
	for _, want := range []string{
		`"X-Tenant": "engineering"`,
		`"X-Gateway-Key": "$GATEWAY_KEY"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
}

func TestRejectsBearerFromEnv(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Headers = map[string]ir.HeaderValue{
		"Authorization": {BearerFromEnv: "GITHUB_TOKEN"},
	}
	expectInvalid(t, cfg, "$NAME expression syntax")
}

func TestRejectsMCP(t *testing.T) {
	cfg := exampleConfig()
	cfg.MCP = []ir.MCPServer{{
		ID:        "context7",
		Transport: ir.TransportStdio,
		Command:   []string{"npx"},
	}}
	expectInvalid(t, cfg, "no built-in MCP support")
}

func TestEmitsThinkingLevelMap(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Models[0].Reasoning = boolPtr(true)
	cfg.Providers[0].Models[0].Variants = []ir.ReasoningEffort{"low", "high", "max"}
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	out := string(arts[0].Content)
	for _, want := range []string{`thinkingLevelMap`, `"low": "low"`, `"medium": null`, `"high": "high"`, `"max": "max"`} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
}

func boolPtr(v bool) *bool { return &v }

func TestSkipsUnsupportedReasoningEffort(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Models[0].Reasoning = boolPtr(true)
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

func TestEmitsProviderHeadersInNameOrder(t *testing.T) {
	cfg := exampleConfig()
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	out := string(arts[0].Content)
	if strings.Index(out, `"X-Gateway-Key"`) > strings.Index(out, `"X-Tenant"`) {
		t.Fatalf("headers are not name-sorted:\n%s", out)
	}
}

func TestEmitsModelsDocument(t *testing.T) {
	cfg := exampleConfig()
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(arts) != 1 {
		t.Fatalf("expected 1 artifact, got %d", len(arts))
	}
	if arts[0].Name != "models.json" || arts[0].SuggestedPath != "~/.pi/agent/models.json" {
		t.Fatalf("artifact = %+v", arts[0])
	}
	out := string(arts[0].Content)
	for _, want := range []string{`"providers"`, `"volcengine"`, `"api": "openai-completions"`, `"baseUrl": "https://example.com/v1"`} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
}

func TestRejectsUnsupportedModelInput(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Models[0].Input = []ir.Modality{ir.ModalityAudio}
	expectInvalid(t, cfg, "text and image only")
}

func TestEmitsAuthHeaderForBearerAuthType(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].AuthType = ir.AuthTypeBearer
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	out := string(arts[0].Content)
	for _, want := range []string{`"apiKey": "$VOLC_API_KEY"`, `"authHeader": true`} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
}

func TestOmitsAPIKeyWithoutCredential(t *testing.T) {
	for _, authType := range []ir.AuthType{"", ir.AuthTypeNone} {
		t.Run("auth_type="+string(authType), func(t *testing.T) {
			cfg := exampleConfig()
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
