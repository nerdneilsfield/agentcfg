// Package target defines the emitter contract and the compiled-in
// target registry.
package target

import (
	"fmt"
	"sort"
	"strings"

	"agentcfg/internal/artifact"
	"agentcfg/internal/diag"
	"agentcfg/internal/ir"
)

// Target converts the IR into native configuration artifacts for one CLI.
type Target interface {
	// ID is the stable target identifier used by --to and targets:.
	ID() string
	// Validate reports diagnostics for configurations this target
	// cannot represent faithfully. It must never silently drop meaning.
	Validate(cfg ir.Config) []diag.Diagnostic
	// Emit renders native artifacts. It is called only after Validate
	// reports no errors.
	Emit(cfg ir.Config) ([]artifact.Artifact, error)
}

// registry maps target IDs to emitters.
var registry = map[string]Target{}

// Register adds a target to the compiled-in registry.
func Register(t Target) {
	if _, dup := registry[t.ID()]; dup {
		panic("target: duplicate registration of " + t.ID())
	}
	registry[t.ID()] = t
}

// IDs returns all registered target IDs in sorted order.
func IDs() []string {
	ids := make([]string, 0, len(registry))
	for id := range registry {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// Lookup returns the target for an ID.
func Lookup(id string) (Target, bool) {
	t, ok := registry[id]
	return t, ok
}

// Select resolves the target list using the CLI --to value first, then
// the IR targets list. An empty --to value of "all" selects every
// registered target.
func Select(toFlag string, cfgTargets []string) ([]Target, error) {
	var ids []string
	switch {
	case toFlag != "":
		ids = strings.Split(toFlag, ",")
	case len(cfgTargets) > 0:
		ids = cfgTargets
	default:
		return nil, fmt.Errorf("no targets: pass --to <id|all> or set targets: in agentcfg.yaml")
	}

	var out []Target
	for _, raw := range ids {
		id := strings.TrimSpace(raw)
		if id == "" {
			return nil, fmt.Errorf("empty target id in %q", toFlag)
		}
		if id == "all" && toFlag != "" {
			return allTargets(), nil
		}
		t, ok := Lookup(id)
		if !ok {
			return nil, fmt.Errorf("unknown target %q (available: %s)", id, strings.Join(IDs(), ", "))
		}
		out = append(out, t)
	}
	return out, nil
}

func allTargets() []Target {
	var out []Target
	for _, id := range IDs() {
		t, _ := Lookup(id)
		out = append(out, t)
	}
	return out
}
