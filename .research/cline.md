# Research Report: Cline CLI (`cline`) — coding-agent CLI target

- **Research date:** 2026-09-07
- **Identity status:** CONFIRMED — official CLI exists and is installed-adjacent
  on this machine (`~/.cline/data` present, `clineVersion: 4.1.16` in
  `globalState.json`).
- **Confidence:** HIGH for identity, file paths, and the provider/MCP schema
  (read directly from source at a pinned commit). MEDIUM for a few
  behavioral edges noted inline (env fallback for custom providers, and
  `models.json` writer semantics) because I could not execute the binary.

## 1. Identity

- **Official package:** `cline` on npm — latest `3.0.61` (published
  2026-09-02; nightly `3.0.61-nightly.1788783728` on 2026-09-07).
  `npm view cline`: bin `{cline: bin/cline}`, repository
  `git+https://github.com/cline/cline.git` (directory `apps/cli`), homepage
  `https://cline.bot`, license Apache-2.0. Install: `npm install -g cline`
  (platform binaries via optional deps `@cline/cli-<os>-<arch>`; no runtime
  needed).
- **Also on Homebrew:** `brew info cline` → stable 3.0.3, bottled, **deprecated**
  ("uses non-FOSS @anthropic-ai/claude-agent-sdk and pre-built binaries",
  disable date 2027-05-18). Not installed.
- **Repo:** https://github.com/cline/cline — local clone
  `/tmp/cline-research/cline`, HEAD `c21b17255b228e88a1518c18a73a473ee5876362`
  (2026-09-07T10:36:42+09:00). Source-of-truth package dir `apps/cli`
  (`@cline/cli 3.0.61`, bin `cline`).
- **NOT the target (decoys checked):**
  - `cline-cli` npm package → **404, does not exist** (parent's hint
    `npm view cline-cli version` → 0.0.13 appears to be stale/fabricated;
    current registry returns E404).
  - `@cline/cli` on the public registry → `0.0.13`, an *experimental* "lightweight
    Cline CLI built with the Cline SDKs" whose bin is `clite`, repository
    `github.com/cline/sdk` (directory `apps/cli`). Same name as the source
    package of the real CLI but a different, older published artifact —
    **do not target it**. The real CLI is the bare `cline` package.
- **Local machine:** `~/.cline` exists (see §2). `which cline` → not on PATH
  in this shell (installed state is from the VS Code extension migration +
  possibly a since-removed global install; `globalState.json` records
  `clineVersion: 4.1.16` and a `__vscodeLegacyMcpSettingsMigration` from
  Cursor's globalStorage `saoudrizwan.claude-dev`).
- Docs: https://docs.cline.bot (JS-rendered; the CLI README in-repo is the
  best doc). `websearch` not needed — npm registry + official repo + local
  files were sufficient primary sources.

## 2. Config files the CLI reads

All under the Cline home dir. Defaults from
`sdk/packages/shared/src/storage/paths.ts` (`resolveClineDir`,
`resolveClineDataDir`):

- Base dir: `~/.cline` (env override `CLINE_DIR`; CLI flag `--config <dir>`
  calls `setClineDir`).
- Data dir: `~/.cline/data` (env override `CLINE_DATA_DIR`; CLI flag
  `--data-dir <path>`).

| File | Path (default) | Env override | Purpose |
|---|---|---|---|
| Provider profiles | `~/.cline/data/settings/providers.json` | `CLINE_PROVIDER_SETTINGS_PATH` | custom + built-in provider settings (see §3). Written `0600`. |
| Custom model registry | `~/.cline/data/settings/models.json` | (same dir as providers.json) | per-provider model metadata for custom providers (see §3.4) |
| MCP servers | `~/.cline/data/settings/cline_mcp_settings.json` | `CLINE_MCP_SETTINGS_PATH` | `mcpServers` map (see §4) |
| Global settings | `~/.cline/data/settings/global-settings.json` | `CLINE_GLOBAL_SETTINGS_PATH` | telemetry/auto-update/compaction/tool toggles (out of scope) |
| State | `~/.cline/data/globalState.json` | — | clineVersion, migration markers, telemetry |
| Legacy secrets | `~/.cline/data/secrets.json` | — | **legacy VS Code extension secret store, read only by the one-time migration** into providers.json (`provider-settings-legacy-migration.ts`). NOT read by the current provider path. Local file contains `openAiApiKey`. |
| Sessions DB | `~/.cline/data/db/sessions.db` | — | SQLite sessions (out of scope) |

