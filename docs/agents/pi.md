# Pi Coding Agent

- **Target id:** `pi`
- **Verified against:** upstream `packages/coding-agent/docs/models.md` and
  `packages/coding-agent/src/core/provider-composer.ts` (`authHeader`), both
  inspected 2026-09-20.
- **Native locations:** `~/.pi/agent/models.json` (global); `.pi/` per project.
- **v1 artifact:** one JSON document containing `providers`.

This document and `docs/agents/prime-agent.md` describe forks of the same
codebase. Their provider and model field names match, but their credential value
syntax does not; see [Value syntax](#value-syntax).

## Provider route

Pi reads custom providers and models from `~/.pi/agent/models.json`. Custom
entries merge over the built-in catalog: built-in models stay, and a custom
model with the same id replaces the built-in entry for that provider.

```json
{
  "providers": {
    "volcengine": {
      "baseUrl": "https://example.com/v1",
      "api": "openai-completions",
      "apiKey": "$VOLC_API_KEY",
      "headers": { "X-Tenant": "engineering" },
      "models": [
        {
          "id": "glm-5.3",
          "name": "GLM-5.3",
          "input": ["text", "image"],
          "reasoning": true,
          "contextWindow": 128000,
          "maxTokens": 8192,
          "cost": { "input": 0, "output": 0, "cacheRead": 0, "cacheWrite": 0 }
        }
      ]
    }
  }
}
```

All three IR protocols map 1:1 to Pi's `api` values: `openai-completions`,
`openai-responses`, and `anthropic-messages`.

Model `input` accepts `text` and `image` only; other modalities are rejected.
`cost` is emitted as zeros because the IR has no pricing scope. `contextWindow`
and `maxTokens` are emitted only when the IR sets them, so Pi's own defaults
(128000 and 16384) stay in place when a model omits them — writing a zero would
replace that default with a zero-token cap. `thinkingLevelMap` is emitted only
for a model that declares `variants`.

### Value syntax

`apiKey` and header values use Pi's value syntax:

- `"!command"` runs a command and uses its stdout,
- `"$NAME"` and `"${NAME}"` interpolate an environment variable,
- `"$$"` and `"$!"` escape a leading `$` or `!`,
- any other string is a literal.

IR `ENV:NAME` therefore renders as `"$NAME"`. A literal API key that starts with
`$` or `!` would be read as that syntax rather than as a token, so it is
rejected.

An IR `Authorization: "Bearer ENV:NAME"` provider header stays rejected for this
target. Pi has no bearer form for an arbitrary header, and the equivalent route
is `api_key` plus `auth_type: bearer`.

### Authentication

Which header Pi sends depends on the protocol, so `auth_type` overrides it
instead of being inferred:

| `auth_type` | Emitted | Effect |
|---|---|---|
| `official` (default) | nothing extra | the protocol's native header: `Authorization: Bearer` for the OpenAI transports, `x-api-key` for `anthropic-messages` |
| `bearer` | `"authHeader": true` | Pi sends `Authorization: Bearer <resolved apiKey>` for this provider |
| `none` | `apiKey` omitted | no credential, for keyless local endpoints |

### Reasoning variants

`models[].variants` emits a model `thinkingLevelMap`. Declared Pi levels map to
themselves and every other documented Pi level is `null`, which marks it
unsupported and hides it. Pi documents `off`, `minimal`, `low`, `medium`,
`high`, `xhigh`, and `max`; names such as `ultra` are skipped. No default
thinking level is emitted.

## MCP

Pi ships without built-in MCP support. The third-party `pi-mcp-adapter` reads a
standard `mcpServers` document from its own search paths, but it is not part of
Pi and its schema is not vendored here. `gen --to pi` therefore fails when the
IR declares MCP servers rather than emitting a document Pi would ignore.

## Defaults

`defaultProvider` and `defaultModel` are settings fields rather than part of this
document, and v1 emits no settings mutation, so defaults stay deferred until the
provider is loaded.

## Sources

- `https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/models.md`
- `packages/coding-agent/src/core/provider-composer.ts` (`withConfiguredAuth`)
- `packages/ai/src/api/openai-completions.ts`, `api/anthropic-messages.ts` (client construction)
