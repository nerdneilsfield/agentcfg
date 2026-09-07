# MiMo Code (mimocode) native configuration contract

## Official sources (with version/date evidence)

- GitHub repo: https://github.com/XiaomiMiMo/MiMo-Code (MIT license, ~12.9k stars).
  Repo created 2026-06-10; `main` last pushed 2026-09-07; latest release **v0.1.14**, published 2026-09-02 (GitHub API, checked 2026-09-07).
  The repo root `package.json` is still named `opencode` — MiMo Code is an official Xiaomi fork/rebrand of sst/opencode.
- npm package (official): `@mimo-ai/cli`, latest **0.1.14** published 2026-09-02, `repository` points to `github.com/XiaomiMiMo/MiMo-Code.git`, binary `mimo`.
  Source: https://www.npmjs.com/package/@mimo-ai/cli (registry metadata).
- Official docs: https://mimo.xiaomi.com/mimocode/ (Rspress site). Key pages:
  - Install: https://mimo.xiaomi.com/mimocode/install
  - Config files: https://mimo.xiaomi.com/mimocode/config-files
  - Config overrides (paths, merge): https://mimo.xiaomi.com/mimocode/config-overrides
  - Models: https://mimo.xiaomi.com/mimocode/models and https://mimo.xiaomi.com/mimocode/models-provider
  - MCP: https://mimo.xiaomi.com/mimocode/mcp-servers
  - Environment variables: https://mimo.xiaomi.com/mimocode/env-vars
- Live JSON Schema (fetched 2026-09-07, ~268 KB): https://mimo.xiaomi.com/mimocode/config.json (draft 2020-12, `allowComments: true`, `allowTrailingCommas: true`).
- Source-of-truth schema in repo: `packages/opencode/src/config/config.ts` + `packages/opencode/src/config/provider.ts` + `packages/opencode/src/config/mcp.ts` (Effect schemas); variable substitution in `packages/opencode/src/config/variable.ts`; config load pipeline in `packages/opencode/src/config/config.ts` (`loadConfig`); provider model merge logic in `packages/opencode/src/provider/provider.ts`.

⚠️ Name collision warning: the npm package **`mimocode`** (0.38.9, author "eurocybersecurite", repo github.com/eurocybersecurite/mimocode) is an **unrelated, unofficial** project. The official package is `@mimo-ai/cli`; the official binary and config names use "mimocode".

## Install command / how users get it

