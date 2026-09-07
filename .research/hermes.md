# Research Report: "hermes" — Hermes Agent (Nous Research)

**Date:** 2026-09-07 (UTC)
**Researcher:** subagent `research-hermes` for agentcfg
**Verdict:** An official coding-agent CLI named **Hermes Agent** exists, published by **Nous Research**. It is the only credible match for "hermes" in a coding-agent CLI list. Full config contract verified against the official GitHub repo source (loader/normalizer code, not just README).

---

## 1. Identity & disambiguation

| Candidate | What it is | Verdict |
|---|---|---|
| **NousResearch/hermes-agent** | Official open-source coding-agent CLI from Nous Research ("The agent that grows with you" / "self-improving AI agent"). Python, `uv`-installed, `hermes` CLI entry point. | **TARGET** |
| Nous Research "Hermes" LLMs | Hermes 2/3/4 model family by the same org. Models, not a CLI — but the same org ships the agent CLI above, and the CLI can be pointed at Hermes models via OpenRouter/Nous Portal. | Related, not the CLI |
| npm `hermes` v0.4.4 | Segment's 2014 HipChat/Slack chat bot (`github.com/segmentio/hermes`). Dead, unrelated. | Rejected |
| npm `hermes-agent` v0.21.0 | Third-party "unofficial npm bridge" (`github.com/wyrtensi/hermes-agent-npm`, 1 star, created 2026-05-25) that wraps the PyPI package. Not official. | Wrapper, not authoritative |
| PyPI `hermes` v0.10.0 | "Workflow to publish research software with rich metadata" — unrelated. | Rejected |
| PyPI `hermes-cli` v0.0.1.4 | Random 4-commit repo. | Rejected |
| Facebook Hermes JS engine / `hermes-parser` | JS engine, unrelated. | Rejected |

Evidence for the official identity:
- GitHub org `NousResearch`, repo `NousResearch/hermes-agent`, ~242.9k stars, description "The agent that grows with you": https://github.com/NousResearch/hermes-agent (fetched via GitHub API, 2026-09-07).
- Official docs site, live, title "Hermes Agent — Open-Source AI Agent That Grows With You | Nous Research": https://hermes-agent.nousresearch.com (HTTP 200, fetched 2026-09-07).
- Homebrew **core** formula `hermes-agent` (stable 2026.8.31): `brew info hermes-agent` → homepage https://hermes-agent.nousresearch.com, license MIT, `install: 3,897 (30 days)`. Cask `hermes-desktop` 0.21.0. Homebrew-core inclusion + NousResearch org + nousresearch.com domain = official.
- Entry points in `pyproject.toml`: `hermes = "hermes_cli.main:main"`, `hermes-agent = "run_agent:main"`, `hermes-acp = "acp_adapter.entry:main"`.

