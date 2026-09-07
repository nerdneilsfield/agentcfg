# Crush

- **Target id:** `crush`
- **Verified against:** Charmbracelet Crush v0.92.0 (`github.com/charmbracelet/crush`; live schema `https://charm.land/crush.json`, fetched 2026-09-07). Local install via `charmbracelet/tap/crush`; config at `~/.config/crush/crush.json`.
- **Native file:** `~/.config/crush/crush.json` (JSON; `$schema` optional).
- **v1 artifact:** a single JSON fragment. v1 does not edit this file.

## Provider route

Custom providers live under `providers.<id>` (`ProviderConfig`, `additionalProperties: false`):

```json
{
  "providers": {
    "volcengine": {
      "id": "volcengine",
      "name": "Volcengine",
      "base_url": "https://example.com/v1",
      "type": "openai-compat",
      "api_key": "$VOLC_API_KEY",
      "extra_headers": {"X-Gateway-Key": "$GATEWAY_KEY"},
      "models": [{
        "id": "glm-5.3",
        "name": "GLM-5.3",
        "cost_per_1m_in": 0,
        "cost_per_1m_out": 0,
        "cost_per_1m_in_cached": 0,
        "cost_per_1m_out_cached": 0,
        "context_window": 128000,
        "default_max_tokens": 8192,
        "can_reason": true,
        "supports_attachments": true
      }]
    }
  }
}
```

- `type` enum includes `openai` (Responses API), `openai-compat` (Chat Completions), `anthropic`, and several hosted backends. IR `openai-completions` maps to `openai-compat`; `openai-responses` maps to `openai`; `anthropic-messages` maps to `anthropic`. Crush `type=openai` always calls the Responses API, so a completions-only gateway should use `openai-completions` instead.
- `api_key` is a string whose documented example is `"$OPENAI_API_KEY"`; IR `api_key_env` maps to `"$VAR"` and no secret is rendered.
- `extra_headers` is a flat `Record<string,string>`. IR `from_env` renders as `"$VAR"`; `Authorization.bearer_from_env` as `"Bearer ${VAR}"`. `bearer_from_env` on any other header name is rejected. Explicit model lists set `discover_models: false` so Catwalk cannot merge unexpected models.
- Model schema **requires** `id`, `name`, `context_window`, `default_max_tokens`, `can_reason`, `supports_attachments`, and four cost fields. The emitter writes `cost_per_1m_*` as `0` (IR has no cost fields). Missing `context_window` or `max_output_tokens` is rejected. `can_reason` comes from IR `reasoning`; `supports_attachments` is true when IR input includes `image`. IR `tool_calling` and output modalities have no Crush field and are not emitted.

## Defaults

`defaults.model "provider/model"` maps to both `models.large` and `models.small` (`SelectedModel` requires `provider` + `model`). Crush has no single default; large/small are the documented pair.

## MCP

`mcp.<id>` (`MCPConfig`) requires `type` in `stdio|sse|http`. IR `http` maps to `http` (not `sse`). stdio uses `command`/`args`/`env`; http uses `url`/`headers`. `enabled: false` maps to `disabled: true`. `timeout` is whole seconds (`timeout_ms / 1000`); values not divisible by 1000 are rejected. IR `cwd` has no Crush MCP field and is rejected. Env/header interpolation uses the same `$VAR` / `Bearer ${VAR}` rendering as providers. When the IR has no MCP servers, the `mcp` object is omitted.
