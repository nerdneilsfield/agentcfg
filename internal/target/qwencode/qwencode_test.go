package qwencode

import (
	"encoding/json"
	"testing"

	"agentcfg/internal/ir"
)

func TestProviderWireDefaultAndCredentials(t *testing.T) {
	cfg := ir.Config{Providers: []ir.Provider{{ID: "custom", Protocol: ir.ProtocolOpenAIResponses, BaseURL: "https://example.com/v1", APIKey: ir.HeaderValue{Value: "literal-key"}, Models: []ir.Model{{ID: "vendor/model"}}}}, Defaults: &ir.Defaults{Model: "custom/vendor/model"}}
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(arts[0].Content, &doc); err != nil {
		t.Fatal(err)
	}
	if doc["providerProtocol"].(map[string]any)["custom"] != "openai" {
		t.Fatal("provider protocol")
	}
	m := doc["modelProviders"].(map[string]any)["custom"].([]any)[0].(map[string]any)
	if m["id"] != "vendor/model" || m["wireApi"] != "responses" {
		t.Fatalf("model: %+v", m)
	}
	if doc["env"].(map[string]any)[m["envKey"].(string)] != "literal-key" {
		t.Fatal("literal credential not referenced")
	}
	auth := doc["security"].(map[string]any)["auth"].(map[string]any)
	if auth["selectedType"] != "openai-responses" || auth["baseUrl"] != "https://example.com/v1" {
		t.Fatalf("default route: %+v", auth)
	}
}
