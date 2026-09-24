# OpenClaw

- **Target id:** `openclaw`
- **Verified against:** official repo `openclaw/openclaw` (steipete/OpenClaw Foundation; research report `.research/openclaw.md`, 2026-09-07) and live docs `https://docs.openclaw.ai/tools/mcp` plus `https://docs.openclaw.ai/gateway/configuration` (fetched 2026-09-20). The npm package is `openclaw` (bin `openclaw`). OpenClaw is a personal/team AI-assistant gateway rather than a pure coding-agent CLI, but its provider/MCP config surfaces fit agentcfg scope. (Ruled out: the `pjasicek/OpenClaw` Captain Claw game engine and the PyPI `openclaw` cmdop installer.)
- **Native file:** `~/.openclaw/openclaw.json` (JSON5, comments + `$include`; `OPENCLAW_STATE_DIR` overrides). The generated agent-local catalog `~/.openclaw/agents/<id>/agent/models.json` must NOT be written.
- **v1 artifact:** a single JSON fragment for `openclaw.json`. v1 does not edit this file.

## Provider route

An IR literal such as `api_key: "example-key"` is emitted in the native
`apiKey` field. The example key is a placeholder; real literals also appear in
generated output. A literal that contains `${` is skipped with a warning, since
it would be read as the native env shorthand.

`models.providers.<id>` (strict zod) requires `baseUrl` + `models[]`:

```json
{
  "models": {
    "providers": {
      "volcengine": {
        "baseUrl": "https://example.com/v1",
        "api": "openai-completions",
        "apiKey": "${VOLC_API_KEY}",
        "headers": {"X-Gateway-Key": "${GATEWAY_KEY}"},
        "models": [{
          "id": "glm-5.3",
          "name": "GLM-5.3",
          "contextWindow": 128000,
          "maxTokens": 8192,
          "input": ["text", "image"],
          "reasoning": true,
          "compat": {"supportsTools": true}
        }]
      }
    }
  }
}
```

- `api` is a protocol enum including `openai-completions`, `openai-responses`, and `anthropic-messages`; all three IR protocols map 1:1. A provider on any other protocol is skipped with a warning.
- `apiKey` is a `SecretInput`: it accepts a `$VAR`/`${VAR}` env shorthand, so IR `api_key: "ENV:NAME"` maps to `"${VAR}"` without resolving the reference. Provider `headers` are also `SecretInput`-capable; IR `ENV:NAME` renders as `"${VAR}"` and `Authorization: "Bearer ENV:NAME"` as `"Bearer ${VAR}"`.
- Model entries require `id` and a non-empty `name` (agentcfg falls back to `id`) and support `reasoning`, `input` (`text|image|video|audio`), `contextWindow`, `maxTokens`, and `compat` (including `supportsTools`). IR `tool_calling` maps to `compat.supportsTools`. IR output modalities have no OpenClaw field and are not emitted; `pdf` input is omitted with a warning because the native model input enum is limited to text, image, audio, and video.
- Whole-config load-time `${VAR}` interpolation applies to MCP env/headers too (uppercase vars only).

## Reasoning variants

`models[].variants` emits the model `thinkingLevelMap`. Listed OpenClaw levels
map to themselves; every other documented level is `null`. Its native map has
the fixed names `off`, `minimal`, `low`, `medium`, `high`, `xhigh`, and `max`.
An unrecognized model/provider-specific name such as `ultra` is omitted rather
than remapped. No default thinking level or request parameters are generated.

## Defaults

`defaults.model "provider/model"` maps to `agents.defaults.model` as a plain string. A default that does not resolve to an emitted provider model is skipped with a warning, and the `agents.defaults` block is omitted. Live docs also allow `agents.defaults.model.primary` plus fallbacks; v1 still writes the string form, which the 2026-09-20 reference still accepts.

## MCP

`mcp.servers.<id>` supports stdio (`command`, `args`, `env`, `cwd`) and
`transport: "streamable-http"` with `url` + `headers` (IR `http` maps here;
`sse` also exists). A stdio server with no command is dropped with a warning.
The 2026-09-20 docs still show `connectionTimeoutMs` /
`requestTimeoutMs`; v1 duplicates IR `timeout_ms` onto both. IR `enabled` maps
to the native `enabled` flag. First-party OAuth (`auth: "oauth"`) and mTLS
options have no IR counterpart and are not emitted.

## Sources

- https://docs.openclaw.ai/gateway/configuration
- https://docs.openclaw.ai/tools/mcp
