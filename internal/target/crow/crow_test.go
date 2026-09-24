package crow

import (
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"agentcfg/internal/diag"
	"agentcfg/internal/ir"
)

func TestDefaultOrderAndPartialGeneration(t *testing.T) {
	off := false
	cfg := ir.Config{
		Providers: []ir.Provider{
			{ID: "unsupported", Protocol: ir.ProtocolAnthropicMessages, Models: []ir.Model{{ID: "claude"}}},
			{ID: "gateway", Protocol: ir.ProtocolOpenAICompletions, BaseURL: "https://example.com/v1", APIKey: ir.HeaderValue{FromEnv: "CROW_KEY"}, Models: []ir.Model{{ID: "a"}, {ID: "vendor/z", Input: []ir.Modality{ir.ModalityText, ir.ModalityImage, ir.ModalityPDF}}}},
		},
		Defaults: &ir.Defaults{Model: "gateway/vendor/z"},
		MCP:      []ir.MCPServer{{ID: "disabled", Transport: ir.TransportStdio, Command: []string{"false"}, Enabled: &off}, {ID: "remote", Transport: ir.TransportHTTP, URL: "https://example.com/mcp", Headers: map[string]ir.HeaderValue{"Authorization": {BearerFromEnv: "MCP_KEY"}}}},
	}
	ds := (Target{}).Validate(cfg)
	if len(ds) != 3 || diag.HasErrors(ds) {
		t.Fatalf("diagnostics: %v", ds)
	}
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(arts[0].Content, &doc); err != nil {
		t.Fatal(err)
	}
	root := doc.Content[0]
	for i := 0; i < len(root.Content); i += 2 {
		if root.Content[i].Value == "models" && root.Content[i+1].Content[0].Value != cfg.Defaults.Model {
			t.Fatal("default model was not first")
		}
	}
	out := string(arts[0].Content)
	for _, want := range []string{"${CROW_KEY}", "Bearer ${MCP_KEY}", "model: vendor/z", "- image"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q: %s", want, out)
		}
	}
	for _, absent := range []string{"unsupported", "disabled", "pdf"} {
		if strings.Contains(out, absent) {
			t.Fatalf("unsupported entry survived: %s", out)
		}
	}
}
