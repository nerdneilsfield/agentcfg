# Grok Build (`grok`) native configuration contract

- Official CLI exists. Product name is **Grok Build**. Binary is `grok`. xAI documents it as the coding agent TUI / headless / ACP tool. "grok build" in user speech is this product, not a subcommand.

## Official source URLs (repo, docs) with version/commit/date evidence

- Docs site: https://docs.x.ai/build/overview (Grok Build getting started).
- Settings: https://docs.x.ai/build/settings
- TOML + env reference: https://docs.x.ai/build/settings/reference
- MCP: https://docs.x.ai/build/features/mcp-servers
- CLI reference: https://docs.x.ai/build/cli/reference
- Enterprise / config layers: https://docs.x.ai/build/enterprise
- Machine-readable docs dump used for this note: https://docs.x.ai/llms.txt (fetched 2026-09-07).
- Official source repo: https://github.com/xai-org/grok-build (Apache-2.0, Rust). GitHub API: created 2026-07-14, last pushed 2026-09-01, ~26.5k stars. Tarball `main` extracted 2026-09-07 as GitHub archive prefix `xai-org-grok-build-72a6125`; `SOURCE_REV` file in that tree is `a549186d9d39311f2d3ee4208db62af8c65aa476`.
- Config types in source:
  - MCP: `crates/codegen/xai-grok-config-types/src/mcp.rs` (`McpServerTransportConfig`, `KNOWN_MCP_SERVER_FIELDS`).
  - Env expansion: `crates/codegen/xai-grok-config/src/loader.rs` (`expand_env_vars_in_string` via `shellexpand::env_with_context_no_errors`).
- npm wrapper: https://www.npmjs.com/package/@xai-official/grok (`bin.grok = bin/grok`). npm metadata: created 2025-10-22, modified 2026-09-07; dist-tags `latest = 1.0.13`, `alpha = 1.0.22`. Package has no `homepage` / `repository` fields; docs still name it as the npm install path.

No other xAI coding CLI is official. Community names (`grok-cli`, `grok-code`, vibe-kit grok) are not this product.

## Install command / how users get it

From https://docs.x.ai/build/overview :

```bash
curl -fsSL https://x.ai/cli/install.sh | bash
```

Windows:

```powershell
irm https://x.ai/cli/install.ps1 | iex
```

npm alternative (enterprise docs):

```bash
npm install -g @xai-official/grok
```

First run: `grok` (browser OAuth) or `export XAI_API_KEY=xai-...` then `grok`. Headless: `grok -p "..."`. ACP: `grok agent stdio`. Inspect merge: `grok inspect`. MCP CLI: `grok mcp add|list|remove|doctor`.

## Config file location(s) and format

Format: **TOML**.

| Scope | Path | What it may hold |
|---|---|---|
| User | `~/.grok/config.toml`, or `$GROK_HOME/config.toml` | Full config (models, MCP, UI, tools, …). Windows: `%USERPROFILE%\.grok\config.toml`. |
| Project | `.grok/config.toml` from CWD up to git root | **Only** `[mcp_servers]`, `[plugins]`, `[permission]`. Same-name project MCP server replaces the user one entirely. |
| Managed | `~/.grok/managed_config.toml`, `/etc/grok/managed_config.toml` | Enterprise-served defaults. |
| Requirements (pin, fail-closed) | `~/.grok/requirements.toml`, `/etc/grok/requirements.toml` | Policy pins; cannot be overridden by user config / env / remote settings. |

Enterprise merge order (lowest → highest): `/etc/grok/managed_config.toml` → `~/.grok/managed_config.toml` → `~/.grok/config.toml` → `~/.grok/requirements.toml` → `/etc/grok/requirements.toml`. Settings docs also list env `GROK_*` as a session/CI overlay.

Suggested agentcfg artifact: user-scope TOML fragment for `~/.grok/config.toml` (or `$GROK_HOME/config.toml`). Do not write managed/requirements files.