From the official README (https://github.com/XiaomiMiMo/MiMo-Code#quick-start) and docs:

```bash
# macOS / Linux
curl -fsSL https://mimo.xiaomi.com/install | bash
# Windows PowerShell
powershell -ep Bypass -c "irm https://mimo.xiaomi.com/install.ps1 | iex"
# or npm (all platforms)
npm install -g @mimo-ai/cli
# Run
mimo
```

## Config file location(s) and format

- Format: **JSON / JSONC** (comments + trailing commas allowed). File names:
  - Global dir (`$XDG_CONFIG_HOME/mimocode/`, i.e. `~/.config/mimocode/`): accepts `config.json`, `mimocode.json`, `mimocode.jsonc`, merged in that order (later wins).
  - Project dir: `mimocode.json` / `mimocode.jsonc`, searched from CWD up to the worktree root, deep-merged (parent first, current last).
  - `.mimocode/` and `MIMOCODE_CONFIG_DIR` likewise use `mimocode.json(c)`.
  - `MIMOCODE_HOME` relocates the whole profile root (`$MIMOCODE_HOME/{config,data,state,cache}`; must be absolute). `MIMOCODE_CONFIG` / `MIMOCODE_CONFIG_CONTENT` point at a config source. There is **no** `--config` flag.
- Add `"$schema": "https://mimo.xiaomi.com/mimocode/config.json"` for editor completion/validation.
- Runtime data: `~/.local/share/mimocode/` (SQLite DB, `auth.json`, `mcp-auth.json`), state in `~/.local/state/mimocode/`, cache in `~/.cache/mimocode/`. TUI options go in `~/.config/mimocode/tui.json`.
- Source: docs page "Config Overrides" and README "Configuration" section.

## Exact schema for custom providers/models

Exact JSON key names (verified against live schema and `packages/opencode/src/config/provider.ts`):

Top-level keys: `model`, `small_model`, `vision_model`, `provider`, `mcp`, `agent`, `default_agent`, `permission`, `server`, `plugin`, `skills`, `instructions`, `disabled_providers`, `enabled_providers`, `command`, `formatter`, `lsp`, `tool`, `tools`, `share`, `autoupdate`, `compaction`, `watcher`, `logLevel`, `experimental`, ...

- `provider` — object keyed by provider id (the key IS the provider id used in `model`).
  - `provider.<id>.npm` — string, AI SDK provider package name (wire-protocol selector; default `@ai-sdk/openai-compatible`).
  - `provider.<id>.name` — display name.
  - `provider.<id>.env` — array of env var names the CLI reads/prompts for.
  - `provider.<id>.id` — optional string id override.
  - `provider.<id>.options` — object; known keys `apiKey`, `baseURL`, `enterpriseUrl`, `setCacheKey`, `timeout` (ms | false), `headerTimeout`, `chunkTimeout`; extra keys pass through to the AI SDK.
  - `provider.<id>.models` — object keyed by model id.
- `provider.<id>.models.<model_id>`:
  - `id`, `name`, `family`, `release_date` — strings.
  - `reasoning`, `temperature`, `tool_call`, `attachment`, `experimental` — booleans.
  - `interleaved` — `true` or `{ "field": "reasoning" | "reasoning_content" | "reasoning_details" }`.
  - `limit` — `{ "context": N, "output": N, "input"?: N }` (token counts).
  - `modalities` — `{ "input": [...], "output": [...] }`, values from `text|audio|image|video|pdf`.
  - `cost` — `{ "input": N, "output": N, "cache_read"?, "cache_write"?, "context_over_200k"? }`.
  - `status` — `alpha|beta|deprecated`; `cachePromptTTL` — `5m|1h`.
  - `provider` — per-model override `{ "npm": ..., "api": "<base URL>" }` (note: `api` here is a URL, not a protocol).
  - `options` — free-form record passed to the AI SDK (e.g. `reasoningEffort`).
  - `headers` — per-model extra request headers (record string→string).
  - `variants` — record of variant configs.

Full documented custom-provider example (quoted from https://mimo.xiaomi.com/mimocode/models-provider, "MiMo Platform (recommended)"):

```jsonc
{
  "provider": {
    "mimo": {
      "npm": "@ai-sdk/openai-compatible",
      "name": "MiMo",
      "options": {
        "baseURL": "https://api.xiaomimimo.com/v1",
        "headers": { "api-key": "{env:MIMO_API_KEY}" }
      },
      "models": {
        "mimo-v2.5-pro": { "name": "MiMo V2.5 Pro" }
      }
    }
  },
  "model": "mimo/mimo-v2.5-pro"
}
```

(The official README says: "Use the exact keys `baseURL` and `apiKey`" for custom OpenAI-compatible endpoints; `apiKey` is an alternative to header-based auth: `"options": { "apiKey": "{env:MIMO_API_KEY}" }`.)

Key semantics (from `provider/provider.ts`):

- **Protocol/wire type is NOT an enum field.** No `api: openai-completions|...` switch exists. Wire protocol is selected by `npm` (the Vercel AI SDK provider package). Bundled values (`BUNDLED_PROVIDERS` map): `@ai-sdk/openai-compatible`, `@ai-sdk/openai`, `@ai-sdk/anthropic`, `@ai-sdk/google`, `@ai-sdk/google-vertex`, `@ai-sdk/azure`, `@ai-sdk/amazon-bedrock`, `@ai-sdk/mistral`, `@ai-sdk/xai`, `@ai-sdk/groq`, `@ai-sdk/cohere`, `@ai-sdk/cerebras`, `@ai-sdk/perplexity`, `@ai-sdk/togetherai`, `@ai-sdk/deepinfra`, `@openrouter/ai-sdk-provider`, etc. Any npm package implementing the AI SDK provider interface works; `file://` local paths also accepted.
- Provider-level `api` field = default **base URL** (`url: model.provider?.api ?? provider.api ?? ""`), not a protocol.
- Responses-API routing exists only as hard-coded per-provider model loaders for built-in providers (openai, azure, github-copilot, `xiaomi` models selected by `usesMimoResponsesApi`); NOT selectable from config for custom providers. Custom providers always call `sdk.languageModel(model.api.id)` (chat completions path).
- Provider options pass-through: `options.headers` (used by the docs example above) and any other keys are forwarded to the AI SDK factory (`factory({ name: model.providerID, ...options })`).

## Exact schema for MCP servers, env interpolation

`mcp` — object keyed by server name. Two shapes (`config/mcp.ts`: `McpLocalConfig` / `McpRemoteConfig`; docs https://mimo.xiaomi.com/mimocode/mcp-servers):

Local (stdio):

```jsonc
{
  "$schema": "https://mimo.xiaomi.com/mimocode/config.json",
  "mcp": {
    "my-local-mcp-server": {
      "type": "local",                            // REQUIRED, literal "local"
      "command": ["npx", "-y", "my-mcp-command"], // REQUIRED argv array
      "environment": { "MY_ENV_VAR": "my_env_var_value" },
      "enabled": true,
      "timeout": 5000,                            // ms, default 5000
      "sampling": "ask"                           // "deny" | "ask" | "allow"
    }
  }
}
```

Remote (HTTP):

```jsonc
{
  "mcp": {
    "my-remote-mcp": {
      "type": "remote",          // REQUIRED, literal "remote"
      "url": "https://my-mcp-server.com",
      "headers": { "Authorization": "Bearer MY_API_KEY" },
      "enabled": true,
      "timeout": 5000,
      "oauth": {                 // optional; false disables auto-OAuth
        "clientId": "{env:MY_MCP_CLIENT_ID}",
        "clientSecret": "{env:MY_MCP_CLIENT_SECRET}",
        "scope": "tools:read tools:execute",
        "redirectUri": "http://127.0.0.1:19876/mcp/oauth/callback"
      }
    }
  }
}
```

- Legacy `{ "<name>": { "enabled": false } }` is accepted to disable servers.
- Native type names are literally `"local"` and `"remote"`. (`"stdio"` / `"http"` / `"streamable-http"` are accepted ONLY as aliases in the Claude Code import compat layer, `ConfigMCP.fromClaude`; the native schema discriminant is exactly `local|remote`.)
- `environment` is `Record<string, string>`; `headers` is `Record<string, string>`.

**Env/file interpolation — verified against source** (`packages/opencode/src/config/variable.ts` + `config.ts` `loadConfig`):

- Substitution runs on the **entire raw config file text BEFORE JSONC parsing**, for every config source: `text.replace(/\{env:([^}]+)\}/g, (_, varName) => process.env[varName] || "")`, then `{file:...}` handling, then `parseJsonc`. Because it is text-level, it applies to **every string value in the file**, not just `apiKey`:
  1. `{env:VAR}` works inside `provider.<id>.options.headers` values — YES (docs example above uses `"headers": { "api-key": "{env:MIMO_API_KEY}" }`, and the text-level regex makes it location-independent). Also works mid-string, e.g. `"Bearer {env:TOKEN}"`.
  2. `{env:VAR}` works inside MCP `local` `environment` values and MCP `remote` `headers` values — YES, same text-level mechanism (schema is `Record<string,string>`, values are plain strings at rest; expansion happens pre-parse).
  3. Unset env var → replaced with **empty string** (no error).
  - `{file:path}` — replaced with file contents (trimmed, JSON-escaped); relative paths resolve against the config file directory; `~/` and absolute paths allowed. Missing file → config error (default `missing: "error"`).
  - `{file:...}` tokens inside `//` single-line comments are skipped; note the code does NOT skip `{env:...}` in comments (docs claim both are skipped — code only special-cases `{file:}`; harmless in practice).

## Default model configuration

Top-level key **`model`** (exact JSON key), value format `"provider_id/model_id"`:

```json
{
  "$schema": "https://mimo.xiaomi.com/mimocode/config.json",
  "model": "xiaomi/mimo-v2.5-pro"
}
```

- Related top-level keys: `small_model` (lightweight tasks; falls back to primary), `vision_model` (vision subagent; auto-chosen if unset).
- `provider_id` = the key in `provider` for custom providers, or the built-in models.dev provider id (`xiaomi` built-in: env `XIAOMI_API_KEY`, base `https://api.xiaomimimo.com/v1`, models `mimo-v2.5-pro`, `mimo-v2.5`, `mimo-v2-pro`, `mimo-v2-omni`, `mimo-v2-flash`, `mimo-v2.5-pro-ultraspeed` — from https://models.dev/api.json, provider entry `xiaomi`).
- Precedence: `--model` / `-m` CLI flag > config `model` > last-used model > internal priority.

## Version / evidence date

- Latest release: v0.1.14 (2026-09-02, GitHub release + npm `@mimo-ai/cli` 0.1.14 same day). Repo default branch `main`, last push 2026-09-07.
- Live schema https://mimo.xiaomi.com/mimocode/config.json fetched and inspected on **2026-09-07** (draft 2020-12; top-level props include `model`, `small_model`, `vision_model`, `provider`, `mcp`, `agent`, `permission`, `server`, `plugin`, `skills`, `instructions`, `disabled_providers`, `enabled_providers`, ...).
- Repo sources re-fetched and quoted on 2026-09-07: `config/variable.ts`, `config/config.ts` (`loadConfig`), `config/mcp.ts`, `config/provider.ts`, `provider/provider.ts`.

## Constraints for agentcfg emitter

### Which IR protocols are representable

- `openai-completions` → **representable.** Emit `npm: "@ai-sdk/openai-compatible"` (the documented default for custom providers) with `options.baseURL`, `options.apiKey: "{env:VAR}"` and/or `options.headers`.
- `anthropic-messages` → **representable with caveats.** Emit `npm: "@ai-sdk/anthropic"` with `options.baseURL` + `options.apiKey`. The anthropic SDK package is bundled. Caveat: anthropic-style auth headers (`x-api-key`, `anthropic-version`) are package defaults; arbitrary provider-level extra headers are only available via `options.headers` pass-through and per-model `models.<id>.headers` — verify at runtime; treat as best-effort.
- `openai-responses` → **NOT representable for custom providers; must be rejected.** Responses API usage is hard-coded in built-in provider model loaders (`sdk.responses(modelID)` for openai/azure/copilot and specific `xiaomi` models). No config field (no `shape`, no protocol enum) selects Responses for a custom provider; custom providers always use `sdk.languageModel(...)` (chat completions).

### Fields with no native location (reject or drop loudly — never silently)

- IR `protocol` itself: no native key; maps to `npm` package choice. Unknown/unsupported protocols must be rejected.
- IR provider headers:
  - `value` → plain string (`options.headers`, per-model `headers`, or MCP `headers`).
  - `from_env` → `"{env:NAME}"` — fully supported anywhere in config values.
  - `bearer_from_env` → `"Bearer {env:NAME}"` — supported (verified: text-level substitution works mid-string).
- Per-model IR modality lists: native enum is `text|audio|image|video|pdf`; anything else must be rejected.
- IR structured `reasoning` config → native is boolean `reasoning` (+ optional `interleaved` field name). Granular reasoning settings only fit free-form `models.<id>.options` pass-through (e.g. `reasoningEffort`) — map booleans only, or drop with warning.
- IR `tool_calling` → native boolean `tool_call`.
- IR `context_window` → `limit.context`; `max_output_tokens` → `limit.output`.
- MCP IR type names must be renamed: `stdio` → `"local"`, `http` → `"remote"` (the aliases only exist in the Claude import layer, not in native config).
- MCP env values are plain strings; `{env:...}` interpolation works (verified), so IR `from_env` in MCP env can be emitted as a literal `{env:...}` string.
- Provider fields with no IR source that the emitter simply omits: `whitelist`, `blacklist`, `env`, `enterpriseUrl`, `setCacheKey`, `timeout`/`headerTimeout`/`chunkTimeout`, `variants`, `cost`, `small_model`, `vision_model`, MCP `sampling`, MCP `oauth`.

### Suggested artifact

- **Artifact name:** `mimocode.jsonc` (JSONC; comments/trailing commas allowed by schema).
- **Format:** JSON (JSONC-safe).
- **Suggested path:** global `~/.config/mimocode/mimocode.jsonc` (i.e. `$XDG_CONFIG_HOME/mimocode/mimocode.jsonc`), or project-local `mimocode.jsonc` when compiling per-repo. Always include `"$schema": "https://mimo.xiaomi.com/mimocode/config.json"`.
- Default model goes to top-level `"model": "<provider_id>/<model_id>"`.
