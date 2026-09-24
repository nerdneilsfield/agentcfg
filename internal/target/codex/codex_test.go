package codex

import (
	"strings"
	"testing"

	"agentcfg/internal/ir"
)

func exampleConfig() ir.Config {
	disabled := false
	return ir.Config{
		Version: 1,
		Providers: []ir.Provider{{
			ID:       "volcengine",
			Name:     "Volcengine",
			Protocol: ir.ProtocolOpenAIResponses,
			BaseURL:  "https://example.com/v1",
			APIKey:   ir.HeaderValue{FromEnv: "VOLC_API_KEY"},
			Headers: map[string]ir.HeaderValue{
				"X-Tenant":      {Value: "engineering"},
				"X-Gateway-Key": {FromEnv: "GATEWAY_KEY"},
			},
		}},
		MCP: []ir.MCPServer{{
			ID:        "context7",
			Transport: ir.TransportStdio,
			Command:   []string{"npx", "-y", "@upstash/context7-mcp"},
			Env:       map[string]ir.HeaderValue{"CONTEXT7_API_KEY": {FromEnv: "CONTEXT7_API_KEY"}},
		}, {
			ID:        "github",
			Transport: ir.TransportHTTP,
			URL:       "https://api.githubcopilot.com/mcp/",
			Enabled:   &disabled,
			Headers: map[string]ir.HeaderValue{
				"Authorization": {BearerFromEnv: "GITHUB_TOKEN"},
				"X-Region":      {Value: "us-east-1"},
				"X-Team":        {FromEnv: "TEAM_HEADER"},
			},
		}, {
			ID:        "docs",
			Transport: ir.TransportHTTP,
			URL:       "https://mcp.example.com",
			TimeoutMS: int64p(20000),
		}},
	}
}

func int64p(v int64) *int64 { return &v }

func gen(t *testing.T, cfg ir.Config) string {
	t.Helper()
	if diags := (Target{}).Validate(cfg); len(diags) > 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatalf("emit: %v", err)
	}
	if len(arts) != 1 {
		t.Fatalf("expected 1 artifact, got %d", len(arts))
	}
	return string(arts[0].Content)
}

