# Research Report: Gajae Code (`gjc`) — coding-agent CLI target

- **Research date:** 2026-09-07
- **Identity status:** CONFIRMED — official CLI exists.
- **Confidence:** HIGH (see confidence notes at the end).

## 1. Identity

"Gajae Code" is a real, actively developed AI coding-agent CLI. `gajae` is
Korean for "crayfish"; the TUI binary also ships a Korean-locale alias
`가재씨` ("crayfish-sir").

- **CLI name:** `gjc` (bin: `gjc`, plus `가재씨` → `bin/gajaessi.js`)
- **npm wrapper package:** `gajae-code` — latest `0.16.4`, published 2026-09-05
  (`npm view gajae-code`); it is a one-line install wrapper depending on
  `@gajae-code/coding-agent@0.16.4`. Requires Bun as runtime (not Node).
- **Repo:** https://github.com/Yeachan-Heo/gajae-code (MIT). Local clone:
  `/tmp/gajae-code`, HEAD `f238c66de513d9a8b1b8d2544d413bd04e1c43db`
  (2026-09-07T23:08:24+09:00). Workspace/coding-agent version at HEAD: `0.16.6`.
- **Site/docs:** https://gajae-code.com (fetched, HTTP 200); extensive `docs/`
  directory in the repo.
- **Not installed on this machine:** `which gjc` → not found; no `~/.gjc` dir.
- `websearch` skill was unavailable (no Serper key); discovery was done via the
  npm registry, the live website, and the official GitHub repo (primary sources).

Positioning: an "external coding-agent harness for reviewable work" — BYO
coding plan (OAuth subscription providers like Claude Max / Codex / Copilot) or
API-key endpoints, with SDK session CLI, Coordinator MCP, and ACP interfaces.

## 2. Config files the CLI reads

Config root defaults to `~/.gjc` (env override `GJC_CONFIG_DIR`, legacy alias
`PI_CONFIG_DIR`). Agent dir defaults to `<configRoot>/agent` (env override
`GJC_CODING_AGENT_DIR`). XDG redirection exists on Linux behind a migration.
Source: `packages/utils/src/dirs.ts`, `packages/coding-agent/src/config/settings.ts:563,1559`.

| File | Path | Purpose |
|---|---|---|
| Settings | `~/.gjc/agent/config.yml` | main user settings (huge schema; see `schemas/config.schema.json`) |
| Custom providers/models | `~/.gjc/agent/models.yml` | custom provider + model registry (legacy `models.json` auto-migrated to `.yml`) |
| MCP (user scope) | `~/.gjc/agent/mcp.json` | `mcpServers` map (`.mcp.json` name variant also read) |
| MCP (project scope) | `<project>/.gjc/mcp.json` | `mcpServers` map (`.gjc/.mcp.json` variant also read); enabled by default in real sessions, disabled only if `mcp.enableProjectConfig: false` is explicitly set in `config.yml` (the JSON-schema default `false` is a legacy placeholder — `sdk/session.ts:3138-3143` shows sessions default project config ON). Project root standalone `mcp.json`/`.mcp.json` exist as a discovery-provider fallback but conventional sessions use `nativeOnly` discovery (`.gjc` scopes only). |

Precedence notes: project settings layer can override user `config.yml` keys;
`auth.credentialPins` is global-only. Strict schemas — unknown keys fail
validation (`ModelsConfigSchema` is `.strict()`).

## 3. Custom providers — YES (models.yml)

`models.yml` shape (verbatim from `packages/coding-agent/src/config/models-config-schema.ts`,
zod, and `schemas/models.schema.json`):