Verified locally — `~/.cline/data/settings/` contains exactly
`providers.json`, `cline_mcp_settings.json`, `global-settings.json`, and the
machine's real `providers.json` matches the schema below verbatim:

```json
{
  "version": 1,
  "lastUsedProvider": "openai-compatible",
  "providers": {
    "openai-compatible": {
      "settings": {
        "provider": "openai-compatible",
        "apiKey": "sk-...",
        "model": "gpt-4o"
      },
      "updatedAt": "2026-08-07T07:24:16.828Z",
      "tokenSource": "migration"
    }
  }
}
```

(The local file also validates against `StoredProviderSettingsSchema` from
source — the `zod` schema in
`sdk/packages/core/src/types/provider-settings.ts`.)

**Project-scope MCP:** no `mcp.json`/`.cline/mcp.json` project discovery for
the CLI (grep over `sdk/packages/core/src` + `apps/cli/src` finds project MCP
only inside *agent plugins* — `<pluginRoot>/mcp.json` with keys `$schema`,
`mcpServers`). User-scope `cline_mcp_settings.json` is the only CLI MCP file.

## 3. Custom providers — YES

### 3.1 Where they live

`providers.json` → `providers: Record<providerId, {settings, updatedAt, tokenSource}>`.
Any entry whose `settings.provider` is **not** a built-in id
(`sdk/packages/llms/src/providers/ids.ts` `BUILT_IN_PROVIDER` enum:
`anthropic, cline, cline-pass, openai-compatible, openai-native, openai-codex,
bedrock, vertex, gemini, ollama, lmstudio, deepseek, xai, together, fireworks,
groq, cerebras, sambanova, nebius, baseten, requesty, litellm, huggingface,
vercel-ai-gateway, v0, aihubmix, hicap, nousResearch, huawei-cloud-maas, wandb,
xiaomi, tencent-tokenhub, kilo, zai, zai-coding-plan, qwen, qwen-code, doubao,
mistral, moonshot, asksage, minimax, dify, oca, sapaicore, openrouter,
elevenlabs, claude-code, opencode, openai-codex-cli, poolside` + generated
ids) and has a non-empty `baseUrl` is auto-registered at startup as a custom
provider (`local-provider-registry.ts` `registerProviderSettingsProvider`,
invoked from `ProviderSettingsManager` constructor + every read/write). So
**writing a `providers.json` entry is sufficient — no other registration
step**.

### 3.2 `settings` schema (verbatim zod, `provider-settings.ts` `ProviderSettingsSchema`)

