# Research Report: `jcode` coding-agent CLI target

- **Research date**: 2026-09-07 (UTC)
- **Researcher**: sub-agent `research-jcode` for the agentcfg project
- **Confidence**: **HIGH** overall (identity, config paths, provider schema, and MCP schema verified against cloned official source code). **MEDIUM** only on two minor points noted below.

---

## 1. Identity — what "jcode" actually is

### 1.1 Ruling in: `1jehuang/jcode` (the official CLI)

| Fact | Value | Source |
|---|---|---|
| GitHub repo | `1jehuang/jcode` | https://github.com/1jehuang/jcode (fetched 2026-09-07) |
| Tagline | "The most RAM efficient harness" | repo description |
| Language / License | Rust / MIT | repo metadata |
| Stars | 19,273 (at fetch time) | repo metadata |
| Homepage / Docs | https://jcode.sh · https://jcode.sh/docs | repo homepage; HTTP 200 verified |
| Latest release | **v0.84.0** (tag `v0.84.0`, GitHub release id 383811261) | https://api.github.com/repos/1jehuang/jcode/releases/latest |
| Latest commit examined | `b06ef5dab30c2c47f9befcbe8265550631beab0c` (2026-09-07T02:38:47-07:00, branch `master`) | local clone `/tmp/jcode-src` (`git clone --depth 1 https://github.com/1jehuang/jcode.git`) |
| Version in tree | `0.84.0` (`Cargo.toml` line 3) | `/tmp/jcode-src/Cargo.toml` |
| Package managers | homebrew/core formula `jcode` (stable 0.84.0, executables `jcode` and `jcode-harness`); install script `curl -fsSL https://jcode.sh/install \| bash`; Windows `irm https://jcode.sh/install.ps1`; npm binary packages under scope `@1jehuang/jcode-{darwin,linux,win32}-{x64,arm64}` v1.1.0 | https://formulae.brew.sh/api/formula/jcode.json (fetched 2026-09-07); README "Installation" section; npm registry |
| Repo topics | ai, ai-agent, ai-coding-agent, claude, cli, coding-agent, llm, mcp, openai, rust, terminal, tui | repo metadata |

This is a terminal AI coding agent harness with a TUI, daemon/server mode, MCP support,
built-in providers (claude OAuth, openai, copilot, openrouter, gemini, antigravity,
bedrock, ollama, lmstudio) plus **named custom provider profiles** — the surface that
matters for agentcfg.

### 1.2 Ruling out the npm `jcode` package (the local hint)

`npm view jcode` returns `0.0.0`. Verified full metadata (2026-09-07):

- Author: Johnny Bakker (`johnnybakker1997@gmail.com`), created 2019-10-07.
- Description: "Just another nodejs mvc framework...". Repo: `github.com/johnnybakker/JCode`.
- Single version `0.0.0`, ISC license, zero dependencies, 6.6 kB tarball.
- Conclusion: name-squatted/unrelated MVC framework stub. **Not** a coding agent. The real
  CLI is a Rust binary distributed via GitHub releases / homebrew / install script, not via
  the bare npm `jcode` name (its npm presence is only the `@1jehuang/jcode-*` platform
  binary packages and `@1jehuang/jcode-sdk`).

### 1.3 Other candidates checked and excluded

- GitHub repo search for `jcode` (top 30 by stars): `1jehuang/jcode` dominates at 19.3k
  stars; next candidates are `jcodec/jcodec` (Java video codec), `NLPchina/Jcoder` (Java
  dynamic-code hosting), `akira-cn/jcode-awesome` / `xitu/jcode-*` (码上掘金, a
  ByteDance/掘金 online code-playground ecosystem — not a CLI), `JarvisPMS/JCode`
  (a Claude Code launcher wrapper, 19 stars), `cnjack/jcode` (34 stars, "An AI coding
  agent that works where your code lives" — a small personal project, not the
  user-intended target; `1jehuang/jcode` is the canonical, homebrew-packaged tool).