```yaml
providers:
  <provider-id>:
    baseUrl: https://api.example.com/v1        # optional (required if models listed)
    apiKey: MY_PROVIDER_API_KEY                # optional; env-name-or-literal semantics
    apiKeyEnv: MY_PROVIDER_API_KEY             # optional; env-var name, rotating-aware
    api: openai-completions                    # optional; default per provider
    headers: {X-Team: platform}                # optional Record<string,string>
    authHeader: true                           # optional; inject "Authorization: Bearer <key>"
    auth: apiKey                               # optional enum: apiKey | none | oauth
    disableStrictTools: false
    cacheRetention: short                      # none | short | long
    discovery: {type: ollama}                  # optional; see enum below
    modelOverrides:
      some-model-id: {name: Renamed model}
    models:                                    # ARRAY of objects (see note below)
      - id: some-model-id                      # required
        name: Some Model
        api: openai-completions
        reasoning: false
        input: [text]                          # [text|image]
        output: [text]
        cost: {input: 0, output: 0, cacheRead: 0, cacheWrite: 0}
        contextWindow: 128000
        maxTokens: 16384
        headers: {X-Model: value}
        thinking:                              # optional
          minLevel: low
          maxLevel: xhigh
          mode: effort                         # effort|budget|google-level|anthropic-adaptive|anthropic-budget-effort
          defaultLevel: high
          levels: [low, medium, high, xhigh]
        compat: {...}                          # large flag set (see schema)
        wireModelId: ...
        requestTransform: {profile: openai-proxy, stripHeaders: [...], setHeaders: {...}, extraBody: {...}}
        cacheRetention: none
modelBindings:
  modelRoles:
    default: my-provider/some-model-id:high    # "provider/modelId[:effort]"
  agentModelOverrides:
    executor: my-provider/some-model-id
equivalence: {overrides: {...}, exclude: [...]}
profiles:
  <profile-id>:
    required_providers: [...]
    display_name: ...
    model_mapping: {default: provider/modelId, executor: ..., architect: ..., planner: ..., critic: ...}
```

**NOTE — doc/schema mismatch:** `docs/custom-providers-and-multi-account.md`
shows `models:` as a YAML *map* (`my-model: {name: ...}`), but the actual zod
schema (`ModelDefinitionSchema`) and `schemas/models.schema.json` define
`models` as an **array of objects each with a required `id`**, and the loader
(`model-registry.ts #parseModels`) iterates the array. `docs/models.md` uses the
array form. Treat the array form as authoritative (it matches code); the map
form likely fails strict validation.

### Protocols (`api` enum, provider- and model-level)

`openai-completions`, `openai-responses`, `openai-codex-responses`,
`azure-openai-responses`, `anthropic-messages`, `bedrock-converse-stream`,
`google-generative-ai`, `google-vertex`, `google-gemini-cli`, `ollama-chat`,
`cursor-agent`.

### API key / env-var handling (IMPORTANT, unusual semantics)

There is **no `${VAR}` interpolation in models.yml**. Resolution semantics
(`model-registry.ts:1009-1018`, `resolve-config-value.ts`):

- `apiKey: <string>` — **env-name-or-literal**: if `Bun.env[<string>]` exists,
  its value is used; otherwise the string itself is treated as the literal key.
  Precedence: `apiKey` wins over `apiKeyEnv`.
- `apiKeyEnv: <NAME>` — reads env var `NAME` via `$rotatingCredentialEnv`:
  process env → trusted agent `~/.gjc/agent/.env` (authoritative, re-read on
  rotation; project `.env` files are NOT consulted for credentials) → legacy
  `~/.pi/agent/.env` → `~/.gjc/.env` → `~/.pi/.env`.
- Any header value (provider/model `headers`) is resolved with the same
  env-name-or-literal semantics, plus a `!command` prefix: a value starting
  with `!` executes the rest as a shell command and uses stdout (cached) —
  the Claude Code `apiKeyHelper` convention.
- `authHeader: true` + resolved key → injects `Authorization: Bearer <key>`.
- `auth: none` → keyless provider (local runtimes). `auth: oauth` marks the
  provider OAuth (credential store/broker).
- Precedence overall: runtime `--api-key` > models.yml `apiKey`/`apiKeyEnv` >
  stored OAuth (local SQLite `agent.db` or auth-broker).

## 4. MCP — YES (mcp.json)

Schema verbatim (`packages/coding-agent/src/config/mcp-schema.json`,
`packages/coding-agent/src/runtime-mcp/types.ts`):

