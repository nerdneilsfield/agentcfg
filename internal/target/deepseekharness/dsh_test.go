package deepseekharness

import (
	"strings"
	"testing"

	"agentcfg/internal/ir"
)

func boolPtr(v bool) *bool { return &v }

func variantConfig(efforts ...ir.ReasoningEffort) ir.Config {
	return ir.Config{Version: 1, Providers: []ir.Provider{{
		ID: "provider", Protocol: ir.ProtocolOpenAICompletions, BaseURL: "https://example.com/v1",
		APIKey: ir.HeaderValue{FromEnv: "API_KEY"},
		Models: []ir.Model{{ID: "model", Reasoning: boolPtr(true), Variants: efforts}},
	}}}
}

func TestEmitsReasoningEfforts(t *testing.T) {
	arts, err := (Target{}).Emit(variantConfig("low", "high", "max"))
	if err != nil {
		t.Fatal(err)
	}
	out := string(arts[0].Content)
	for _, want := range []string{"reasoningEfforts:", "low: low", "high: high", "max: max"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
	if strings.Contains(out, "medium:") {
		t.Fatalf("undeclared level must not be emitted:\n%s", out)
	}
	if strings.Contains(out, "defaultReasoning") {
		t.Fatalf("must not select a default effort:\n%s", out)
	}
}

func TestSkipsUnsupportedReasoningEffort(t *testing.T) {
	cfg := variantConfig("low", "ultra")
	if diags := (Target{}).Validate(cfg); len(diags) > 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	out := string(arts[0].Content)
	if !strings.Contains(out, "low: low") {
		t.Fatalf("missing supported effort:\n%s", out)
	}
	if strings.Contains(out, "ultra") {
		t.Fatalf("unsupported effort must be skipped:\n%s", out)
	}
}

func TestEmitsNativeCordisMCPPatch(t *testing.T) {
	cfg := variantConfig("low")
	cfg.Defaults = &ir.Defaults{Model: "provider/model"}
	disabled := false
	cfg.MCP = []ir.MCPServer{
		{ID: "local", Transport: ir.TransportStdio, Command: []string{"npx", "-y", "server"}, Env: map[string]ir.HeaderValue{"TOKEN": {FromEnv: "TOKEN"}}, TimeoutMS: i64(1500)},
		{ID: "remote", Transport: ir.TransportHTTP, URL: "https://mcp.example", Headers: map[string]ir.HeaderValue{"Authorization": {BearerFromEnv: "TOKEN"}}, Enabled: &disabled},
	}
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(arts) != 2 || arts[1].Name != "cordis.patch.yml" {
		t.Fatalf("unexpected artifacts: %#v", arts)
	}
	out := string(arts[1].Content)
	for _, want := range []string{"@deepseek-ai/dsh-agent-default-model", "provider: provider", "@deepseek-ai/dsh-mcp-client", "transport: stdio", "toolCallTimeoutMs: 1500", "!!js process.env.TOKEN", "transport: streamable-http", "!!js `Bearer ${process.env.TOKEN}`", "disabled: true"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
}

func TestDSHValidationAndProviderFields(t *testing.T) {
	cfg := variantConfig("low")
	cfg.Providers[0].Name = "Provider"
	cfg.Providers[0].Headers = map[string]ir.HeaderValue{"X-Route": {Value: "stable"}}
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	out := string(arts[0].Content)
	for _, unwanted := range []string{"reasoning:", "cost:"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("unexpected %q:\n%s", unwanted, out)
		}
	}
	for _, want := range []string{"displayName: Provider", "headers:", "X-Route: stable"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	cfg.Providers[0].Models[0].Input = []ir.Modality{ir.ModalityAudio}
	if got := (Target{}).Validate(cfg); len(got) == 0 {
		t.Fatal("expected input diagnostic")
	}
}
func i64(v int64) *int64 { return &v }

func TestDropsUnsupportedInputModalities(t *testing.T) {
	cfg := variantConfig("low")
	cfg.Providers[0].Models[0].Input = []ir.Modality{ir.ModalityText, ir.ModalityAudio}
	if diags := (Target{}).Validate(cfg); len(diags) == 0 {
		t.Fatal("expected an input diagnostic")
	}
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	out := string(arts[0].Content)
	if strings.Contains(out, "audio") {
		t.Fatalf("unsupported modality must be dropped:\n%s", out)
	}
	if !strings.Contains(out, "text") {
		t.Fatalf("supported modality must remain:\n%s", out)
	}
}

func TestSkipsStdioMCPWithoutCommand(t *testing.T) {
	cfg := variantConfig("low")
	cfg.Defaults = &ir.Defaults{Model: "provider/model"}
	cfg.MCP = []ir.MCPServer{{ID: "broken", Transport: ir.TransportStdio}}
	if diags := (Target{}).Validate(cfg); len(diags) == 0 {
		t.Fatal("expected a command diagnostic")
	}
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(arts) != 2 || arts[1].Name != "cordis.patch.yml" {
		t.Fatalf("unexpected artifacts: %#v", arts)
	}
	out := string(arts[1].Content)
	if strings.Contains(out, "broken") || strings.Contains(out, "dsh-mcp-client") {
		t.Fatalf("commandless stdio server must be skipped:\n%s", out)
	}
	if !strings.Contains(out, "dsh-agent-default-model") {
		t.Fatalf("other patch rows must still emit:\n%s", out)
	}
}
