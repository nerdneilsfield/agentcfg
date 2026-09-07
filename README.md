# agentcfg

English | [中文](README.zh-CN.md)

[![CI](https://github.com/nerdneilsfield/agentcfg/actions/workflows/ci.yml/badge.svg)](https://github.com/nerdneilsfield/agentcfg/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/nerdneilsfield/agentcfg?display_name=tag&label=release)](https://github.com/nerdneilsfield/agentcfg/releases/latest)
[![Go Reference](https://pkg.go.dev/badge/github.com/nerdneilsfield/agentcfg.svg)](https://pkg.go.dev/github.com/nerdneilsfield/agentcfg)
[![Go Report Card](https://goreportcard.com/badge/github.com/nerdneilsfield/agentcfg)](https://goreportcard.com/report/github.com/nerdneilsfield/agentcfg)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

agentcfg compiles one `agentcfg.yaml` into native config fragments for coding
agent CLIs. Describe your custom model providers and MCP servers **once**, then
render them for every supported CLI.

Design principles:

- **No secrets.** The config references environment variables (`from_env`,
  `api_key_env`, `bearer_from_env`); agentcfg never reads or writes key values.
- **stdout only.** `gen` prints fragments; it never writes target files. You
  review, then merge them into the native config yourself.
- **Never guess.** When a target cannot represent an IR field, the emitter
  rejects it with a diagnostic instead of silently dropping it.

## Supported targets

| Target | Agent | Providers | MCP | Notes |
|---|---|---|---|---|
| `codex` | [openai/codex](https://github.com/openai/codex) | Responses only | stdio | `[model_providers]` TOML fragment; Codex requires the Responses API |
| `opencode` | [sst/opencode](https://github.com/sst/opencode) | OpenAI-compatible only | stdio + HTTP | `provider` + `mcp` in `opencode.json`; Anthropic/Responses providers rejected in v1 |
| `pi` | [earendil-works/pi](https://github.com/earendil-works/pi) | any (TypeScript extension) | via `pi-mcp-adapter` | emits a provider extension; defaults stay deferred until it is loaded |
| `prime-agent` | [contract](docs/agents/prime-agent.md) | any (`models.json`) | stdio + HTTP | `models.json` + `settings.json` `mcpServers` fragments |
| `deepseek-harness` | [deepseek-ai/deepseek-harness](https://github.com/deepseek-ai/deepseek-harness) | any (`api` route field) | stdio (Cordis patch) | YAML provider route + `@deepseek-ai/dsh-mcp-client` patch |
| `grok` | [xai-org/grok-build](https://github.com/xai-org/grok-build) | Chat · Responses · Anthropic | stdio + HTTP | per-model TOML tables; duplicate model ids rejected; literal headers |
| `kimi` | [MoonshotAI/kimi-code](https://github.com/MoonshotAI/kimi-code) | Chat · Responses · Anthropic | stdio + HTTP | flat `[models]` aliases; duplicate model ids and env refs rejected |
| `zcode` | [zcode.z.ai](https://zcode.z.ai) | Chat · Responses · Anthropic | stdio | closed-source CLI; MCP env/headers are literal-only |
| `mimocode` | [XiaomiMiMo/MiMo-Code](https://github.com/XiaomiMiMo/MiMo-Code) | Chat · Responses · Anthropic | stdio + HTTP | single JSON fragment against the official live schema |
| `jcode` | [1jehuang/jcode](https://github.com/1jehuang/jcode) | Chat · Anthropic | stdio + HTTP | custom providers only; Responses API is built-in-provider-only; literal headers |
| `cline` | [cline/cline](https://github.com/cline/cline) | Chat · Responses · Anthropic | stdio + HTTP | literal `apiKey` only (`api_key_env` rejected); MCP env refs rejected; timeout clamped to 1–3600 s |
| `gajae` | [Yeachan-Heo/gajae-code](https://github.com/Yeachan-Heo/gajae-code) | Chat · Responses · Anthropic | stdio + HTTP | all three protocols map 1:1 |
| `hermes` | [NousResearch/hermes-agent](https://github.com/NousResearch/hermes-agent) | Chat · Responses · Anthropic | stdio + HTTP | provider display `name` not emitted |
| `openclaw` | [openclaw/openclaw](https://github.com/openclaw/openclaw) | Chat · Responses · Anthropic | stdio + HTTP | JSON5 native; never write the agent-local `models.json` |
| `crush` | [charmbracelet/crush](https://github.com/charmbracelet/crush) | Chat · Responses · Anthropic | stdio + HTTP | per-model `context_window` + `default_max_tokens` required; no MCP `cwd`; whole-second timeouts |
| `goose` | [block/goose](https://github.com/block/goose) | Chat · Responses (`base_path`) · Anthropic | stdio + streamable HTTP | strict rejections: per-model max tokens, non-text modalities, `tool_calling: false`, env refs in MCP env, fractional timeouts; provider headers are literal |

"Chat" = OpenAI Chat Completions, "Responses" = OpenAI Responses API. The full
field-by-field contract per target — including what is rejected and why — lives
in [`docs/agents/`](docs/agents/README.md). `--to all` selects every target
above.

## Install

Every `v*` tag is published by GoReleaser as static binaries for
darwin/linux/windows on amd64/arm64 (`CGO_ENABLED=0`).

**Download a release** (see the [Releases page](https://github.com/nerdneilsfield/agentcfg/releases)):

```sh
VERSION=vX.Y.Z        # pick the latest tag
OS=darwin             # darwin | linux
ARCH=arm64            # amd64 | arm64

curl -fsSL -o agentcfg.tar.gz \
  "https://github.com/nerdneilsfield/agentcfg/releases/download/${VERSION}/agentcfg_${VERSION}_${OS}_${ARCH}.tar.gz"
curl -fsSL -O \
  "https://github.com/nerdneilsfield/agentcfg/releases/download/${VERSION}/agentcfg_${VERSION}_checksums.txt"

grep "_${OS}_${ARCH}" agentcfg_${VERSION}_checksums.txt | shasum -a 256 -c -
tar -xzf agentcfg.tar.gz
install -m 0755 agentcfg /usr/local/bin/agentcfg
agentcfg version
```

Windows assets are `.zip`; use `certutil -hashfile` and expand-archive instead
of `tar`.

**With Go 1.27+:**

```sh
go install github.com/nerdneilsfield/agentcfg/cmd/agentcfg@latest
```

**From source:**

```sh
git clone https://github.com/nerdneilsfield/agentcfg
cd agentcfg && make build   # produces ./agentcfg
```

## Quick start

1. Write `agentcfg.yaml` next to your project (full field reference:
   [`docs/protocol.md`](docs/protocol.md)):

```yaml
version: 1

providers:
  - id: volcengine
    name: Volcengine
    protocol: openai-completions
    base_url: https://example.com/v1
    api_key_env: VOLC_API_KEY
    headers:
      X-Tenant:
        value: engineering
    models:
      - id: glm-5.3
        name: GLM-5.3
        context_window: 128000
        max_output_tokens: 8192
        input: [text, image]
        output: [text]
        reasoning: true
        tool_calling: true

mcp:
  - id: context7
    transport: stdio
    command: [npx, -y, "@upstash/context7-mcp"]
    env:
      CONTEXT7_API_KEY:
        from_env: CONTEXT7_API_KEY
  - id: github
    transport: http
    url: https://api.githubcopilot.com/mcp/
    headers:
      Authorization:
        bearer_from_env: GITHUB_TOKEN

defaults:
  model: volcengine/glm-5.3

targets: [crush, goose, codex]
```

2. Validate the IR and every selected target:

```sh
agentcfg validate --config agentcfg.yaml
```

Diagnostics name the target, the IR path, and the reason, e.g.:

```
[crush] providers[0].models[0].context_window: error: crush Model.context_window is required
```

3. Generate the native fragments:

```sh
agentcfg gen --config agentcfg.yaml
```

A single artifact is printed raw. Multiple artifacts are printed as a stable
bundle stream, one block per file:

```
===== BEGIN agentcfg artifact =====
target: crush
name: crush.json
format: json
suggested-path: ~/.config/crush/crush.json
===== artifact content begins =====
{ ... }
===== END agentcfg artifact =====
```

`suggested-path` tells you where the fragment belongs. agentcfg never writes
target files in v1: copy each block into the native config (new files can be
used verbatim; existing files need a manual merge).

## Documentation

- [`docs/protocol.md`](docs/protocol.md) — the IR contract: every provider,
  model, MCP, and defaults field, and what validation enforces.
- [`docs/architecture.md`](docs/architecture.md) — how the compiler is put
  together.
- [`docs/agents/`](docs/agents/README.md) — one verified contract per target:
  native file layout, schema sources, field mappings, limitations.

## Development

Requires Go 1.27+. Optional: `gofumpt`, `goimports`, `golangci-lint` (all
installed by `make tools` targets below or manually), and `goreleaser` for
release-only targets.

```sh
make build      # ./agentcfg
make test       # CGO_ENABLED=0 go test ./...
make fmt        # gofumpt + goimports + gofmt
make lint       # golangci-lint run ./...
make vet        # go vet ./...
make check      # fmt-check + lint + vet + test (what CI runs)
make bench      # YAML decode benchmark
make release-check      # goreleaser check (needs goreleaser; not part of check)
make release-snapshot   # goreleaser release --snapshot --clean
```

Layout:

```
cmd/agentcfg/          CLI (Cobra)
internal/ir/           agentcfg.yaml loader and validation (the IR)
internal/target/<id>/  one emitter package per target: Validate + Emit
internal/target/all/   blank imports that register every emitter
internal/artifact/     stdout rendering (raw or bundle stream)
internal/diag/         diagnostics with stable source paths
docs/agents/           per-target native config contracts
```

Adding a target:

1. Research the agent's native config and write `docs/agents/<id>.md`
   (schema source, field mappings, rejections).
2. Create `internal/target/<id>/` with `Validate` and `Emit`; `Emit` returns
   artifacts and must never silently drop unrepresentable IR fields.
3. Register it with a blank import in `internal/target/all/all.go`.
4. Add golden tests; run `make check`.

## License

[MIT](LICENSE)
