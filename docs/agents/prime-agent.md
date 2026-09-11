# Prime Agent

- **Target id:** `prime-agent`
- **Verified against:** local `prime-agent 0.9.3`; installed source maps and CLI help inspected 2026-09-07.
- **Native locations:** global `~/.prime/agent/models.json` and `~/.prime/agent/settings.json`; project `.prime/settings.json`.
- **v1 artifact:** JSON provider registry fragment. MCP output is a `settings.json` fragment.

## Provider route

An IR literal such as `api_key: "example-key"` is emitted in the native
`apiKey` field. The example key is a placeholder; real literals also appear in
generated output.

The installed `~/.prime/agent/models.json` is Pi-compatible. It contains a `providers` map with `baseUrl`, `api`, `apiKey`, and an array of models. Model records use `id`, `name`, `input`, `reasoning`, `contextWindow`, `maxTokens`, and can use `thinkingLevelMap`.

```json
{
  "providers": {
    "volcengine": {
      "baseUrl": "https://example.com/v1",
      "api": "openai-completions",
      "apiKey": "$VOLC_API_KEY",
      "models": [{
        "id": "glm-5.3",
        "name": "GLM-5.3",
        "input": ["text"],
        "reasoning": true,
        "contextWindow": 128000,
        "maxTokens": 8192
      }]
    }
  }
}
```

Unlike Pi's extension, the observed Prime registry accepts model records without Pi's required cost object. The v1 emitter maps all three IR protocols to the same Pi-AI names. Provider records have no `headers` field, so IR provider headers are rejected in v1 rather than silently dropped.

`~/.prime/agent/settings.json` has `defaultProvider` and `defaultModel`. The v1 emitter writes a `settings.json` fragment containing `mcpServers` (below) and, when `defaults.model` is present, `defaultProvider`/`defaultModel`. Automatic merge into an existing settings file is deliberately deferred, so the fragment is copied manually.

## MCP

The installed source declares `settings.json.mcpServers` as a map. Its verified shape is:

```json
{
  "mcpServers": {
    "context7": {
      "type": "stdio",
      "command": "npx",
      "args": ["-y", "@upstash/context7-mcp"],
      "env": { "CONTEXT7_API_KEY": { "env": "CONTEXT7_API_KEY" } }
    },
    "github": {
      "type": "http",
      "url": "https://api.githubcopilot.com/mcp/",
      "bearerTokenEnvVar": "GITHUB_TOKEN"
    }
  }
}
```

The source also supports static HTTP `headers`, `oauth`, `enabled`, tool allow/deny lists, and startup/call timeouts. v1 maps only safe env-derived fields, `enabled`, and `timeout_ms` (to the target's start/call timeout policy when that policy is finalized). Concretely, the v1 emitter writes `settings.json.mcpServers` with:

- stdio `env` entries as `{ "env": "SOURCE" }` objects; renamed references (`CHILD` key mapping to a different `SOURCE` name) are representable, but literal values are rejected because only env-derived fields are verified;
- static HTTP `headers` (constant values only; IR `ENV:NAME` header references are rejected because no env interpolation syntax is verified for MCP headers);
- `bearerTokenEnvVar` from an `Authorization` IR `Bearer ENV:NAME` entry only; bearer references on other header names are rejected;
- `enabled: false` when the IR server is disabled.

The CLI independently exposes `prime-agent mcp add`, including `--url`, `--bearer-token-env-var`, `--oauth`, and stdio `--env CHILD=SOURCE`.

## Sources

- Local `prime-agent mcp add --help`, version 0.9.3
- Installed source map: `dist/core/settings-manager.js.map`
- Installed `~/.prime/agent/models.json` (structure inspected; credentials not copied)
