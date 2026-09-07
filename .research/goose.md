# Goose (Block) — native config contract (agentcfg target)

Research date: **2026-09-07**. Primary sources: official repo clone `/tmp/goose-src` (`block/goose`), local `~/.config/goose/config.yaml` + `custom_providers/custom_newapi.json`. Homebrew formula `goose` is **pressly/goose** (Go DB migrations) — decoy; Block Goose is `block-goose-cli`.

## Identity

- Official: Block Goose coding agent, repo `github.com/block/goose`.
- Config root: `~/.config/goose` (`Paths::config_dir()`). Custom providers: `~/.config/goose/custom_providers/<id>.json`.
- Local config present; `GOOSE_PROVIDER: custom_newapi`, `GOOSE_MODEL: CP-qwen3-max`.

## Custom providers (`DeclarativeProviderConfig`)

Verified in `crates/goose-providers/src/declarative.rs`:

- `name`, `engine` (`openai` / `anthropic` / `ollama`; aliases `openai_compatible` / `anthropic_compatible` / `ollama_compatible`), `display_name`, `description`, `api_key_env`, `base_url`, `models[]`, `headers` (`HashMap<String,String>`), `timeout_seconds`, `supports_streaming`, `requires_auth`, `base_path`, `auth` (command-based, mutually exclusive with `api_key_env`), `dynamic_models`.
- Models (`ModelInfo` in `crates/goose-provider-types/src/base.rs`): `name`, `context_limit`, `reasoning`, costs, `supports_cache_control`.
- Chat vs Responses is **not** an engine: OpenAI custom providers pick it via `base_path` (`v1/chat/completions` vs `v1/responses`) in `crates/goose-providers/src/openai.rs` (`OPEN_AI_DEFAULT_BASE_PATH = "v1/chat/completions"`, `should_use_responses_api`).
- Provider headers are literal strings (inserted as `HeaderValue::from_str`); no `${VAR}` interpolation.

## Defaults

`config.yaml` keys `GOOSE_PROVIDER` and `GOOSE_MODEL`. IR `defaults.model "provider/model"` maps 1:1.

## MCP (`extensions` in config.yaml)

Verified in `crates/goose/src/agents/extension.rs`:

- stdio: `type: stdio`, `cmd`, `args`, `envs` / `env` alias, `env_keys`, `timeout` (seconds), `cwd`.
- http: `type: streamable_http`, `uri`, `headers`, `envs`/`env_keys`, `timeout`. `${VAR}` / `$VAR` substitution via `substitute_env_vars` on `uri`, `headers`, `cwd`, `client_id`.
- `env_keys` are secret names resolved from the Goose config/secret store (`merge_environments`).
- Builtin / platform extensions exist but are out of IR scope.

## IR mapping (emitter policy)

- `openai-completions` → `engine: openai` (default chat completions path).
- `anthropic-messages` → `engine: anthropic`.
- `openai-responses` **rejected** (would require `base_path: v1/responses`; IR has no base_path field, and silently emitting chat would drop the protocol).
- `api_key_env` native. Provider `from_env`/`bearer_from_env` headers rejected (literal-only).
- MCP stdio `from_env` → `env_keys`; http `from_env`/`bearer_from_env` → `${VAR}` / `Bearer ${VAR}` in headers.
- `timeout_ms` → seconds (`/ 1000`).
