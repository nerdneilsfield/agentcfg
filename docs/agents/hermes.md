# Hermes Agent

- **Target id:** `hermes`
- **Verified against:** official repo `NousResearch/hermes-agent` (research report `.research/hermes.md`, fetched 2026-09-07).
- **Native file:** `$HERMES_HOME/config.yaml` (default `~/.hermes/config.yaml`), plus a sibling `.env` for secrets.
- **v1 artifact:** YAML fragment. v1 does not edit this file.

## Provider route

An IR literal such as `api_key: "example-key"` is emitted in the native
`api_key` field. The example key is a placeholder; real literals also appear in
generated output.

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

- `api_mode` is one of `chat_completions`, `codex_responses`, `anthropic_messages`; the three IR protocols map 1:1. A provider on any other protocol is skipped with a warning.
- `api_key_env` (alias `key_env`) is native; IR `api_key: "ENV:NAME"` maps directly. A literal `api_key` containing `${` is skipped with a warning, since the value would read as a native environment expression.
- Hermes expands `${VAR}` and `${env:VAR}` recursively over **all** config strings at load, so every IR env-reference form is native: provider header `ENV:NAME` → `"${VAR}"`, `Authorization: "Bearer ENV:NAME"` → `"Bearer ${VAR}"`.
- `models` is a `<model-id> → {context_length}` mapping. IR `max_output_tokens`, `reasoning`, `tool_calling`, and input/output modalities have no Hermes config field and are not emitted; Hermes auto-detects capabilities from the provider.
- Provider display `name` has no Hermes field and is not emitted.

## Defaults

`defaults.model "provider/model"` splits into `model.provider` +
`model.default`. A value that does not resolve to an emitted provider model is
skipped with a warning.

## MCP

`mcp_servers.<id>` supports stdio (`command`, `args`, `env`, `cwd`) and http
(`url`, `headers`), with `${VAR}` expansion native at connect time. A stdio
server with no command is dropped with a warning. IR `timeout_ms` is emitted as
`timeout` in seconds (`ms/1000`); a non-positive value is skipped with a
warning. IR `enabled` maps to the native `enabled` flag.

## Reasoning variants

Hermes exposes one active effort per model through `agent.reasoning_overrides`,
not a selectable effort list. `models[].variants` is omitted rather than being
reduced to a selected override.
