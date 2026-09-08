package example

import (
	"os"
	"path/filepath"
	"testing"
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
