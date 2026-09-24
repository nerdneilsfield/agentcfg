package pifamily

import (
	"testing"

	"agentcfg/internal/ir"
)

func TestProvidersSkipUnrepresentableFields(t *testing.T) {
	cfg := ir.Config{Providers: []ir.Provider{{
		ID:       "p",
		Protocol: ir.ProtocolOpenAICompletions,
		BaseURL:  "https://example.com",
		APIKey:   ir.HeaderValue{Value: "$TOKEN"},
		Headers: map[string]ir.HeaderValue{
			"Authorization": {BearerFromEnv: "TOK"},
			"X-Keep":        {Value: "yes"},
		},
		Models: []ir.Model{{
			ID:    "m",
			Input: []ir.Modality{ir.ModalityText, ir.ModalityAudio, ir.ModalityImage},
		}},
	}}}
	opts := Options{ID: "p", Syntax: DollarEnv, Levels: []ir.ReasoningEffort{"low", "high"}}
	entry, ok := Providers(cfg, opts)["p"].(map[string]any)
	if !ok {
		t.Fatalf("provider entry missing: %#v", Providers(cfg, opts))
	}
	if _, ok := entry["apiKey"]; ok {
		t.Fatalf("unrepresentable literal key must be omitted: %#v", entry)
	}
	headers, ok := entry["headers"].(map[string]string)
	if !ok {
		t.Fatalf("headers missing: %#v", entry)
	}
	if _, ok := headers["Authorization"]; ok {
		t.Fatalf("a bearer header with no renderer must be omitted: %#v", headers)
	}
	if headers["X-Keep"] != "yes" {
		t.Fatalf("a representable header must remain: %#v", headers)
	}
	model := entry["models"].([]any)[0].(map[string]any)
	input, ok := model["input"].([]string)
	if !ok || len(input) != 2 || input[0] != "text" || input[1] != "image" {
		t.Fatalf("only text and image are representable: %#v", model["input"])
	}
}

func TestProvidersRenderBearerWhenSupported(t *testing.T) {
	cfg := ir.Config{Providers: []ir.Provider{{
		ID:       "p",
		Protocol: ir.ProtocolOpenAICompletions,
		BaseURL:  "https://example.com",
		Headers:  map[string]ir.HeaderValue{"Authorization": {BearerFromEnv: "TOK"}},
	}}}
	opts := Options{ID: "p", Syntax: BareEnv, Bearer: func(name string) string { return "Bearer ${" + name + "}" }}
	headers := Providers(cfg, opts)["p"].(map[string]any)["headers"].(map[string]string)
	if headers["Authorization"] != "Bearer ${TOK}" {
		t.Fatalf("a fork that supports bearer headers must render one: %#v", headers)
	}
}

func TestProvidersOmitInputWhenNoModalitySupported(t *testing.T) {
	cfg := ir.Config{Providers: []ir.Provider{{
		ID:       "p",
		Protocol: ir.ProtocolOpenAICompletions,
		BaseURL:  "https://example.com",
		Models:   []ir.Model{{ID: "m", Input: []ir.Modality{ir.ModalityAudio}}},
	}}}
	entry := Providers(cfg, Options{ID: "p", Syntax: DollarEnv})["p"].(map[string]any)
	model := entry["models"].([]any)[0].(map[string]any)
	if _, ok := model["input"]; ok {
		t.Fatalf("a model with no representable modality must omit input: %#v", model)
	}
	if model["id"] != "m" {
		t.Fatalf("model entry must survive the omission: %#v", model)
	}
}