```jsonc
{
  "provider": "my-provider",            // required; /^[a-z0-9][a-z0-9-]*$/i
  "apiKey": "sk-...",                   // optional, LITERAL string
  "auth": {                             // optional OAuth block
    "apiKey": "...", "accessToken": "...", "refreshToken": "...",
    "expiresAt": 1735689600, "accountId": "...", "organizationId": "...",
    "organizationName": "...", "memberId": "...",
    "metadata": {"k": "v"}
  },
  "model": "model-id",                  // optional default model id (bare id, NO provider prefix)
  "protocol": "anthropic | gemini | openai-chat | openai-responses | openai-r1 | ai-sdk",
  "client":   "anthropic | ai-sdk | ai-sdk-community | openai | openai-compatible | openai-r1 | gemini | bedrock | custom | fetch | vertex",
  "routingProviderId": "openai-native", // optional; openai-responses routing
  "maxTokens": 16384,                   // per-provider output cap
  "contextWindow": 128000,
  "baseUrl": "https://api.example.com/v1",   // custom endpoint — THIS is the custom-provider enabler
  "headers": {"X-Team": "platform"},    // optional Record<string,string>, literal values
  "timeout": 300,                       // seconds (int, positive)
  "reasoning": {"enabled": true, "effort": "none|minimal|low|medium|high|xhigh|max", "budgetTokens": 8192},
  "aws": {...}, "gcp": {...}, "azure": {"apiVersion","useIdentity"},
  "sap": {...}, "oca": {"mode":"internal|external","usePromptCache"},
  "region": "us-east-1",
  "apiLine": "china | international",   // regional endpoint switching (qwen/moonshot/z.ai/minimax)
  "capabilities": ["reasoning","prompt-cache","streaming","tools","vision","computer-use","oauth","popular"],
  "modelCatalog": {"loadLatestOnInit": true, "loadPrivateOnAuth": true, "url": "...", "cacheTtlMs": 600000, "failOnError": true}
}
```

Envelope per provider id:
`{ "settings": <above>, "updatedAt": "<ISO8601>", "tokenSource": "manual" | "oauth" | "migration" }`.
Top-level: `{ "version": 1, "lastUsedProvider"?: string, "modes": {"voiceInput"?: {...}}, "providers": {...} }`.
`modes` defaults to `{}` when absent (zod `.default({})`), so a hand-written
file may omit it. `lastUsedProvider` selects the default profile; the CLI
falls back to provider `cline` (its own OAuth) when unset.

### 3.3 Protocol / client mapping (custom provider wire protocols)

- Defaults for a custom provider with no `protocol`/`client`:
  `protocol: "openai-chat"`, `client: "openai-compatible"`
  (`resolveProviderProtocol` / `resolveProviderClient` in
  `local-provider-registry.ts`). So a bare `{provider, baseUrl, model}`
  entry is an **OpenAI chat-completions compatible endpoint** — this is the
  direct answer to "custom OpenAI-compatible endpoints supported?": **yes**.
- `protocol: "openai-responses"` (+ `client: "openai"`) routes through the
  OpenAI Responses client; `protocol: "anthropic"` uses the Anthropic
  messages client; `gemini` → Google client. `openai-r1` and `ai-sdk` also
  exist. The IR's three protocols map: `openai-completions` →
  `openai-chat` (there is no literal `openai-completions` value),
  `openai-responses` → `openai-responses`, `anthropic-messages` →
  `anthropic` (IR name has `-messages` suffix, Cline value does not).

### 3.4 Model metadata — `models.json`

Providers.json `settings.model` is just the default model **id**. Rich
per-model metadata (context window, modalities, reasoning, prices) for custom
providers lives in a sibling file `~/.cline/data/settings/models.json`
(`resolveModelsRegistryPath` = same dir as providers.json):

```jsonc
{
  "version": 1,
  "providers": {
    "my-provider": {
      "provider": {                     // optional; if absent, only models merge
        "name": "My Provider",
        "baseUrl": "https://api.example.com/v1",
        "defaultModelId": "my-model",
        "protocol": "openai-chat",      // optional
        "client": "openai-compatible",  // optional
        "capabilities": ["tools"],      // optional
        "modelsSourceUrl": "..."        // optional
      },
      "models": {
        "<model-key>": {                // key = model id (or "id" field inside)
          "id": "my-model", "name": "My Model",
          "maxTokens": 16384, "contextWindow": 128000, "maxInputTokens": 128000,
          "capabilities": ["images","video","tools","streaming","prompt-cache","reasoning","reasoning-effort","computer-use"],
          "supportsVision": true, "supportsAttachments": true, "supportsReasoning": true,
          "operation": "language", "operationModes": ["chat"],
          "modalities": {"input": ["text","image"], "output": ["text"]},
          "inputPrice": 0, "outputPrice": 0, "cacheReadsPrice": 0, "cacheWritesPrice": 0,
          "temperature": 0.7, "apiFormat": "default | openai-responses | r1"
        }
      }
    }
  }
}
```

