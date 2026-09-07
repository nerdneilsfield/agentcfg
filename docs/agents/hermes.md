# Hermes Agent

- **Target id:** `hermes`
- **Verified against:** official repo `NousResearch/hermes-agent` (research report `.research/hermes.md`, fetched 2026-09-07).
- **Native file:** `$HERMES_HOME/config.yaml` (default `~/.hermes/config.yaml`), plus a sibling `.env` for secrets.
- **v1 artifact:** YAML fragment. v1 does not edit this file.

## Provider route

Hermes uses top-level `providers.<id>` tables:

```yaml
model:
  provider: volcengine
  default: glm-5.3
providers:
  volcengine:
    base_url: https://example.com/v1
    api_key_env: VOLC_API_KEY
    api_mode: chat_completions
    extra_headers:
      X-Gateway-Key: ${GATEWAY_KEY}
    models:
      glm-5.3:
        context_length: 128000
```

- `api_mode` is one of `chat_completions`, `codex_responses`, `anthropic_messages`; the three IR protocols map 1:1.
- `api_key_env` (alias `key_env`) is native; IR `api_key_env` maps directly.
- Hermes expands `${VAR}` and `${env:VAR}` recursively over **all** config strings at load, so every IR env-reference form is native: provider `headers.from_env` → `"${VAR}"`, `Authorization.bearer_from_env` → `"Bearer ${VAR}"`.
- `models` is a `<model-id> → {context_length}` mapping. IR `max_output_tokens`, `reasoning`, `tool_calling`, and input/output modalities have no Hermes config field and are not emitted; Hermes auto-detects capabilities from the provider.
- Provider display `name` has no Hermes field and is not emitted.

## Defaults

`defaults.model "provider/model"` splits into `model.provider` +
`model.default`.

## MCP

`mcp_servers.<id>` supports stdio (`command`, `args`, `env`, `cwd`) and http
(`url`, `headers`), with `${VAR}` expansion native at connect time. IR
`timeout_ms` is emitted as `timeout` in seconds (`ms/1000`). IR `enabled`
maps to the native `enabled` flag.
