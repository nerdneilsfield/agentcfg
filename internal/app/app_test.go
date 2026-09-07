package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agentcfg/internal/logging"
	_ "agentcfg/internal/target/all"
)

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
	if err := Generate(newRequest(fixture(t, "ir", "example.yaml"), "opencode", &stdout, &stderr)); err != nil {
		t.Fatalf("Generate: %v\nstderr: %s", err, stderr.String())
	}
	if got, want := stdout.String(), readGolden(t, "targets", "opencode", "opencode.json.golden"); got != want {
		t.Fatalf("opencode output mismatch\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestGenerateBundleGolden(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := Generate(newRequest(fixture(t, "ir", "example.yaml"), "codex,opencode", &stdout, &stderr)); err != nil {
		t.Fatalf("Generate: %v\nstderr: %s", err, stderr.String())
	}
	if got, want := stdout.String(), readGolden(t, "bundles", "codex-opencode.txt.golden"); got != want {
		t.Fatalf("bundle output mismatch\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestGenerateAllTargets(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := Generate(newRequest(fixture(t, "ir", "example.yaml"), "all", &stdout, &stderr))
	if err != nil {
		t.Fatalf("Generate: %v\nstderr: %s", err, stderr.String())
	}
	out := stdout.String()
	for _, marker := range []string{"target: codex", "target: opencode", "target: pi", "target: prime-agent", "target: deepseek-harness"} {
		if !strings.Contains(out, marker) {
			t.Fatalf("bundle missing %q:\n%s", marker, out)
		}
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

func TestValidateRejectsPiMCP(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := Generate(newRequest(fixture(t, "ir", "example-mcp.yaml"), "pi", &stdout, &stderr))
	if err == nil {
		t.Fatal("expected pi MCP rejection")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout must stay empty on failure: %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "pi has no built-in MCP support") {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}

func TestValidateRejectsUnknownTarget(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := Validate(newRequest(fixture(t, "ir", "example.yaml"), "nope", &stdout, &stderr))
	if err == nil {
		t.Fatal("expected unknown target error")
	}
	if !strings.Contains(stderr.String(), `unknown target "nope"`) {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}

func TestGenerateWritesStdoutOnlyAfterSuccess(t *testing.T) {
	var stdout, stderr bytes.Buffer
	// pi rejects the MCP fixture, so generation must fail with empty stdout.
	err := Generate(newRequest(fixture(t, "ir", "example-mcp.yaml"), "codex,pi", &stdout, &stderr))
	if err == nil {
		t.Fatal("expected validation failure")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout must stay empty on failure: %q", stdout.String())
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
	if err == nil {
		t.Fatal("expected codex protocol rejection")
	}
	if !strings.Contains(stderr.String(), "wire_api=responses") {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}