`StoredModelEntrySchema` is `.passthrough()`; capabilities enum from
`sdk/packages/shared/src/llms/model-info.ts` `ModelCapabilitySchema`
(`images, video, tools, streaming, prompt-cache, reasoning, reasoning-effort,
computer-use, ...`). NOTE the writer seeds `"tools"` into any non-empty
custom capability list, so declaring `capabilities: ["reasoning"]` still
keeps tool calling on. There is **no per-model reasoning/effort object** like
the IR's `reasoning` block — reasoning is provider-level (`settings.reasoning`)
plus catalog metadata on models.

### 3.5 API key / env-var handling (IMPORTANT)

- **Config is literal-only.** `apiKey` is `z.string()`. There is **no
  `${VAR}`, `{{VAR}}`, or `{env:VAR}` interpolation anywhere in
  providers.json, models.json, or cline_mcp_settings.json** (grepped the SDK
  for expand/interpolate helpers — none).
- **Runtime env fallback for built-in providers only.** At request time
  `resolveApiKey` (`sdk/packages/llms/src/providers/http.ts`) resolves:
  `settings.apiKey` → `settings.apiKeyResolver?.()` → `apiKeyEnv` list from
  the built-in provider spec (`builtins.ts`: `ANTHROPIC_API_KEY`,
  `OPENAI_API_KEY`, `OPENROUTER_API_KEY`, `GEMINI_API_KEY` /
  `GOOGLE_GENERATIVE_AI_API_KEY`, `DEEPSEEK_API_KEY`, `GROQ_API_KEY`, ...).
  This fallback is wired via the *generated provider catalog*
  (`providers.generated.ts`), i.e. only for built-in provider ids. For a
  custom provider id the spec list is empty, so a stored literal `apiKey` is
  effectively required (MEDIUM confidence — could not run the binary to
  end-to-end test a custom provider with no key; source path is clear).
- CLI `-k/--key` overrides env; README states "`--key` takes precedence over
  environment variables". CLI flags: `-P/--provider <id>`, `-m/--model <id>`,
  `-k/--key`, `--baseurl` (via `cline auth --provider X --apikey ... --baseurl ...`).
- `headers` values are literal strings (zod `Record<string,string>`) — no
  env indirection, no bearer-from-env helper. To send a bearer token you
  must inline `Authorization: Bearer <token>` (or rely on the runtime adding
  `Authorization: Bearer <apiKey>` automatically — which it does for
  openai-compatible/anthropic clients from the resolved api key).

## 4. MCP — YES (`cline_mcp_settings.json`)

File: `~/.cline/data/settings/cline_mcp_settings.json`
(`resolveMcpSettingsPath`; env `CLINE_MCP_SETTINGS_PATH`). Schema verbatim
from `sdk/packages/core/src/extensions/mcp/config-loader.ts` (zod) and
`types.ts`:

```jsonc
{
  "mcpServers": {
    "<name>": {
      // ---- canonical nested form (discriminated union on transport.type)
      "transport": {
        "type": "stdio",                  // "stdio" | "sse" | "streamableHttp"
        "command": "node",                // stdio: required
        "args": ["server.js"],            // stdio: optional
        "cwd": "/path",                   // stdio: optional
        "env": {"KEY": "value"},          // stdio: optional Record<string,string>
        // sse / streamableHttp instead:
        // "url": "https://example.com/mcp",
        // "headers": {"Authorization": "Bearer x"}   // optional Record<string,string>
      },
      "disabled": false,                  // optional (note: `disabled`, not `enabled`)
      "timeout": 120,                     // optional, SECONDS (int); default 60, clamp [1, 3600]
      "metadata": {"k": "v"},             // optional
      "oauthClient": {"clientId": "...", "clientSecret": "..."},  // optional
      "oauth": { /* clientInformation, tokens, codeVerifier, discoveryState,
                    redirectUrl, lastError, lastAuthenticatedAt,
                    authorizationRequired */ }                   // optional, runtime state
    }
  }
}
```

