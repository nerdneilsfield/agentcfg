package ir

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agentcfg/internal/diag"
)

func loadFixture(t *testing.T, name string) Config {
	t.Helper()
	src, err := os.ReadFile(filepath.Join("..", "..", "testdata", "ir", name))
	if err != nil {
		t.Fatal(err)
	}
	cfg, diags, err := Load(src)
	if err != nil {
		t.Fatalf("Load(%s): %v", name, err)
	}
	if diag.HasErrors(diags) {
		t.Fatalf("Load(%s): unexpected diagnostics: %v", name, diags)
	}
	return cfg
}

func TestLoadExample(t *testing.T) {
	cfg := loadFixture(t, "example.yaml")
	if cfg.Version != 1 {
		t.Fatalf("version = %d, want 1", cfg.Version)
	}
	if len(cfg.Providers) != 1 || cfg.Providers[0].ID != "volcengine" {
		t.Fatalf("providers = %+v", cfg.Providers)
	}
	p := cfg.Providers[0]
	if p.Protocol != ProtocolOpenAIResponses {
		t.Fatalf("protocol = %q", p.Protocol)
	}
	if len(p.Models) != 1 || p.Models[0].ID != "glm-5.3" {
		t.Fatalf("models = %+v", p.Models)
	}
	if p.Models[0].ContextWindow == nil || *p.Models[0].ContextWindow != 128000 {
		t.Fatalf("context_window = %+v", p.Models[0].ContextWindow)
	}
	if got := p.Headers["X-Tenant"]; got.Value != "engineering" {
		t.Fatalf("X-Tenant header = %+v", got)
	}
	if got := p.Headers["X-Gateway-Key"]; got.FromEnv != "GATEWAY_KEY" {
		t.Fatalf("X-Gateway-Key header = %+v", got)
	}
	if cfg.Defaults == nil || cfg.Defaults.Model != "volcengine/glm-5.3" {
		t.Fatalf("defaults = %+v", cfg.Defaults)
	}
	if len(cfg.Targets) != 3 {
		t.Fatalf("targets = %v", cfg.Targets)
	}
}

func TestLoadExampleMCP(t *testing.T) {
	cfg := loadFixture(t, "example-mcp.yaml")
	if len(cfg.MCP) != 2 {
		t.Fatalf("mcp = %+v", cfg.MCP)
	}
	stdio, http := cfg.MCP[0], cfg.MCP[1]
	if stdio.Transport != TransportStdio || len(stdio.Command) != 3 {
		t.Fatalf("stdio server = %+v", stdio)
	}
	if http.Transport != TransportHTTP || http.URL == "" {
		t.Fatalf("http server = %+v", http)
	}
	if got := http.Headers["Authorization"]; got.BearerFromEnv != "GITHUB_TOKEN" {
		t.Fatalf("Authorization header = %+v", got)
	}
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	src := []byte("version: 1\nproviders:\n  - id: x\n    protocol: openai-responses\n    base_url: https://e.com\n    bogus_field: 1\n    models:\n      - id: m\n")
	_, _, err := Load(src)
	if err == nil {
		t.Fatal("expected strict decode error for unknown field")
	}
}