```json
{
  "$schema": "...",
  "mcpServers": {
    "<name>": {
      "enabled": true,
      "autoload": true,
      "sharing": "per-session",
      "timeout": 30000,
      "protocol": "auto",
      "type": "stdio",
      "command": "node",
      "args": ["server.js"],
      "env": {"KEY": "value"},
      "noInheritEnv": false,
      "cwd": "/path"
    },
    "<remote>": {
      "type": "http",
      "url": "https://example.com/mcp",
      "headers": {"Authorization": "Bearer ${MCP_TOKEN}"},
      "auth": {"type": "oauth", "credentialId": "...", "tokenUrl": "...", "clientId": "...", "clientSecret": "..."},
      "oauth": {"clientId": "...", "clientSecret": "...", "redirectUri": "...", "callbackPort": 8080, "callbackPath": "..."}
    }
  },
  "disabledServers": ["name-to-deny"]
}
```

- Transports: `stdio` (command/args/env/noInheritEnv/cwd), `http`, `sse`
  (legacy). `type` omitted defaults to stdio. `protocol`: `auto` (default,
  negotiates MCP 2026-07-28 stateless, legacy fallback) | `2026-07-28` |
  `legacy`; ignored for stdio.
- `enabled: false` or presence in `disabledServers` keeps the server out.
  `autoload: false` = on-demand only (via `/mcp`); conventional sessions load
  autoload servers at startup. `--no-mcp` opts a session out. `--mcp-config
  <path>` loads an exact file (tools-only mode).
- Server names must match `^[a-zA-Z0-9_.-]{1,100}$`.
- **Env interpolation here IS `${VAR}`-style**: at discovery, values in
  `command`, `args`, `env`, `cwd`, `url`, `headers`, `auth`, `oauth` are
  recursively expanded with `${VAR}` and `${VAR:-default}` syntax
  (`discovery/helpers.ts:736` `expandEnvVars`; unset without default stays
  literal). THEN, at connect time, stdio `env` values and http/sse `headers`
  values go through `resolveConfigValue` (env-name-or-literal + `!command`) —
  `runtime-mcp/manager.ts:2509-2545`. So both mechanisms compose: you can write
  `env: {API_KEY: MY_VAR_NAME}` (bare env name) or `env: {API_KEY: ${MY_VAR}}`.
- **No dedicated bearer-from-env field.** For a bearer token on an http server,
  write `headers: {Authorization: "Bearer ${MCP_TOKEN}"}` (or bare env name,
  but then no `Bearer ` prefix is added — only `authHeader` on *providers*
  adds the prefix). First-party OAuth is supported via `auth.type: "oauth"` +
  stored credential (login flow through `/mcp`), with proactive refresh and
  `Authorization: Bearer <access>` injection; `auth.type: "apikey"` exists in
  the schema but has no dedicated runtime path beyond stored credentials.

## 5. Defaults (model selection)

- Default model is `modelRoles.default` in `~/.gjc/agent/config.yml`
  (a record; also `executor`, `architect`, `planner`, `critic`, `image` roles).
  Selector format: `provider/modelId` with optional `:effort` suffix
  (`:off|:minimal|:low|:medium|:high|:xhigh`), e.g. `my-provider/some-model-id:high`.
  This matches the IR's `defaults.model = "provider/model"`.
- Resolution precedence (`findInitialModel`): CLI `--model` > scoped model >
  saved default > provider defaults > first available.
- Custom providers join alias routing automatically: `hosted/glm-5.2` also
  resolves as bare `glm-5.2`.

## 6. IR mapping (agentcfg → GJC)

Clean fits:
- `providers[].id` → `providers.<id>` key. `base_url` → `baseUrl`.
  `protocol` → `api` (rename; values map 1:1 for openai-completions /
  openai-responses / anthropic-messages; `openai-responses` covers the IR's
  responses protocol; codex/azure/bedrock/google/ollama/cursor extras have no
  IR counterpart).
- `api_key_env` → `apiKeyEnv: <NAME>` (recommended) or `apiKey: <NAME>`.
- Model `id/name/context_window/max_output_tokens` →
  `models[].id/name/contextWindow/maxTokens`. Modalities → `input/output`
  arrays (text|image only — no audio). `reasoning` → `reasoning: bool` plus
  optional `thinking` block. `tool_calling` → only via `compat` flags
  (`supportsToolChoice`, `toolChoiceSupport`, ...) — boolean tool_calling has
  no direct field; best omitted (default true for completions/responses APIs).
