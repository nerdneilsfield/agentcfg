# Goose (Block / AAIF) native configuration contract

Research date: 2026-09-07. Target is Block's goose agent (CLI binary `goose`, desktop `Goose.app`), now hosted as Linux Foundation AAIF project `aaif-goose/goose`. All schema claims below are from the cloned source at `/tmp/goose-src` (clone URL still `https://github.com/block/goose`, GitHub API redirects to `aaif-goose/goose`) plus this machine's live `~/.config/goose`.

Pinned version: **1.49.0** (workspace `Cargo.toml`, GitHub latest release tag `v1.49.0` published 2026-09-03T19:34:26Z, Homebrew formula `block-goose-cli` 1.49.0, Homebrew cask `block-goose` 1.49.0, local Goose.app `CFBundleShortVersionString` 1.49.0). Clone HEAD `5e90925962f05acf8e255032de44d16c4a7768a2` (2026-09-05T02:25:17Z). License Apache-2.0.

## 1. Identity

**Official.** Not a decoy.

| Check | Result |
|---|---|
| GitHub | `block/goose` 301 → `aaif-goose/goose` (repo id 846698999). Description: "an open source, extensible AI agent that goes beyond code suggestions". ~54k stars. Default branch `main`. |
| Binary | `crates/goose-cli/Cargo.toml` `[[bin]] name = "goose"`. |
| Desktop | macOS app `Goose.app` (`/Applications/Goose.app/Contents/MacOS/Goose`). |
| Docs | https://goose-docs.ai/ and https://block.github.io/goose/ |
| Homebrew | **CLI:** `brew install block-goose-cli` (conflicts with unrelated formula `goose` because both install a `goose` binary). **Desktop:** `brew install --cask block-goose`. |
| Install script | `curl -fsSL https://github.com/aaif-goose/goose/releases/download/stable/download_cli.sh \| bash` (`GOOSE_BIN_DIR` default `$HOME/.local/bin`). Windows: `download_cli.ps1`. |
| Cargo | Source is a Rust workspace; users do not `cargo install` as the documented path. |
| Local this machine | CLI **not** on PATH (`which goose` → not found; `block-goose-cli` formula not installed). Desktop **is** installed: cask `block-goose` 1.49.0. Config dir **exists**: `~/.config/goose/config.yaml` + `custom_providers/custom_newapi.json`. No `~/.goose`. |

**Ruled out:** Homebrew core formula `goose` (unrelated, binary-name conflict); any PyPI/npm package named goose that is not this Rust project.

## 2. Config files

Format: **YAML**. File name constant `CONFIG_YAML_NAME = "config.yaml"` (`crates/goose/src/config/base.rs`).

### Paths

User config dir from `Paths::config_dir()` (`crates/goose/src/config/paths.rs`):

- If `GOOSE_PATH_ROOT` is set to an **absolute** path: `$GOOSE_PATH_ROOT/config/` (also `data/`, `state/`). Relative values are ignored.
- Else `etcetera` app strategy with `top_level_domain/author = "Block"`, `app_name = "goose"` (kept for backwards compatibility). On this macOS host that is XDG-style **`~/.config/goose`** (observed live). Docs also list Windows `%APPDATA%\Block\goose\config\config.yaml`.

There is **no `GOOSE_CONFIG` env**. Relocation is `GOOSE_PATH_ROOT`.

Load merge order (`Config::default`):

1. System: Unix `/etc/goose/config.yaml`; Windows `%PROGRAMDATA%\goose\config.yaml`
2. Extra files from `GOOSE_ADDITIONAL_CONFIG_FILES` (`env::split_paths`)
3. User: `$config_dir/config.yaml` (write target)

Precedence for a **param** (`get_param`): env var named `KEY.to_uppercase()` first, then merged YAML. Secrets (`get_secret`): env (uppercase) then keyring service `"goose"` then, if `GOOSE_DISABLE_KEYRING` is set/true, `$config_dir/secrets.yaml`. **API keys are never read from `config.yaml`.** Putting `OPENAI_API_KEY:` in YAML is ignored (`documentation/docs/guides/config-files.md`).

Related files in the same dir (not IR-mapped except as noted):

- `custom_providers/<id>.json` — custom / declarative provider metadata (JSON, not YAML)
- `secrets.yaml` — file-backed secrets
- `permission.yaml` — tool permissions
- `permissions/tool_permissions.json`

### Root YAML keys relevant to agentcfg

