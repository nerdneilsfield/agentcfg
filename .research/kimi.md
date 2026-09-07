# Kimi Code CLI native configuration contract

"Kimi Code" / "kimi code" is the official coding-agent CLI from Moonshot AI. There are two generations:

- **Kimi Code CLI (current)**: repo `MoonshotAI/kimi-code` (TypeScript monorepo, MIT license). This is what new users install; the binary/command is `kimi`.
- **Kimi CLI (legacy, being wound down)**: repo `MoonshotAI/kimi-cli` (Python, Apache-2.0). Its README states: "Kimi CLI is evolving into Kimi Code CLI ... Installing Kimi Code CLI automatically migrates your configuration and sessions. This project will be gradually wound down." Legacy config lived at `~/.kimi/config.toml` + `~/.kimi/mcp.json`; the migration moves it to the new layout.

This document targets the **current Kimi Code CLI** (`MoonshotAI/kimi-code`), with legacy notes where relevant.

## Official source URLs (repo, docs) with version/commit/date evidence

- Repo: https://github.com/MoonshotAI/kimi-code (default branch `main`, license MIT, last push 2026-09-07T10:39:19Z at research time)
- Latest release at research time: `@moonshot-ai/kimi-code@0.41.0`, published 2026-09-04T11:01:07Z (GitHub releases API)
- Official docs: https://moonshotai.github.io/kimi-code/en/ (source in repo `docs/en/`)
  - `docs/en/configuration/config-files.md` — config file reference
  - `docs/en/configuration/providers.md` — provider types and model capabilities
  - `docs/en/configuration/overrides.md` — credential resolution rules
  - `docs/en/configuration/env-vars.md` — env var semantics
  - `docs/en/configuration/data-locations.md` — data directory layout
  - `docs/en/customization/mcp.md` — MCP server configuration
- Schema source of truth (zod): `packages/node-sdk/src/config/schema.ts` (providers/models/config), `packages/agent-core-v2/src/mcpCore/config-schema.ts` (mcp.json server entries), `packages/agent-core-v2/src/app/mcpConfig/configLoader.ts` (mcp.json load paths)
- Legacy repo: https://github.com/MoonshotAI/kimi-cli (Apache-2.0, last push 2026-09-01), latest changelog entry 0.29.1 (2026-07-24), docs at https://moonshotai.github.io/kimi-cli/en/

## Install command / how users get it

From `MoonshotAI/kimi-code` README:

```sh
# macOS or Linux
curl -fsSL https://code.kimi.com/kimi-code/install.sh | bash
# Windows (PowerShell)
irm https://code.kimi.com/kimi-code/install.ps1 | iex
```

Single-binary distribution; npm package is `@moonshot-ai/kimi-code` (`npm install -g @moonshot-ai/kimi-code`). Legacy Python CLI: `uv tool install --python 3.13 kimi-cli` (PyPI `kimi-cli`). First-run setup is interactive via the `/login` TUI command.

## Config file location(s) and format

- Main config: **`~/.kimi-code/config.toml`** (TOML, snake_case keys), created on first run with mode 0600. Relocate everything with `KIMI_CODE_HOME=/path` (config becomes `$KIMI_CODE_HOME/config.toml`).
- TUI preferences: `~/.kimi-code/tui.toml` (not needed for providers/models).
- MCP servers: **`~/.kimi-code/mcp.json`** (JSON), plus optional project-level `.kimi-code/mcp.json` (cwd) and `.mcp.json` (git worktree root); same-name project entries override user entries. Source: `packages/agent-core-v2/src/app/mcpConfig/configLoader.ts` (`resolveMcpJsonPaths`: user = `$KIMI_CODE_HOME/mcp.json`, projectRoot = `<gitRoot>/.mcp.json`, project = `<cwd>/.kimi-code/mcp.json`).
- There is also an in-TUI `/mcp-config` command to manage mcp.json interactively.
- No project-level config.toml mechanism: exactly one user-level config file (`docs/en/configuration/overrides.md`: "The CLI currently reads a single user-level config file and has no project-level config file mechanism").
- Quoted TOML keys are required for names containing `.` (e.g. `[models."gpt-4.1"]`).

## Exact schema for custom providers/models

Source: `docs/en/configuration/config-files.md`, `docs/en/configuration/providers.md`, and `packages/node-sdk/src/config/schema.ts` (`ProviderConfigSchema`, `ModelAliasSchema`, `ProviderTypeSchema`).