func TestMapsStreamableHTTPServers(t *testing.T) {
	out := gen(t, exampleConfig())
	for _, want := range []string{
		`[mcp_servers.github]`,
		`url = "https://api.githubcopilot.com/mcp/"`,
		`bearer_token_env_var = "GITHUB_TOKEN"`,
		`enabled = false`,
		`[mcp_servers.github.http_headers]`,
		`X-Region = "us-east-1"`,
		`[mcp_servers.github.env_http_headers]`,
		`X-Team = "TEAM_HEADER"`,
		`[mcp_servers.docs]`,
		`url = "https://mcp.example.com"`,
		`startup_timeout_ms = 20000`,
		`[mcp_servers.context7]`,
		`command = "npx"`,
		`args = ["-y", "@upstash/context7-mcp"]`,
		`env_vars = ["CONTEXT7_API_KEY"]`,
		`[model_providers.volcengine.env_http_headers]`,
		`X-Gateway-Key = "GATEWAY_KEY"`,
		`[model_providers.volcengine.http_headers]`,
		`X-Tenant = "engineering"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
}

func TestRejectsUnrepresentableFields(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*ir.Config)
	}{
		{"provider bearer header", func(c *ir.Config) {
			c.Providers[0].Headers["Authorization"] = ir.HeaderValue{BearerFromEnv: "TOK"}
		}},
		{"non-auth bearer header", func(c *ir.Config) {
			c.MCP[1].Headers["X-Other"] = ir.HeaderValue{BearerFromEnv: "TOK"}
		}},
		{"http cwd", func(c *ir.Config) { c.MCP[2].CWD = "/tmp" }},
		{"renamed env ref", func(c *ir.Config) {
			c.MCP[0].Env["RENAMED"] = ir.HeaderValue{FromEnv: "OTHER_NAME"}
		}},
		{"stdio without command", func(c *ir.Config) { c.MCP[0].Command = nil }},
		{"default names unsupported provider", func(c *ir.Config) {
			c.Providers[0].Protocol = ir.ProtocolOpenAICompletions
			c.Defaults = &ir.Defaults{Model: "volcengine/glm-5.3"}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := exampleConfig()
			tc.mutate(&cfg)
			if diags := (Target{}).Validate(cfg); len(diags) == 0 {
				t.Fatalf("expected rejection, got none; output would be:\n%s", gen(t, cfg))
			}
		})
	}
}

func TestIgnoresModelReasoningVariants(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Models = []ir.Model{{ID: "m", Reasoning: boolPtr(true), Variants: []ir.ReasoningEffort{"low", "ultra"}}}
	if diags := (Target{}).Validate(cfg); len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	for _, art := range arts {
		if strings.Contains(string(art.Content), `"variants"`) {
			t.Fatalf("variants must be skipped:\n%s", art.Content)
		}
	}
}

func TestEmitsDefaultProviderAndModel(t *testing.T) {
	cfg := exampleConfig()
	cfg.Providers[0].Models = []ir.Model{{ID: "glm-5.3"}}
	cfg.Defaults = &ir.Defaults{Model: "volcengine/glm-5.3"}
	out := gen(t, cfg)
	for _, want := range []string{`model_provider = "volcengine"`, `model = "glm-5.3"`} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
}

func TestEmitsMillisecondMCPTimeout(t *testing.T) {
	cfg := exampleConfig()
	cfg.MCP[2].TimeoutMS = int64p(1500)
	out := gen(t, cfg)
	if !strings.Contains(out, `startup_timeout_ms = 1500`) {
		t.Fatalf("missing millisecond timeout:\n%s", out)
	}
}

func TestSortsMCPEnvVars(t *testing.T) {
	cfg := exampleConfig()
	cfg.MCP[0].Env = map[string]ir.HeaderValue{
		"ZETA":  {FromEnv: "ZETA"},
		"ALPHA": {FromEnv: "ALPHA"},
		"MIKE":  {FromEnv: "MIKE"},
		"BRAVO": {FromEnv: "BRAVO"},
	}
	out := gen(t, cfg)
	if !strings.Contains(out, `env_vars = ["ALPHA", "BRAVO", "MIKE", "ZETA"]`) {
		t.Fatalf("env_vars are not sorted:\n%s", out)
	}
}

func emitContent(t *testing.T, cfg ir.Config) string {
	t.Helper()
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if len(arts) != 1 {
		t.Fatalf("expected 1 artifact, got %d", len(arts))
	}
	return string(arts[0].Content)
}

// TestEmitSkipsUnrepresentableInput drives Emit directly with IR that Validate
// now only warns about, and pins every skip.
func TestEmitSkipsUnrepresentableInput(t *testing.T) {
	cfg := ir.Config{
		Providers: []ir.Provider{
			{
				ID: "bad", Protocol: ir.ProtocolOpenAICompletions, BaseURL: "https://bad.example.com",
				APIKey:  ir.HeaderValue{FromEnv: "BAD_KEY"},
				Headers: map[string]ir.HeaderValue{"Authorization": {BearerFromEnv: "BAD_TOKEN"}},
			},
			{
				ID: "good", Protocol: ir.ProtocolOpenAIResponses, BaseURL: "https://good.example.com",
				APIKey: ir.HeaderValue{FromEnv: "GOOD_KEY"},
				Headers: map[string]ir.HeaderValue{
					"Authorization": {BearerFromEnv: "GOOD_TOKEN"},
					"X-Gateway":     {FromEnv: "GATEWAY"},
					"X-Plain":       {Value: "yes"},
				},
			},
		},
		MCP: []ir.MCPServer{
			{ID: "no-command", Transport: ir.TransportStdio},
			{
				ID: "renamed", Transport: ir.TransportStdio, Command: []string{"npx", "-y", "x"},
				Env: map[string]ir.HeaderValue{
					"KEPT":    {FromEnv: "KEPT"},
					"RENAMED": {FromEnv: "OTHER"},
				},
			},
			{
				ID: "remote", Transport: ir.TransportHTTP, URL: "https://m.example.com",
				Headers: map[string]ir.HeaderValue{
					"Authorization": {BearerFromEnv: "REMOTE_TOKEN"},
					"X-Other":       {BearerFromEnv: "OTHER_TOKEN"},
					"X-Env":         {FromEnv: "ENV_NAME"},
					"X-Lit":         {Value: "lit"},
				},
			},
		},
		Defaults: &ir.Defaults{Model: "bad/m1"},
	}
	out := emitContent(t, cfg)
	for _, want := range []string{
		`[model_providers.good]`,
		`env_key = "GOOD_KEY"`,
		`[model_providers.good.env_http_headers]`,
		`X-Gateway = "GATEWAY"`,
		`[model_providers.good.http_headers]`,
		`X-Plain = "yes"`,
		`[mcp_servers.renamed]`,
		`env_vars = ["KEPT"]`,
		`[mcp_servers.remote]`,
		`bearer_token_env_var = "REMOTE_TOKEN"`,
		`[mcp_servers.remote.env_http_headers]`,
		`X-Env = "ENV_NAME"`,
		`[mcp_servers.remote.http_headers]`,
		`X-Lit = "lit"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	for _, forbidden := range []string{
		`[model_providers.bad]`,
		`model_provider =`,
		`experimental_bearer_token = "BAD_TOKEN"`,
		`[mcp_servers.no-command]`,
		`"OTHER"`,
		`OTHER_TOKEN`,
		`= ""`,
	} {
		if strings.Contains(out, forbidden) {
			t.Errorf("unexpected %q:\n%s", forbidden, out)
		}
	}
}