```yaml
active_provider: anthropic          # current selection
providers:
  anthropic:
    enabled: true
    model: claude-sonnet-4-5-20250929
    configured: true
GOOSE_PROVIDER: openai              # legacy flat keys still read
GOOSE_MODEL: gpt-4o
GOOSE_MAX_TOKENS: 4096              # per-response cap, not model metadata
OPENAI_HOST: https://api.openai.com
OPENAI_BASE_PATH: v1/chat/completions
OPENAI_BASE_URL: https://api.openai.com/v1
OPENAI_TIMEOUT: 600
extensions: { ... }
```

`providers` here is **not** custom-provider metadata. It is only `{enabled, model, configured}` per built-in/custom provider **name** (`crates/goose/src/config/providers.rs`).

## 3. Custom providers

Custom providers are **JSON files**, one per provider, in `$config_dir/custom_providers/<name>.json`. CLI: `goose configure` → Custom Providers. Desktop: Settings → Models → Add Custom Provider.

Schema: `DeclarativeProviderConfig` in `crates/goose-providers/src/declarative.rs` (serde, field names as written):

```json
{
  "name": "custom_corp_api",
  "engine": "openai",
  "display_name": "Corporate API",
  "description": "Custom Corporate API provider",
  "api_key_env": "CUSTOM_CORP_API_API_KEY",
  "base_url": "https://api.company.com/v1/chat/completions",
  "models": [
    { "name": "gpt-4o", "context_limit": 128000 }
  ],
  "headers": { "x-origin-client-id": "YOUR_CLIENT_ID" },
  "timeout_seconds": 600,
  "supports_streaming": true,
  "requires_auth": true,
  "base_path": null,
  "env_vars": null,
  "auth": null,
  "dynamic_models": null,
  "preserves_thinking": false
}
```

### `engine` (protocol)

`ProviderEngine` serde `rename_all = "lowercase"`:

| JSON `engine` | aliases | Runtime |
|---|---|---|
| `openai` | `openai_compatible` | OpenAI-compatible HTTP (`OpenAiProvider`) |
| `anthropic` | `anthropic_compatible` | Anthropic Messages |
| `ollama` | `ollama_compatible` | Ollama |

Invalid engine → `Invalid provider type`. There is **no** `databricks` engine for custom JSON; Databricks is a built-in provider.

`generate_id(display_name)` produces `custom_<normalized>` (lowercase, non `[a-z0-9_-]` → `_`). `generate_api_key_name(id)` → `{ID.to_uppercase()}_API_KEY` (e.g. `custom_newapi` → `CUSTOM_NEWAPI_API_KEY`). Observed locally: `~/.config/goose/custom_providers/custom_newapi.json` with `engine: "openai"`, `api_key_env: "CUSTOM_NEWAPI_API_KEY"`.

### Model metadata (`ModelInfo`)

`crates/goose-provider-types/src/base.rs`:

| Field | Type | Notes |
|---|---|---|
| `name` | string | **required**; this is the model id sent to the API |
| `resolved_model` | string? | alias resolution |
| `context_limit` | usize? | context window |
| `input_token_cost` / `output_token_cost` | f64? | USD per token |
| `currency` | string? | |
| `supports_cache_control` | bool? | |
| `reasoning` | bool, default false | thinking controls |
| `thinking_preservation_format` | `content_prepend` \| `content_xml` \| `reasoning_content` | |
| `request_params` | map? | merged into request body |

**No** `max_tokens` / `max_output_tokens` on the model object. **No** `vision` / modality fields. Per-response cap is global `GOOSE_MAX_TOKENS`, not per-model.

### `api_key_env`

Stores the **environment / secret key name**, not the literal key. Resolution (`get_secret`): process env (`KEY.to_uppercase()`) then keyring/`secrets.yaml`. Mutually exclusive with `auth.command`. Empty `api_key_env` + `requires_auth: false` = no auth (local). Empty + `requires_auth: true` is an error at use time.

Headers on the JSON are **literal** `HashMap<String, String>`. They are **not** `${VAR}`-expanded unless listed in `env_vars` **and** the placeholder appears in `base_url` (expansion currently applied to `base_url` only in `resolve_config`). Do not put secrets in `headers`.

### Completions vs Responses (openai engine)

Default derived `base_path` from `base_url` (`derive_base_path`):

- empty path → `v1/chat/completions`
- path already ending `chat/completions` → kept
- path ending `/vN` → `{path}/chat/completions`
- else → `{path}/v1/chat/completions`

Routing (`should_use_responses_api`):

