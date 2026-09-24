package app

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"agentcfg/internal/artifact"
	"agentcfg/internal/diag"
	"agentcfg/internal/example"
	"agentcfg/internal/ir"
	"agentcfg/internal/logging"
	"agentcfg/internal/target"
	_ "agentcfg/internal/target/all"
)

type errorTarget struct{ emitted bool }

func (*errorTarget) ID() string { return "app-test-error" }

func (*errorTarget) Validate(ir.Config) []diag.Diagnostic {
	return []diag.Diagnostic{{Severity: diag.SeverityError, Target: "app-test-error", Message: "unsafe output"}}
}

func (t *errorTarget) Emit(ir.Config) ([]artifact.Artifact, error) {
	t.emitted = true
	return nil, nil
}

func TestTargetErrorsBlockValidateAndGenerate(t *testing.T) {
	// A test subprocess keeps this target out of the global --to all registry
	// used by the other application tests.
	if os.Getenv("AGENTCFG_TEST_ERROR_TARGET") != "1" {
		cmd := exec.Command(os.Args[0], "-test.run=^TestTargetErrorsBlockValidateAndGenerate$")
		cmd.Env = append(os.Environ(), "AGENTCFG_TEST_ERROR_TARGET=1")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("error target check: %v\n%s", err, out)
		}
		return
	}
	targetWithError := &errorTarget{}
	target.Register(targetWithError)
	for _, run := range []func(Request) error{Validate, Generate} {
		var stdout, stderr bytes.Buffer
		err := run(newRequest(fixture(t, "ir", "example.yaml"), targetWithError.ID(), &stdout, &stderr))
		if err == nil || targetWithError.emitted || stdout.Len() != 0 || strings.Contains(stderr.String(), ": OK") || !strings.Contains(stderr.String(), "unsafe output") {
			t.Fatalf("error diagnostic did not block operation: err=%v emitted=%v stdout=%q stderr=%q", err, targetWithError.emitted, stdout.String(), stderr.String())
		}
	}
}

func newRequest(cfg, to string, stdout, stderr *bytes.Buffer) Request {
	return Request{
		ConfigPath: cfg,
		To:         to,
		Stdout:     stdout,
		Stderr:     stderr,
		Log:        logging.New(logging.LevelWarn),
	}
}

func fixture(t *testing.T, parts ...string) string {
	t.Helper()
	return filepath.Join(append([]string{"..", "..", "testdata"}, parts...)...)
}