func TestLoadRejectsEmpty(t *testing.T) {
	if _, _, err := Load([]byte("  \n")); err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestValidateFindings(t *testing.T) {
	cases := []struct {
		name string
		yaml string
		want string // substring of a diagnostic message
	}{
		{"bad version", "version: 2\n", "version must be 1"},
		{"dup provider", "version: 1\nproviders:\n  - id: a\n    protocol: openai-responses\n    base_url: https://e.com\n    models: [{id: m}]\n  - id: a\n    protocol: openai-responses\n    base_url: https://e.com\n    models: [{id: m}]\n", "duplicate provider id"},
		{"bad protocol", "version: 1\nproviders:\n  - id: a\n    protocol: gemini\n    base_url: https://e.com\n    models: [{id: m}]\n", "unknown protocol"},
		{"missing base_url", "version: 1\nproviders:\n  - id: a\n    protocol: openai-responses\n    models: [{id: m}]\n", "base_url is required"},
		{"empty models", "version: 1\nproviders:\n  - id: a\n    protocol: openai-responses\n    base_url: https://e.com\n", "at least one model"},
		{"bearer wrong header", "version: 1\nproviders:\n  - id: a\n    protocol: openai-responses\n    base_url: https://e.com\n    headers:\n      X-Other: Bearer ENV:TOKEN\n    models: [{id: m}]\n", "only valid for the Authorization header"},
		{"defaults unknown model", "version: 1\nproviders:\n  - id: a\n    protocol: openai-responses\n    base_url: https://e.com\n    models: [{id: m}]\ndefaults:\n  model: a/nope\n", "references unknown model"},
		{"defaults bad ref", "version: 1\nproviders:\n  - id: a\n    protocol: openai-responses\n    base_url: https://e.com\n    models: [{id: m}]\ndefaults:\n  model: noref\n", "provider-id/model-id"},
		{"stdio with url", "version: 1\nproviders: []\nmcp:\n  - id: s\n    transport: stdio\n    command: [npx]\n    url: https://e.com\n", "url is invalid for stdio"},
		{"http with command", "version: 1\nproviders: []\nmcp:\n  - id: s\n    transport: http\n    url: https://e.com\n    command: [npx]\n", "command is invalid for http"},
		{"dup mcp id", "version: 1\nproviders: []\nmcp:\n  - id: s\n    transport: stdio\n    command: [npx]\n  - id: s\n    transport: stdio\n    command: [npx]\n", "duplicate MCP server id"},
		{"bad modality", "version: 1\nproviders:\n  - id: a\n    protocol: openai-responses\n    base_url: https://e.com\n    models:\n      - id: m\n        input: [smell]\n", "unknown modality"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, diags, err := Load([]byte(tc.yaml))
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if !diag.HasErrors(diags) {
				t.Fatalf("expected diagnostics, got none")
			}
			joined := diags[0].String()
			for _, d := range diags[1:] {
				joined += "\n" + d.String()
			}
			if !strings.Contains(joined, tc.want) {
				t.Fatalf("diagnostics %q do not contain %q", joined, tc.want)
			}
		})
	}
}

func TestScalarValues(t *testing.T) {
	t.Setenv("SOURCE", "not-the-IR-value")
	cfg, ds, err := Load([]byte(`version: 1
providers:
  - id: p
    protocol: openai-responses
    base_url: https://example.com
    api_key: ENV:SOURCE
    headers:
      Empty: ""
      X-Literal: plain
      X-Env: ENV:SOURCE
      Authorization: Bearer ENV:SOURCE
    models: [{id: m}]
mcp:
  - id: s
    transport: stdio
    command: [cmd]
    env:
      CHILD: ENV:SOURCE
      EMPTY: ""
`))
	if err != nil || diag.HasErrors(ds) {
		t.Fatalf("load: %v %v", err, ds)
	}
	if cfg.Providers[0].APIKey.FromEnv != "SOURCE" || cfg.MCP[0].Env["CHILD"].FromEnv != "SOURCE" {
		t.Fatal("environment references were not preserved")
	}
	if cfg.Providers[0].Headers["X-Literal"].Value != "plain" || cfg.Providers[0].Headers["Authorization"].BearerFromEnv != "SOURCE" {
		t.Fatal("incorrect scalar semantics")
	}
}

func TestRejectsLegacyAndInvalidScalars(t *testing.T) {
	for _, field := range []string{
		`api_key_env: OLD`, `api_key: {value: secret}`, `api_key: {from_env: OLD}`,
		`headers: {X: {value: literal}}`, `headers: {X: {from_env: OLD}}`,
		`headers: {Authorization: {bearer_from_env: OLD}}`,
		`api_key: 123`, `api_key: true`, `api_key: []`,
		`api_key: "ENV:"`, `api_key: "ENV:BAD-NAME"`, `api_key: "ENV:1BAD"`,
		`headers: {Authorization: "Bearer ENV:"}`,
	} {
		t.Run(field, func(t *testing.T) {
			_, _, err := Load([]byte("version: 1\nproviders:\n  - " + field + "\n"))
			if err == nil {
				t.Fatal("expected scalar decode error")
			}
		})
	}
}