Provider `type` values (zod enum, `ProviderTypeSchema`):
`kimi`, `anthropic`, `openai`, `openai_responses`, `google-genai`, `vertexai`.
(Legacy Python CLI used `openai_legacy` instead of `openai`, plus `gemini`; `google-genai` equals legacy `gemini`.)

```toml
[providers.openai]
type = "openai"                      # wire protocol: OpenAI Chat Completions
base_url = "https://api.openai.com/v1"
api_key = "sk-xxxxx"

[providers.anthropic]
type = "anthropic"                   # Anthropic Messages
api_key = "sk-ant-xxxxx"

[providers.openai-responses]
type = "openai_responses"            # OpenAI Responses API
base_url = "https://api.openai.com/v1"
api_key = "sk-xxxxx"
```

Provider fields (from `ProviderConfigSchema`; TOML keys are snake_case):

| TOML field | Type | Required | Notes |
| --- | --- | --- | --- |
| `type` | string | yes | one of the 6 enum values above |
| `api_key` | string | no* | literal plaintext; see credential resolution below |
| `base_url` | string | no* | literal URL |
| `env` | table<string,string> | no | fallback credential source, see below |
| `custom_headers` | table<string,string> | no | literal header name/value pairs |
| `oauth` | table | no | OAuth credential reference (`storage`, `key`); written by `/login`, not hand-edited |

*If both `api_key` and the `env` fallback are absent, startup fails with an error.

**Credential resolution** (`docs/en/configuration/overrides.md` → "Provider credentials"): for each provider, `api_key` field first, then the matching conventional key inside `[providers.<name>.env]` (e.g. `KIMI_API_KEY`, `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`), then fail. Same for `base_url` (`*_BASE_URL`). **The `env` sub-table is just TOML in the config file — it is NOT read from the process environment and does NOT write to the shell environment.** Official docs are explicit: "The CLI does not fall back to shell environment variables for credentials. Running `export KIMI_API_KEY=xxx` in the terminal does not give any provider its key."

Credential key names per provider type (`docs/en/configuration/env-vars.md`): `KIMI_API_KEY`/`KIMI_BASE_URL`, `ANTHROPIC_API_KEY`/`ANTHROPIC_BASE_URL`, `OPENAI_API_KEY`/`OPENAI_BASE_URL`, `GOOGLE_API_KEY`, `VERTEXAI_API_KEY`, `GOOGLE_CLOUD_PROJECT`, `GOOGLE_CLOUD_LOCATION` (the latter two required in `[providers.vertexai.env]`).

Model aliases (`ModelAliasSchema`):

```toml
[models."gpt-4.1"]
provider = "openai"                # must be a key in [providers]
model = "gpt-4.1"                  # model id sent on the wire
max_context_size = 1047576         # REQUIRED, int >= 1
capabilities = ["thinking"]        # optional: thinking, always_thinking, image_in, video_in, audio_in, tool_use
max_input_size = 128000            # optional per-request input cap
max_output_size = 32000            # optional output cap (max_tokens); docs say currently only anthropic reads it
display_name = "GPT-4.1"           # optional UI label
reasoning_key = "reasoning_content" # optional, openai provider only: non-standard reasoning field name
support_efforts = ["low", "high", "max"]  # optional thinking effort list
default_effort = "high"            # optional
off_effort = "none"                # optional wire value to disable thinking
# identity/routing fields exist on the model too but should not be emitted:
#   protocol = "anthropic" | "openai_responses"  (per-model protocol override)
#   base_url, beta_api (per-model endpoint override, base_url only effective with protocol)
```

Model identity fields `provider` + `model` are required; `max_context_size` is required (validation error message: `Model "<name>" must define a positive max_context_size in config.toml.`). Capabilities are largely auto-detected by model name prefix by the CLI; explicit `capabilities` only ever add, never remove.

Default model: top-level `default_model = "<alias>"`, where the alias is a key in `[models]` (validated: "Default model ... not found in models"). There is also `default_provider` in the zod schema, but the documented/default mechanism is `default_model`. Secondary/subagent model pool: `[secondary_model]` table (out of scope for a simple emitter).

## Exact schema for MCP servers (stdio/http), env var interpolation syntax

