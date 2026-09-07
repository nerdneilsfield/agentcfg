# ZCode native configuration contract

ZCode is Z.ai (Zhipu) GLM's official coding-agent product: a closed-source Electron desktop ADE plus a bundled headless CLI (`zcode.cjs`). There is **no public official GitHub source** for the agent runtime, and **no official npm/PyPI CLI package**. The npm name `zcode@0.0.1` is an unrelated 2020 placeholder. PyPI `zcode` is an unrelated `.zee` compressor.

Closest official artifacts: the desktop app download, the bundled CLI inside the app, official product docs at `zcode.z.ai`, and the plugins marketplace repo.

## Official source URLs (repo, docs) with version/commit/date evidence

- Product / download: https://zcode.z.ai (redirects to `/cn`; title "ZCode | GLM-5.3 官方 Harness"). macOS ARM download observed 2026-09-07: `https://cdn-zcode.z.ai/zcode/electron/releases/3.11.2/macos-arm64/ZCode-3.11.2-mac-arm64.dmg`
- Official docs (Chinese, English sibling under `/en/docs/...`):
  - Welcome: https://zcode.z.ai/cn/docs/welcome
  - Install: https://zcode.z.ai/cn/docs/install
  - Connect models / providers: https://zcode.z.ai/cn/docs/configuration
  - MCP: https://zcode.z.ai/cn/docs/mcp-services
- Official GitHub (plugins marketplace, not the runtime): https://github.com/zai-org/zcode-plugins (README: "official plugins marketplace for ZCode")
- Official GitHub (feedback): https://github.com/zai-org/feedback
- Official API docs (models, not the ADE config): https://docs.z.ai
- Installed runtime evidence (this machine, 2026-09-07):
  - App: `/Applications/ZCode.app` `CFBundleShortVersionString` **3.11.2**, `CFBundleVersion` **3.11.2.6792**, bundle id `dev.zcode.app`
  - CLI entry: `/Applications/ZCode.app/Contents/Resources/glm/zcode.cjs`
  - Bundle meta `.node-bundle-meta.json`: `runtime: electron-node`, `source: apps/zcode-cli/packages/cli/dist/zcode.cjs`
