# Program architecture

This document is the implementation plan for the stdout-only first release. It deliberately describes a small compiler, not a config-file manager.

## Release boundary

The first executable has two commands:

```text
agentcfg validate [--config agentcfg.yaml] [--to targets]
agentcfg gen      [--config agentcfg.yaml] [--to targets]
```

There is no target-file discovery, read, merge, write, backup, `--in-place`, `--output`, or `diff`. `gen` writes only to stdout; diagnostics go to stderr. A non-zero exit means no complete valid artifact bundle was emitted.

Target selection precedence is:

```text
--to > targets in agentcfg.yaml > error
```

`--to` is a comma-separated list or `all`. `all` means all targets compiled into this binary, not all installed programs on the machine.

## Artifact-first output

An emitter returns zero or more named artifacts. A target does not have to correspond to one native file:

- Codex: one TOML fragment.
- OpenCode: one JSON fragment.
- Pi: a TypeScript provider extension; later, an adapter-owned MCP JSON file if explicitly enabled.
- Prime Agent: `models.json` provider fragment and `settings.json` MCP fragment.
- DeepSeek Harness: `settings.yaml` provider fragment and Cordis MCP patch.
- Grok Build: one TOML fragment (`config.toml`).
- Kimi Code: `config.toml` fragment and `mcp.json` fragment.
- ZCode: one JSON fragment (`config.json`).
- MiMo Code: one JSON fragment (`mimocode.jsonc`).
- jcode: `config.toml` fragment and `mcp.json` fragment.
- Cline CLI: `providers.json`, `models.json`, and `cline_mcp_settings.json` fragments.
- Gajae Code: `models.yml` fragment, a `config.yml` `modelRoles` fragment, and `mcp.json`.
- Hermes Agent: one YAML fragment (`config.yaml`).
- OpenClaw: one JSON fragment (`openclaw.json`).
- Crush: one JSON fragment (`crush.json`).
- Goose: one JSON file per custom provider plus a `config.yaml` fragment.

```go
type Artifact struct {
    Target      string // e.g. "pi"
    Name        string // stable artifact name: "provider-extension"
    Format      string // toml, json, yaml, typescript
    SuggestedPath string // informational; never written in v1
    Content     []byte
}
```

A single selected target with exactly one artifact writes `Content` unchanged to stdout. All other results use a textual, deliberately non-native bundle:

```text
===== BEGIN agentcfg artifact =====
target: prime-agent
name: mcp-settings
format: json
suggested-path: ~/.prime/agent/settings.json
-----
{ "mcpServers": { ... } }
===== END agentcfg artifact =====
```

The `BEGIN` line, metadata order, separator, final newline, and `END` line are stable. This makes the stream readable and lets a later `--format bundle-json` be added without changing emitters. A native fragment is never mixed with bundle metadata when it is the only artifact.


## Required Go toolchain and dependencies

The implementation has these non-negotiable choices:

| Concern | Choice | Reason and boundary |
|---|---|---|
| CLI | `github.com/spf13/cobra` | Root command plus `validate` and `gen` subcommands; Cobra owns flag parsing, usage, and completion generation. Business logic must not depend on `*cobra.Command`. |
| YAML | `go.yaml.in/yaml/v3` | Pure Go, no CGO, maintained YAML-org fork, and the fastest eligible typed-decoding baseline identified in the research. It provides `Decoder.KnownFields(true)` for strict IR decoding. A repository benchmark must confirm this choice against the exact IR fixtures before implementation is finalized. |
| Logging | `github.com/GoFarsi/zapper` | Required structured logging facade over Zap. Application code receives a logger; it must not use package-global logging. |
| Releases | GoReleaser v2 | Builds reproducible release archives and checksums; release configuration lives in `.goreleaser.yaml`. |

`go.mod` must declare a current supported Go version. CI and release builds set `CGO_ENABLED=0`. Dependency review must reject packages that require CGO on any release target. `go list -deps` and GoReleaser's build matrix are part of the release verification.

