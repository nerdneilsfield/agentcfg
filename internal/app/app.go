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
	"agentcfg/internal/example"
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
		return err
	}
	req.Log.InfoW("validation started", "command", "validate", "source", req.ConfigPath, "targets", len(targets))
	if diags := runTargetValidation(cfg, targets); len(diags) > 0 {
		printDiagnostics(req.Stderr, diags)
		if diag.HasErrors(diags) {
			return fmt.Errorf("target validation failed")
		}
	}
	_, _ = fmt.Fprintf(req.Stderr, "%s: OK (%d providers, %d MCP servers, %d targets)\n",
		req.ConfigPath, len(cfg.Providers), len(cfg.MCP), len(targets))
	return nil
}

// GenExample writes the bundled example IR to req.Stdout, or to the file at
// path when path is non-empty. An existing file is never overwritten:
// generating a starter config must not clobber user work.
func GenExample(req Request, path string) error {
	if req.Stdout == nil || req.Stderr == nil {
		return fmt.Errorf("stdout and stderr are required")
	}
	if path == "" {
		if _, err := req.Stdout.Write([]byte(example.IR)); err != nil {
			return err
		}
		return nil
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("%s already exists; refusing to overwrite it", path)
		}
		return fmt.Errorf("creating %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	if _, err := f.WriteString(example.IR); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(req.Stderr, "wrote %s\n", path)
	return nil
}

// Generate loads, validates, emits, and renders artifacts to stdout.
// stdout is written only after every selected target emits successfully.
func Generate(req Request) error {
	cfg, targets, err := loadAndSelect(req)
	if err != nil {
		return err
	}
	if diags := runTargetValidation(cfg, targets); len(diags) > 0 {
		printDiagnostics(req.Stderr, diags)
		if diag.HasErrors(diags) {
			return fmt.Errorf("target validation failed")
		}
	}

	var arts []artifact.Artifact
	for _, t := range targets {
		emitted, err := t.Emit(cfg)
		if err != nil {
			return fmt.Errorf("emitting %s: %w", t.ID(), err)
		}
		req.Log.InfoW("emitted artifacts", "command", "gen", "target", t.ID(), "artifacts", len(emitted))
		arts = append(arts, emitted...)
	}
	arts = artifact.SortedByName(arts)

	var buf bytesBuf
	if err := artifact.Render(&buf, len(targets), arts); err != nil {
		return err
	}
	if _, err := req.Stdout.Write(buf.Bytes()); err != nil {
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
	}
	if diag.HasErrors(diags) {
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
		diags = append(diags, target.AuthTypeDiagnostics(t, cfg)...)
	}
	return diags
}

func printDiagnostics(w io.Writer, diags []diag.Diagnostic) {
	for _, d := range diags {
		_, _ = fmt.Fprintln(w, d.String())
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