- Third-party (not official, but documents the same bundled CLI contract):
  - Unofficial npm wrapper: `zcode-app-cli` (https://github.com/kingsword09/zcode-cli) wrapping the extracted official `zcode.cjs`
  - Ouroboros runtime guide pinned to CLI 0.15.0/0.15.2: https://github.com/q00/ouroboros/blob/03714ba446186423dcb46e25d12bd19c3a2e82f6/docs/runtime-guides/zcode.md

Z.ai does **not** currently publish a stable public CLI source tree or JSON-output contract. The native config schema below is taken from official docs plus the Zod schema embedded in `zcode.cjs` 3.11.2.

## Install command / how users get it

Official install is the **desktop app**, not npm:

- Website: https://zcode.z.ai → "立即下载 ZCode"
- macOS Apple Silicon (observed): `https://cdn-zcode.z.ai/zcode/electron/releases/3.11.2/macos-arm64/ZCode-3.11.2-mac-arm64.dmg`
- Windows installers exist; Linux builds have been described as beta in third-party writeups.

Headless CLI is the app-bundled Node script, launched with the Electron binary:

```text
ELECTRON_RUN_AS_NODE=1 /Applications/ZCode.app/Contents/MacOS/ZCode \
  /Applications/ZCode.app/Contents/Resources/glm/zcode.cjs \
  --json --prompt <PROMPT> --mode <edit|yolo>
```

If a `zcode` executable is on `PATH`, it is typically a wrapper around that bundled runtime (unofficial `npm i -g zcode-app-cli` is one such wrapper; **not** vendor-published).

## Config file location(s) and format (JSON)

Format: **JSON**. Not TOML/YAML.

| Scope | Path | What it holds |
| --- | --- | --- |
| User CLI (native) | `~/.zcode/cli/config.json` (Windows: `%USERPROFILE%\.zcode\cli\config.json`) | Full CLI schema: `provider`, `model`, `mcp.servers`, plugins, hooks, … |
| User desktop providers | `~/.zcode/v2/config.json` | Same `provider` registry shape used by the GUI |
| User desktop settings | `~/.zcode/v2/setting.json` | UI/locale/window; **not** the model-id default string |
| Project (workspace) | `<repo>/.zcode/config.json` | Same schema subset; MCP listed in official MCP docs |
| Project (alternate) | `<repo>/zcode.json` | Discovered by CLI (`configFileKind` enum: `zcode.json`, `.zcode/config.json`, `explicit`) |
| `.agents` fallback | `~/.agents/mcp.json` and `<repo>/.agents/mcp.json` | Industry `{ "mcpServers": { ... } }` **only if** the corresponding `.zcode` file has **no** MCP servers |

Official MCP docs (https://zcode.z.ai/cn/docs/mcp-services): user native MCP lives at `~/.zcode/cli/config.json` key `mcp.servers`; workspace native MCP at `<project>/.zcode/config.json` key `mcp.servers`. Workspace is loaded first, then user. Same-scope `.zcode` wins over `.agents` with **no merge**. Panel writes always go to `.zcode`, never `.agents`.

CLI hard error without an effective `model.main` (from `zcode.cjs`):

```text
Error: Model config is missing. Create ~/.zcode/cli/config.json
with an explicit model provider before running ZCode.
```

`model.main` must be the string `"provider-id/model-id"`. An object-form `model.main` is dropped by the parser (Ouroboros measurement; still present as a `g.union([xot, bAo])` in 3.11.2, with `bAo` requiring `main` and/or `lite` strings).

## Exact schema for custom providers/models

From `zcode.cjs` (app 3.11.2) Zod:

```text
kind enum uAo = ["anthropic", "openai", "openai-compatible"]
apiFormat (desktop catalog) WCt = ["anthropic-messages", "openai-chat-completions", "openai-responses"]
model-supported-format GCt = ["anthropic", "openai", "responses", "gemini"]
modalities = ["text", "audio", "image", "video", "pdf"]
```

CLI user-config provider object (`xAo`):

```json
{
  "provider": {
    "<provider-id>": {
      "kind": "anthropic",
      "name": "optional display name",
      "options": {
        "apiKey": "<literal key>",
        "baseURL": "https://api.example.com/anthropic",
        "apiKeyRequired": true,
        "includeUsage": true,
        "timeout": 180000,
        "chunkTimeout": 60000,
        "headers": { "X-Custom": "value" }
      },
      "headers": { "X-Also": "allowed at provider root" },
      "models": {
        "<model-id>": {
          "name": "display",
          "limit": { "context": 1000000, "input": 1000000, "output": 128000 },
          "contextWindow": 1000000,
          "maxOutputTokens": 128000,
          "modalities": { "input": ["text"], "output": ["text"] },
          "reasoning": true,
          "tool_call": true,
          "structured_output": true,
          "cost": { "input": 0, "output": 0, "cache_read": 0, "cache_write": 0 }
        }
      }
    }
  }
}
```

`kind` is required. `npm` is `z.never()` (rejected). `options` is passthrough at the Zod layer, but official docs state `~/.zcode/v2/config.json` `options` only honors **`apiKey`, `baseURL`, `apiKeyRequired`, `headers`** as connection fields; extra keys such as `reasoning_effort` are **not** written into the request body.

Protocol mapping (official docs + `kind` enum):

| IR protocol | Native `kind` | Native wire notes |
| --- | --- | --- |
| `anthropic-messages` | `anthropic` | Anthropic Messages; `baseURL` is API **root**, not `/messages` |
| `openai-completions` | `openai-compatible` | Chat Completions; `baseURL` normally ends in `/v1`, not `/chat/completions` |
| `openai-responses` | `openai` (official OpenAI Responses) or `openai-compatible` with catalog `apiFormat` `openai-responses` | CLI user schema exposes **`kind` only**, not `apiFormat`. Desktop catalog has `openai-responses`. |

Official docs examples of custom providers (https://zcode.z.ai/cn/docs/configuration):

- Anthropic: Anthropic URL `https://api.anthropic.com`
- OpenAI: API base `https://api.openai.com`
- OpenRouter: `https://openrouter.ai/api`
- Moonshot: Anthropic URL `https://api.moonshot.cn/anthropic`
- MiniMax: Anthropic URL `https://api.minimaxi.com/anthropic`
- Xiaomi MiMo: OpenAI-compatible `https://api.xiaomimimo.com/v1`
- DeepSeek: Anthropic `https://api.deepseek.com/anthropic` and OpenAI `https://api.deepseek.com/v1`

Builtin Z.ai / BigModel Coding Plan (measured + Ouroboros quote):

```json
{
  "model": { "main": "builtin:zai-coding-plan/GLM-5.3" },
  "provider": {
    "builtin:zai-coding-plan": {
      "name": "Z.ai - Coding Plan",
      "kind": "anthropic",
      "options": {
        "baseURL": "https://api.z.ai/api/anthropic",
        "apiKey": "<your key>",
        "apiKeyRequired": true
      },
      "enabled": true
    }
  }
}
```

BigModel anthropic root: `https://open.bigmodel.cn/api/anthropic`. OpenAI coding endpoint (docs): `https://open.bigmodel.cn/api/coding/paas/v4` / `https://api.z.ai/api/coding/paas/v4`.

Model reference: `provider.<id>.models.<model-id>` → `"<id>/<model-id>"`. IDs are case-sensitive.

Model catalog extras that **do** have native fields: `limit.context` / `limit.output` (and aliases `contextWindow` / `maxOutputTokens`), `modalities.input|output`, `reasoning` (boolean or `{enabled, levels, defaultLevel, providerOptionsByLevel}`), `tool_call` / `supportsToolCall`. Pricing `cost.*` exists. There is **no** native field for IR `input`/`output` prices as a required pair beyond optional `cost`.

Headers are **literal strings** (`record<string,string>`). No `from_env` / `bearer_from_env` object form.

API keys are **inline strings** in `options.apiKey`. No `${ENV}` interpolation in the provider schema. Unofficial `zcode-app-cli` docs: environment-only keys do **not** satisfy the login gate.

## Exact schema for MCP servers (stdio/http), env var interpolation syntax

Official docs example (`mcp.servers` in `config.json`):

```json
{
  "mcp": {
    "servers": {
      "memory": {
        "command": "npx",
        "args": ["-y", "@modelcontextprotocol/server-memory"],
        "env": {}
      }
    }
  }
}
```

`zcode.cjs` 3.11.2 Zod (preprocess infers `type` from `command` vs `url`; aliases `http_headers` → `headers`):

Shared: `enabled?: boolean`, `timeoutMs?: number` (positive finite).

- **stdio** (`type: "stdio"`): `command` (required string), `args?: string[]`, `cwd?: string`, `env?: record<string,string>`
- **http** (`type: "http"`): `url` (required), `headers?: record<string,string>`, `oauth?`
- **sse** (`type: "sse"`): same as http

OAuth (http/sse only):

```json
{
  "oauth": {
    "type": "client_credentials",
    "clientId": "...",
    "clientSecret": "...",
    "clientName": "optional",
    "scope": "optional"
  }
}
```

or `authorization_code` with optional `clientId` / `clientSecret` / `clientName` / `redirectPath` / `scope`.

Runtime also accepts transport `"stdio"|"http"|"sse"`. Missing `type` is inferred: non-empty `command` → stdio, else non-empty `url` → http.

Disable: schema field is **`enabled`**. Official MCP docs say writing `"enable": false` — that doc field name does **not** match the 3.11.2 schema (`enabled`). Emitter must use `enabled`.

Interpolation: plugin MCP resolution replaces `${NAME}` via `/\$\{([^}]+)\}/g` with allowlist `^[A-Za-z_][A-Za-z0-9_]*$`. Built-in substitutions for **plugins**: `CLAUDE_PROJECT_DIR`, `ZCODE_PLUGIN_DATA`, `ZCODE_PLUGIN_ROOT`, `ZCODE_PROJECT_DIR`, `CLAUDE_PLUGIN_DATA`, `CLAUDE_PLUGIN_ROOT`. User `config.json` MCP `env` / `headers` / `url` / `command` / `args` are typed as **plain strings**; there is no documented `${ENV}` expansion for user-level `mcp.servers`. Do not emit env-object `from_env` forms; native env is `{"VAR": "literal-or-expanded-string"}`.

`.agents/mcp.json` uses the common `{ "mcpServers": { "<name>": { ... } } }` envelope. Native emitter target is `mcp.servers`, not `mcpServers`.

## Default model configuration

CLI (`~/.zcode/cli/config.json`):

```json
{
  "model": {
    "main": "zai/glm-5.3",
    "lite": "zai/glm-5-turbo"
  }
}
```

- `main`: conversation / agent default. Required for headless CLI.
- `lite`: lightweight + subagent work.
- Schema: `model` is either the string `"provider/model"` **or** `{ main?: string, lite?: string }` with at least one of `main`/`lite`. Refine message: `"Model references must use provider/model format"`.
- `small_model` is `z.never()` (rejected).

Desktop GUI selection is **not** the same file: Ouroboros notes desktop `~/.zcode/v2/config.json` shares the `provider` registry but does not write `~/.zcode/cli/config.json` `model.main`. agentcfg should emit CLI `model.main` (and `model.lite` if IR has a secondary default; otherwise omit or duplicate `main`).

## Version/evidence date

- Evidence date: **2026-09-07**
- ZCode.app **3.11.2** (build 3.11.2.6792), bundled `zcode.cjs` from `apps/zcode-cli/packages/cli/dist/zcode.cjs`
- Official docs pages `configuration` and `mcp-services` fetched the same day
- Unofficial `zcode-app-cli` npm **3.11.2-21** (2026-09-06) tracks the same app version line

## Constraints for agentcfg emitter

### Which IR protocols are representable; what must be rejected

| IR protocol | Representable? | Native mapping |
| --- | --- | --- |
| `anthropic-messages` | yes | `provider.<id>.kind = "anthropic"` |
| `openai-completions` | yes | `kind = "openai-compatible"` (Chat Completions `/v1`) |
| `openai-responses` | **partial** | CLI user schema has no `apiFormat` field. Use `kind = "openai"` for official OpenAI; for a third-party Responses endpoint there is **no first-class CLI `kind`**. Reject a custom Responses URL unless/until desktop `apiFormat: "openai-responses"` is confirmed writable into CLI `config.json`. Do not silently emit `openai-compatible` (that is Chat Completions). |

Unknown protocols: reject.

### Fields with no native location (reject or drop explicitly; never silently)

Must **reject** (cannot be expressed without lying):

- Provider `api_key_env` — native wants literal `options.apiKey`. No env-name field.
- Header `from_env` / `bearer_from_env` — native headers are `record<string,string>` literals only.
- MCP env interpolation objects other than `env: { "NAME": "value" }`.
- MCP transports other than stdio / http / sse (IR `http` → native `http`; do not invent SSE unless IR says SSE).
- `openai-responses` on a non-OpenAI custom `base_url` (see above).
- IR fields that would require `provider.*.npm` (`z.never()`).
- CLI `small_model` (`z.never()`).

Must **not silently drop** if present in IR (surface as explicit drop/reject in the emitter report):

- Per-model `input` / `output` prices: optional native `models.*.cost.{input,output,cache_read,cache_write}` exists; if IR prices are required policy, map to `cost` or reject if the emitter policy is "exact native field or reject". Prefer mapping to `cost` when both exist.
- `tool_calling` boolean: map to `tool_call` / `supportsToolCall`; if mapping is not implemented, reject rather than omit capability.
- `reasoning` structured IR: native has boolean or `{enabled, levels, defaultLevel}` — map or reject; do not omit when IR set reasoning required.
- `context_window` / `max_output_tokens`: map to `limit.context` / `limit.output` (also `contextWindow` / `maxOutputTokens` aliases).
- Provider `enabled` defaults to on when omitted.

Safe optional maps:

- IR `headers[].value` → `options.headers` or provider `headers` as literal strings.
- MCP stdio `command` + `argv` → `command` + `args`; MCP http `url` + headers → `type: "http"`, `url`, `headers`.
- IR `defaults.model.provider` + `defaults.model.model` → `model.main = "<provider>/<model>"`. Also set `model.lite` to the same string unless IR defines a lite model.

### Suggested artifact name, format, suggested-path

- Artifact name: `zcode`
- Format: JSON
- Primary suggested path: `~/.zcode/cli/config.json` (user CLI; providers + default model + MCP)
- Workspace overlay (MCP / project overrides only): `<repo>/.zcode/config.json`
- Do **not** emit `~/.zcode/v2/config.json` as the CLI artifact (desktop-only provider store; OAuth/encrypted keys). Do **not** emit `.agents/mcp.json` as the native target (fallback only).