### Cobra command boundary

```text
agentcfg
├── validate  [--config FILE] [--to TARGETS]
└── gen       [--config FILE] [--to TARGETS]
```

The root command constructs shared dependencies once: stdout, stderr, filesystem reader for the source YAML, target registry, and logger. Each Cobra handler converts flags into an `app.Request` then calls `app.Validate` or `app.Generate`. It contains no IR validation or target-switch logic.

Help, version, shell completion, and a future `--log-level` are Cobra concerns. The first release has no flags that mutate target files.

### YAML loading policy

`ir.Load` accepts bytes and a source label; it does not open paths. It decodes into typed structs with `go.yaml.in/yaml/v3`, calling `Decoder.KnownFields(true)` in strict mode, returns source-positioned diagnostics where available, then runs semantic normalization and validation.

The accepted format is the `agentcfg.yaml` IR in `protocol.md`, not arbitrary YAML. Anchors and aliases may be accepted by the parser but must resolve to values allowed by the typed IR. Custom YAML tags are rejected. `go test -bench` includes realistic small and multi-provider IR fixtures; changing the YAML library requires recording a faster eligible result without changing parsing correctness. YAML is used for input only in v1; no emitter serializes the source document.

### Logging with zapper

Logs are operational diagnostics, not generated output. `gen` reserves stdout for artifacts, so zapper always writes to stderr. The default level is `warn`; `--verbose` switches to `info`, and `--debug` switches to `debug`. Logs use stable structured fields:

```text
command, target, artifact, source, duration_ms
```

No logger may log `api_key_env` values after environment resolution because agentcfg never resolves them. It may log environment **names**. No log line may contain generated secret material; this is enforced through tests using representative credential references.

### GoReleaser v2 release design

`.goreleaser.yaml` will use schema version 2 and build the Cobra binary with:

```text
CGO_ENABLED=0
goos:   darwin, linux, windows
goarch: amd64, arm64
```

It will produce archives (`tar.gz` for Unix, `zip` for Windows), SHA-256 checksums, and a Homebrew formula only after the repository has a release destination. Version, commit, and build date are injected with `-ldflags` into a narrow `internal/buildinfo` package and shown by Cobra's version command. GoReleaser must run `goreleaser check` in CI before a release tag is accepted.

## Packages

```text
cmd/agentcfg/
  main.go                         # process exit code only

internal/buildinfo/
  buildinfo.go                    # version, commit, date injected by GoReleaser

internal/logging/
  logger.go                       # zapper construction; stderr-only logger

internal/app/
  run.go                          # application request orchestration
  targets.go                      # --to parsing and target selection

internal/artifact/
  artifact.go                     # target-neutral Artifact type
  bundle.go                       # deterministic stdout rendering

internal/ir/
  types.go                        # YAML-facing semantic structs
  load.go                         # YAML decode with known-field checking
  normalize.go                    # defaults such as name/id and modalities
  validate.go                     # source-level validation and diagnostics

internal/target/
  target.go                       # Target interface and registry
  capabilities.go                 # protocol/MCP/artifact capability declarations
  codex/emit.go
  opencode/emit.go
  pi/emit.go
  primeagent/emit.go
  deepseekharness/emit.go
  grok/grok.go
  kimi/kimi.go
  zcode/zcode.go
  mimocode/mimocode.go
  jcode/jcode.go
  cline/cline.go
  gajae/gajae.go
  hermes/hermes.go
  openclaw/openclaw.go
  crush/crush.go
  goose/goose.go

internal/diag/
  diagnostic.go                   # severity, source path, target, stable formatter

testdata/
  ir/                             # valid and invalid YAML input
  targets/<target>/               # native artifact golden files
  bundles/                        # multi-artifact stdout golden files

.goreleaser.yaml                  # GoReleaser v2 release matrix and archives
```

