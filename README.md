# agentcfg

English | [中文](README.zh-CN.md)

[![CI](https://github.com/nerdneilsfield/agentcfg/actions/workflows/ci.yml/badge.svg)](https://github.com/nerdneilsfield/agentcfg/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/nerdneilsfield/agentcfg?display_name=tag&label=release)](https://github.com/nerdneilsfield/agentcfg/releases/latest)
[![Go Reference](https://pkg.go.dev/badge/github.com/nerdneilsfield/agentcfg.svg)](https://pkg.go.dev/github.com/nerdneilsfield/agentcfg)
[![Go Report Card](https://goreportcard.com/badge/github.com/nerdneilsfield/agentcfg)](https://goreportcard.com/report/github.com/nerdneilsfield/agentcfg)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

agentcfg compiles one `agentcfg.yaml` into native config fragments for coding
agent CLIs. Describe custom model providers and MCP servers once, then render
them for each supported CLI.

Design principles:

- **Explicit credentials.** Use a quoted string literal or `ENV:NAME`.
  agentcfg never reads the process environment; it emits literals as supplied
  and translates references into each CLI's native syntax.
- **stdout only.** `gen` prints fragments. It never writes target files. Review
  the output, then merge it into the native config yourself.
- **Never guess.** When a target cannot represent an IR field, the emitter
  rejects it with a diagnostic instead of silently dropping it.

## Supported targets

| Target | Agent | Providers | MCP | Notes |
|---|---|---|---|---|
| `codex` | [openai/codex](https://github.com/openai/codex) | Responses only | stdio + HTTP | `[model_providers]` TOML fragment; `ENV:NAME` → `env_key`; MCP `timeout_ms` → `startup_timeout_ms` |
| `opencode` | [sst/opencode](https://github.com/sst/opencode) | OpenAI-compatible only | stdio + HTTP | `provider` + `mcp` in `opencode.json`; Anthropic/Responses providers rejected in v1 |
| `pi` | [earendil-works/pi](https://github.com/earendil-works/pi) | any (`models.json`) | rejected in v1 (no built-in MCP) | `~/.pi/agent/models.json`; `ENV:NAME` → `"$NAME"`; `auth_type: bearer` → `authHeader: true`; unset limits are omitted so Pi's defaults apply |
| `prime-agent` | [contract](docs/agents/prime-agent.md) | any (`models.json`) | stdio + HTTP | `models.json` + `settings.json` `mcpServers` fragments; `ENV:NAME` → bare environment-variable names; `auth_type: bearer` → `authHeader: true` |
| `deepseek-harness` | [deepseek-ai/deepseek-harness](https://github.com/deepseek-ai/deepseek-harness) | any (`api` route field) | stdio + HTTP (Cordis patch) | YAML provider route + `@deepseek-ai/dsh-mcp-client` patch; literal API keys rejected |
| `grok` | [xai-org/grok-build](https://github.com/xai-org/grok-build) | Chat · Responses · Anthropic | stdio + HTTP | per-model TOML tables; duplicate model ids rejected; `ENV:NAME` headers → `env_http_headers` |
| `kimi` | [MoonshotAI/kimi-code](https://github.com/MoonshotAI/kimi-code) | Chat · Responses · Anthropic | stdio + HTTP | flat `[models]` aliases; duplicate model ids rejected; `ENV:NAME` API keys → `api_key_env`; MCP `timeout_ms` → `startupTimeoutMs`; provider headers stay literal |
| `zcode` | [zcode.z.ai](https://zcode.z.ai) | Chat · Anthropic | stdio + HTTP | closed-source CLI; MCP env/headers are literal-only |
| `mimocode` | [XiaomiMiMo/MiMo-Code](https://github.com/XiaomiMiMo/MiMo-Code) | Chat · Anthropic | stdio + HTTP | single JSON fragment against the official live schema |
| `jcode` | [1jehuang/jcode](https://github.com/1jehuang/jcode) | Chat · Anthropic | stdio | custom providers only; Responses API is built-in-provider-only; literal headers |
| `cline` | [cline/cline](https://github.com/cline/cline) | Chat · Responses · Anthropic | stdio + HTTP | literal `apiKey` only (`api_key: "ENV:NAME"` rejected); MCP env refs rejected; timeout is seconds (`ms/1000`) |
| `gajae` | [Yeachan-Heo/gajae-code](https://github.com/Yeachan-Heo/gajae-code) | Chat · Responses · Anthropic | stdio + HTTP | all three protocols map 1:1 |
| `hermes` | [NousResearch/hermes-agent](https://github.com/NousResearch/hermes-agent) | Chat · Responses · Anthropic | stdio + HTTP | provider display `name` not emitted |
| `openclaw` | [openclaw/openclaw](https://github.com/openclaw/openclaw) | Chat · Responses · Anthropic | stdio + HTTP | JSON5 native; never write the agent-local `models.json` |
| `crush` | [charmbracelet/crush](https://github.com/charmbracelet/crush) | Chat · Responses · Anthropic | stdio + HTTP | per-model `context_window` + `default_max_tokens` required; no MCP `cwd`; whole-second timeouts |
| `goose` | [block/goose](https://github.com/block/goose) | Chat · Responses (`base_path`) · Anthropic | stdio + streamable HTTP | strict rejections: per-model max tokens, non-text modalities, `tool_calling: false`, renamed MCP env refs, fractional timeouts; provider headers are literal |

"Chat" = OpenAI Chat Completions, "Responses" = OpenAI Responses API. The full
field-by-field contract per target, including what is rejected and why, lives
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

**With Go 1.27+**, clone and install. The Go module path is `agentcfg`, so
`go install github.com/nerdneilsfield/agentcfg/...` cannot resolve:

```sh
git clone https://github.com/nerdneilsfield/agentcfg
cd agentcfg && go install ./cmd/agentcfg
```

**From source:**

```sh
git clone https://github.com/nerdneilsfield/agentcfg
cd agentcfg && make build   # produces ./agentcfg
```

## Quick start

Literal secrets in the input also appear in generated output. Keep both out of
Git and logs. Prefer `ENV:NAME` when the target supports environment references.

1. Write `agentcfg.yaml` next to your project. The field reference is
   [`docs/protocol.md`](docs/protocol.md). The repository ships
   [`example.yaml`](example.yaml). `agentcfg gen-example -o agentcfg.yaml`
   writes a copy next to your project; without `-o` it prints to stdout.
   An existing file is never overwritten.

```yaml
version: 1

providers:
  - id: volcengine
    name: Volcengine
    protocol: openai-completions
    base_url: https://example.com/v1
    api_key: "ENV:VOLC_API_KEY"
    headers:
      X-Tenant: "engineering"
      X-Gateway-Key: "ENV:GATEWAY_KEY"
    models:
      - id: glm-5.3
        name: GLM-5.3
        context_window: 128000
        max_output_tokens: 8192
        input: [text, image]
        output: [text]
        reasoning: true
        # Selectable provider/model reasoning efforts. These are not defaults.
        variants: [low, medium, high, xhigh, max, ultra]
        tool_calling: true

  - id: anthropic-internal
    name: Anthropic Internal
    protocol: anthropic-messages
    base_url: https://anthropic-internal.example.com
    api_key: "ENV:ANTHROPIC_INTERNAL_TOKEN"
    models:
      - id: claude-internal
        name: Claude Internal
        context_window: 200000
        max_output_tokens: 64000
        input: [text, image]
        output: [text]
        reasoning: true

mcp:
  - id: context7
    transport: stdio
    command: [npx, -y, "@upstash/context7-mcp"]
    env:
      CONTEXT7_API_KEY: "ENV:CONTEXT7_API_KEY"
  - id: github
    transport: http
    url: https://api.githubcopilot.com/mcp/
    headers:
      Authorization: "Bearer ENV:GITHUB_TOKEN"

defaults:
  model: volcengine/glm-5.3

targets: [crush, gajae]
```

What this file says:

- `version: 1` is required. Unknown keys and a second YAML document (`---`)
  are errors.
- Each provider has exactly one `protocol`. `openai-completions` is Chat
  Completions; `openai-responses` is the Responses API; `anthropic-messages`
  is Anthropic. A gateway that speaks two protocols is two providers.
- `api_key` and header values are quoted strings: `"literal"`, `"ENV:NAME"`,
  or `"Bearer ENV:NAME"` on an `Authorization` header. agentcfg never reads
  those environment variables.
- `command` is an argv array, not a shell string.
- `variants` lists selectable reasoning efforts for that model. It does not
  choose a default effort. Crush keeps names such as `ultra`. Gajae's native
  ladder is `minimal`, `low`, `medium`, `high`, `xhigh`, `max`; it skips
  `ultra` and still emits the rest of the model.
- `targets: [crush, gajae]` is the default selection when `--to` is omitted.
  It is not a wildcard. `targets: [all]` is an unknown id.

This example is valid for Crush and Gajae. It is not valid for every CLI:
Codex rejects `openai-completions`, OpenCode rejects `anthropic-messages`,
Cline/ZCode reject `ENV:NAME` API keys, Kimi rejects env-derived provider
headers, and Pi rejects MCP. Put the CLIs you actually generate for in
`targets:`, then override with `--to`.

2. Validate the IR and every selected target. `--config` defaults to
   `agentcfg.yaml` in the current directory:

```sh
agentcfg validate --config agentcfg.yaml
```

Success prints a one-line summary on stderr:

```
agentcfg.yaml: OK (2 providers, 2 MCP servers, 2 targets)
```

A failure names the target, the IR path, and the reason, for example:

```
[crush] providers[0].models[0].context_window: error: crush Model.context_window is required
```

3. Generate the native fragments:

```sh
agentcfg gen --config agentcfg.yaml
agentcfg gen --config agentcfg.yaml --to crush
agentcfg gen --config agentcfg.yaml --to all
```

`--to` overrides `targets:` in the YAML. `--to all` means every emitter
compiled into this binary, not every CLI installed on the machine.

A single artifact is printed raw (valid JSON, TOML, YAML, or TypeScript).
Multiple artifacts are printed as a stable bundle stream, one block per file:

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
target files in v1. A new file can use the block contents as-is; an existing
file needs a manual merge. On any validation or emit error, stdout stays empty
and the process exits `1`.

## Writing agentcfg.yaml

The IR is target-independent. Native spelling lives in
[`docs/protocol.md`](docs/protocol.md) and [`docs/agents/`](docs/agents/README.md).
The patterns below are the ones that usually matter first.

### Credentials

```yaml
api_key: "ENV:VOLC_API_KEY"          # environment reference
api_key: "sk-example-not-a-real-key" # literal; appears in generated output
headers:
  X-Tenant: "engineering"            # literal
  X-Gateway-Key: "ENV:GATEWAY_KEY"   # environment reference
  Authorization: "Bearer ENV:TOKEN"  # Authorization header only
```

Quote values that YAML would treat as a boolean or a number (`true`, `no`,
`1e6`). Cline and ZCode store inline API keys and reject `ENV:NAME` on
`api_key`. Kimi maps `api_key: "ENV:NAME"` to `api_key_env`. Goose and
DeepSeek Harness reject literal API keys (they only have an environment-name
field).

### Protocol

```yaml
protocol: openai-completions   # Chat Completions; OpenCode accepts only this
protocol: openai-responses     # Responses API; Codex accepts only this
protocol: anthropic-messages   # Anthropic Messages
```

`base_url` is the endpoint the CLI should call, often including `/v1`.

### Models

Crush and Kimi require `context_window`. Crush also requires
`max_output_tokens`. Goose rejects per-model `max_output_tokens` and any
non-text modality.

```yaml
models:
  - id: glm-5.3
    name: GLM-5.3
    context_window: 128000
    max_output_tokens: 8192
    input: [text, image]
    output: [text]
    reasoning: true
    variants: [low, medium, high]
    tool_calling: true
```

`id` is the upstream model name on the wire. `variants` requires
`reasoning: true`. Names match `[a-z][a-z0-9_-]*` (`Ultra` is invalid).

### MCP

```yaml
mcp:
  - id: context7
    transport: stdio
    command: [npx, -y, "@upstash/context7-mcp"]
    cwd: /var/lib/context7          # stdio only; Crush/MiMo Code/jcode reject it
    timeout_ms: 30000               # milliseconds; some targets convert to seconds
    env:
      CONTEXT7_API_KEY: "ENV:CONTEXT7_API_KEY"
    enabled: true

  - id: github
    transport: http
    url: https://api.githubcopilot.com/mcp/
    headers:
      Authorization: "Bearer ENV:GITHUB_TOKEN"
```

There is no `sse` transport in the IR. Targets that distinguish SSE from
streamable HTTP map `http` to streamable HTTP. Pi rejects every MCP server
in v1. jcode loads stdio only.

`timeout_ms` stays milliseconds on OpenCode, Gajae, Codex, Kimi
(`startupTimeoutMs`), ZCode, and several others. Crush, Goose, and jcode
require a whole number of seconds (`timeout_ms` divisible by 1000). Grok and
Prime Agent reject `timeout_ms` because they have no verified native field.
Omit it unless every selected target can represent it.

### Defaults and target selection

```yaml
defaults:
  model: volcengine/glm-5.3   # provider-id/model-id; must exist in this file

targets: [crush, gajae]       # used when --to is omitted
```

Selection order: `--to` flag, then YAML `targets:`, then an error. Listing
every compiled-in id in YAML is almost never useful: one document rarely
represents faithfully on Codex, OpenCode, Cline, Kimi, and Pi at the same time.

## CLI

```text
agentcfg validate [--config FILE] [--to TARGETS] [--verbose|--debug]
agentcfg gen      [--config FILE] [--to TARGETS] [--verbose|--debug]
agentcfg gen-example [--output FILE]
agentcfg version
```

| Flag | Default | Meaning |
|---|---|---|
| `-c`, `--config` | `agentcfg.yaml` | Path to the IR document. |
| `-t`, `--to` | (YAML `targets:`) | Comma-separated target ids, or `all`. |
| `-v`, `--verbose` | off | Info logs on stderr. |
| `-d`, `--debug` | off | Debug logs on stderr. |
| `-o`, `--output` | stdout | `gen-example` only: write the bundled example to this path. |

`--config`, `--to`, `--verbose`, and `--debug` are root flags, so
`agentcfg -c file.yaml gen` and `agentcfg gen -c file.yaml` are equivalent.
`gen-example` does not read `--config`; it writes the embedded copy of
[`example.yaml`](example.yaml).

stdout is reserved for artifacts (`gen`) or the example YAML (`gen-example`).
Diagnostics, the `OK` summary, and logs go to stderr. Exit `0` on success,
`1` on usage, decode, validation, or emit errors.

## Documentation

- [`docs/protocol.md`](docs/protocol.md) — the IR contract: every provider,
  model, MCP, and defaults field, validation rules, and worked examples.
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
