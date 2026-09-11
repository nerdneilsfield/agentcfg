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

func TestValidationRules(t *testing.T) {
	withProviderHeaders := exampleConfig()
	withProviderHeaders.Providers[0].Headers = map[string]ir.HeaderValue{"X-Tenant": {Value: "engineering"}}
	expectInvalid(t, withProviderHeaders, "no headers field")

	literalStdioEnv := exampleConfig()
	literalStdioEnv.MCP[0].Env = map[string]ir.HeaderValue{"KEY": {Value: "literal"}}
	expectInvalid(t, literalStdioEnv, "environment references only")

	envRefHTTPHeader := exampleConfig()
	envRefHTTPHeader.MCP[1].Headers = map[string]ir.HeaderValue{"X-Team": {FromEnv: "TEAM"}}
	expectInvalid(t, envRefHTTPHeader, "static strings")

	nonAuthBearer := exampleConfig()
	nonAuthBearer.MCP[1].Headers = map[string]ir.HeaderValue{"X-Api-Key": {BearerFromEnv: "API_KEY"}}
	expectInvalid(t, nonAuthBearer, "Authorization only")
}