- If `base_path` is the default `v1/chat/completions`, Responses is used when the **model name** matches `is_openai_responses_model`: `(?i)(?:^|[-/])(?:o\d+(?:$|-)|gpt-(?:5|6)(?:$|[-.]))`.
- If `base_path` contains `responses` → always Responses.
- If `base_path` contains `chat/completions` (and is not the default) → always Chat Completions (gateways).
- Custom `base_path: "v1/responses"` forces Responses for that provider.

So IR `openai-completions` vs `openai-responses` is representable by `engine: openai` plus `base_path` (and/or a `base_url` whose path includes `/responses`). Mixing both protocols on one custom provider is not first-class.

### Built-in OpenAI (not custom JSON)

Provider name `"openai"`. Host keys: `OPENAI_HOST` (deprecated session override), `OPENAI_BASE_URL`, `OPENAI_BASE_PATH` (default `v1/chat/completions`), `OPENAI_API_KEY` (secret), `OPENAI_CUSTOM_HEADERS` (secret, comma `k=v` pairs), `OPENAI_TIMEOUT`, `OPENAI_ORGANIZATION`, `OPENAI_PROJECT`. This is a **single** built-in provider; multiple OpenAI-compatible endpoints must be custom JSON files.

### `GOOSE_PROVIDER__*`

Documented in `documentation/docs/guides/environment-variables.md` (`GOOSE_PROVIDER__TYPE`, `GOOSE_PROVIDER__HOST`, `GOOSE_PROVIDER__API_KEY`) and desktop bundling README. **No Rust `rg` hits** in `crates/` at 1.49.0. Treat as **legacy/docs-only**, not a write target. Do not emit these.

### Fixed bundled declarative providers

Shipped JSON under `crates/goose-providers/src/declarative/definitions/` (aimlapi, alibaba, cerebras, deepseek, groq, minimax, moonshot, ollama_cloud, together, zai, zhipu, …). Same schema. Example zai uses `engine: "anthropic"` and `base_url: "${ZAI_BASE_URL}"` with `env_vars`. Custom files override by `name` via `custom_providers/<name>.json` first in `load_provider`.

## 4. Env handling

| Surface | Interpolation / indirection |
|---|---|
| YAML param keys | Exact env override: `OPENAI_HOST` in env beats YAML `OPENAI_HOST`. Snake keys documented as converting to UPPERCASE. |
| Provider `api_key_env` | Name of env/secret. Value is never stored in the JSON. |
| Custom `base_url` | `${VAR}` expanded **only** for names listed in that provider's `env_vars` (`expand_env_vars`). Unknown placeholders left as-is. |
| Custom `headers` | Literal strings. No `${}` expansion in `resolve_config`. |
| Extension `envs` | Literal map `key: value` (alias `env`). Disallowed keys (PATH, LD_PRELOAD, …) skipped with warning. |
| Extension `env_keys` | List of **names**; values resolved from env then secret store (`merge_environments`). |
| Extension `uri`, `headers` values, `cwd`, `socket`, `client_id` | `$VAR` and `${VAR}` substituted from the **merged** env map (not arbitrary process env). Regex: `\$\{\s*([A-Za-z_][A-Za-z0-9_]*)\s*\}` and `\$([A-Za-z_][A-Za-z0-9_]*)`. **stdio `cmd`/`args` are not substituted.** |
| YAML API keys | Not read. |

There is no generic `${VAR}` interpolation across `config.yaml`.

## 5. Defaults (active provider / model)

Resolution (`crates/goose/src/config/providers.rs`):

**Provider** (`get_active_provider`):

1. env `GOOSE_PROVIDER`
2. YAML `active_provider`
3. YAML `GOOSE_PROVIDER` (legacy)

**Model** (`get_active_model`):

1. env `GOOSE_MODEL`
2. `providers.<active>.model` if non-empty
3. YAML `GOOSE_MODEL`

`set_active_provider` writes `active_provider` plus `providers.<name> = {enabled: true, model, configured: true}`.

Recipe override (`documentation/docs/guides/recipes/recipe-reference.md`):

```yaml
settings:
  goose_provider: "anthropic"
  goose_model: "claude-sonnet-4-20250514"
```

IR `defaults.model` is `provider/model`. Goose stores **two fields**, not a slash-joined string. Split on first `/`.

Planner: `GOOSE_PLANNER_PROVIDER` / `GOOSE_PLANNER_MODEL` (out of IR scope unless we later map it).

## 6. MCP (extensions)

