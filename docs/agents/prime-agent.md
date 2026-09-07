# Prime Agent

- **Target id:** `prime-agent`
- **Verified against:** local `prime-agent 0.9.3`; installed source maps and CLI help inspected 2026-09-07.
- **Native locations:** global `~/.prime/agent/models.json` and `~/.prime/agent/settings.json`; project `.prime/settings.json`.
- **v1 artifact:** JSON provider registry fragment. MCP output is a `settings.json` fragment.

## Provider route

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

Unlike Pi's extension, the observed Prime registry accepts model records without Pi's required cost object. The v1 emitter maps all three IR protocols to the same Pi-AI names.

`~/.prime/agent/settings.json` has `defaultProvider` and `defaultModel`. The first version does not emit it: writing two separate JSON fragments to copy manually is error-prone and automatic merge is deliberately deferred.

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

The source also supports static HTTP `headers`, `oauth`, `enabled`, tool allow/deny lists, and startup/call timeouts. v1 maps only safe env-derived fields, `enabled`, and `timeout_ms` (to the target's start/call timeout policy when that policy is finalized). The CLI independently exposes `prime-agent mcp add`, including `--url`, `--bearer-token-env-var`, `--oauth`, and stdio `--env CHILD=SOURCE`.

## Sources

- Local `prime-agent mcp add --help`, version 0.9.3
- Installed source map: `dist/core/settings-manager.js.map`
- Installed `~/.prime/agent/models.json` (structure inspected; credentials not copied)
