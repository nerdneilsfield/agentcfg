package artifact

import (
	"bytes"
	"strings"
	"testing"
)

func TestRenderRawSingleArtifact(t *testing.T) {
	var buf bytes.Buffer
	arts := []Artifact{{Target: "codex", Name: "config.toml", Format: "toml", SuggestedPath: "p", Content: []byte("a = 1")}}
	if err := Render(&buf, 1, arts); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "a = 1\n" {
		t.Fatalf("got %q", buf.String())
	}
}

func TestRenderBundle(t *testing.T) {
	var buf bytes.Buffer
	arts := []Artifact{
		{Target: "b", Name: "two.json", Format: "json", SuggestedPath: "p2", Content: []byte("{}")},
		{Target: "a", Name: "one.toml", Format: "toml", SuggestedPath: "p1", Content: []byte("x = 1\n")},
	}
	if err := Render(&buf, 2, arts); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, beginLine) || !strings.Contains(out, endLine) || !strings.Contains(out, contentLine) {
		t.Fatalf("missing bundle markers: %q", out)
	}
	if !strings.Contains(out, "target: a\nname: one.toml\nformat: toml\nsuggested-path: p1") {
		t.Fatalf("bad metadata block: %q", out)
	}
	if strings.Count(out, beginLine) != 2 {
		t.Fatalf("expected 2 artifacts, got %q", out)
	}
}

func TestRenderBundleAddsFinalNewline(t *testing.T) {
	var buf bytes.Buffer
	arts := []Artifact{
		{Target: "a", Name: "one", Format: "toml", SuggestedPath: "p1", Content: []byte("x = 1\n")},
		{Target: "a", Name: "two", Format: "toml", SuggestedPath: "p2", Content: []byte("y = 2")},
	}
	if err := Render(&buf, 1, arts); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "y = 2\n"+endLine) {
		t.Fatalf("missing final newline before END: %q", buf.String())
	}
}

func TestSortedByName(t *testing.T) {
	arts := []Artifact{{Target: "b", Name: "z"}, {Target: "a", Name: "y"}, {Target: "a", Name: "x"}}
	got := SortedByName(arts)
	if got[0].Name != "x" || got[1].Name != "y" || got[2].Name != "z" {
		t.Fatalf("bad order: %+v", got)
	}
}
