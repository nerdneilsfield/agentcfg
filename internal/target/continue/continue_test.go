package continueagent

import (
	"testing"

	"go.yaml.in/yaml/v3"

	"agentcfg/internal/ir"
)

func TestPublishedSchemaFieldsAndPartialOutput(t *testing.T) {
	n := int64(32000)
	off := false
	cfg := ir.Config{Providers: []ir.Provider{{ID: "chat", Protocol: ir.ProtocolOpenAICompletions, APIKey: ir.HeaderValue{FromEnv: "KEY"}, Models: []ir.Model{{ID: "m", ContextWindow: &n}}}, {ID: "responses", Protocol: ir.ProtocolOpenAIResponses, Models: []ir.Model{{ID: "r"}}}}, MCP: []ir.MCPServer{{ID: "off", Enabled: &off}, {ID: "remote", Transport: ir.TransportHTTP, URL: "https://example.com/mcp", Headers: map[string]ir.HeaderValue{"Authorization": {BearerFromEnv: "TOKEN"}}}}}
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(arts[0].Content, &doc); err != nil {
		t.Fatal(err)
	}
	models := doc["models"].([]any)
	if len(models) != 1 {
		t.Fatal("Responses not skipped")
	}
	m := models[0].(map[string]any)
	if m["apiKey"] != "${{ secrets.KEY }}" || m["defaultCompletionOptions"].(map[string]any)["contextLength"] != 32000 {
		t.Fatalf("model: %+v", m)
	}
	servers := doc["mcpServers"].([]any)
	if len(servers) != 1 || servers[0].(map[string]any)["type"] != "streamable-http" {
		t.Fatalf("MCP: %+v", servers)
	}
}