- No JetBrains product named jcode (JetBrains' agent is Junie). No Chinese-vendor CLI
  named jcode surfaced in npm/GitHub search results.

---

## 2. Config files the CLI reads

### 2.1 Main config (providers, defaults)

- **Path**: `~/.jcode/config.toml`, or `$JCODE_HOME/config.toml` when `JCODE_HOME` is set.
- Format: **TOML**. Loaded by `Config::load` in `crates/jcode-base/src/config.rs`
  (doc comment line 3: "Config is loaded from `~/.jcode/config.toml` (or `$JCODE_HOME/config.toml`).
  Environment variables override config file settings."). Deserialization is deliberately
  lenient (`crates/jcode-config-types/src/serde_lenient.rs`).

### 2.2 Provider secret env files

- Directory: platform app config dir + `jcode` — e.g. **`~/.config/jcode/` on Linux/macOS**
  (`crates/jcode-storage/src/lib.rs`, `app_config_dir()`); sandboxed under
  `$JCODE_HOME/config/jcode` when `JCODE_HOME` is set.
- Files are `KEY=value` line format, looked up by prefix (e.g. `provider-my-api.env`
  referenced from a profile's `env_file` field; default `openai-compatible.env`).

### 2.3 MCP config

- **Primary**: `~/.jcode/mcp.json` (global), `.jcode/mcp.json` (project-local). Format: **JSON**.
- Compat sources read live (not copied): `~/.claude.json` (top-level `mcpServers` plus
  per-project `projects.<abs_path>.mcpServers`; disable with `JCODE_DISABLE_CLAUDE_MCP`),
  `.mcp.json` and `.claude/mcp.json` at repo root.
- One-time migration: if `~/.jcode/mcp.json` does not exist, servers are imported once
  from `~/.codex/config.toml` (`[mcp_servers.*]` sections), then jcode-owned.
- Load/merge order: jcode global → Claude global → per-project Claude → project-local
  files; later sources override. Env expansion happens after the merge.
- Evidence: `crates/jcode-base/src/mcp/protocol.rs` (`McpConfig::load_for_dir`,
  `load_project_locals`, `load_claude_json`, `import_from_codex_once`),
  `crates/jcode-app-core/src/tool/mcp.rs` (user-facing messages), README "MCP config files".

---

## 3. Native schema — providers (verbatim from source)

From `crates/jcode-config-types/src/lib.rs` (types) and `crates/jcode-base/src/config.rs`
(`Config` struct). Serialized shape in `~/.jcode/config.toml`:

```toml
[provider]
default_provider = "my-api"        # profile id (also built-in names like "openai")
default_model = "my-model-id"      # bare model id, NOT "provider/model"
# plus built-in-provider knobs: openai_reasoning_effort, anthropic_reasoning_effort,
# openai_transport (auto|websocket|https), stream_idle_timeout_secs, max_retries, ...

[providers.my-api]                  # TOML table keyed by profile id
type = "openai-compatible"          # NamedProviderType
base_url = "https://llm.example.com/v1"
api_key_env = "JCODE_PROVIDER_MY_API_API_KEY"   # env var NAME; value never in config
env_file = "provider-my-api.env"                # fallback secret file in app_config_dir
api_key = "sk-..."                  # inline key (exists; avoid for agentcfg)
auth = "bearer"                     # "bearer" | "header" | "none"
auth_header = "X-Api-Key"           # used when auth = "header"
default_model = "my-model-id"
requires_api_key = true
provider_routing = false            # OpenRouter-style provider routing
model_catalog = false               # use endpoint /models response
allow_provider_pinning = false
disable_reasoning_heuristics = false
# supports_reasoning_effort = true  # bool, auto-detected from model id when unset
# api = "..."                       # Option<String>, defined but NO consumer found (legacy?)

[providers.my-api.headers]          # BTreeMap<String,String>, LITERAL strings only
x-tenant-id = "tenant-42"

[providers.my-api.extra_body]       # merged verbatim into request JSON body
chat_template_kwargs = { thinking = true, reasoning_effort = "high" }

[[providers.my-api.models]]         # Vec<NamedProviderModelConfig>
id = "my-model-id"                  # required
context_window = 128000             # aliases: context_limit, context-length, context-window
reasoning = true                    # Option<bool>: enable/disable /effort
reasoning_effort = "high"           # Option<String>, alias "reasoning-effort"
input = ["image"]                   # Vec<String>; only "image" is acted on (vision support)
```

Key enums (verbatim serde representations):

- `NamedProviderType`: `openai-compatible` (default; aliases `openai_compatible`) |
  `anthropic-compatible` (aliases `anthropic_compatible`) | `openrouter`.
- `NamedProviderAuth`: `bearer` (default) | `header` | `none`.

API-key resolution order (verified in `crates/jcode-provider-env/src/lib.rs`,
`load_api_key_from_env_or_config`): process env var `api_key_env` → secret file
`app_config_dir/<env_file>` line `KEY=value` → registered fallback resolvers
(e.g. macOS Keychain). Values are sanitized of invisible Unicode chars. So custom
providers are fully env-reference friendly.

**Protocols supported by custom profiles**: OpenAI-compatible profiles use streaming
**chat completions** (`crates/jcode-provider-openrouter-runtime` handles named profiles;
jcode uses streaming chat completions + function/tool calling, per README). `anthropic-compatible`
profiles use the **Anthropic Messages** API. There is **no openai-responses option for named
profiles** — the OpenAI Responses API is only used by the built-in `openai` provider
(`openai_transport = auto|websocket|https`). The `api: Option<String>` field exists on
`NamedProviderConfig` but a repo-wide search found no reader — treat as unused/legacy
(MEDIUM confidence note).

**Env interpolation in config.toml: NONE.** Values are literal strings. The only
env indirection is the `api_key_env` / `env_file` mechanism (and `JCODE_*` env vars that
override specific built-in settings). This matches agentcfg's "env references only" rule.

**Model reference syntax**: config uses two separate keys (`default_provider` +
`default_model`), not a combined `"provider/model"` string. At the CLI/TUI layer profiles
are addressed as `openai-compatible:<profile-id>` (e.g. `api_method =
"openai-compatible:my-api"`, seen in `tests/provider_matrix.rs`,
`crates/jcode-provider-openrouter-runtime/src/lib.rs:1318`,
`crates/jcode-config-types/src/lib.rs:1250` docs).

---

## 4. Native schema — MCP (verbatim from source)

`McpConfig` / `McpServerConfig` in `crates/jcode-base/src/mcp/protocol.rs`:

```json
{
  "mcpServers": {                  // top-level key; alias "servers" also accepted
    "filesystem": {
      "command": "/path/to/mcp-server",   // stdio command (REQUIRED to be stdio)
      "args": ["--root", "/workspace"],
      "env": { "KEY": "value" },          // values support ${VAR} / ${VAR:-default}
      "shared": true,                     // bool, default true (cross-session reuse)
      "enabled": true,                    // opencode-style; default true
      "disabled": false,                  // Claude-Code-style; WINS over enabled
      "timeout_secs": 120,                // u64 seconds; default 30s per request
      "type": "stdio",                    // "stdio"|"http"|"sse" — http/sse SKIPPED at load
      "url": "https://...",               // parsed, env-expanded, but UNUSED today
      "headers": { "Authorization": "Bearer ${TOKEN}" }  // parsed, env-expanded, UNUSED today
    }
  }
}
```

Hard facts from the loader code:

- **stdio only**: `McpServerConfig::is_stdio()` returns false for `type` =
  `http`/`sse`/`streamable-http`, and such entries are skipped at load time
  ("jcode currently only supports stdio (command-based) MCP servers... such entries are
  skipped at load time"). There is no MCP HTTP transport at all in this version.
- **Env interpolation**: `expand_environment_string` implements Claude Code's documented
  `${VAR}` and `${VAR:-default}` syntax, applied to `command`, every `args` entry, every
  `env` value, `url`, and every `headers` value, after all sources are merged. Malformed
  expressions are preserved; unset vars are left unexpanded with a logged warning.
- **No `cwd` field** on MCP servers. **No bearer-token env fields** (moot while HTTP MCP
  is unsupported; `headers` is the only auth hook and it is currently unused).
- **Timeout unit is seconds** (`timeout_secs: Option<u64>`), default 30 s per request.
- Both `enabled` and `disabled` booleans are supported; `disabled` wins when both present.
- `JCODE_DISABLE_CLAUDE_MCP` env var disables live reading of `~/.claude.json`.

---

## 5. Verbatim config examples from the official README (master, 2026-09-07)

README.md lines ~392-520 (OpenAI-compatible and Anthropic-compatible profiles):

```toml
[provider]
default_provider = "my-api"
default_model = "my-model-id"

[providers.my-api]
type = "openai-compatible"
base_url = "https://llm.example.com/v1"
api_key_env = "JCODE_PROVIDER_MY_API_API_KEY"
env_file = "provider-my-api.env"
default_model = "my-model-id"
disable_reasoning_heuristics = true

[[providers.my-api.models]]
id = "my-model-id"
context_window = 128000
reasoning = true
reasoning_effort = "high"
```

```toml
[provider]
default_provider = "corp-claude"
default_model = "claude-sonnet-4-6"

[providers.corp-claude]
type = "anthropic-compatible"
base_url = "https://gateway.example.com/anthropic/v1"
auth = "bearer"
api_key_env = "CORP_CLAUDE_TOKEN"
default_model = "claude-sonnet-4-6"

[providers.corp-claude.headers]
x-tenant-id = "tenant-42"

[[providers.corp-claude.models]]
id = "claude-sonnet-4-6"
context_window = 200000
```

One-shot setup command (writes profile + env file):

```bash
printf '%s' "$MY_API_KEY" | jcode provider add my-api \
  --base-url https://llm.example.com/v1 --model my-model-id \
  --api-key-stdin --set-default --json
# or --api-key-env NAME to reference an existing env var instead of storing a key
jcode --provider-profile my-api run 'hello'
```

---

## 6. Mapping analysis — agentcfg IR → jcode

IR shape compiled: `providers[].{id, name, protocol, base_url, api_key_env, headers{value|from_env|bearer_from_env}, models[].{id, name, context_window, max_output_tokens, input/output modalities, reasoning, tool_calling}}`, `mcp[].{id, transport stdio|http, enabled, command argv, env, url, headers, cwd, timeout_ms}`, `defaults.model = "provider/model"`.

### 6.1 What maps cleanly

| IR field | jcode target |
|---|---|
| `providers[].id` | `[providers.<id>]` TOML table key (also the display name) |
| `protocol: openai-completions` | `type = "openai-compatible"` |
| `protocol: anthropic-messages` | `type = "anthropic-compatible"` |
| `base_url` | `base_url` |
| `api_key_env` | `api_key_env` (+ optional `env_file`) |
| `headers value` | `[providers.<id>.headers]` literal strings |
| `models[].id` | `[[providers.<id>.models]] id` |
| `models[].context_window` | `context_window` (aliases accepted) |
| `models[].reasoning` (bool) | `reasoning = true/false` |
| `mcp[].id` | key in `mcpServers` map |
| `mcp[].enabled` | `enabled` / `disabled` |
| `mcp[].command argv` | `command` + `args[]` |
| `mcp[].env` | `env` map (values may use `${VAR}`) |
| `mcp[].transport: stdio` | (default; omit `type`) |

### 6.2 What the IR cannot represent (jcode-only native features)

- `auth = "header"` + `auth_header` custom header-name auth; `auth = "none"`.
- `env_file` secret-file fallback; `extra_body` request-body injection;
  `supports_reasoning_effort` / `disable_reasoning_heuristics` /
  model-level `reasoning_effort`; `provider_routing` / `model_catalog` /
  `allow_provider_pinning` (OpenRouter-ish features); `requires_api_key`.
- MCP `shared: false` (per-session spawn, e.g. Playwright).
- Built-in provider knobs under `[provider]` (reasoning efforts, transports, retries).

### 6.3 What jcode cannot represent (IR features with no native target)

- `providers[].name` — no display-name field; the TOML key is the name.
- `models[].name` — no per-model display name.
- `models[].max_output_tokens` — no per-model max-tokens in profile config
  (a global per-request max exists via env override, e.g. `JCODE_OPENAI_MAX_TOKENS`-style
  knobs; MEDIUM confidence — no per-model field found in schema).
- `models[].output modalities`, `models[].tool_calling` — not configurable; tool calling
  is always attempted; only `input = ["image"]` (vision) is modeled.
- `headers.from_env` / `headers.bearer_from_env` — header values are literal-only in
  config.toml. Workaround: `auth = "bearer"` + `api_key_env` covers the common
  bearer-from-env case; arbitrary per-header env refs are impossible (inline secret
  would be required, which agentcfg forbids).
- `mcp[].transport: http` — entries are skipped at load; **jcode stdio MCP only**.
- `mcp[].url`, `mcp[].headers` — parsed but unused (no HTTP MCP transport).
- `mcp[].cwd` — no such field.
- `mcp[].timeout_ms` — must convert to integer `timeout_secs` (seconds).
- `defaults.model = "provider/model"` — must split into `[provider] default_provider` +
  `default_model` (or use `--provider-profile <id>` at launch).

### 6.4 Env-reference compatibility verdict

Excellent fit for agentcfg's "env references only, never inline secrets" rule:
`api_key_env`/`env_file` are first-class; MCP `env` values support `${VAR}` and
`${VAR:-default}` interpolation; the only literal-only area is provider `headers`
(acceptable: bearer auth via env is covered by `auth` + `api_key_env`).

---

## 7. Evidence log (all fetched/run 2026-09-07)

- `npm view jcode [--json]` — stub package metadata (johnnybakker, 0.0.0, 2019).
- `https://registry.npmjs.org/-/v1/search?text=jcode` — npm candidates incl. `@1jehuang/jcode-*` v1.1.0 platform binaries.
- `https://api.github.com/search/repositories?q=jcode` — GitHub candidates (1jehuang/jcode 19,273 stars; exclusions listed in §1.3).
- `https://api.github.com/repos/1jehuang/jcode` — repo metadata (Rust, MIT, homepage jcode.sh).
- `git clone --depth 1 https://github.com/1jehuang/jcode.git /tmp/jcode-src` — commit `b06ef5d`, Cargo.toml `0.84.0`.
- Source files read: `crates/jcode-config-types/src/lib.rs` (NamedProviderConfig, NamedProviderModelConfig, ProviderConfig, enums), `crates/jcode-base/src/config.rs` (Config, load paths), `crates/jcode-storage/src/lib.rs` (jcode_dir, app_config_dir), `crates/jcode-provider-env/src/lib.rs` (api-key resolution), `crates/jcode-provider-openrouter-runtime/src/lib.rs` (named-profile runtime, image-input handling), `crates/jcode-base/src/mcp/protocol.rs` (McpConfig/McpServerConfig, load order, env expansion, stdio-only), `crates/jcode-app-core/src/tool/mcp.rs` (MCP paths/messages), `src/cli/commands/provider_setup.rs` (`jcode provider add` writer), `README.md` (install, provider profile docs, MCP docs).
- `https://api.github.com/repos/1jehuang/jcode/releases/latest` — v0.84.0.
- `https://formulae.brew.sh/api/formula/jcode.json` — homebrew/core `jcode` 0.84.0, executables `jcode`, `jcode-harness`.
- `https://jcode.sh/docs` — HTTP 200, official docs index.
- Local machine: `which jcode` — not installed; no `~/.jcode` or `~/.config/jcode` present.

### Known unknowns (MEDIUM confidence items)

1. `NamedProviderConfig.api` (`Option<String>`): defined in the serde schema but no
   consuming code found in the cloned tree; likely legacy. Do not emit it.
2. Per-model max output tokens: not in the named-profile model schema; only global/env
   overrides exist.
3. Project-local `~/.jcode`-style overrides for the main `config.toml` were not observed;
   only MCP has project-local config.