Also loaded below `config.toml` (compat, can be disabled): `~/.claude.json`, `.cursor/mcp.json`, project `.mcp.json`. Irrelevant to the emitter.

## Exact schema for custom providers/models

Grok has **no `[providers]` table**. Custom / BYOK models are **per-model tables** `[model."<id>"]` plus a `[models]` catalog of defaults.

Official example from https://docs.x.ai/build/settings (also the overview custom-models snippet):

```toml
[models]
default = "grok-build"                       # recommended for coding / agent sessions
web_search = "grok-4.6"                      # model used by client-side web_search tool

[model."grok-4.6"]
model = "grok-4.6"                           # id sent to the API
base_url = "https://api.x.ai/v1"             # provider endpoint
name = "Grok 4.6"                            # shown in model picker
description = "Grok 4.6 from xAI"
env_key = "XAI_API_KEY"                      # env var holding the API key
api_backend = "responses"                    # chat_completions | responses | messages
temperature = 0.7
top_p = 0.95
max_completion_tokens = 8192
context_window = 1000000
extra_headers = { "x-api-key" = "xai-..." }
supports_backend_search = true               # if the endpoint supports Grok-hosted server-side search tools
```

Minimal custom-model example from https://docs.x.ai/build/overview :

```toml
[model.my-model]
model = "model-id"
base_url = "https://api.example.com/v1"
name = "Display Name"
env_key = "API_KEY"

[models]
default = "my-model"
```

`[model.<id>]` fields (settings reference):

| Field | Meaning |
|---|---|
| `model` | Wire model id sent to the API (may differ from the table key). |
| `base_url` | Provider endpoint. |
| `name` / `description` | Picker label. |
| `api_key` | Inline key. Docs say prefer `env_key`. |
| `env_key` | Env var name holding the API key. |
| `api_backend` | **`chat_completions` \| `responses` \| `messages`**. This is the protocol/wire-type field. |
| `temperature` / `top_p` / `max_completion_tokens` | Sampling. `max_completion_tokens` is max output tokens. |
| `context_window` | Context size (drives auto-compact). |
| `extra_headers` | Per-request header map (string → string). Also allowed globally on `[models]`. |
| `supports_backend_search` | Grok-hosted server-side search tools. |
| `supports_reasoning_effort` / `reasoning_effort` | Reasoning controls. |
| `stream_tool_calls` | Tool-call streaming shape. |
| `max_retries` / `inference_idle_timeout_secs` | Reliability. |

Protocol mapping from agentcfg IR:

| IR `protocol` | Native `api_backend` |
|---|---|
| `openai-completions` | `chat_completions` |
| `openai-responses` | `responses` |
| `anthropic-messages` | `messages` |

All three IR protocols are representable. There is no fourth native value.

Because native config is model-centric, one IR **provider** with N models becomes N `[model."<id>"]` tables. Suggested table key: the IR model id (quote if it contains `.`). Set `model = "<wire id>"` to the same id unless a future IR field splits catalog id vs wire id. Repeat `base_url`, `env_key`, `api_backend`, and mapped headers on every model of that provider.

`[models]` fields relevant to agentcfg: `default`, `web_search`, `default_reasoning_effort`, `session_summary`, `image_description`, `extra_headers`, global sampling, `allowed_models` / `hidden_models` / `disabled_models`.

Env overlay for models: `GROK_DEFAULT_MODEL`, `GROK_WEB_SEARCH_MODEL`, `GROK_MODELS_BASE_URL`, `GROK_MODELS_LIST_URL`, `GROK_XAI_API_BASE_URL` (default `https://api.x.ai/v1`). Auth env: `XAI_API_KEY`.

## Exact schema for MCP servers