func readGolden(t *testing.T, parts ...string) string {
	t.Helper()
	b, err := os.ReadFile(fixture(t, parts...))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestGenerateCodexGolden(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := Generate(newRequest(fixture(t, "ir", "example.yaml"), "codex", &stdout, &stderr)); err != nil {
		t.Fatalf("Generate: %v\nstderr: %s", err, stderr.String())
	}
	if got, want := stdout.String(), readGolden(t, "targets", "codex", "config.toml.golden"); got != want {
		t.Fatalf("codex output mismatch\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestGenerateOpenCodeGolden(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := Generate(newRequest(fixture(t, "ir", "example-completions.yaml"), "opencode", &stdout, &stderr)); err != nil {
		t.Fatalf("Generate: %v\nstderr: %s", err, stderr.String())
	}
	if got, want := stdout.String(), readGolden(t, "targets", "opencode", "opencode.json.golden"); got != want {
		t.Fatalf("opencode output mismatch\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestGenerateBundleGolden(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := Generate(newRequest(fixture(t, "ir", "example.yaml"), "codex,pi", &stdout, &stderr)); err != nil {
		t.Fatalf("Generate: %v\nstderr: %s", err, stderr.String())
	}
	if got, want := stdout.String(), readGolden(t, "bundles", "codex-pi.txt.golden"); got != want {
		t.Fatalf("bundle output mismatch\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestGenerateAllTargets(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := Generate(newRequest(fixture(t, "ir", "example.yaml"), "all", &stdout, &stderr)); err != nil {
		t.Fatalf("Generate all: %v\nstderr: %s", err, stderr.String())
	}
	if stdout.Len() == 0 {
		t.Fatal("expected supported target artifacts")
	}
}

func TestValidateExampleOK(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := Validate(newRequest(fixture(t, "ir", "example.yaml"), "", &stdout, &stderr)); err != nil {
		t.Fatalf("Validate: %v\nstderr: %s", err, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("validate wrote to stdout: %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "OK") {
		t.Fatalf("missing OK summary: %q", stderr.String())
	}
}

func TestValidateExampleCompletionsOK(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := Validate(newRequest(fixture(t, "ir", "example-completions.yaml"), "", &stdout, &stderr)); err != nil {
		t.Fatalf("Validate: %v\nstderr: %s", err, stderr.String())
	}
	if !strings.Contains(stderr.String(), "OK") {
		t.Fatalf("missing OK summary: %q", stderr.String())
	}
}

func TestGenerateSkipsUnsupportedPiMCP(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := Generate(newRequest(fixture(t, "ir", "example-mcp.yaml"), "pi", &stdout, &stderr)); err != nil {
		t.Fatalf("Generate: %v\nstderr: %s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"providers"`) {
		t.Fatalf("expected models output: %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "warning:") {
		t.Fatalf("expected warning: %q", stderr.String())
	}
	stderr.Reset()
	if err := Validate(newRequest(fixture(t, "ir", "example-mcp.yaml"), "pi", &stdout, &stderr)); err != nil {
		t.Fatalf("warning blocked validation: %v", err)
	}
	if !strings.Contains(stderr.String(), "warning:") || !strings.Contains(stderr.String(), ": OK") {
		t.Fatalf("expected warning and OK summary: %q", stderr.String())
	}
}

func TestValidateRejectsUnknownTarget(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := Validate(newRequest(fixture(t, "ir", "example.yaml"), "nope", &stdout, &stderr))
	if err == nil {
		t.Fatal("expected unknown target error")
	}
	if !strings.Contains(err.Error(), `unknown target "nope"`) {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(stderr.String(), "error:") {
		t.Fatalf("app must not print error: itself: %q", stderr.String())
	}
}

func TestGenerateWritesStdoutOnlyAfterSuccess(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := Generate(newRequest(fixture(t, "ir", "example-mcp.yaml"), "pi", &stdout, &stderr)); err != nil {
		t.Fatalf("Generate: %v\nstderr: %s", err, stderr.String())
	}
	if stdout.Len() == 0 || !strings.Contains(stdout.String(), `"providers"`) {
		t.Fatalf("expected generated artifacts: %q", stdout.String())
	}
}

func TestGenerateMCPFixtureForOpenCode(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := Generate(newRequest(fixture(t, "ir", "example-mcp.yaml"), "opencode", &stdout, &stderr)); err != nil {
		t.Fatalf("Generate: %v\nstderr: %s", err, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{`"type": "local"`, `"type": "remote"`, "Bearer {env:GITHUB_TOKEN}", "{env:CONTEXT7_API_KEY}"} {
		if !strings.Contains(out, want) {
			t.Fatalf("opencode MCP output missing %q:\n%s", want, out)
		}
	}
}

func TestCodexRejectsCompletionsProvider(t *testing.T) {
	bad := filepath.Join(t.TempDir(), "bad.yaml")
	src := strings.ReplaceAll(`version: 1
providers:
  - id: legacy
    protocol: openai-completions
    base_url: https://e.com
    models: [{id: m}]
`, "\t", "")
	if err := os.WriteFile(bad, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	err := Generate(newRequest(bad, "codex", &stdout, &stderr))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !strings.Contains(stderr.String(), "wire_api=responses") {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}

func TestGenExampleStdout(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := GenExample(newRequest("", "", &stdout, &stderr), ""); err != nil {
		t.Fatalf("GenExample: %v\nstderr: %s", err, stderr.String())
	}
	if got, want := stdout.String(), example.IR; got != want {
		t.Fatalf("stdout mismatch: got %d bytes, want %d", len(got), len(want))
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestGenExampleWritesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agentcfg.yaml")
	var stdout, stderr bytes.Buffer
	if err := GenExample(newRequest("", "", &stdout, &stderr), path); err != nil {
		t.Fatalf("GenExample: %v\nstderr: %s", err, stderr.String())
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading written file: %v", err)
	}
	if got, want := string(b), example.IR; got != want {
		t.Fatalf("written file mismatch: got %d bytes, want %d", len(got), len(want))
	}
	if stdout.Len() != 0 {
		t.Fatalf("unexpected stdout: %s", stdout.String())
	}
}

func TestGenExampleRefusesExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agentcfg.yaml")
	if err := os.WriteFile(path, []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if err := GenExample(newRequest("", "", &stdout, &stderr), path); err == nil {
		t.Fatal("expected error overwriting an existing file")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(b); got != "mine" {
		t.Fatalf("existing file was modified: %q", got)
	}
}
