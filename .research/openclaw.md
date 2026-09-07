# Research Report: OpenClaw — coding-agent CLI target for agentcfg

Research date: 2026-09-07. All findings verified against primary sources: the official
GitHub repo source code (actual zod schemas and loaders, not README), npm registry,
Homebrew formulae/cask, and the project's own docs tree inside the repo.

## 1. Identity verdict

**OpenClaw is a real, official AI-agent project with a first-party CLI — but it is a
personal/team AI assistant gateway, not a pure coding-agent CLI.** It qualifies as an
agentcfg target with that caveat.

- Official project: <https://github.com/openclaw/openclaw> — "The AI that really does
  things. Any OS. Any Platform. The lobster way." TypeScript, stewarded by the OpenClaw
  Foundation (Peter Steinberger / steipete et al., formerly "Clawdbot"/"Moltbot" — the
  legacy config filename `clawdbot.json` and the Homebrew formula rename
  `clawdbot-cli` → `openclaw-cli` confirm the rename history).
  Fetched 2026-09-07: 389k stars, default branch `main`, HEAD `3e100912c005` (2026-09-07).
- npm: `openclaw` package, latest `2026.9.2` (published 2026-09-05), bin `openclaw` →
  `openclaw.mjs`, MIT. Registry record created 2026-01-29. dist-tags: latest=2026.9.2,
  extended-stable=2026.6.34, beta=2026.9.1. Maintainers: steipete, vincentkoc.
- Homebrew: formula `openclaw-cli` (2026.9.2, "Your own personal AI assistant",
  old name `clawdbot-cli`) and cask `openclaw` (OpenClaw.app, macOS >= 15, auto_updates).
