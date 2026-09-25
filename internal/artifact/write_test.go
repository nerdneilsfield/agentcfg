package artifact

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMergeOwnedSections(t *testing.T) {
	tests := []struct {
		name         string
		a            Artifact
		old          string
		want, absent []string
	}{
		{"qwen", Artifact{Target: "qwen-code", Format: "json", Content: []byte(`{"modelProviders":{"new":[]},"security":{"auth":{"selectedType":"openai"}},"env":{"AGENTCFG_KEY_NEW":"key"}}`)}, `{"modelProviders":{"stale":[]},"mcpServers":{"old":{}},"security":{"auth":{"selectedType":"old","unrelated":true},"other":true},"env":{"AGENTCFG_KEY_OLD":"secret","USER_SETTING":"stay"},"theme":"dark"}`, []string{"unrelated", "USER_SETTING", "theme", "AGENTCFG_KEY_NEW"}, []string{"stale", "mcpServers", "AGENTCFG_KEY_OLD"}},
		{"nested", Artifact{Target: "openclaw", Format: "json", Content: []byte(`{"models":{"providers":{"new":{}}},"agents":{"defaults":{"model":{"primary":"new/m"}}},"mcp":{"servers":{}}}`)}, `{"models":{"providers":{"old":{}},"mode":"merge"},"agents":{"defaults":{"workspace":"/keep","model":{"primary":"old","fallbacks":["stale"]}}},"mcp":{"servers":{"old":{}},"enabled":true}}`, []string{"workspace", "mode", "enabled"}, []string{"stale", "fallbacks"}},
		{"jsonc", Artifact{Target: "kilo", Format: "json", Content: []byte(`{"provider":{}}`)}, "{ // comment\n\"provider\":{\"old\":{}},\"theme\":\"https://host/*literal*/\",}", []string{"https://host/*literal*/"}, []string{"old"}},
		{"toml", Artifact{Target: "codex", Format: "toml", Content: []byte("[model_providers.new]\nname = 'new'\n")}, "approval_policy = 'never'\n[model_providers.old]\nname = 'stale'\n[mcp_servers.old]\ncommand = 'stale'\n", []string{"approval_policy", "new"}, []string{"stale", "mcp_servers"}},
		{"yaml", Artifact{Target: "continue", Format: "yaml", Content: []byte("name: agentcfg\nmodels: []\n")}, "name: Personal\nmodels:\n  - name: stale\nmcpServers: []\nrules:\n  - keep this\n", []string{"Personal", "keep this"}, []string{"stale", "mcpServers"}},
		{"goose", Artifact{Target: "goose", Name: "config.yaml", Format: "yaml", Content: []byte("{}\n")}, "extensions:\n  builtin:\n    type: builtin\n  stale:\n    type: stdio\n", []string{"builtin"}, []string{"stale"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := merge(tt.a, []byte(tt.old))
			if err != nil {
				t.Fatal(err)
			}
			for _, s := range tt.want {
				if !strings.Contains(string(b), s) {
					t.Fatalf("missing %q: %s", s, b)
				}
			}
			for _, s := range tt.absent {
				if strings.Contains(string(b), s) {
					t.Fatalf("retained %q: %s", s, b)
				}
			}
		})
	}
}

func TestWritePreflightAndPermissions(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "models.json")
	second := filepath.Join(dir, "settings.json")
	old := []byte(`{"providers":{"old":{}},"keep":true}`)
	if err := os.WriteFile(first, old, 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, []byte("broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	arts := []Artifact{{Target: "prime-agent", Name: "models.json", Format: "json", Content: []byte(`{"providers":{}}`)}, {Target: "prime-agent", Name: "settings.json", Format: "json", Content: []byte(`{"mcpServers":{}}`)}}
	if err := WriteFiles(arts, 1, dir, false, io.Discard); err == nil {
		t.Fatal("expected parse error")
	}
	b, _ := os.ReadFile(first)
	if string(b) != string(old) {
		t.Fatal("first file modified before preflight finished")
	}
	if err := os.WriteFile(second, []byte(`{"theme":"dark"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteFiles(arts, 1, dir, false, io.Discard); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(first)
	if info.Mode().Perm() != 0o640 {
		t.Fatal("permissions changed")
	}
	b, _ = os.ReadFile(first)
	if strings.Contains(string(b), "old") || !strings.Contains(string(b), "keep") {
		t.Fatalf("bad merge: %s", b)
	}
	before := string(b)
	if err := WriteFiles(arts, 1, dir, false, io.Discard); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(first)
	if string(b) != before {
		t.Fatal("merge not idempotent")
	}
}

func TestNativeAndMultiFileLayout(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	a := Artifact{Target: "kilo", Name: "opencode.json", Format: "json", SuggestedPath: "~/.config/kilo/opencode.json", Content: []byte(`{"provider":{}}`)}
	if err := WriteFiles([]Artifact{a}, 1, "", true, io.Discard); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, ".config/kilo/opencode.json")); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	arts := []Artifact{{Target: "fast-agent", Name: "fastagent.config.yaml", Format: "yaml", Content: []byte("{}\n")}, {Target: "fast-agent", Name: "model-overlays/test.yaml", Format: "yaml", Content: []byte("model: test\n")}}
	if err := WriteFiles(arts, 2, dir, false, io.Discard); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "fast-agent/.fast-agent/model-overlays/test.yaml")); err != nil {
		t.Fatal(err)
	}
}

func TestCordisPreservesTaggedValues(t *testing.T) {
	a := Artifact{Target: "deepseek-harness", Name: "cordis.patch.yml", Format: "yaml", Content: []byte("- insert:\n    - name: '@deepseek-ai/dsh-mcp-client'\n      config:\n        token: !!js `Bearer ${process.env.KEY}`\n")}
	old := []byte("- insert:\n    - name: keep-plugin\n      value: !!js process.env.OTHER\n    - name: '@deepseek-ai/dsh-mcp-client'\n      id: stale\n")
	b, err := merge(a, old)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "!!js") || !strings.Contains(string(b), "keep-plugin") || strings.Contains(string(b), "stale") {
		t.Fatalf("bad patch: %s", b)
	}
}