No `internal/document`, merge engine, native-config parser, filesystem target-path resolver, or plugin loader belongs in v1.

## Compilation pipeline

```text
YAML bytes
  -> ir.Load
  -> ir.Normalize
  -> ir.ValidateSource
  -> select targets
  -> target.Validate(config) for each selected target
  -> target.Emit(config) for each selected target
  -> artifact.RenderBundle
  -> stdout
```

`validate` stops after target validation. `gen` runs the same validation before any emission. Validation collects all independent diagnostics, sorts them by input path then target, and emits none of the artifacts if any error exists.

## Interfaces

```go
type Target interface {
    ID() string
    Validate(ir.Config) []diag.Diagnostic
    Emit(ir.Config) ([]artifact.Artifact, error)
}
```

Target validation is not a generic boolean capability matrix. It receives the actual normalized configuration and can issue precise diagnostics, such as:

```text
error [codex] providers[0].protocol: Codex 0.153.4 supports only openai-responses
error [deepseek-harness] mcp[1]: HTTP MCP has no verified dsh-mcp-client mapping
error [pi] mcp: Pi requires the external pi-mcp-adapter; no v1 artifact is enabled
```

The registry is a static map populated by constructors. Targets must not self-register through package `init`; deterministic available targets are easier to test and show in help.

## IR ownership

`internal/ir` owns only cross-target semantics defined in [`protocol.md`](protocol.md). It does not import a target package. Emitters own spelling and narrowing:

- `openai-responses` -> Codex `wire_api = "responses"`
- `api_key_env` -> OpenCode `{env:NAME}`, Pi `$NAME`, DSH `apiKeyEnv: NAME`
- `command` argv -> Codex `command` plus `args`, or OpenCode `command` array

A field unavailable in a target is omitted only if omitting it preserves the documented semantics. Optional model capability metadata (`context_window`, modalities, `reasoning`, and `tool_calling`) may be omitted for a target that has no custom model catalog at all, such as Codex; the emitter is not claiming it configured those properties. A provider protocol, endpoint, credential reference, request header, MCP transport, MCP command/URL, MCP environment, or MCP authentication rule may never be dropped. Otherwise validation rejects the selected target. This rule prevents “successful” generation that silently makes a route or MCP server unusable.

## Failure and stdout contract

- Usage or YAML/IR/target validation errors: formatted diagnostics to stderr; exit `1`; stdout empty.
- Internal emitter or serialization error: contextual error to stderr; exit `1`; stdout empty.
- Successful `validate`: human-readable `ok` line(s) to stdout; exit `0`.
- Successful `gen`: only native content or an artifact bundle on stdout; exit `0`.

Emitters build artifacts in memory. `app` writes stdout only after every target emits successfully, so a later emitter failure cannot leave a partial copy-paste bundle.

## Tests

1. **IR unit tests**: YAML decoding, defaults, identifiers, secret-reference restrictions, duplicate IDs, model references, and transport constraints.
2. **Target golden tests**: one fixture per supported protocol/MCP combination; compare each artifact byte-for-byte.
3. **Target rejection tests**: unsupported protocol, unsupported transport, adapter prerequisite, and no-default-mapping diagnostics.
4. **Bundle tests**: one artifact stays raw; multiple artifacts receive stable wrappers in target/artifact sort order.
5. **CLI black-box tests**: default targets, `--to` precedence, `--to all`, stderr/stdout separation, and exit codes.

Tests do not contact a provider, inspect installed CLIs, or read user config. Compatibility tests against a real target version are a separate manual/release check, documented in `docs/agents/`.

## Deferred extension seams

Future file editing attaches after emission, not inside emitters:

```go
type Destination interface {
    Preview(Artifact) ([]byte, error)
    Write(Artifact) error
}
```

This is intentionally not implemented in v1. It permits later target-specific TOML/JSON/YAML merge logic while keeping `Target.Emit` pure and current golden tests unchanged.