MCP servers are YAML under root key `extensions` (not `mcp`). Outer wrapper `ExtensionEntry { enabled, #[serde(flatten)] config }`. Inner tagged enum `#[serde(tag = "type")]` (`crates/goose/src/agents/extension.rs`).

Supported types: `stdio`, `builtin`, `platform`, `streamable_http`. **`sse` is not a variant.** Presence of `type: sse` emits a warning: `'<key>': SSE is unsupported, migrate to streamable_http` and the entry is skipped as malformed.

### stdio (verbatim keys)

```yaml
extensions:
  filesystem:
    enabled: true
    type: stdio
    name: filesystem
    description: ""
    cmd: npx
    args: ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
    envs: {}           # alias: env
    env_keys: []
    timeout: 300       # seconds
    cwd: null
    bundled: false
    available_tools: []
```

### streamable HTTP (verbatim keys)

```yaml
extensions:
  remote-tools:
    enabled: true
    type: streamable_http
    name: remote-tools
    description: ""
    uri: "https://example.com/mcp"
    envs: {}
    env_keys: []
    headers: {}
    timeout: 300
    socket: null
    client_id: null          # $VAR/${VAR} ok
    client_secret_key: null  # name of secret, never inline
    scopes: []
    bundled: false
    available_tools: []
```

There is **no** `type: http`. Remote MCP is `streamable_http` with `uri` (not `url`). Timeout is **seconds**, default 300 for builtins.

Map key in `extensions` is `name_to_key(name)`: alphanumerics/`_`/`-` kept, whitespace dropped, other chars → `_`, lowercased. Missing `name` is injected from the map key.

## 7. IR mapping

Emit two artifacts:

1. `~/.config/goose/config.yaml` fragment: `active_provider`, `providers`, `extensions`, optional `GOOSE_PROVIDER`/`GOOSE_MODEL` for legacy readers.
2. One JSON per custom provider: `~/.config/goose/custom_providers/<id>.json`.

### providers[]

| IR | Goose | Notes |
|---|---|---|
| `id` | JSON `name`; YAML `providers.<id>` | Prefer IR id as filename/name. Built-in names (`openai`, `anthropic`, …) collide with bundled providers — if IR id is a built-in, either reuse that built-in (host/key env) or reject. Custom ids historically prefixed `custom_`. |
| `name` | `display_name` | |
| `protocol: openai-completions` | `engine: openai` + `base_path: "v1/chat/completions"` (or omit and let derive from `base_url`) | Completions-only gateways **must** set a chat-completions `base_path` so gpt-5* names are not auto-routed to Responses. |
| `protocol: openai-responses` | `engine: openai` + `base_path: "v1/responses"` | Also works if `base_url` path contains `/responses`. |
| `protocol: anthropic-messages` | `engine: anthropic` | |
| `base_url` | JSON `base_url` | Scheme required at parse (`ensure_url_scheme`). Path is used to derive `base_path` unless `base_path` set. |
| `api_key_env` | JSON `api_key_env` | Name only. Also `requires_auth: true` if set, `false` if empty. Do **not** write the value into YAML. |
| `headers[].value` | JSON `headers` literal | |
| `headers[].from_env` | **Not representable in JSON headers** | JSON headers are literals. Options: (a) reject; (b) emit `env_vars` + only if we also template `base_url` (headers themselves are not expanded). **Must reject** rather than write `$VAR` hoping for expansion. |
| `headers[].bearer_from_env` | **Not representable** as env-indirect header | Built-in OpenAI auth is Bearer from `api_key_env`. Extra `Authorization: Bearer ${X}` will not expand. Reject, or if `bearer_from_env == api_key_env` drop as redundant (auth header is added by the client). |

### models[]

| IR | Goose |
|---|---|
| `id` / `name` | `models[].name` ← IR `id` (API id). IR `name` has no separate field; put human title in provider `display_name` only. |
| `context_window` | `context_limit` |
| `max_output_tokens` | **Not on ModelInfo.** Global `GOOSE_MAX_TOKENS` is process-wide. If any model sets this, **reject** (cannot express per-model; do not silently apply globally). |
| `reasoning` | `reasoning` bool |
| `input` / `output` modalities | **Not representable.** Reject if any non-text modality is declared (or if we later decide "informational only" — current agentcfg rule: never silently drop). |
| `tool_calling` | **Not representable.** Reject if `false` (cannot disable). `true`/unset OK. |

### defaults.model

Split `"provider/model"` → YAML:

