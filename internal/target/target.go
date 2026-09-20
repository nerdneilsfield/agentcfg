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

// AuthTypeValidator is implemented by targets that map auth_type values to
// native fields. A target that does not implement it maps only "official".
type AuthTypeValidator interface {
	ValidateAuthTypes(cfg ir.Config) []diag.Diagnostic
}

// AuthTypeDiagnostics reports auth_type values the target does not map, so a
// non-official value is never silently ignored.
func AuthTypeDiagnostics(t Target, cfg ir.Config) []diag.Diagnostic {
	if v, ok := t.(AuthTypeValidator); ok {
		return v.ValidateAuthTypes(cfg)
	}
	var diags []diag.Diagnostic
	for i, p := range cfg.Providers {
		if p.EffectiveAuthType() == ir.AuthTypeOfficial {
			continue
		}
		diags = append(diags, diag.TargetErrorf(t.ID(), fmt.Sprintf("providers[%d].auth_type", i),
			"target %s does not implement auth_type: %s", t.ID(), p.EffectiveAuthType()))
	}
	return diags
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
// the IR targets list. --to all selects every registered target; the IR
// targets list does not treat "all" as a wildcard.
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
			if toFlag != "" {
				return nil, fmt.Errorf("empty target id in --to %q", toFlag)
			}
			return nil, fmt.Errorf("empty target id in the IR targets list")
		}
		if id == "all" {
			if toFlag != "" {
				return allTargets(), nil
			}
			return nil, fmt.Errorf(`unknown target "all" (pass --to all to select every compiled-in target; available: %s)`, strings.Join(IDs(), ", "))
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
