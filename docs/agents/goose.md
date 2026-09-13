# Goose

- **Target id:** `goose`
- **Verified against:** official repo `block/goose` (full research in [`../../.research/goose.md`](../../.research/goose.md), 2026-09-07). Local config at `~/.config/goose/config.yaml` plus `~/.config/goose/custom_providers/<id>.json`. Homebrew formula `goose` is **pressly/goose** (Go DB migrations) — decoy; Block Goose is `block-goose-cli`.
- **Native files:** `~/.config/goose/custom_providers/<id>.json` (one file per custom provider) and `~/.config/goose/config.yaml` (`active_provider` / `providers` plus `extensions`).
- **v1 artifacts:** one JSON file per IR provider plus a YAML fragment. v1 does not edit these files.

## Provider route

Custom providers are declarative JSON files (`DeclarativeProviderConfig` in `crates/goose-providers/src/declarative.rs`):

```json
{
  "name": "volcengine",
  "engine": "openai",
  "display_name": "Volcengine",
  "api_key_env": "VOLC_API_KEY",
  "base_url": "https://example.com/v1",
  "base_path": "v1/responses",
  "models": [{"name": "glm-5.3", "context_limit": 128000, "reasoning": true}],
  "headers": {"X-Tenant": "engineering"},
  "supports_streaming": true,
  "requires_auth": true,
  "dynamic_models": false
}
```

- `engine` is `openai` / `anthropic` / `ollama` (with `*_compatible` aliases). IR `openai-completions` and `openai-responses` → `openai`; `anthropic-messages` → `anthropic`.
- Chat vs Responses is **not** an engine: OpenAI custom providers pick it via `base_path` (`from_declarative_config` in `openai.rs`). IR `openai-responses` emits `base_path: "v1/responses"`; an explicit `base_path` replaces the `base_url` path entirely, so no path doubling. IR `openai-completions` omits `base_path` and lets Goose derive it from `base_url` (`derive_base_path`). Note the Go-side caveat: with the derived default path, Goose routes `o*` / `gpt-5*` model names to the Responses API (`should_use_responses_api`); completions-only gateways serving those names need a custom `base_path` outside this emitter's scope.
- Literal IR API keys are rejected: the verified custom-provider field accepts an environment-variable name, not a literal key.
- `api_key_env` is native; IR `api_key: "ENV:NAME"` maps directly. `requires_auth` is true when an env key is present.
- Provider `headers` are literal strings with no interpolation; IR `ENV:NAME` / `Bearer ENV:NAME` provider headers are rejected. Literal headers are emitted.
- Models are `{name, context_limit, reasoning}`. IR model `id` maps to `name`; `context_window` → `context_limit`; `reasoning` → `reasoning`. `dynamic_models: false` so Goose uses the static list (construction fails if models is empty).
- Rejected as unrepresentable: per-model `max_output_tokens` (goose only has the global `GOOSE_MAX_TOKENS`; never silently apply it globally), any non-text input/output modality, and `tool_calling: false` (goose agents always expose tools).

## Defaults

`defaults.model "provider/model"` maps to the modern keys in `config.yaml` (`set_active_provider` shape):

```yaml
active_provider: volcengine
providers:
  volcengine:
    enabled: true
    model: glm-5.3
    configured: true
```

`GOOSE_PROVIDER` / `GOOSE_MODEL` still resolve but are legacy; agentcfg emits the structured keys.

## MCP

Goose MCP servers are `extensions.<id>` in `config.yaml`:

- stdio: `type: stdio`, `cmd`, `args`, `envs` (literal map), `env_keys` (names resolved from env then the Goose secret store), `cwd`, `timeout` (seconds). IR `ENV:NAME` MCP env values become `env_keys` entries; literal env values become `envs`. `Bearer ENV:NAME` on MCP env is rejected.
- http: `type: streamable_http`, `uri`, `headers`. `$VAR` / `${VAR}` substitution runs on `uri`, header values, `cwd`, and `socket` from the merged env map, so IR `ENV:NAME` renders as `${VAR}` and `Authorization: "Bearer ENV:NAME"` as `Bearer ${VAR}` — and each referenced name is also added to `env_keys` so substitution can resolve it.
- `enabled` maps 1:1; `timeout_ms` must be divisible by 1000 (whole seconds) and is rejected otherwise. Builtin / platform extensions are not in IR scope and are not emitted.

## Reasoning variants

Goose has an active `GOOSE_THINKING_EFFORT` setting and a fixed parser ladder,
but not a model-local effort availability list. `models[].variants` is omitted
rather than selected as a global effort. In particular, it does not apply
Goose's lossy `xhigh` to `max` normalization to the source list.