```yaml
active_provider: <provider>
providers:
  <provider>:
    enabled: true
    model: <model>
    configured: true
GOOSE_PROVIDER: <provider>   # optional legacy
GOOSE_MODEL: <model>
```

Reject if not `provider/model` or if provider/model were not emitted.

### mcp[]

| IR | Goose |
|---|---|
| `id` | map key + `name` |
| `enabled` | `enabled` |
| `transport: stdio` | `type: stdio`; `cmd` = `command[0]`; `args` = rest. Reject empty command. |
| `transport: http` | `type: streamable_http`; `uri` = `url`. There is no SSE/plain-HTTP type. |
| `env` value | `envs` literal |
| `env` from_env | `env_keys: [NAME]` (do not put a placeholder in `envs`) |
| `env` bearer_from_env | **Reject** (env values are not Authorization headers) |
| `headers` value | `headers` literal |
| `headers` from_env | `headers: { K: "${NAME}" }` **and** `env_keys: [NAME]` so substitution can see it |
| `headers` bearer_from_env | `headers: { Authorization: "Bearer ${NAME}" }` + `env_keys` |
| `cwd` | `cwd` (stdio only; `${}` expanded from merged envs) |
| `timeout_ms` | `timeout` seconds = `timeout_ms / 1000` (integer). Reject if `0 < timeout_ms < 1000` if we refuse to round; otherwise document truncation. Prefer reject on non-multiples of 1000 to avoid silent precision loss. |

### Must reject (never silently drop)

1. `protocol` outside `{openai-completions, openai-responses, anthropic-messages}` — no ollama-only IR protocol; if we need ollama, that is a new IR protocol.
2. Provider header `from_env` / `bearer_from_env` (except redundant Bearer == `api_key_env`).
3. Per-model `max_output_tokens`.
4. Non-text `input`/`output` modalities.
5. `tool_calling: false`.
6. MCP `env` `bearer_from_env`.
7. Writing API key material into `config.yaml`.
8. `type: sse` / non-streamable HTTP MCP.
9. Collision: IR provider `id` equal to a bundled declarative name (`groq`, `deepseek`, …) if we would overwrite shipped JSON; custom files with the same `name` shadow bundled ones.
10. Mixing completions and responses models in one IR provider without a single `base_path` that is valid for all of them — split into two goose providers or reject.

### Representable extras we do not have in IR

`auth.command` refreshable creds, `dynamic_models`, `preserves_thinking`, `timeout_seconds`, `supports_streaming`, OAuth `client_id`/`scopes`/`socket` on MCP, `available_tools` filter, `builtin`/`platform` extensions. Do not invent them from IR.

## 8. Evidence log

| What | Where / result |
|---|---|
| Date | 2026-09-07 |
| GitHub API | `GET https://api.github.com/repos/block/goose` → 301 `repositories/846698999` = `aaif-goose/goose`. Latest release `v1.49.0` @ 2026-09-03T19:34:26Z. |
| Clone | `git clone --depth 1 https://github.com/block/goose /tmp/goose-src`. HEAD `5e90925962f05acf8e255032de44d16c4a7768a2` 2026-09-05T02:25:17Z. `git remote` still `block/goose`. Workspace version `1.49.0`. |
| Homebrew | `brew info block-goose-cli` → 1.49.0, not installed, conflicts with `goose`. `brew info --cask block-goose` → 1.49.0 **installed** 2026-09-04. Goose.app version 1.49.0. |
| Local CLI | `which goose` / `goose --version` → not found. |
| Local config | `ls ~/.config/goose` → `config.yaml`, `custom_providers/custom_newapi.json`. No `~/.goose`. Live YAML has `GOOSE_PROVIDER: custom_newapi`, `GOOSE_MODEL: CP-qwen3-max`, `OPENAI_HOST`, `OPENAI_BASE_PATH: v1/chat/completions`. JSON matches `DeclarativeProviderConfig`. |
| Schema sources | `crates/goose/src/config/{base,paths,providers,extensions,declarative_providers}.rs`; `crates/goose/src/agents/{extension,extension_manager}.rs`; `crates/goose-providers/src/{declarative,openai}.rs`; `crates/goose-provider-types/src/{base.rs,formats/openai.rs}`; docs `documentation/docs/guides/{config-files,environment-variables}.md`, `documentation/docs/getting-started/{installation,providers,using-extensions}.md`. |
| `GOOSE_PROVIDER__*` | Documented in env-vars.md + `ui/desktop/README.md`; **zero** matches in `crates/**/*.rs` at this commit. |

Docs site used in-tree (same commit), not a separately versioned published site.