Versions observed (all 2026-09-07):
- Repo `main` HEAD cloned: commit `d9833c5615b80e199a174cd67d90ab430695a972`, dated 2026-09-07; `pyproject.toml` version **0.21.0**.
- Homebrew formula `hermes-agent` stable **2026.8.31** (bottled).
- PyPI `hermes-agent` latest **0.19.0** (https://pypi.org/pypi/hermes-agent/json).
- npm `hermes-agent` (unofficial bridge) **0.21.0**, created 2026-05-25.
- Note: PyPI lags the repo/brew; brew tracks the repo closely.

Install channels: `brew install hermes-agent`, `uv tool install hermes-agent` (PyPI), `curl -fsSL https://raw.githubusercontent.com/NousResearch/hermes-agent/main/setup-hermes.sh`, Docker, Nix flake. Not installed on this machine (`which hermes` → not found; no `~/.hermes`).

---

## 2. Config file paths (verified in source)

Resolution order for the home dir: context-local override → `$HERMES_HOME` env var → platform default.

- **macOS/Linux home:** `~/.hermes` (`hermes_constants.py::_get_platform_default_hermes_home`).
- **Windows home:** `%LOCALAPPDATA%\hermes`.
- **Main config:** `$HERMES_HOME/config.yaml` (`hermes_constants.py::get_config_path`).
- **Secrets env file:** `$HERMES_HOME/.env` (`get_env_path`); loaded by `hermes_cli/env_loader.py::load_hermes_dotenv`. Docs: "Secrets (API keys, bot tokens, passwords) go in `.env`"; `hermes config set OPENROUTER_API_KEY sk-...` auto-routes to `.env`.
- **Profiles:** `~/.hermes/profiles/<name>/` each with own `config.yaml`/`.env`; active profile in `~/.hermes/active_profile`.
- Other state: `state.db`, `auth.json` (OAuth creds), `SOUL.md`, `memories/`, `skills/`, `mcp-tokens/<server>.json`, `logs/mcp-stderr.log`.
- Official doc `website/docs/user-guide/configuration.md` confirms `~/.hermes/config.yaml` + `~/.hermes/.env` as the two files that matter.
- Config can also be managed via `hermes config get/set/unset/edit/check/migrate`.

Config is YAML; env-var interpolation `${VAR}` and `${env:VAR}` is applied **recursively to all string values at load time** (`hermes_cli/config.py::_expand_env_vars`, regex `\$\{([^}]+)\}`). Resolution: `${env:NAME}` = explicit env ref; bare `${NAME}` = legacy env ref; other `${source:...}` SecretRef shapes (e.g. `bitwarden:`) are NOT resolved in config.yaml (external secret sources inject env vars at startup; docs tell you to use `${env:NAME}`). Unset vars keep the literal placeholder (with a warning for `env:` refs). Per-profile secret scoping exists for multiplexed gateways. MCP entries additionally get Cursor-style context vars (`${userHome}`, `${workspaceFolder}`, `${workspaceFolderBasename}`, `${pathSeparator}`, `${/}`) substituted at connect time (`tools/mcp_tool_config.py::_interpolate_env_vars`).

---

## 3. Custom providers (models) — VERIFIED from loader source

Two config locations (deduped list view in `hermes_cli/config_providers.py::get_compatible_custom_providers`):
1. **Current (config v12+):** top-level `providers:` mapping — `providers.<id>:`.
2. **Legacy:** top-level `custom_providers:` list (auto-migrated to `providers:` on `hermes update`).

Normalization is done by `_normalize_custom_provider_entry` in `hermes_cli/config_providers.py` (read directly). Accepted keys per entry (`_KNOWN_PROVIDER_KEYS`):

```yaml
providers:
  <id>:                       # mapping key = provider id (stringified)
    name: <str>               # display name; defaults to the mapping key
    base_url: <url>           # aliases accepted: url, api
    api_key: <str>            # inline key (secret; hermes never logs it)
    key_env: <ENV_VAR>        # env var holding the key; alias api_key_env (also camelCase apiKeyEnv)
    key_cmd: "<shell cmd>"    # command printing a token (bare or JSON access_token/expires_in); runs per request, cached
    api_mode: <mode>          # alias: transport; see protocol enum below
    model: <str>              # default model for this provider; alias default_model
    models: { <model_id>: {...} }   # or list of ids, or [{id: ..., ...}]
    models_discovered: <bool> # marks models map as auto-discovered
    context_length: <int>     # total context window (input+output)
    rate_limit_delay: <num>
    request_timeout_seconds: <int>
    stale_timeout_seconds: <int>
    discover_models: <bool>   # default true; probes /v1/models
    extra_body: { ... }       # provider-specific request fields
    extra_headers: { <header>: <value> }  # per-provider HTTP headers; never logged
    capabilities: { <capability>: <bool> }  # + per-model capabilities under models.<id>.capabilities
    ssl_ca_cert: <path>
    ssl_verify: <bool|path>
    enabled: <bool>           # default true; explicit false hides from picker/runtime
  # camelCase aliases auto-mapped (with warning): apiKey, baseUrl, apiMode, keyEnv, apiKeyEnv,
  #   defaultModel, contextLength, rateLimitDelay
```

- **Protocol (`api_mode`) canonical values** (from `_canonical_api_mode` + docs): `chat_completions`, `codex_responses`, `anthropic_messages`, `bedrock_converse`. Aliases: `openai`, `openai_chat`, `chat-completions` → `chat_completions`; `responses`, `openai_responses` → `codex_responses`; `anthropic`, `messages` → `anthropic_messages`; `bedrock` → `bedrock_converse`; docs also show `openai_chat`. So: **openai-completions ↔ chat_completions, openai-responses ↔ codex_responses, anthropic-messages ↔ anthropic_messages** — all three IR protocols are supported. Unknown api_mode falls back to hostname-based guessing (this is why the alias map exists).
- **api key via env:** `key_env: VAR` reads the var (from `~/.hermes/.env` or process env) at client build / rotation time (`agent/client_lifecycle.py`, `hermes_cli/runtime_provider_custom.py`). `api_key: ${VAR}` interpolation also works. `key_cmd` is the enterprise short-lived-token path.
- **Headers:** per-provider `extra_headers` is a flat `dict[str,str]` merged onto SDK default_headers (entry wins). `${VAR}`/`${env:VAR}` interpolation at load means env-referenced header values work (docs example: `CF-Access-Client-Secret: "${CF_ACCESS_SECRET}"`). No dedicated bearer/env/bearer_from_env struct — compose `"Authorization": "Bearer ${VAR}"` as a plain string.
- **Per-model data** under `providers.<id>.models.<model_id>`: free-form mapping consumed keys include `context_length` (int), `capabilities` (bool map), and timeout overrides (`timeout_seconds` per model via `providers.<name>.models.<model>.timeout_seconds`); the normalizer keeps all other keys too. Known capability booleans: `prompt_caching` (agent_runtime_helpers.py), `openai_native_compaction` / `native_compaction` (native_compaction.py), `openai_native_compaction` shown in docs. **No max_output_tokens / modality / reasoning fields are read from config** — output caps are provider-owned ("Output-token limits are provider-owned, not user configuration", cli-config.yaml.example); context window auto-detected from `/v1/models` metadata otherwise.
- Selecting a custom provider: `model.provider: <id>` (or `custom` with `model.base_url`; `ollama`/`vllm`/`llamacpp` alias to `custom`), `model.default: <model_id>`; CLI flags `--provider`, `--model`; `/model` picker. **`provider/model` strings ARE accepted** in `model.default` when the prefix matches a configured provider (`hermes_cli/model_switch.py::_resolve_aliases_and_provider_prefix`, `val.split("/", 1)`) — so the IR `defaults.model = "provider/model"` maps directly.
- Built-in first-class providers (env-var keys hardcoded, from `model.provider` docs in `cli-config.yaml.example`): openrouter (`OPENROUTER_API_KEY` or `OPENAI_API_KEY`), nous (OAuth), nous-api (`NOUS_API_KEY`), anthropic (`ANTHROPIC_API_KEY`), openai-codex (OAuth), copilot (`GITHUB_TOKEN`), gemini (`GOOGLE_API_KEY`/`GEMINI_API_KEY`), zai (`GLM_API_KEY`), kimi-coding (`KIMI_API_KEY`), minimax (`MINIMAX_API_KEY`), minimax-cn (`MINIMAX_CN_API_KEY`), huggingface (`HF_TOKEN`), nvidia (`NVIDIA_API_KEY`), xiaomi (`XIAOMI_API_KEY`), arcee (`ARCEEAI_API_KEY`), ollama-cloud (`OLLAMA_API_KEY`), deepinfra (`DEEPINFRA_API_KEY`), kilocode (`KILOCODE_API_KEY`), ai-gateway (`AI_GATEWAY_API_KEY`), azure-foundry (key or Entra ID), lmstudio (`LM_API_KEY` optional, default `http://127.0.0.1:1234/v1`), custom (any OpenAI-compatible; env fallbacks `OPENAI_BASE_URL`/`OPENAI_API_KEY`, `OPENROUTER_BASE_URL` — `hermes_cli/runtime_provider.py:307,474`).

Top-level `model:` section keys (from example + docs): `default` (alias key `model`), `provider`, `api_key`, `base_url`, `api_mode` (docs show dashboard writing `api_mode: chat_completions` here), `streaming`, `entra:`, plus `provider_routing:` (OpenRouter), `openrouter:` cache, `fallback_providers` chain, `auxiliary:` per-task model overrides.

---

## 4. MCP — VERIFIED from loader source + reference docs

Hermes is an MCP **client** (`mcp_servers` in config.yaml; it can also *serve* its tools via `mcp_serve.py`, out of scope). Root shape (`website/docs/reference/mcp-config-reference.md`, matching `tools/mcp_tool_server_run.py::_prepare_run`):

```yaml
mcp_servers:
  <name>:
    # stdio:
    command: "npx"            # required for stdio
    args: ["-y", "pkg"]       # list
    env: { KEY: value }       # merged into a FILTERED child env (safe baseline + XDG_* + env)
    # http (streamable HTTP; 'url' wins if both url and command present):
    url: "https://.../mcp"
    headers: { Authorization: "Bearer ${TOKEN}" }   # flat str map; ${VAR} interpolated at connect
    transport: "sse"          # optional; otherwise Streamable HTTP
    ssl_verify: true | false | <ca-bundle-path>
    client_cert: <pem> | [cert, key] | [cert, key, password]
    client_key: <pem>
    identity_header: { name: X-User-Id, value_from: static|profile, value: ... }
    auth: "oauth"             # OAuth 2.1 w/ PKCE, DCR/CIMD; tokens at ~/.hermes/mcp-tokens/<server>.json
    oauth: { client_id, client_secret, redirect_uri, redirect_port, redirect_host, client_name }
    enabled: <bool>           # false = skip entirely
    timeout: 120              # tool call timeout (default 300 per reference; _resolve_tool_timeout)
    connect_timeout: 60
    keepalive_interval: 180
    idle_timeout_seconds: 0   # stdio recycle
    max_lifetime_seconds: 0
    supports_parallel_tool_calls: <bool>
    skip_preflight: <bool>
    protocol: auto | stateless | legacy
    tools: { include: [], exclude: [], resources: true, prompts: true }
    trust: full | untrusted
    sampling: { enabled, model, max_tokens_cap, timeout, max_rpm, allowed_models, max_tool_rounds, log_level }
    elicitation: { enabled, timeout }
    cwd: <path>               # stdio working directory (passed to StdioServerParameters)
```

- Transports: **stdio** (`command`/`args`/`env`/`cwd`) and **HTTP** streamable (`url`/`headers`), plus legacy **SSE** via `transport: sse`. Protocol-era negotiation `auto|stateless|legacy` (2025-03-26 handshake vs 2026-07-28 stateless).
- **Env refs:** `env` values and `headers` values get `${VAR}`/`${env:VAR}` interpolation at connect time from env + `~/.hermes/.env`; unset vars keep the literal placeholder. stdio child env is deliberately filtered (no host secrets leak unless declared in the server's `env` or injected via external secret source).
- **Bearer token env fields:** no dedicated field. The supported pattern is a header: `headers: { Authorization: "Bearer ${MY_TOKEN}" }` (docs literally show `Authorization: "Bearer ***"`). OAuth servers get `Authorization: Bearer <token>` injected automatically from stored tokens. `identity_header` supports `value_from: static|profile` only (no env option).
- Malware/exfil guard: suspicious entries are dropped before any spawn (`_filter_suspicious_mcp_servers` → `hermes_cli/mcp_security.py::validate_mcp_server_entry`); OSV malware preflight on npx-style commands. There is also an interactive `hermes mcp` catalog/installer UI.
- `hermes import-agent claude-code` migrates Claude Code's `mcpServers` (from `~/.claude.json`) into `mcp_servers` — useful as a schema cross-check (same shape).

---

## 5. Mapping the agentcfg IR → Hermes

Direct mapping (providers):

| IR field | Hermes field | Notes |
|---|---|---|
| `providers[].id` | `providers.<id>` key | |
| `name` | `name` | |
| `protocol: openai-completions` | `api_mode: chat_completions` | aliases `openai`, `openai_chat`, docs' `openai_chat` |
| `protocol: openai-responses` | `api_mode: codex_responses` | alias `responses` |
| `protocol: anthropic-messages` | `api_mode: anthropic_messages` | |
| `base_url` | `base_url` | |
| `api_key_env` | `key_env` | or `api_key: ${VAR}` |
| `headers value` | `extra_headers.<h>: "<literal>"` | |
| `headers from_env` | `extra_headers.<h>: "${VAR}"` | load-time interpolation |
| `headers bearer_from_env` | `extra_headers.Authorization: "Bearer ${VAR}"` | no dedicated field |
| `models[].id` | `models` list/dict entry | |
| `models[].context_window` | `models.<id>.context_length` | |
| `defaults.model "provider/model"` | `model.default: "provider/model"` | split on first `/`, prefix must be a configured provider |

MCP mapping:

| IR field | Hermes field |
|---|---|
| `mcp[].id` | `mcp_servers.<id>` key (no per-server `enabled` needed unless false → `enabled: false`) |
| `transport: stdio` | `command` + `args` + `env` (+ `cwd`) |
| `transport: http` | `url` (+ `headers`, `transport: sse` if SSE) |
| `env map` | `env` (values literal or `${VAR}`) |
| `headers value/from_env` | `headers` flat map, `${VAR}` interpolation |
| `headers bearer_from_env` | `headers.Authorization: "Bearer ${VAR}"` |
| `cwd` | `cwd` (stdio) |
| `timeout_ms` | `timeout` is in **seconds** (default 300) — divide by 1000 |

## 6. What the IR cannot represent (gaps)

1. **`key_cmd`** (command-minted short-lived tokens) — Hermes-only feature; IR has no equivalent.
2. **Per-provider TLS** (`ssl_ca_cert`, `ssl_verify`, mTLS `client_cert`/`client_key` for MCP) — no IR fields.
3. **Capabilities** (`prompt_caching`, `openai_native_compaction` booleans per provider/model) — no IR field.
4. **`extra_body`** (provider-specific request fields, e.g. vLLM `chat_template_kwargs`) — no IR field.
5. **MCP extras:** `auth: oauth` + `oauth:` block, `identity_header`, `sampling`/`elicitation` policy, `trust`, `tools.include/exclude`, `keepalive_interval`, `connect_timeout`, lifecycle recycles, `protocol` negotiation — none in IR.
6. **Discovery flag** (`discover_models: false`, `models_discovered`) — no IR field.
7. **Timeout units:** IR `timeout_ms` vs Hermes seconds.
8. **No `enabled` on providers** in IR (Hermes supports `enabled: false` on `providers.<id>`; MCP `enabled` maps fine).
9. **Model metadata Hermes ignores:** `max_output_tokens` (provider-owned), input/output modalities, reasoning, tool_calling — Hermes auto-detects these from `/v1/models` and error metadata; config only honors `context_length` + boolean capabilities per model. IR fields beyond context window are dropped (acceptable: they're informational elsewhere).
10. Hermes-specific global knobs (`model.provider: auto`, fallback chains, `auxiliary:` task models, OpenRouter `provider_routing`) are out of agentcfg's provider/MCP scope.

## 7. Evidence log

- `https://github.com/NousResearch/hermes-agent` — cloned `main` @ `d9833c56` (2026-09-07) to `/tmp/hermes-agent`; files read: `pyproject.toml`, `cli-config.yaml.example`, `hermes_constants.py`, `hermes_cli/config.py` (env expansion), `hermes_cli/config_providers.py` (provider normalizer, full read), `hermes_cli/runtime_provider.py`, `hermes_cli/runtime_provider_custom.py`, `hermes_cli/model_switch.py`, `agent/client_lifecycle.py`, `agent/agent_runtime_helpers.py`, `agent/native_compaction.py`, `tools/mcp_tool_config.py` (full read), `tools/mcp_tool_server_run.py`, `tools/mcp_tool_transport.py`, `tools/mcp_tool_errors.py`, `website/docs/user-guide/{configuration,configuring-models,features/mcp,which-file-does-what}.md`, `website/docs/reference/mcp-config-reference.md`.
- `https://hermes-agent.nousresearch.com` — HTTP 200, Nous Research branding (2026-09-07).
- GitHub API: org repo listing + `NousResearch/hermes-agent` metadata (242,914 stars, fetched 2026-09-07).
- `brew info hermes-agent` / `brew info hermes-desktop` — core formula stable 2026.8.31, homepage hermes-agent.nousresearch.com (2026-09-07).
- `npm view hermes-agent` — 0.21.0, unofficial bridge by wyrtensi (2026-09-07). `npm view hermes` — Segment 2014 bot.
- PyPI JSON API: `hermes-agent` 0.19.0; `hermes` 0.10.0; `hermes-cli` 0.0.1.4 (2026-09-07).
- Local machine: `which hermes` → not found; no `~/.hermes`, `~/.config/hermes`, `~/.nous` (2026-09-07).
- Note: websearch skill unavailable (no Serper key); discovery done via npm/brew/PyPI/Homebrew-core metadata + GitHub API + direct fetches of official docs/repo. This is sufficient — all claims trace to primary source code/docs.

## 8. Confidence

**HIGH.** Identity is unambiguous (official Nous Research CLI; homebrew-core formula, nousresearch.com domain, NousResearch GitHub org). Every schema claim above was verified in the repo's own loader/normalizer source at a pinned commit, cross-checked against the official docs site and the shipped `cli-config.yaml.example`. Residual uncertainty is version drift only (repo 0.21.0 vs PyPI 0.19.0 vs brew 2026.8.31 — brew/repo agree; the schema shown is from current main).