- **Legacy flat form also accepted** (`.union` of three shapes): a server can
  be `{type?, transportType?, command, args?, cwd?, env?, disabled?, timeout?...}`
  (stdio) or `{type?, transportType?, url, headers?...}` (URL; defaults to
  `sse` when no type given; legacy `transportType: "http"` maps to
  `streamableHttp`). agentcfg should emit the **nested `transport` form**.
- **Transports:** `stdio` (spawn; `env` is merged *over* the CLI's full
  `process.env` in `client.ts#spawnProcess` — values are literal strings, no
  interpolation), `sse`, `streamableHttp`. HTTP headers are literal
  `Record<string,string>`; OAuth is first-party (interactive
  `authorizeMcpServerOAuth`; `auth: "oauth"`-style state lives in the `oauth`
  block). There is **no dedicated bearer-token-from-env field** — a bearer
  header must be a literal `Authorization: Bearer ...` string.
- **No `${VAR}` interpolation** in command/args/env/cwd/url/headers (grep of
  config-loader + client + manager: no expansion step).
- Server names: any string key (no regex constraint in schema).
- Timeout unit is **seconds** (`sdk/packages/shared/src/mcp.ts`:
  `DEFAULT_MCP_TIMEOUT_SECONDS = 60`, `MIN 1`, `MAX 3600`). The IR's
  `timeout_ms` must be divided by 1000 (and clamped).
- Top-level object is `.passthrough()` — unknown keys tolerated.
- Management UX: `cline mcp install <name> -- <cmd> [args...]` (stdio),
  `cline mcp install <name> --transport http|sse <url>` — but the wizard
  still asks for auth details and needs a TTY (`--yes` installs
  non-interactively). `cline config mcp` and `cline mcp` open interactive
  managers. For agentcfg, **direct file writes are the reliable path** (the
  file is read fresh per session; a JSON merge is safe given atomic
  write-to-temp+rename on their side and plain JSON on ours).

## 5. Defaults (model selection)

- Default provider = `providers.json:lastUsedProvider`, else `cline`
  (`main.ts`: `args.provider || lastUsedProviderSettings?.provider || "cline"`).
- Default model = CLI `-m` flag > `settings.model` of the selected provider >
  first known catalog model > hardcoded `anthropic/claude-sonnet-4.6`.
- **Model id format:** the `-m` flag and README use `provider/model`
  (`-m anthropic/claude-opus-4-6`), but `settings.model` stores the **bare
  model id** (`"gpt-4o"` in the local file). On startup the CLI persists
  `model: config.modelId` back into the selected provider's settings — and
  `config.modelId` is the full `-m` value (e.g. `anthropic/claude-opus-4-6`)
  when the flag is used. So the provider-prefixed form can land in
  `settings.model` as an opaque string; the runtime matches it against known
  model ids of the selected provider (custom providers registered from
  providers.json/models.json make both `bare-id` and `provider/bare-id`
  resolvable). For agentcfg, writing the bare model id into `settings.model`
  is the clean mapping for `defaults.model = "provider/model"`: set
  `lastUsedProvider` to the provider id and `settings.model` to the bare
  model id.

## 6. IR mapping (agentcfg → Cline)

Clean fits:
- `providers[].id` → `providers.<id>` key (custom id, any
  `/^[a-z0-9][a-z0-9-]*$/i`); built-in ids collide with the built-in catalog
  (allowed — settings override/extend the built-in entry).
- `base_url` → `settings.baseUrl`. `protocol` → `settings.protocol`
  (rename values: `openai-completions`→`openai-chat`, `anthropic-messages`→
  `anthropic`; `openai-responses`→`openai-responses`).
- `models[].{id, context_window, max_output_tokens, input/output modalities,
  reasoning, tool_calling}` → `models.json providers.<id>.models.<model-id>`
  (`contextWindow`, `maxTokens`, `modalities.input/output`,
  `capabilities: ["reasoning","tools",...]`, `supportsVision` etc.).