- `defaults.model` → `config.yml` `modelRoles.default: "provider/model"`.
- `mcp[]` → `mcpServers.<id>`: stdio → `type: "stdio"`, command+args
  (IR `command` argv: first element → `command`, rest → `args`), `env`, `cwd`,
  `timeout_ms` → `timeout`; http → `type: "http"`, `url`, `headers`; `enabled`
  → `enabled`. IR transport `stdio|http` maps; GJC also has `sse`.

Lossy / not representable:
- Provider `name` (display name) — NOT in the strict provider schema (the
  `name:` shown in one doc example would fail validation; models do have
  `name`). Omit or post-process.
- Header map `{value, from_env, bearer_from_env}` → GJC headers are flat
  `Record<string,string>`. Mapping: `from_env` → bare env name (or `${VAR}`);
  `value` → literal, with the caveat that a literal colliding with an env var
  name resolves to the env value (ambiguity risk); `bearer_from_env` has NO
  native equivalent for providers — emulate with `!command` (e.g.
  `!sh -c 'printf "Bearer $VAR"'`) or pre-compose; for MCP http servers use
  `Authorization: "Bearer ${VAR}"` which works natively.
- `headers` per-provider and per-model exist; `bearer_from_env` at model level
  same limitation.
- IR model `tool_calling` boolean and any finer modality than text/image.
- MCP `timeout_ms` maps to `timeout` (ms, number). MCP env values from IR that
  are literal strings will pass through env-name-or-literal resolution — a
  literal that happens to name an env var gets replaced; use `${VAR}`-free
  literals carefully (e.g. prefix with a character, or accept the semantics).
- Discovery extras (`discovery.type`, `openaiCompat`, `transport: pi-native`,
  `requestTransform`, `compat.*`, `equivalence`, `profiles`) have no IR
  counterpart — optional loss, not a blocker.

## 7. Evidence log

- `npm view gajae-code --json` (registry.npmjs.org): 0.16.4, 2026-09-05,
  repo `Yeachan-Heo/gajae-code`, bin `gjc`/`가재씨`, dep `@gajae-code/coding-agent`.
- `https://gajae-code.com/` fetched 2026-09-07 (HTTP 200) — product identity.
- Git clone of the official repo (primary source), HEAD f238c66 (2026-09-07),
  version 0.16.6.
- Code read (not just README): `packages/utils/src/dirs.ts`,
  `packages/coding-agent/src/config/{settings.ts,settings-schema.ts,config-file.ts,models-config-schema.ts,model-registry.ts,resolve-config-value.ts,mcp-schema.json}`,
  `packages/coding-agent/src/runtime-mcp/{config.ts,manager.ts,loader.ts,types.ts}`,
  `packages/coding-agent/src/discovery/{builtin.ts,mcp-json.ts,helpers.ts}`,
  `packages/ai/src/auth-storage.ts`, `packages/utils/src/env.ts`,
  `schemas/{config.schema.json,models.schema.json}`,
  `docs/{models.md,custom-providers-and-multi-account.md}`.
- `gjc` is NOT installed on this machine (no binary, no `~/.gjc`); no local
  config observed.
- `brew search gajae` → no formula.

## 8. Confidence

HIGH, with caveats:
- Identity, file paths, schema field names, and resolution semantics are all
  read directly from the official repo source at a pinned HEAD, cross-checked
  against two repo JSON schemas and `docs/models.md`.
- Caveat 1: docs disagree on the `models:` shape (map vs array); I reported the
  code-verified array form.
- Caveat 2: the project is young (v0.16.x, first npm publish ~v0.1.1, rapid
  releases — 110 npm versions) and pre-1.0; schemas may churn.
- Caveat 3: behavior notes about session defaults (project MCP config enabled
  by default) come from `sdk/session.ts` reading of `mcp.enableProjectConfig`;
  the JSON schema's `default: false` is contradicted by an explicit code
  comment — medium-high confidence, flagged in text.
- No websearch (Serper key not configured), so no third-party corroboration;
  but all claims rest on primary sources (npm registry, official site, official
  repo code).
