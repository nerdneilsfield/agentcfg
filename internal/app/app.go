// Package app orchestrates load, validate, emit, and render. It has no
// CLI framework dependencies.
package app

import (
	"fmt"
	"io"
	"os"

	"github.com/GoFarsi/zapper"

	"agentcfg/internal/artifact"
	"agentcfg/internal/diag"
	"agentcfg/internal/ir"
	"agentcfg/internal/target"
)

// Request is one CLI invocation independent of Cobra.
type Request struct {
	ConfigPath string
	To         string // --to value; empty means use IR targets
	Stdout     io.Writer
	Stderr     io.Writer
	Log        zapper.Zapper
}

// Validate loads and validates the IR and every selected target.
func Validate(req Request) error {
	cfg, targets, err := loadAndSelect(req)
	if err != nil {
		fmt.Fprintln(req.Stderr, "error:", err)
		return err
	}
	req.Log.InfoW("validation started", "command", "validate", "source", req.ConfigPath, "targets", len(targets))
	if diags := runTargetValidation(cfg, targets); len(diags) > 0 {
		printDiagnostics(req.Stderr, diags)
		err := fmt.Errorf("validation failed with %d diagnostic(s)", len(diags))
		fmt.Fprintln(req.Stderr, "error:", err)
		return err
	}
	fmt.Fprintf(req.Stderr, "%s: OK (%d providers, %d MCP servers, %d targets)\n",
		req.ConfigPath, len(cfg.Providers), len(cfg.MCP), len(targets))
	return nil
}

// Generate loads, validates, emits, and renders artifacts to stdout.
// stdout is written only after every selected target emits successfully.
func Generate(req Request) error {
	cfg, targets, err := loadAndSelect(req)
	if err != nil {
		fmt.Fprintln(req.Stderr, "error:", err)
		return err
	}
	if diags := runTargetValidation(cfg, targets); len(diags) > 0 {
		printDiagnostics(req.Stderr, diags)
		err := fmt.Errorf("validation failed with %d diagnostic(s)", len(diags))
		fmt.Fprintln(req.Stderr, "error:", err)
		return err
	}

	var arts []artifact.Artifact
	for _, t := range targets {
		emitted, err := t.Emit(cfg)
		if err != nil {
			err = fmt.Errorf("emitting %s: %w", t.ID(), err)
			fmt.Fprintln(req.Stderr, "error:", err)
			return err
		}
		req.Log.InfoW("emitted artifacts", "command", "gen", "target", t.ID(), "artifacts", len(emitted))
		arts = append(arts, emitted...)
	}
	arts = artifact.SortedByName(arts)

	var buf bytesBuf
	if err := artifact.Render(&buf, len(targets), arts); err != nil {
		fmt.Fprintln(req.Stderr, "error:", err)
		return err
	}
	if _, err := req.Stdout.Write(buf.Bytes()); err != nil {
		fmt.Fprintln(req.Stderr, "error:", err)
		return err
	}
	return nil
}

func loadAndSelect(req Request) (ir.Config, []target.Target, error) {
	if req.ConfigPath == "" {
		return ir.Config{}, nil, fmt.Errorf("config path is required")
	}
	if req.Stdout == nil || req.Stderr == nil || req.Log == nil {
		return ir.Config{}, nil, fmt.Errorf("stdout, stderr, and logger are required")
	}
	src, err := os.ReadFile(req.ConfigPath)
	if err != nil {
		return ir.Config{}, nil, fmt.Errorf("reading %s: %w", req.ConfigPath, err)
	}
	cfg, diags, err := ir.Load(src)
	if err != nil {
		return cfg, nil, err
	}
	if len(diags) > 0 {
		printDiagnostics(req.Stderr, diags)
		return cfg, nil, fmt.Errorf("invalid agentcfg.yaml (%d diagnostic(s))", len(diags))
	}
	targets, err := target.Select(req.To, cfg.Targets)
	if err != nil {
		return cfg, nil, err
	}
	return cfg, targets, nil
}

func runTargetValidation(cfg ir.Config, targets []target.Target) []diag.Diagnostic {
	var diags []diag.Diagnostic
	for _, t := range targets {
		diags = append(diags, t.Validate(cfg)...)
	}
	return diags
}

func printDiagnostics(w io.Writer, diags []diag.Diagnostic) {
	for _, d := range diags {
		fmt.Fprintln(w, d.String())
	}
}

// bytesBuf is a small indirection so Render writes to a buffer before
// stdout, keeping the stdout-only-after-success contract.
type bytesBuf struct{ b []byte }

func (b *bytesBuf) Write(p []byte) (int, error) {
	b.b = append(b.b, p...)
	return len(p), nil
}

func (b *bytesBuf) Bytes() []byte { return b.b }
