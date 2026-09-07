# Goose

- **Target id:** `goose`
- **Verified against:** official repo `block/goose` (local clone `/tmp/goose-src`; research notes 2026-09-07). Local config at `~/.config/goose/config.yaml` plus `~/.config/goose/custom_providers/<id>.json`. Homebrew formula `goose` is **pressly/goose** (Go DB migrations) — decoy; Block Goose is `block-goose-cli`.
- **Native files:** `~/.config/goose/custom_providers/<id>.json` (one file per custom provider) and `~/.config/goose/config.yaml` (`GOOSE_PROVIDER` / `GOOSE_MODEL` plus `extensions`).
- **v1 artifacts:** one JSON file per IR provider plus a YAML fragment. v1 does not edit these files.

## Provider route

Custom providers are declarative JSON files (`DeclarativeProviderConfig`):

```json
{
  "name": "volcengine",
  "engine": "openai",
  "display_name": "Volcengine",
  "api_key_env": "VOLC_API_KEY",
  "base_url": "https://example.com/v1",
  "models": [{"name": "glm-5.3", "context_limit": 128000, "reasoning": true}],
  "headers": {"X-Tenant": "engineering"},
  "supports_streaming": true,
  "requires_auth": true,
  "dynamic_models": false
}
```

- `engine` is `openai` / `anthropic` / `ollama` (`openai_compatible` / `anthropic_compatible` aliases). IR `openai-completions` → `openai`; `anthropic-messages` → `anthropic`.
- Chat vs Responses is **not** an engine: OpenAI custom providers pick it via `base_path` (`v1/chat/completions` vs `v1/responses`). IR `openai-responses` is therefore rejected rather than silently emitting the wrong path.
- `api_key_env` is native; IR `api_key_env` maps directly. `requires_auth` is true when an env key is present.
- Provider `headers` are literal strings with no interpolation; IR `from_env` / `bearer_from_env` provider headers are rejected. Constant `value` headers are emitted.
- Models are `{name, context_limit, reasoning, ...}`. IR model `id` maps to `name`; `context_window` → `context_limit`; `reasoning` → `reasoning`. IR `max_output_tokens`, modalities, and `tool_calling` have no Goose model field and are not emitted. `dynamic_models: false` so Goose uses the static list (construction fails if models is empty).

## Defaults

`defaults.model "provider/model"` maps to `GOOSE_PROVIDER` + `GOOSE_MODEL` keys in `config.yaml`.

## MCP

Goose MCP servers are `extensions.<id>` in `config.yaml`:

- stdio: `type: stdio`, `cmd`, `args`, `envs` (literal map), `env_keys` (secret names resolved from the Goose secret store / process env), `cwd`, `timeout` (seconds). IR `from_env` MCP env values become `env_keys` entries; literal `value` env values become `envs`.
- http: `type: streamable_http`, `uri`, `headers`. `${VAR}` / `$VAR` substitution is native on `uri` and header values, so IR `from_env` renders as `${VAR}` and `Authorization.bearer_from_env` as `Bearer ${VAR}`.
- `enabled` and `timeout` (`timeout_ms / 1000`) map 1:1. Builtin / platform extensions are not in IR scope and are not emitted.
