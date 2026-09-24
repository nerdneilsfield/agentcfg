package fastagent

import (
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"agentcfg/internal/ir"
)

func TestOverlaysKeepRoutesAndDefault(t *testing.T) {
	limit := int64(4096)
	cfg := ir.Config{Defaults: &ir.Defaults{Model: "responses/vendor/model"}}
	for _, p := range []struct {
		id       string
		protocol ir.Protocol
		key      ir.HeaderValue
	}{
		{"chat", ir.ProtocolOpenAICompletions, ir.HeaderValue{FromEnv: "CHAT_KEY"}},
		{"responses", ir.ProtocolOpenAIResponses, ir.HeaderValue{Value: "literal-test-key"}},
		{"messages", ir.ProtocolAnthropicMessages, ir.HeaderValue{FromEnv: "MESSAGES_KEY"}},
	} {
		cfg.Providers = append(cfg.Providers, ir.Provider{ID: p.id, Protocol: p.protocol, BaseURL: "https://" + p.id + ".example/v1", APIKey: p.key, Headers: map[string]ir.HeaderValue{"Authorization": {BearerFromEnv: "HEADER_KEY"}}, Models: []ir.Model{{ID: "vendor/model", MaxOutputTokens: &limit}}})
	}
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(arts) != 5 {
		t.Fatalf("artifacts: %d", len(arts))
	}
	seen := map[string]bool{}
	for _, a := range arts {
		var doc map[string]any
		if err := yaml.Unmarshal(a.Content, &doc); err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(a.Name, "model-overlays/") {
			name := doc["name"].(string)
			if seen[name] {
				t.Fatal("overlay collision")
			}
			seen[name] = true
			if doc["model"] != "vendor/model" {
				t.Fatal("wire model changed")
			}
			connection := doc["connection"].(map[string]any)
			if connection["default_headers"].(map[string]any)["Authorization"] != "Bearer ${HEADER_KEY}" {
				t.Fatal("header reference lost")
			}
			if doc["defaults"].(map[string]any)["max_tokens"] != 4096 {
				t.Fatal("output limit lost")
			}
		}
		if a.Name == "fastagent.config.yaml" && doc["default_model"] != overlayName(cfg.Defaults.Model) {
			t.Fatal("default does not select its overlay")
		}
	}
	if !seen[overlayName(cfg.Defaults.Model)] {
		t.Fatal("default overlay missing")
	}
}

func TestMCPTimeoutAndDisabled(t *testing.T) {
	off := false
	exact := int64(2000)
	fractional := int64(1500)
	cfg := ir.Config{MCP: []ir.MCPServer{
		{ID: "exact", Transport: ir.TransportStdio, Command: []string{"server", "arg"}, CWD: "/work", TimeoutMS: &exact},
		{ID: "fractional", Transport: ir.TransportHTTP, URL: "https://example.com/mcp", TimeoutMS: &fractional},
		{ID: "disabled", Transport: ir.TransportHTTP, URL: "https://example.com/off", Enabled: &off},
	}}
	if ds := (Target{}).Validate(cfg); len(ds) != 2 {
		t.Fatalf("diagnostics: %v", ds)
	}
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		MCP struct {
			Servers map[string]map[string]any `yaml:"servers"`
		} `yaml:"mcp"`
	}
	if err := yaml.Unmarshal(arts[0].Content, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.MCP.Servers) != 2 || doc.MCP.Servers["exact"]["read_timeout_seconds"] != 2 || doc.MCP.Servers["exact"]["cwd"] != "/work" {
		t.Fatalf("MCP mapping: %+v", doc)
	}
	if _, ok := doc.MCP.Servers["fractional"]["read_timeout_seconds"]; ok {
		t.Fatal("fractional timeout was rounded")
	}
}
