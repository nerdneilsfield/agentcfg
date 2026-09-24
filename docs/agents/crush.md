# Crush

- **Target id:** `crush`
- **Verified against:** Charmbracelet Crush v0.92.0 (`github.com/charmbracelet/crush`; live schema `https://charm.land/crush.json`, re-fetched 2026-09-20). Local install via `charmbracelet/tap/crush`; config at `~/.config/crush/crush.json`.
- **Native file:** `~/.config/crush/crush.json` (JSON; `$schema` optional).
- **v1 artifact:** a single JSON fragment. v1 does not edit this file.

## Provider route

An IR literal such as `api_key: "example-key"` is emitted in the native
`api_key` field. The example key is a placeholder; real literals also appear in
generated output.

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

- `type` enum includes `openai` (Responses API), `openai-compat` (Chat Completions), `anthropic`, and several hosted backends. IR `openai-completions` maps to `openai-compat`; `openai-responses` maps to `openai`; `anthropic-messages` maps to `anthropic`. Crush `type=openai` always calls the Responses API, so a completions-only gateway should use `openai-completions` instead. A provider whose IR protocol maps to no Crush type is skipped with a warning and its entry is omitted.
- `api_key` is a string whose documented example is `"$OPENAI_API_KEY"`; IR `api_key: "ENV:NAME"` maps to `"$VAR"` without resolving the reference.
- Crush evaluates shell-style expressions in these native configuration strings.
  IR literals containing `$` or backticks are skipped with a warning rather than
  being evaluated by Crush: a provider API key, URL, or header is omitted, an
  MCP environment value or header is omitted, and an MCP command, argument, or
  URL drops the whole entry.
- `extra_headers` is a flat `Record<string,string>`. IR `ENV:NAME` renders as `"$VAR"`; `Authorization: "Bearer ENV:NAME"` as `"Bearer ${VAR}"`. `Bearer ENV:NAME` on any other header name is skipped with a warning and the header is omitted. Explicit model lists set `discover_models: false` so Catwalk cannot merge unexpected models.
- Model schema **requires** `id`, `name`, `context_window`, `default_max_tokens`, `can_reason`, `supports_attachments`, and four cost fields. The emitter writes `cost_per_1m_*` as `0` (IR has no cost fields). Missing `context_window` or `max_output_tokens` is skipped with a warning and drops the model entry. `can_reason` comes from IR `reasoning`; `supports_attachments` is true when IR input includes `image`. Input is `text` and `image` only; non-text output is skipped with a warning. `tool_calling: false` is skipped with a warning (Crush agents always expose tools); `true` has no native field and is not emitted.

## Defaults

`defaults.model "provider/model"` maps to both `models.large` and `models.small` (`SelectedModel` requires `provider` + `model`). Crush has no single default; large/small are the documented pair. A `defaults.model` that does not resolve to an emitted provider model is skipped with a warning and no `models` block is written.

## MCP

`mcp.<id>` (`MCPConfig`) requires `type` in `stdio|sse|http`. IR `http` maps to `http` (not `sse`). stdio uses `command`/`args`/`env`; http uses `url`/`headers`. `enabled: false` maps to `disabled: true`. `timeout` is whole seconds (`timeout_ms / 1000`, native default 15); a value not divisible by 1000 is skipped with a warning and `timeout` is omitted. IR `cwd` has no Crush MCP field and is skipped with a warning. A stdio server with no `command` is dropped from `mcp`. Env/header interpolation uses the same `$VAR` / `Bearer ${VAR}` rendering as providers. When the IR has no MCP servers, the `mcp` object is omitted.

IR:

```yaml
mcp:
  - id: context7
    transport: stdio
    command: [npx, -y, "@upstash/context7-mcp"]
    timeout_ms: 30000
    env:
      CONTEXT7_API_KEY: "ENV:CONTEXT7_API_KEY"
  - id: github
    transport: http
    url: https://api.githubcopilot.com/mcp/
    headers:
      Authorization: "Bearer ENV:GITHUB_TOKEN"
```

Native:

```json
{
  "mcp": {
    "context7": {
      "type": "stdio",
      "command": "npx",
      "args": ["-y", "@upstash/context7-mcp"],
      "timeout": 30,
      "env": { "CONTEXT7_API_KEY": "$CONTEXT7_API_KEY" }
    },
    "github": {
      "type": "http",
      "url": "https://api.githubcopilot.com/mcp/",
      "headers": { "Authorization": "Bearer ${GITHUB_TOKEN}" }
    }
  }
}
```

## Reasoning variants

`models[].variants` maps to the model catalog's `reasoning_levels` array without
setting `default_reasoning_effort` or the selected-model `reasoning_effort`. Names
including provider-specific `ultra` are preserved unchanged. The live schema's
selected-model `reasoning_effort` enum is only `low|medium|high`; that field is
not written.

## Sources

- https://charm.land/crush.json
