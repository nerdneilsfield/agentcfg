package main

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIFlagsAndErrors(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "agentcfg")
	build := exec.Command("go", "build", "-o", bin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	cfg := filepath.Join("..", "..", "testdata", "ir", "example.yaml")

	quiet := runCLI(t, bin, "validate", "--config", cfg)
	if strings.Contains(quiet.stderr, "validation started") {
		t.Fatalf("default level must not log info: %q", quiet.stderr)
	}
	if quiet.code != 0 {
		t.Fatalf("validate: exit %d\n%s", quiet.code, quiet.stderr)
	}

	verbose := runCLI(t, bin, "--verbose", "validate", "--config", cfg)
	if !strings.Contains(verbose.stderr, "validation started") {
		t.Fatalf("--verbose did not enable info logs: %q", verbose.stderr)
	}

	debug := runCLI(t, bin, "-d", "validate", "--config", cfg)
	if !strings.Contains(debug.stderr, "validation started") {
		t.Fatalf("--debug did not enable logs: %q", debug.stderr)
	}

	unknown := runCLI(t, bin, "validate", "--config", cfg, "--to", "nope")
	if unknown.code == 0 {
		t.Fatal("expected unknown target to fail")
	}
	if got := strings.Count(unknown.stderr, "error:"); got != 1 {
		t.Fatalf("error printed %d times:\n%s", got, unknown.stderr)
	}
	if !strings.Contains(unknown.stderr, `unknown target "nope"`) {
		t.Fatalf("unexpected stderr: %q", unknown.stderr)
	}
}

type cliResult struct {
	code   int
	stderr string
}

func runCLI(t *testing.T, bin string, args ...string) cliResult {
	t.Helper()
	cmd := exec.Command(bin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			t.Fatalf("run %v: %v", args, err)
		}
	}
	return cliResult{code: code, stderr: stderr.String()}
}