Source: `docs/en/customization/mcp.md` + `packages/agent-core-v2/src/mcpCore/config-schema.ts` (zod). File is JSON with a top-level `mcpServers` object. Transport is a discriminated union on `transport`; when `transport` is omitted, a `command` field implies `stdio` and a `url` field implies `http` (zod `preprocess` in `McpServerConfigSchema`).

stdio:

```json
{
  "mcpServers": {
    "filesystem": {
      "transport": "stdio",
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"],
      "env": {"SOME_VAR": "value"},
      "cwd": "/optional/workdir",
      "enabled": true,
      "startupTimeoutMs": 30000,
      "toolTimeoutMs": 60000,
      "enabledTools": ["tool1"],
      "disabledTools": ["tool2"]
    }
  }
}
```

HTTP / SSE:

```json
{
  "mcpServers": {
    "linear": {
      "transport": "http",
      "url": "https://mcp.linear.app/mcp",
      "headers": {"CONTEXT7_API_KEY": "your-key"},
      "bearerTokenEnvVar": "LINEAR_TOKEN",
      "auth": "oauth"
    },
    "legacy-events": {
      "transport": "sse",
      "url": "https://mcp.example.com/sse"
    }
  }
}
```

Field reference (`McpServerStdioConfigSchema` / `McpServerHttpConfigSchema` / `McpServerSseConfigSchema`):
- stdio: `transport: "stdio"`, `command` (string, required, min 1), `args?` (string[]), `env?` (record<string,string>), `cwd?` (string), `executor?` ("local"|"kaos", optional).
- http/sse: `transport`, `url` (required, must be URL), `headers?` (record<string,string>), `auth?` (only `"oauth"`), `bearerTokenEnvVar?` (string): name of a **process environment variable** that holds the bearer token; looked up at runtime via `process.env` (`client-remote.ts` `buildMcpHttpHeaders`); if unset/empty, startup fails with `MCP HTTP bearer token env var "X" is not set or empty`. When set, it overrides any `Authorization` header with `Bearer <token>`.
- common optional: `enabled?` (bool), `startupTimeoutMs?` / `toolTimeoutMs?` (int 1..2147483647), `enabledTools?` / `disabledTools?` (string[]).
- Global defaults in config.toml `[mcp]`: `startup_timeout_ms` (default 30000), `tool_timeout_ms` (default 60000); per-server field > env var (`KIMI_MCP_STARTUP_TIMEOUT_MS` / `KIMI_MCP_TOOL_TIMEOUT_MS`) > config.toml > built-in.

**Env var interpolation syntax**: there is **no `${VAR}` string interpolation anywhere** — not in config.toml values, not in mcp.json `env`, not in `headers`. Values are literal. The only runtime environment references are:
1. MCP http/sse `bearerTokenEnvVar` (name of process env var holding a token — the only native "secret by env var name" mechanism).
2. The `KIMI_MODEL_*` shell-env channel (`KIMI_MODEL_NAME`, `KIMI_MODEL_API_KEY`, `KIMI_MODEL_BASE_URL`, `KIMI_MODEL_PROVIDER_TYPE` in {`kimi`,`anthropic`,`openai`}, `KIMI_MODEL_MAX_CONTEXT_SIZE`, `KIMI_MODEL_CAPABILITIES`), which synthesizes a temporary in-memory provider/model at startup and overrides `default_model`; nothing is written to config files.

Legacy Python CLI (`MoonshotAI/kimi-cli`): `~/.kimi/config.toml` (same `providers`/`models` shape, provider type `openai_legacy`), api_key was a required literal string; `~/.kimi/mcp.json` same `mcpServers` format managed via `kimi mcp add --transport http|stdio ...`; legacy env overrides `KIMI_API_KEY`/`OPENAI_API_KEY`/`KIMI_BASE_URL` etc. were read from the process environment (unlike the new CLI).

## Default model configuration

- Top-level `default_model = "<model-alias>"` in `config.toml`, where the alias is a key of the `[models]` table. Validated at load: alias must exist and its `provider` must exist (`Config.validate_model` equivalent in zod load path).
- Per-session override: `kimi -m <alias>`; temporary env-channel override: `KIMI_MODEL_NAME=...` (+ `KIMI_MODEL_API_KEY`, `KIMI_MODEL_BASE_URL`).
- The IR shape `defaults.model = {provider, model}` maps to: find/create a `[models.<alias>]` entry with `provider = <provider-id>` and `model = <model-id>`, then set `default_model = "<alias>"`. Kimi's own convention uses aliases like `kimi-code/k3`, but any unique string works (quote it if it contains `.`).

