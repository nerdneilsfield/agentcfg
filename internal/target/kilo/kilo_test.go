package kilo

import (
	"encoding/json"
	"testing"

	"agentcfg/internal/ir"
)

func TestPackagesLimitsAndMCP(t *testing.T) {
	n := int64(100)
	off := false
	cfg := ir.Config{Providers: []ir.Provider{{ID: "r", Protocol: ir.ProtocolOpenAIResponses, APIKey: ir.HeaderValue{FromEnv: "KEY"}, Models: []ir.Model{{ID: "m", ContextWindow: &n}}}}, MCP: []ir.MCPServer{{ID: "s", Transport: ir.TransportStdio, Command: []string{"server"}, CWD: "/work", TimeoutMS: &n, Enabled: &off}}}
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(arts[0].Content, &doc); err != nil {
		t.Fatal(err)
	}
	p := doc["provider"].(map[string]any)["r"].(map[string]any)
	if p["npm"] != "@ai-sdk/openai" || p["options"].(map[string]any)["apiKey"] != "{env:KEY}" {
		t.Fatalf("provider: %+v", p)
	}
	if _, ok := p["models"].(map[string]any)["m"].(map[string]any)["limit"]; ok {
		t.Fatal("incomplete limit emitted")
	}
	s := doc["mcp"].(map[string]any)["s"].(map[string]any)
	if s["enabled"] != false || s["timeout"] != float64(100) {
		t.Fatal("MCP flags lost")
	}
	if _, ok := s["cwd"]; ok {
		t.Fatal("unsupported cwd emitted")
	}
}