- PyPI: a package named `openclaw` exists but is **unrelated** ("Installer for the
  cmdop CLI", v2.0.2, <https://cmdop.com>). Do not use PyPI for this target.
- **Ruled out:** `pjasicek/OpenClaw` (C++) is the Captain Claw (1997) game-engine
  reimplementation — confirmed via GitHub search; it is not an agent tool.
- Not installed on this machine: `which openclaw` → not found; no `~/.openclaw` dir;
  brew formula/cask present but "Not installed".

Positioning (from repo README): "open-source AI assistant that runs on your own
computer and meets you in the channels you already use… Models and agent harnesses
(Claude, Codex, local models) are plugins you can swap." So it is a gateway with an
embedded agent (has exec/coding tools, `openclaw acp` can host coding harnesses), and
it reads the same two config surfaces agentcfg cares about: custom model providers and
MCP servers.

## 2. Version and sources actually fetched (2026-09-07)

- Git clone (depth 1) of <https://github.com/openclaw/openclaw.git>, branch `main`,
  package version `2026.9.2`, HEAD `3e100912c005cbcf5b50a4abc7f7d0bdcf1e8245`.
- <https://registry.npmjs.org/openclaw> (metadata JSON).
- `brew info openclaw-cli`, `brew info openclaw` (cask).
- <https://api.github.com/search/repositories?q=openclaw+captain+claw>
- <https://pypi.org/pypi/openclaw/json>

Key source files read verbatim in the clone:

- `src/config/paths.ts` — config path resolution.
- `src/config/zod-schema.core.ts` — `ModelDefinitionSchema`, `ModelProviderSchema`,
  `ModelProvidersSchema` (superRefine rules).
- `src/config/zod-schema.root-support.ts` — `McpServerSchema`, `McpServerNameSchema`.
- `src/config/types.models.ts`, `src/config/types.mcp.ts`, `src/config/types.secrets.ts`
  — TS contract types.
- `src/config/env-substitution.ts`, `src/config/io.read-helpers.ts`
  (`resolveConfigForRead`) — env interpolation semantics and where it runs.
- `packages/llm-core/src/model-data.ts` — `MODEL_DATA_APIS` protocol enum.
- Docs: `docs/concepts/model-providers.md`, `docs/gateway/config-tools.md`,
  `docs/gateway/secrets.md`, `docs/cli/mcp.md`, `docs/cli/models.md`,
  `docs/cli/config.md`, `docs/providers/models.md`, `docs/start/setup.md`.

## 3. Config file path(s) the CLI reads

- **Primary: `~/.openclaw/openclaw.json`** (source of truth for `models.providers`,
  `mcp.servers`, `agents.defaults.model`). Override the state dir with
  `OPENCLAW_STATE_DIR`; `OPENCLAW_HOME` relocates the home root; `OPENCLAW_PROFILE`
  selects a named profile state dir.
  Code: `src/config/paths.ts` → `CONFIG_FILENAME = "openclaw.json"`,
  `LEGACY_CONFIG_FILENAMES = ["clawdbot.json"]`.
- Format: JSON5-ish — comments allowed, plus `$include` file includes; parsed with a
  JSON5 parser at load (`src/config/io.read-helpers.ts`, `json5-comments.ts`).
- **Legacy:** `~/.openclaw/clawdbot.json` still read for migration.
- Generated (not authored) catalog file: `~/.openclaw/agents/<agentId>/agent/models.json`
  — materialized provider-catalog merge output; SecretRef-managed values are stored as
  source markers there. agentcfg should write `openclaw.json`, never this file.
- Auth profiles (OAuth etc.) live in per-agent `openclaw-agent.sqlite`, not in config.
- Managed-write helper: `openclaw config set models.providers.<id> '<json>'
  --strict-json --merge` (and `config get/patch/unset/validate/schema`); docs:
  `docs/cli/config.md`. Interactive wizard: `openclaw configure` / `openclaw config`.

## 4. Custom provider (models) config — verbatim schema

Top-level key: `models` (`ModelsConfig`), i.e. `models.providers.<id>`. Zod
(`src/config/zod-schema.core.ts`, `.strict()` — unknown keys rejected):

```
ModelProviderSchema = {
  baseUrl: string,                       // optional in zod, but REQUIRED for custom
                                         // providers (superRefine: "custom model providers
                                         // must declare baseUrl")
  apiKey: SecretInput (optional),        // string | SecretRef; registered sensitive
  auth: "api-key" | "aws-sdk" | "oauth" | "token" (optional),
  api: ModelApiSchema (optional),        // see protocol enum below; custom provider
                                         // without api defaults to "openai-completions"
                                         // (docs/gateway/config-tools.md)
  maxTokens: positive number (optional), // provider default output cap
  timeoutSeconds: positive int (optional),
  region: string (optional),
  injectNumCtxForOpenAICompat: boolean (optional),
  params: Record<string, unknown> (optional),
  agentRuntime: ModelAgentRuntimePolicySchema,
  localService: ModelProviderLocalServiceSchema,  // {command, args?, cwd?, env?,
                                                  //  healthUrl?, readyTimeoutMs?, idleStopMs?}
  headers: Record<string, SecretInput> (optional), // secret-bearing request headers
  authHeader: boolean (optional),        // default Authorization injection on/off
  request: ConfiguredModelProviderRequestSchema,
  models: ModelDefinitionSchema[]        // REQUIRED for custom providers
                                         // (superRefine: "must declare models")
}
models.mode: "merge" | "replace" (optional, default merge)
models.catalogRefresh: { enabled?, url? } (optional)
```

Model entry (`ModelDefinitionSchema`, `.strict()`):

```
{
  id: string (min 1, required),
  name: string (min 1, required),
  api: ModelApiSchema (optional, per-model adapter override),
  baseUrl: string (optional, per-model override),
  reasoning: boolean (optional),
  input: ("text" | "image" | "video" | "audio")[] (optional),
  cost: { input?, output?, cacheRead?, cacheWrite?, tieredPricing? } (optional, .strict()),
  contextWindow: positive number (optional),
  contextTokens: positive int (optional),  // effective runtime cap, distinct from native
  maxTokens: positive number (optional),
  thinkingLevelMap: ThinkingLevelMapSchema (optional),
  params: Record<string, unknown> (optional),
  agentRuntime: ModelAgentRuntimePolicySchema,
  headers: Record<string, string> (optional),   // plain strings, NOT SecretInput
  compat: ModelCompatConfig (optional),   // capability flags, incl. supportsTools: boolean
  mediaInput: ModelMediaInputSchema (optional),
  metadataSource: "models-add" (optional)
}
```

Protocol enum (`MODEL_DATA_APIS`, `packages/llm-core/src/model-data.ts`), accepted for
both provider `api` and model `api`:

```
"openai-completions", "openai-responses", "openai-chatgpt-responses",
"anthropic-messages", "google-generative-ai", "google-vertex", "github-copilot",
"bedrock-converse-stream", "ollama", "azure-openai-responses"
```

(The old id `openai-codex-responses` was removed; the schema error tells you to use
`openai-chatgpt-responses`.)

Documented custom-provider example (`docs/gateway/config-tools.md`, verbatim shape):

```json5
{
  models: {
    mode: "merge", // merge (default) | replace
    providers: {
      "custom-proxy": {
        baseUrl: "http://localhost:4000/v1",
        apiKey: "LITELLM_KEY",
        api: "openai-completions",
        models: [
          { id: "llama-3.1-8b", name: "Llama 3.1 8B", reasoning: false,
            input: ["text"], cost: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0 },
            contextWindow: 128000, contextTokens: 96000, maxTokens: 32000 },
        ],
      },
    },
  },
}
```

## 5. Default model

`agents.defaults.model` — `AgentModelSchema` (`src/config/zod-schema.agent-model.ts`):

```
agents.defaults.model: string | { primary?: string, fallbacks?: string[] }
```

Documented example: `agents: { defaults: { model: { primary: "anthropic/claude-opus-4-6" } } }`.
Model refs are `"provider/model-id"` using the `models.providers` key and the model `id`.
Per-agent override: `agents.entries.<id>.model`. There is also
`agents.defaults.models["provider/model"]` for per-model alias/metadata — it does not
register a runtime model by itself.

## 6. MCP config — verbatim schema

Top-level key `mcp`, map at `mcp.servers.<name>` (`McpServerSchema`,
`src/config/zod-schema.root-support.ts`; server name regex from
`McpServerNameSchema`; open-world catchall with retired aliases rejected):

```
{
  enabled: boolean (optional),           // false keeps the entry but excludes it
  command: string (optional),            // stdio transport: executable to spawn
  args: string[] (optional),             // stdio argv
  env: Record<string, string | number | boolean> (optional),  // stdio only
  cwd: string (optional),                // stdio working directory
  url: HttpUrl (optional),               // http/https remote server
  transport: "stdio" | "sse" | "streamable-http" (optional),
  headers: Record<string, string | number | boolean> (optional), // HTTP only; literals
  connectionTimeoutMs: positive finite number (optional),
  requestTimeoutMs: positive finite number (optional),
  supportsParallelToolCalls: boolean (optional),
  auth: "oauth" (optional),              // HTTP OAuth; tokens stored in OpenClaw state,
                                         // never in config
  oauth: { identity?: "shared" | "per-requester", authProfileId?, scope?,
           redirectUrl?, clientMetadataUrl? } (optional, strict),
  sslVerify: boolean (optional),
  clientCert: string (optional),         // mTLS
  clientKey: string (optional),
  toolFilter: { include?: string[] (min 1), exclude?: string[] (min 1) } (optional),
  codex: { agents?: string[], defaultToolsApprovalMode?: "auto" | "prompt" | "approve" }
         (optional)                      // Codex app-server projection metadata only
}
mcp.apps: { enabled?, sandboxOrigin?, sandboxPort? } (optional, unrelated to servers)
```

CLI management (`docs/cli/mcp.md`): `openclaw mcp add|set|configure|unset|list|show|
status|doctor|probe|tools|login|logout|reload`. Examples:

```bash
openclaw mcp add memory --command npx --arg -y --arg @modelcontextprotocol/server-memory
openclaw mcp add docs --url https://mcp.example.com/mcp --transport streamable-http \
  --auth oauth --oauth-scope docs.read --timeout 20 --connect-timeout 5
```

Validation extras: `disabled` key is rejected (use `enabled`); snake_case aliases
(`connect_timeout`, `ssl_verify`, …) are rejected; `oauth.identity: "per-requester"`
requires `auth: "oauth"` + a URL and cannot combine with stdio. Runtime adapters
normalize `transport` into downstream-native `type` values (`http`/`sse`/`stdio`) for
Claude Code / Gemini / Codex projection.

## 7. Env-var / secret handling

Three distinct mechanisms, all verified in source:

1. **Load-time `${VAR}` interpolation across the whole config tree.**
   `resolveConfigForRead` (`src/config/io.read-helpers.ts`) runs
   `resolveConfigEnvVars` over the parsed config after JSON5 parse and `$include`
   resolution. Applies to every string value anywhere — provider `apiKey`, provider
   headers, **MCP `env` and `headers` included**. Rules (`src/config/env-substitution.ts`):
   - Only uppercase names match: `[A-Z_][A-Z0-9_]*`.
   - Escape with `$${VAR}` to emit a literal `${VAR}`.
   - Unbraced `$VAR` is **not** interpolated in general strings (only the braced form).
   - Missing/empty var: strict contexts throw `MissingEnvVarError`; config load
     collects a warning and preserves the placeholder.

2. **SecretInput fields** (`apiKey`, provider `headers` values): `string | SecretRef`
   where `SecretRef = { source: "env" | "file" | "exec" | "store", provider: string,
   id: string }`. Shorthand: a whole-string `$NAME` or `${NAME}` (uppercase only) in a
   SecretInput field is treated as an env secret ref and resolved lazily through the
   secrets runtime (env / file / exec / shared SQLite store providers configured under
   `secrets.providers`). Legacy markers `secretref-env:` and `__env__:` are read for
   migration. Plaintext strings still work.

3. **MCP HTTP auth**: no per-header secret machinery. Options are literal headers
   (discouraged for tokens; `openclaw mcp doctor` warns on sensitive-looking literal
   header/env values) or `auth: "oauth"` + `openclaw mcp login`, which stores tokens in
   OpenClaw state, not config. Bearer-from-env is achieved by writing
   `"Authorization": "Bearer ${MCP_TOKEN}"` (load-time interpolation, mechanism 1).

## 8. IR mapping analysis (agentcfg YAML IR → openclaw.json)

Maps cleanly:

| IR field | OpenClaw field |
|---|---|
| `providers[].id` | `models.providers` map key |
| `providers[].name` | no provider display name; use id (model `name` exists) |
| `providers[].protocol` | `api` (subset: openai-completions / openai-responses / anthropic-messages all exist 1:1) |
| `providers[].base_url` | `baseUrl` |
| `providers[].api_key_env` | `apiKey: "${VAR}"` (or `$VAR` / SecretRef `{source:"env",...}`) |
| `providers[].headers` `value` | `headers: {k: "literal"}` — values are SecretInput, so `from_env` → `"${VAR}"`, plain `value` → literal |
| `headers.bearer_from_env` | `headers: {Authorization: "Bearer ${VAR}"}` |
| `models[].id/name/context_window/max_output_tokens` | `models[].id/name/contextWindow/maxTokens` |
| `models[].input` modalities | `input: ["text","image","video","audio"]` (superset of IR) |
| `models[].reasoning` | `reasoning: bool` |
| `models[].tool_calling` | `compat: {supportsTools: bool}` (also `compat` has many more capability flags) |
| `defaults.model` `"provider/model"` | `agents.defaults.model: "provider/model"` (string form), or `{primary}` object form |
| `mcp[].id` | `mcp.servers` map key |
| `mcp[].enabled` | `enabled` |
| `mcp[].transport stdio` | `transport: "stdio"` + `command` + `args` |
| `mcp[].transport http` | `transport: "streamable-http"` (or `"sse"`) + `url` |
| `mcp[].env` | `env` (string values; `${VAR}` interpolates at load) |
| `mcp[].headers` | `headers` (literals; `${VAR}` interpolates; no SecretRef per header) |
| `mcp[].cwd` | `cwd` |
| `mcp[].timeout_ms` | split into `connectionTimeoutMs` + `requestTimeoutMs` — IR's single value must be duplicated or policy-chosen |

What the IR **cannot** represent (OpenClaw-side extras, all optional — safe to omit):

- Provider: `auth` modes (`api-key`/`aws-sdk`/`oauth`/`token`), `region`,
  `localService`, `request`, `injectNumCtxForOpenAICompat`, `params`,
  `agentRuntime`, `models.mode: "replace"`, `catalogRefresh`.
- Model: `contextTokens` (effective cap vs native window), `cost`/`tieredPricing`,
  `thinkingLevelMap`, `params`, per-model `baseUrl`/`api` overrides, `mediaInput`,
  the full `compat` capability matrix (only `supportsTools` maps from IR
  `tool_calling`).
- MCP: `auth: "oauth"` + `oauth` block, `sslVerify`/`clientCert`/`clientKey` (mTLS),
  `supportsParallelToolCalls`, `toolFilter`, `codex` projection block,
  `connectionTimeoutMs` vs `requestTimeoutMs` duality, `mcp.apps`.
- Agents: the whole `agents` tree beyond `agents.defaults.model` (roster, tools
  policy, sandbox, subagents) — out of agentcfg's providers/MCP scope.

Gaps on the OpenClaw side relative to the IR:

- **No `output` modality field on models** — OpenClaw model entries only declare
  `input`. IR `output` has nowhere to go (can be dropped or mapped into a
  provider `params` passthrough if ever needed).
- **No dedicated `apiKeyEnv` field** — env indirection is expressed inside
  `apiKey` (`${VAR}`). Functionally equivalent.
- **MCP `env` values are `string|number|boolean`**, not SecretInput; env refs only
  via load-time `${VAR}`. Fine for agentcfg's env-reference-only rule.
- **Single `timeout_ms` vs two timeouts** — must pick `connectionTimeoutMs` and/or
  `requestTimeoutMs` when compiling.
- Transport naming: IR `http` must compile to OpenClaw `"streamable-http"`.

## 9. Caveats and confidence notes

- OpenClaw config schemas are `.strict()` — the compiler must emit only documented
  keys or validation fails.
- Version churn is high (252 npm versions; 2026.9.x line). Pin the schema facts in
  this report to commit `3e100912c005` / npm `2026.9.2` (2026-09-05).
- The tool is a general assistant gateway; "coding-agent CLI" fit is partial. Its
  provider/MCP surfaces are exactly what agentcfg compiles, and it also projects MCP
  servers into Claude Code/Codex/Gemini native formats, but agentcfg would target
  `openclaw.json` itself.
- Machine-local verification was impossible for runtime behavior (tool not
  installed; no `~/.openclaw`); all contract facts come from repo source + docs +
  registry metadata, which is sufficient for schema compilation.
