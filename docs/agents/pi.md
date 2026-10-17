# Pi Coding Agent

- **Target id:** `pi`
- **Verified against:** upstream `packages/coding-agent/docs/models.md` and
  `packages/coding-agent/src/core/provider-composer.ts` (`authHeader`), both
  inspected 2026-09-20; native MCP docs and installed Pi 1.3.0 MCP config
  parser inspected 2026-10-18.
- **Native locations:** `~/.pi/agent/models.json` and `~/.pi/agent/mcp.json`;
  project MCP configuration uses `.pi/mcp.json` in trusted projects.
- **v1 artifacts:** `models.json` containing `providers`, plus `mcp.json`
  containing `mcpServers` when the IR declares MCP servers.

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

Model `input` accepts `text` and `image` only; other modalities are dropped with
a warning, and a model left with none omits the field.
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
`$` or `!` would be read as that syntax rather than as a token, so the emitter
warns and omits `apiKey`.

An IR `Authorization: "Bearer ENV:NAME"` provider header is skipped with a
warning for this target, and the header is omitted. Pi has no bearer form for an
arbitrary header, and the equivalent route is `api_key` plus `auth_type: bearer`.

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

Pi 1.3.0 has built-in MCP support for stdio and streamable HTTP. agentcfg emits
a separate `~/.pi/agent/mcp.json` document; a third-party adapter is not required.

```json
{
  "mcpServers": {
    "context7": {
      "command": "npx",
      "args": ["-y", "@upstash/context7-mcp"],
      "env": { "CONTEXT7_API_KEY": "${CONTEXT7_API_KEY}" },
      "cwd": "/var/lib/context7",
      "timeout": 1.5
    },
    "github": {
      "url": "https://api.githubcopilot.com/mcp/",
      "headers": { "Authorization": "Bearer ${GITHUB_TOKEN}" },
      "enabled": false
    }
  }
}
```

Stdio `command` is the first IR argv element; the rest become `args`. HTTP uses
`url` and `headers`. The native parser infers transport from `command` or `url`.
IR `enabled` maps to the native boolean. `timeout_ms` becomes `timeout` in seconds
(`1500` becomes `1.5`), a per-request timeout reset by progress notifications.

MCP env/header references are emitted as `${NAME}`; `Bearer ENV:NAME` becomes
`Bearer ${NAME}`. Pi uses the same template resolver as its provider values,
so literal dollar signs are escaped as `$$` and a leading `!` as `$!`.
agentcfg neither resolves references nor runs commands.
Names that differ only in `-` and `_` share a Pi namespace, so the later entry
is skipped with a warning. Models and other supported MCP entries are preserved.

With no MCP declarations, stdout contains only `models.json`. File-writing mode
also writes an empty `mcpServers` companion, clearing stale servers while
preserving unrelated top-level settings such as `autoEnableCodemode`.

Pi connects servers when a session starts. Use `pi mcp list` to check connections,
`/mcp` to inspect them interactively, and `/reload` after changing configuration.
Pi owns OAuth sign-in and tokens; agentcfg does not generate OAuth enrollment.
An extension that registers `/mcp`, such as `pi-mcp-adapter`, replaces built-in
MCP during sessions; remove that extension to use the native `mcp.json` behavior.

## Defaults

`defaultProvider` and `defaultModel` are settings fields rather than part of this
document, and v1 emits no settings mutation, so defaults stay deferred until the
provider is loaded.

## Sources

- `https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/models.md`
- `packages/coding-agent/src/core/provider-composer.ts` (`withConfiguredAuth`)
- `packages/ai/src/api/openai-completions.ts`, `api/anthropic-messages.ts` (client construction)
- [Native MCP documentation](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/mcp.md)
- `packages/coding-agent/src/core/mcp-servers.ts` (native entry validation)
- `packages/coding-agent/src/extensions/mcp/config.ts` (file loading and namespace collisions)