Table `[mcp_servers.<name>]`. Transport is **untagged**: presence of `command` → stdio; presence of `url` → HTTP. Source: `McpServerTransportConfig` in `crates/codegen/xai-grok-config-types/src/mcp.rs`. Known fields (same file `KNOWN_MCP_SERVER_FIELDS`): `args`, `bearer_token_env_var`, `command`, `cwd`, `enabled`, `env`, `expose_image_base64`, `headers`, `oauth`, `oauth_client_id`, `oauth_client_secret_env_var`, `oauth_scopes`, `setup`, `startup_timeout_sec`, `tool_timeout_sec`, `tool_timeouts`, `type`, `url`, plus aliases `urlTemplate` / `url_template`.

Official docs examples (https://docs.x.ai/build/features/mcp-servers and settings page):

```toml
[mcp_servers.filesystem]
command = "npx"
args = ["-y", "@modelcontextprotocol/server-filesystem", "/path/to/dir"]
env = { API_KEY = "${MY_API_KEY}" }   # ${VAR} expands at load time
startup_timeout_sec = 30              # default 30
tool_timeout_sec = 6000               # default 6000

[mcp_servers.linear]
url = "https://mcp.linear.app/mcp"
headers = { "Authorization" = "Bearer ${LINEAR_API_KEY}", "x-mcp-session-id" = "{{session_id}}" }
```

stdio fields: `command` (string), `args` (string array), `env` (map), `cwd` (path). HTTP fields: `url`, `headers` (map), `bearer_token_env_var` (env var name → inject `Authorization: Bearer`), plus OAuth (`oauth_client_id`, `oauth_client_secret_env_var`, `oauth_scopes`). Common: `enabled` (default true), `startup_timeout_sec` (default 30), `tool_timeout_sec` (default 6000), `tool_timeouts` (map name → seconds).

CLI equivalent:

```bash
grok mcp add filesystem -- npx -y @modelcontextprotocol/server-filesystem /path/to/dir
grok mcp add --transport http linear https://mcp.linear.app/mcp
grok mcp add --transport http api https://mcp.example.com/mcp --header "Authorization: Bearer ${API_TOKEN}"
```

`--scope project` writes `.grok/config.toml`.

### Env var interpolation syntax

Docs (MCP page + settings reference): string fields `url`, `command`, `args`, `env`, and `headers` support **`${VAR}`** and **`${VAR:-default}`**. Headers may also use **`{{session_id}}`** (and source also replaces `${session_id}`).

Source implementation (`expand_env_vars_in_string` in `xai-grok-config/src/loader.rs`) is `shellexpand::env_with_context_no_errors`. That expands `$VAR` and `${VAR}`; `${VAR:-default}` is documented for MCP and enterprise layers (`$VAR` expansion is also claimed for managed/requirements). Unset vars become empty (no error). OAuth tokens live in `~/.grok/mcp_credentials.json`, not in config.

Emitter should write `${ENV_NAME}` in `env` values / `headers` / `url` when IR uses `from_env`. Prefer native `bearer_token_env_var = "GITHUB_TOKEN"` for IR `Authorization.bearer_from_env` instead of baking `Bearer ${GITHUB_TOKEN}` into `headers`. Do not render secret literals.

Global MCP timeouts: `GROK_MCP_STARTUP_TIMEOUT_SECS` (seconds); Claude-compat `MCP_TIMEOUT` (milliseconds, checked first).

## Default model configuration

`[models] default = "<model table id>"` (docs example `"grok-build"`). Session override: `-m` / `--model`, or env `GROK_DEFAULT_MODEL`. Related: `[models] web_search`, `GROK_WEB_SEARCH_MODEL`. IR `defaults.model` as `provider/model` does **not** match native ids; native default is the `[model.<id>]` table key (usually the model id, not `provider/model`). Map IR `defaults.model` to that table key (the model id portion) or reject provider-qualified strings that cannot be resolved to a unique `[model.*]` key.

## Version / evidence date

- Docs: https://docs.x.ai/llms.txt fetched **2026-09-07**.
- npm `@xai-official/grok`: latest **1.0.13**, alpha **1.0.22**, registry modified **2026-09-07**.
- GitHub `xai-org/grok-build` last push **2026-09-01**; extracted `SOURCE_REV` **a549186d9d39311f2d3ee4208db62af8c65aa476** (tarball 2026-09-07). GitHub has no Releases API tag as of fetch (`/releases/latest` 404).

## Constraints for agentcfg emitter

### Which IR protocols are representable; what must be rejected

- Representable: `openai-completions` → `api_backend = "chat_completions"`; `openai-responses` → `"responses"`; `anthropic-messages` → `"messages"`.
- Reject unknown protocol strings. Do not invent aliases (`openai`, `anthropic`, `chat`, `completions`).

### Fields with no native location (reject or drop, never silently)

Must have an explicit native mapping or fail:

- IR **provider `id` as a first-class provider object**: no `[providers.<id>]`. Either flatten to `[model.<model_id>]` (and fail on **duplicate model ids across providers**) or reject. Do not drop the provider identity silently if two providers share a model id.
- IR provider `headers` with `from_env` / `bearer_from_env`: native `extra_headers` is a string map, not env-name map. Options: expand as `${VAR}` / `Bearer ${VAR}` in the string value (runtime expansion is documented for MCP/enterprise; **not documented for `extra_headers`** — treat undocumented expansion as unsafe and **reject env-derived provider headers** unless verified), or reject. Static `value` headers can go in `extra_headers`.
- IR model `input` / `output` / `reasoning` / `tool_calling` capability flags: no 1:1 native fields (`supports_reasoning_effort` is not a capability flag). **Reject** if the emitter policy is "no silent drop"; otherwise these have nowhere to go.
- IR model pricing / cost: no native cost fields on `[model.*]`.
- Inline secrets: IR has no secret literals; native `api_key` exists but agentcfg must not emit it. Use `env_key` only.
- MCP stdio `command` as argv: native splits `command` (executable string) + `args` (array). Map IR `command: [npx, -y, pkg]` → `command = "npx"`, `args = ["-y", "pkg"]`. Empty command list: reject.
- MCP HTTP `headers` env refs: native is string map with `${VAR}` or `bearer_token_env_var`. Static `value` → `headers`. `Authorization.bearer_from_env` → `bearer_token_env_var`. Other `from_env` headers → `headers = { "X-Foo" = "${FOO_ENV}" }` (MCP expansion is documented).
- MCP SSE-only transport distinct from HTTP: native HTTP is one `url` streamable-http variant (aliases `urlTemplate`). No separate `sse` key. If IR grows an SSE transport, reject unless it is just a URL.
- Project vs user file: full provider/model tables **cannot** live in project `.grok/config.toml`. Emitter targeting project scope must reject providers/models and emit only MCP (and not plugins/permissions unless in IR).

Do not emit UI, sandbox, permission, plugin, skills, or enterprise pin files from provider/MCP IR.

### Suggested artifact name, format, suggested-path

- Target id: `grok` (product: Grok Build).
- Format: TOML.
- Artifact name: `config.toml` (fragment / generated file).
- Suggested path: `~/.grok/config.toml` (override with `$GROK_HOME/config.toml`). Optional project MCP-only file: `.grok/config.toml`.
- v1 should be a fragment, not a destructive overwrite of an existing user file.

### Suggested mapping sketch (for later emitter, not implemented here)

```toml
[models]
default = "glm-5.3"

[model."glm-5.3"]
model = "glm-5.3"
base_url = "https://example.com/v1"
name = "glm-5.3"
env_key = "VOLC_API_KEY"
api_backend = "responses"

[mcp_servers.context7]
command = "npx"
args = ["-y", "@upstash/context7-mcp"]
env = { CONTEXT7_API_KEY = "${CONTEXT7_API_KEY}" }

[mcp_servers.github]
url = "https://api.githubcopilot.com/mcp/"
bearer_token_env_var = "GITHUB_TOKEN"
```