- `defaults.model "provider/model"` → `lastUsedProvider` + `settings.model`
  (bare model id).
- `mcp[]` → `cline_mcp_settings.json mcpServers.<id>` nested `transport`
  form. `enabled:false` → `disabled:true`. `timeout_ms` → `timeout`
  (seconds, clamp 1..3600). `cwd`, `env`, `url`, `headers` map 1:1.

Gaps / things the IR cannot represent in Cline natively:
- **`api_key_env` has no native target.** Config is literal-only; the env
  fallback only exists for built-in provider ids and only when `apiKey` is
  absent. agentcfg options: (a) document that Cline needs the literal key
  (deviation from the env-references-only policy), or (b) leave `apiKey`
  unset for built-in providers and rely on the documented env names
  (`ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `OPENROUTER_API_KEY`, ...) — works
  for built-ins only, not custom providers, or (c) compile-time render the
  secret into the file (violates policy). Recommend (b) for built-ins and
  flag custom-provider keys as an explicit unsupported deviation.
- `headers[].value/from_env/bearer_from_env` → only literal
  `headers: {name: value}` is supported. `from_env`/`bearer_from_env` cannot
  be expressed (same env-interpolation gap).
- Provider-level `headers` exist, but there is no per-request auth helper or
  key-command equivalent.
- MCP `env` values: literal only (no `${VAR}`). HTTP MCP bearer-from-env must
  be a literal header — same policy deviation as provider keys, or instruct
  users to run `cline mcp` OAuth for first-party auth.
- `mcp[].transport` has no `http` literal — use `streamableHttp` (IR `http`
  → Cline `streamableHttp`; IR `sse` → Cline `sse` exists).
- Reasoning shape differs: IR per-model `reasoning` → Cline provider-level
  `settings.reasoning.{enabled,effort,budgetTokens}` + model catalog
  `capabilities:["reasoning"]`. No per-model effort/budget object.
- `models[].name` maps to `models.json models.<key>.name`; `name` at provider
  level maps to `models.json providers.<id>.provider.name` (providers.json
  `settings` has no display-name field).
- File permissions: Cline writes providers.json `0600` — good hygiene to
  match when agentcfg writes it.

## 7. Quick verification checklist for the compiler

1. Write `~/.cline/data/settings/providers.json` with
   `version:1, lastUsedProvider:<id>, providers.<id>.settings={provider:<id>,
   protocol:"openai-chat"|"anthropic"|"openai-responses", baseUrl, apiKey?,
   model:<bare-id>, reasoning?, headers?, capabilities?}` (+ `updatedAt`,
   `tokenSource:"manual"`).
2. Optionally write `models.json` model metadata next to it.
3. Write `cline_mcp_settings.json` `mcpServers` (nested `transport` form).
4. Respect `0600` on providers.json; keep `secrets.json` untouched (legacy
   migration input only).

## 8. Confidence

- **HIGH** — identity, npm/brew/registry facts, all config file paths, both
  zod schemas (providers + MCP) read verbatim at pinned commit
  `c21b172`, local `~/.cline` files validating against those schemas, CLI
  flags and precedence from `main.ts`, env-fallback mechanism from
  `http.ts#resolveApiKey`, MCP env/headers literal-only from
  `client.ts#spawnProcess` + config-loader (no expansion code exists).
- **MEDIUM** — (a) whether a *custom* provider with no stored apiKey can pick
  up an env var (source says no: `apiKeyEnv` comes from the generated
  built-in catalog only; not executed end-to-end); (b) exact behavior when
  `settings.model` contains a `provider/model` string with a slash for a
  custom provider (matched against registered model ids; our recommendation
  to write bare ids sidesteps it); (c) whether an installed `cline` binary
  exists elsewhere on PATH (not found in this shell; `~/.cline` state proves
  prior use, likely via the VS Code extension migration).
