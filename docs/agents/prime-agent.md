# Prime Agent

- **Target id:** `prime-agent`
- **Verified against:** local `prime-agent 0.9.3`; installed source maps and CLI help inspected 2026-09-07.
- **Native locations:** global `~/.prime/agent/models.json` and `~/.prime/agent/settings.json`; project `.prime/settings.json`.
- **v1 artifact:** JSON provider registry fragment. MCP output is a `settings.json` fragment.

## Provider route

An IR literal such as `api_key: "example-key"` is emitted in the native
`apiKey` field. The example key is a placeholder; real literals also appear in
generated output.

The installed `~/.prime/agent/models.json` is Pi-compatible. It contains a `providers` map with `baseUrl`, `api`, `apiKey`, optional `headers`, and an array of models. Model records use `id`, `name`, `input`, `reasoning`, `contextWindow`, `maxTokens`, and can use `thinkingLevelMap`.

```json
{
  "providers": {
    "volcengine": {
      "baseUrl": "https://example.com/v1",
      "api": "openai-completions",
      "apiKey": "VOLC_API_KEY",
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

Unlike Pi's extension, the observed Prime registry accepts model records without Pi's required cost object. The v1 emitter maps all three IR protocols to the same Pi-AI names. Literal provider headers map to the native `headers` record; `ENV:NAME` maps to the bare environment-variable name expected by Prime Agent's resolver. `Bearer ENV:NAME` provider headers are rejected: Prime has no bearer expansion on provider headers, and emitting an empty string would look like a valid Authorization value. `api_key: "ENV:NAME"` likewise maps to a bare `apiKey` name, not Pi's `$NAME` template. Model input is limited to `text` and `image`.


### Reasoning variants

`models[].variants` emits a model-local `thinkingLevelMap`: listed Prime Agent
levels map to the same provider value, and all other documented levels are
`null`. Prime Agent's fixed names are `off`, `minimal`, `low`, `medium`,
`high`, `xhigh`, and `max`; unsupported names, including `ultra`, are omitted.
This does not set global `defaultThinkingLevel`.

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

The source also supports static HTTP `headers`, `oauth`, `enabled`, tool allow/deny lists, and startup/call timeouts. v1 maps only verified env-derived fields and `enabled`. IR `timeout_ms` is rejected: the native start/call timeout policy is not finalized, so the emitter does not guess a field. Concretely, the v1 emitter writes `settings.json.mcpServers` with:

- stdio `env` entries as `{ "env": "SOURCE" }` objects; renamed references (`CHILD` key mapping to a different `SOURCE` name) are representable, but literal values are rejected because only env-derived fields are verified;
- static HTTP `headers` (constant values only; IR `ENV:NAME` header references are rejected because no env interpolation syntax is verified for MCP headers);
- `bearerTokenEnvVar` from an `Authorization` IR `Bearer ENV:NAME` entry only; that Authorization header is not also written as an empty string; bearer references on other header names are rejected;
- `enabled: false` when the IR server is disabled.

The CLI independently exposes `prime-agent mcp add`, including `--url`, `--bearer-token-env-var`, `--oauth`, and stdio `--env CHILD=SOURCE`.

## Sources

- Local `prime-agent mcp add --help`, version 0.9.3
- Installed source map: `dist/core/settings-manager.js.map`
- Installed `~/.prime/agent/models.json` (structure inspected; credentials not copied)
