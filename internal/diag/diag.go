// Package diag defines user-facing diagnostics for IR and target validation.
package diag

import (
	"fmt"
	"strings"
)

// Severity classifies a diagnostic.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

// Diagnostic is one validation finding.
type Diagnostic struct {
	Severity Severity
	Target   string // empty for IR-level diagnostics
	Path     string // dotted IR path, e.g. "providers[0].models[1].id"
	Message  string
}

func Errorf(path, format string, args ...any) Diagnostic {
	return Diagnostic{Severity: SeverityError, Path: path, Message: fmtSprintf(format, args...)}
}

func TargetErrorf(target, path, format string, args ...any) Diagnostic {
	return Diagnostic{Severity: SeverityError, Target: target, Path: path, Message: fmtSprintf(format, args...)}
}

func fmtSprintf(format string, args ...any) string {
	return fmt.Sprintf(format, args...)
}

// HasErrors reports whether any diagnostic is an error.
func HasErrors(diags []Diagnostic) bool {
	for _, d := range diags {
		if d.Severity == SeverityError {
			return true
		}
	}
	return false
}

// String renders one diagnostic as a single line.
func (d Diagnostic) String() string {
	var b strings.Builder
	if d.Target != "" {
		b.WriteString("[" + d.Target + "] ")
	}
	if d.Path != "" {
		b.WriteString(d.Path + ": ")
	}
	b.WriteString(string(d.Severity) + ": " + d.Message)
	return b.String()
}
