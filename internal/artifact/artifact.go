// Package artifact defines target-neutral generated output and the
// stdout rendering contract.
package artifact

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// Artifact is one generated native config fragment.
type Artifact struct {
	Target        string
	Name          string // native file name, e.g. "config.toml"
	Format        string // "toml", "json", "yaml", "typescript"
	SuggestedPath string
	Content       []byte
}

const (
	beginLine   = "===== BEGIN agentcfg artifact ====="
	contentLine = "===== artifact content begins ====="
	endLine     = "===== END agentcfg artifact ====="
)

// Render writes artifacts to w. A single artifact from a single target
// is written as raw native content. Everything else is written as a
// stable bundle stream.
func Render(w io.Writer, targetCount int, arts []Artifact) error {
	if targetCount == 1 && len(arts) == 1 {
		_, err := w.Write(ensureFinalNewline(arts[0].Content))
		return err
	}
	for i, a := range arts {
		if i > 0 {
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
		}
		if err := renderOne(w, a); err != nil {
			return err
		}
	}
	return nil
}

func renderOne(w io.Writer, a Artifact) error {
	var b strings.Builder
	b.WriteString(beginLine + "\n")
	b.WriteString("target: " + a.Target + "\n")
	b.WriteString("name: " + a.Name + "\n")
	b.WriteString("format: " + a.Format + "\n")
	b.WriteString("suggested-path: " + a.SuggestedPath + "\n")
	b.WriteString(contentLine + "\n")
	b.Write(ensureFinalNewline(a.Content))
	b.WriteString(endLine + "\n")
	_, err := io.WriteString(w, b.String())
	return err
}

func ensureFinalNewline(content []byte) []byte {
	if len(content) == 0 || content[len(content)-1] == '\n' {
		return content
	}
	return append(content, '\n')
}

// SortedByName returns artifacts ordered by target and name for
// deterministic output.
func SortedByName(arts []Artifact) []Artifact {
	out := make([]Artifact, len(arts))
	copy(out, arts)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Target != out[j].Target {
			return out[i].Target < out[j].Target
		}
		return out[i].Name < out[j].Name
	})
	return out
}
