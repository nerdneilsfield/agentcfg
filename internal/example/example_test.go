package example

import (
	"os"
	"path/filepath"
	"testing"

	"agentcfg/internal/diag"
	"agentcfg/internal/ir"
	"agentcfg/internal/target"
	_ "agentcfg/internal/target/all"
)

// The embedded copy feeds "agentcfg gen-example"; the repo-root file is what
// humans read. They must stay byte-identical.
func TestEmbeddedMatchesRepoRoot(t *testing.T) {
	want, err := os.ReadFile(filepath.Join("..", "..", "example.yaml"))
	if err != nil {
		t.Fatalf("reading repo-root example.yaml: %v", err)
	}
	if IR != string(want) {
		t.Fatalf("embedded example drifted from repo-root example.yaml (%d vs %d bytes); re-copy the root file into internal/example/", len(want), len(IR))
	}
}

// The shipped example tells readers to validate it as-is, and docs/protocol.md
// claims it validates for its own default targets. An example that fails its own
// selection is worse than no example, so pin the claim.
func TestEmbeddedExampleValidatesForItsTargets(t *testing.T) {
	cfg, diags, err := ir.Load([]byte(IR))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if diag.HasErrors(diags) {
		t.Fatalf("IR diagnostics: %v", diags)
	}
	selected, err := target.Select("", cfg.Targets)
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	for _, tgt := range selected {
		ds := append(tgt.Validate(cfg), target.AuthTypeDiagnostics(tgt, cfg)...)
		if diag.HasErrors(ds) {
			t.Errorf("%s rejects the shipped example: %v", tgt.ID(), ds)
		}
	}
}