## Version/evidence date

- Researched against `MoonshotAI/kimi-code` `main` @ 2026-09-07 (last push), release `@moonshot-ai/kimi-code@0.41.0` (2026-09-04). Docs pages fetched from repo `docs/en/` at the same commit.
- Legacy `MoonshotAI/kimi-cli`: main @ 2026-09-01, changelog 0.29.1 dated 2026-07-24.

## Constraints for agentcfg emitter

### Which IR protocols are representable; what must be rejected

| IR protocol | Kimi provider `type` | Status |
| --- | --- | --- |
| `openai-completions` | `openai` (new CLI) / `openai_legacy` (legacy CLI) | representable |
| `openai-responses` | `openai_responses` | representable |
| `anthropic-messages` | `anthropic` | representable |
| any other | — | **reject** (native enum is closed: kimi, anthropic, openai, openai_responses, google-genai, vertexai) |

### Fields with no native location and must be rejected or dropped (never silently)

Provider level:
- `api_key_env`: **no native representation.** Kimi requires the key as a literal string in config.toml (`api_key` or `[providers.<name>.env]` conventional key), and explicitly does NOT read shell env vars. Emitting the env var name as the key value would send the literal string as the API key. The only correct options are: (a) resolve the env var at compile time and inline the value into `api_key`; or (b) reject the provider. Note the `[providers.<name>.env]` sub-table is a config-file fallback, not an env lookup — writing `KIMI_API_KEY = "KIMI_API_KEY"` there is wrong.
- `headers[].from_env`: **reject** (no interpolation). `headers[].value` maps to `custom_headers` (literal).
- `headers[].bearer_from_env`: **reject** at provider level (no provider-level bearer-from-env; note `Authorization` can be set literally via `custom_headers`, and `KIMI_CODE_CUSTOM_HEADERS` is a process-env channel, not config-file).

Model level:
- `max_output_tokens` → `max_output_size` exists, but docs state only the `anthropic` provider currently reads it; for other providers it is accepted by the schema but ignored. Either emit with a warning or drop with a warning — never silently.
- per-model token cost/pricing fields (`input`, `output` prices) and `reasoning` / `tool_calling` booleans: **no native location** (capabilities auto-detected; no pricing fields). Kimi capabilities are `thinking|always_thinking|image_in|video_in|audio_in|tool_use`; the IR's `reasoning`/`tool_calling` flags can be approximated as `capabilities = ["thinking"]` / `"tool_use"` only as a best-effort mapping, but the CLI auto-detects by model name and docs say explicit capabilities only add. If exact round-trip fidelity is required, reject; otherwise map `reasoning=true` → include `thinking` and `tool_calling=true` → include `tool_use`, with a warning.
- `context_window` → `max_context_size` (required; int ≥ 1).

MCP level (in `mcp.json`):
- stdio: `command`, `argv` → `args`, `env` (literal values) map directly. If any stdio `env` value is a from-env reference, **reject** (no interpolation).
- http: `url` → `url`; `headers[].value` → `headers`; `headers[].bearer_from_env` → **`bearerTokenEnvVar` is a perfect native match** (env var name, token read from process env at runtime); `headers[].from_env` → **reject**.
- Global MCP timeouts, if the IR ever carries them, map to `[mcp] startup_timeout_ms` / `tool_timeout_ms` in config.toml.

### Suggested artifact name, format, suggested-path

Two artifacts (the CLI splits provider/model config from MCP config):

1. `config.toml` — TOML — suggested path `~/.kimi-code/config.toml` (respect `KIMI_CODE_HOME` if set; file mode 0600, dir 0700). Emit: `default_model`, `[providers.<id>]`, `[models.<alias>]`, optionally `[mcp] startup_timeout_ms/tool_timeout_ms`. Quote TOML keys containing `.`.
2. `mcp.json` — JSON — suggested path `~/.kimi-code/mcp.json` (user level; project-level `.kimi-code/mcp.json` / `.mcp.json` also supported if agentcfg targets project-scoped emission).

Legacy fallback if targeting the old Python CLI: `~/.kimi/config.toml` + `~/.kimi/mcp.json`, provider type `openai_legacy` instead of `openai`, api_key required literal.
