# Cline CLI

- **Target id:** `cline`
- **Verified against:** official repo `cline/cline` (apps/cli; research report `.research/cline.md`, fetched 2026-09-07). The npm package is `cline` v3.0.61 (bin `cline`); the similarly named `@cline/cli` experimental SDK (bin `clite`) is a different tool and is not targeted.
- **Native files:** `~/.cline/data/settings/providers.json` (0600), `~/.cline/data/settings/models.json`, `~/.cline/data/settings/cline_mcp_settings.json`. `CLINE_DIR` / `CLINE_DATA_DIR` override the root.
- **v1 artifacts:** JSON fragments for all three files. v1 does not edit these files.

## Provider route

An IR literal such as `api_key: "example-key"` is emitted in the native
`settings.apiKey` field. The example key is a placeholder; real literals also appear in
generated output.

Custom providers are entries of `providers.json`:

```json
{
  "version": 1,
  "lastUsedProvider": "volcengine",
  "providers": {
    "volcengine": {
      "settings": {
        "provider": "volcengine",
        "baseUrl": "https://example.com/v1",
        "protocol": "openai-chat",
        "model": "glm-5.3",
        "headers": {"X-Tenant": "engineering"}
      },
      "updatedAt": "1970-01-01T00:00:00Z",
      "tokenSource": "manual"
    }
  }
}
```

- `protocol` renames: `openai-completions` → `openai-chat`, `openai-responses` → `openai-responses`, `anthropic-messages` → `anthropic`.
- Cline config is **literal-only**: there is no `${VAR}` expansion anywhere, and the environment fallback for API keys only exists for built-in provider ids. Custom providers therefore effectively need a stored literal `apiKey`, so IR `api_key: "ENV:NAME"` is rejected, and provider `headers` with `ENV:NAME` / `Bearer ENV:NAME` are rejected. Literal headers are emitted.

## Models

Rich model metadata lives in `models.json` under
`providers.<id>.models.<model-id>`: `name`, `contextWindow`, `maxTokens`,
`modalities {input, output}`, and `capabilities` (`images`, `video`, `tools`,
`reasoning`). `tool_calling: true` maps to `tools`. `tool_calling: false` is
rejected: Cline's persisted custom model registry treats missing capabilities as
tool-enabled and cannot faithfully turn tools off. Cline `settings.model` holds
only the bare default model id, and `defaults.model "provider/model"` maps to
`lastUsedProvider` + `settings.model`.

## MCP

`cline_mcp_settings.json` uses a nested `transport` form: stdio
(`command`/`args`/`cwd`/`env`) or `streamableHttp` (`url`/`headers`; the IR
`http` transport maps here). `enabled: false` maps to `disabled: true`.
`timeout` is a numeric number of seconds; the emitter preserves the IR
millisecond value as seconds (for example 1500 ms becomes `1.5`). MCP `env` and `headers` are literal
strings, so IR `ENV:NAME` / `Bearer ENV:NAME` MCP values are rejected. When the
IR has no MCP servers, the artifact is omitted.

Cline-valid MCP therefore uses literals, not `ENV:NAME`:

```yaml
mcp:
  - id: context7
    transport: stdio
    command: [npx, -y, "@upstash/context7-mcp"]
    timeout_ms: 1500
    env:
      CONTEXT7_API_KEY: "literal-key"
```

Native `timeout` is `1.5`. The same `timeout_ms: 1500` is rejected by Crush
(not a whole second) and accepted by OpenCode as milliseconds `1500`.

## Reasoning variants

Cline has provider-wide `settings.reasoning` and model capability metadata, but
no persisted model-local selectable effort list in its custom-model registry.
`models[].variants` is omitted; agentcfg does not reduce the list to the
provider-wide selected effort.
