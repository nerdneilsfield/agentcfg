package all

import (
	"strings"
	"testing"

	"agentcfg/internal/diag"
	"agentcfg/internal/ir"
	"agentcfg/internal/target"
)

func TestScalarAPIKeys(t *testing.T) {
	for _, id := range target.IDs() {
		for _, key := range []string{"literal-test-key", "ENV:AGENTCFG_TEST_KEY"} {
			t.Run(id+"/"+key, func(t *testing.T) {
				t.Setenv("AGENTCFG_TEST_KEY", "must-not-be-resolved")
				protocol := "openai-completions"
				if id == "codex" {
					protocol = "openai-responses"
				}
				cfg, ds, err := ir.Load([]byte("version: 1\nproviders:\n  - id: test\n    protocol: " + protocol + "\n    base_url: https://example.com/v1\n    api_key: " + key + "\n    models: [{id: model, context_window: 128000}]\n"))
				if err != nil || diag.HasErrors(ds) {
					t.Fatalf("load: %v %v", err, ds)
				}
				if id == "crush" {
					n := int64(8192)
					cfg.Providers[0].Models[0].MaxOutputTokens = &n
				}
				emitter, _ := target.Lookup(id)
				ds = emitter.Validate(cfg)
				reject := ((id == "goose" || id == "deepseek-harness") && key == "literal-test-key") || ((id == "cline" || id == "zcode") && key != "literal-test-key")
				if reject {
					if !diag.HasErrors(ds) {
						t.Fatal("expected unsupported key diagnostic")
					}
					return
				}
				if diag.HasErrors(ds) {
					t.Fatalf("validate: %v", ds)
				}
				arts, err := emitter.Emit(cfg)
				if err != nil {
					t.Fatal(err)
				}
				output := ""
				for _, a := range arts {
					output += string(a.Content)
				}
				want := "literal-test-key"
				if key != want {
					want = "AGENTCFG_TEST_KEY"
				}
				if !strings.Contains(output, want) {
					t.Fatalf("missing credential in output: %s", output)
				}
				native := ""
				switch id {
				case "deepseek-harness":
					native = "apiKeyEnv: AGENTCFG_TEST_KEY"
					if strings.Contains(output, "apiKey:") || !strings.Contains(output, "baseURL: https://example.com/v1") {
						t.Fatalf("incorrect DSH provider fields: %s", output)
					}
				case "codex":
					if key == "literal-test-key" {
						native = `experimental_bearer_token = "literal-test-key"`
						if strings.Contains(output, "env_key =") {
							t.Fatal("literal key emitted as environment reference")
						}
					} else {
						native = `env_key = "AGENTCFG_TEST_KEY"`
						if strings.Contains(output, "experimental_bearer_token =") {
							t.Fatal("environment reference emitted as literal key")
						}
					}
				case "kimi":
					if key == "literal-test-key" {
						native = `api_key = "literal-test-key"`
						if strings.Contains(output, "api_key_env") {
							t.Fatal("literal key emitted as environment reference")
						}
					} else {
						native = `api_key_env = "AGENTCFG_TEST_KEY"`
						if strings.Contains(output, `api_key =`) {
							t.Fatal("environment reference emitted as literal key")
						}
					}
				}
				if native != "" && !strings.Contains(output, native) {
					t.Fatalf("missing native credential field %q: %s", native, output)
				}
				if strings.Contains(output, "must-not-be-resolved") || strings.Contains(output, "ENV:") {
					t.Fatalf("untranslated or resolved environment reference: %s", output)
				}
			})
		}
	}
}

func TestRejectsNativeExpressionsInLiteralAPIKeys(t *testing.T) {
	cases := map[string]string{
		"crush": "$(touch /tmp/not-executed)", "pi": "$TOKEN", "prime-agent": "!command",
		"opencode": "{env:TOKEN}", "mimocode": "{file:secret}", "openclaw": "${TOKEN}", "hermes": "${TOKEN}",
	}
	for id, key := range cases {
		t.Run(id, func(t *testing.T) {
			emitter, _ := target.Lookup(id)
			ds := emitter.Validate(ir.Config{Providers: []ir.Provider{{ID: "test", Protocol: ir.ProtocolOpenAICompletions, APIKey: ir.HeaderValue{Value: key}}}})
			found := false
			for _, d := range ds {
				if strings.Contains(d.String(), "literal API key contains native expression") {
					found = true
				}
				if strings.Contains(d.String(), key) {
					t.Fatal("diagnostic exposed key")
				}
			}
			if !found {
				t.Fatalf("missing literal expression diagnostic: %v", ds)
			}
		})
	}
}
