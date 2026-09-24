package aider

import (
	"testing"

	"go.yaml.in/yaml/v3"

	"agentcfg/internal/ir"
)

func TestRouteOverridesAndUnsupportedWire(t *testing.T) {
	cfg := ir.Config{Providers: []ir.Provider{{ID: "a", Protocol: ir.ProtocolOpenAICompletions, BaseURL: "https://a.example/v1", APIKey: ir.HeaderValue{FromEnv: "A_KEY"}, Models: []ir.Model{{ID: "same"}}}, {ID: "b", Protocol: ir.ProtocolAnthropicMessages, BaseURL: "https://b.example", Models: []ir.Model{{ID: "same"}}}, {ID: "r", Protocol: ir.ProtocolOpenAIResponses, Models: []ir.Model{{ID: "other"}}}}, Defaults: &ir.Defaults{Model: "a/same"}}
	arts, err := (Target{}).Emit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var settings []struct {
		Name   string
		Params map[string]any `yaml:"extra_params"`
	}
	if err := yaml.Unmarshal(arts[1].Content, &settings); err != nil {
		t.Fatal(err)
	}
	if len(settings) != 2 || settings[0].Name == settings[1].Name || settings[0].Params["model"] != "openai/same" || settings[1].Params["model"] != "anthropic/same" || settings[0].Params["api_key"] != "os.environ/A_KEY" {
		t.Fatalf("bad routes: %+v", settings)
	}
	if len((Target{}).Validate(cfg)) != 1 {
		t.Fatal("missing Responses warning")
	}
}
