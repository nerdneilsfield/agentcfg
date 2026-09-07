# agentcfg

agentcfg compiles one `agentcfg.yaml` into native config artifacts for
coding-agent CLIs: Codex, OpenCode, Pi, Prime Agent, DeepSeek Harness,
Grok Build, Kimi Code, ZCode, and MiMo Code.

It manages **custom providers (models) and MCP servers only**. It does not
store secrets (environment references only), run providers, or modify
target files. The IR contract lives in [docs/protocol.md](docs/protocol.md)
and the architecture in [docs/architecture.md](docs/architecture.md).

## Install

```sh
go install ./cmd/agentcfg
```

## Usage

```sh
# validate the IR and the selected targets
agentcfg validate --config agentcfg.yaml --to codex,opencode

# generate native configs to stdout (single artifact: raw content,
# multiple artifacts: stable BEGIN/END bundle stream)
agentcfg gen --config agentcfg.yaml --to all
```

`--to` overrides the `targets:` list in `agentcfg.yaml`. `gen` writes only
to stdout; diagnostics and logs go to stderr.

## Development

```sh
CGO_ENABLED=0 go build ./...
CGO_ENABLED=0 go test ./...
CGO_ENABLED=0 go test -bench=. ./internal/ir   # YAML decode baseline
goreleaser check                                # release config check
```

Releases are built with GoReleaser v2 (`.goreleaser.yaml`) with
`CGO_ENABLED=0` for darwin/linux/windows on amd64/arm64.
