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

func TestSkipsBearerFromEnv(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Headers = map[string]ir.HeaderValue{
		"Authorization": {BearerFromEnv: "GITHUB_TOKEN"},
	}
	expectWarning(t, cfg, "$NAME expression syntax")
}

func TestEmitsNativeMCPArtifacts(t *testing.T) {
	cfg := exampleConfig()
	disabled := false
	cfg.MCP = []ir.MCPServer{
		{
			ID: "context7", Transport: ir.TransportStdio,
			Command: []string{"npx", "-y", "@context7/mcp"}, CWD: "/tmp/project",
			Env:       map[string]ir.HeaderValue{"TOKEN": {FromEnv: "CTX_TOKEN"}, "MODE": {Value: "full"}},
			TimeoutMS: i64(1500), Enabled: &disabled,
		},
		{
			ID: "remote", Transport: ir.TransportHTTP, URL: "https://example.com/mcp",
			Headers: map[string]ir.HeaderValue{
				"Authorization": {BearerFromEnv: "REMOTE_TOKEN"},
				"X-Static":      {Value: "literal"},
				"X-Env":         {FromEnv: "REMOTE_ENV"},
			},
		},
	}
	expectValid(t, cfg)
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(arts) != 2 || arts[1].Name != "mcp.json" || arts[1].SuggestedPath != "~/.pi/agent/mcp.json" {
		t.Fatalf("expected models.json and mcp.json, got %+v", arts)
	}
	out := string(arts[1].Content)
	for _, want := range []string{
		`"command": "npx"`, `"args": [`, `"cwd": "/tmp/project"`,
		`"TOKEN": "${CTX_TOKEN}"`, `"MODE": "full"`, `"timeout": 1.5`, `"enabled": false`,
		`"url": "https://example.com/mcp"`, `"Authorization": "Bearer ${REMOTE_TOKEN}"`,
		`"X-Static": "literal"`, `"X-Env": "${REMOTE_ENV}"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
}

func TestMCPLiteralValuesEscapePiResolverSyntax(t *testing.T) {
	cfg := exampleConfig()
	cfg.MCP = []ir.MCPServer{{ID: "safe", Transport: ir.TransportHTTP, URL: "https://example.com/mcp", Headers: map[string]ir.HeaderValue{
		"X-Braced":   {Value: "${SHOULD_NOT_EXPAND}"},
		"X-Command":  {Value: "!echo secret"},
		"X-Variable": {Value: "$LITERAL"},
		"X-Middle":   {Value: "prefix-$NAME-suffix"},
		"X-Dollars":  {Value: "$$LITERAL"},
		"X-Bang":     {Value: "$!LITERAL"},
	}}}
	expectValid(t, cfg)
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	out := string(arts[1].Content)
	for _, want := range []string{`"X-Braced": "$${SHOULD_NOT_EXPAND}"`, `"X-Command": "$!echo secret"`, `"X-Variable": "$$LITERAL"`, `"X-Middle": "prefix-$$NAME-suffix"`, `"X-Dollars": "$$$$LITERAL"`, `"X-Bang": "$$!LITERAL"`} {
		if !strings.Contains(out, want) {
			t.Errorf("missing escaped literal %q:\n%s", want, out)
		}
	}
}

func TestMCPNameCollisionSkipsLaterEntry(t *testing.T) {
	cfg := exampleConfig()
	cfg.MCP = []ir.MCPServer{
		{ID: "foo-bar", Transport: ir.TransportStdio, Command: []string{"first"}},
		{ID: "foo_bar", Transport: ir.TransportStdio, Command: []string{"second"}},
	}
	expectWarning(t, cfg, "after Pi normalization")
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	out := string(arts[1].Content)
	if !strings.Contains(out, `"foo-bar"`) || strings.Contains(out, `"foo_bar"`) || strings.Contains(out, "second") {
		t.Fatalf("expected first normalized server only:\n%s", out)
	}
}

func TestOmitsMCPArtifactWhenNoServers(t *testing.T) {
	arts, err := (Target{}).Emit(exampleConfig())
	if err != nil {
		t.Fatal(err)
	}
	if len(arts) != 1 || arts[0].Name != "models.json" {
		t.Fatalf("expected models.json only, got %+v", arts)
	}
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

func TestSkipsUnsupportedModelInput(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Models[0].Input = []ir.Modality{ir.ModalityAudio}
	expectWarning(t, cfg, "text and image only")
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	out := string(arts[0].Content)
	if strings.Contains(out, "audio") {
		t.Fatalf("unsupported modality must be dropped:\n%s", out)
	}
	if !strings.Contains(out, `"id": "glm-5.3"`) {
		t.Fatalf("model entry must survive the omission:\n%s", out)
	}
}

func TestDropsUnsupportedModelModalities(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Models[0].Input = []ir.Modality{ir.ModalityText, ir.ModalityPDF}
	expectWarning(t, cfg, "text and image only")
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	out := string(arts[0].Content)
	if strings.Contains(out, "pdf") {
		t.Fatalf("unsupported modality must be dropped:\n%s", out)
	}
	if !strings.Contains(out, `"text"`) {
		t.Fatalf("supported modality must remain:\n%s", out)
	}
}

func TestOmitsLiteralExpressionAPIKey(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].APIKey = ir.HeaderValue{Value: "$TOKEN"}
	expectWarning(t, cfg, "native expression syntax")
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	out := string(arts[0].Content)
	if strings.Contains(out, "apiKey") || strings.Contains(out, "$TOKEN") {
		t.Fatalf("unrepresentable literal key must be omitted:\n%s", out)
	}
	if !strings.Contains(out, `"volcengine"`) {
		t.Fatalf("provider entry must survive the omission:\n%s", out)
	}
}

func TestSkipsBearerProviderHeader(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Headers = map[string]ir.HeaderValue{"Authorization": {BearerFromEnv: "TOK"}}
	expectWarning(t, cfg, "$NAME expression syntax")
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	out := string(arts[0].Content)
	if strings.Contains(out, "Authorization") || strings.Contains(out, `""`) {
		t.Fatalf("a bearer header pi cannot expand must be omitted:\n%s", out)
	}
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
