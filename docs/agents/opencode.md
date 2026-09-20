# OpenCode

- **Target id:** `opencode`
- **Verified against:** local `opencode 1.18.20`; official schema downloaded 2026-09-07.
- **Native file:** `opencode.json` / `opencode.jsonc` according to OpenCode configuration discovery.
- **v1 artifact:** JSON fragment containing `provider` and `mcp`. v1 does not edit this file.
- **Schema snapshot:** [`opencode.config.schema.json`](opencode.config.schema.json), downloaded from `https://opencode.ai/config.json`.

## Provider route

An IR literal such as `api_key: "example-key"` is emitted in the native
`options.apiKey` field. The example key is a placeholder; real literals also appear in
generated output.

For OpenAI-compatible routes OpenCode uses the AI SDK adapter package:

```json
{
  "provider": {
    "volcengine": {
      "npm": "@ai-sdk/openai-compatible",
      "name": "Volcengine",
      "options": {
        "baseURL": "https://example.com/v1",
        "apiKey": "{env:VOLC_API_KEY}"
      },
      "models": {
        "glm-5.3": {
          "name": "GLM-5.3",
          "reasoning": true,
          "tool_call": true,
          "limit": { "context": 128000, "output": 8192 },
          "modalities": { "input": ["text", "image"], "output": ["text"] }
        }
      }
    }
  }
}
```

The v1 emitter supports `openai-completions` only. It maps request headers to `options.headers`, using OpenCode's `{env:NAME}` interpolation. `tool_calling: true` maps to the native model `tool_call: true`; the schema's separate top-level/agent `tools` field is not emitted for a model. `openai-responses` and `anthropic-messages` need a verified adapter package and are rejected rather than guessed.

### Reasoning variants

`models[].variants` emits an OpenCode model `variants` object. Each agentcfg variant becomes a child key with only the matching native
`reasoningEffort`; provider/model-specific names such as `ultra` are preserved:

```json
"variants": {
  "low": { "reasoningEffort": "low" },
  "high": { "reasoningEffort": "high" },
  "max": { "reasoningEffort": "max" }
}
```

It does not emit `reasoningSummary`, `textVerbosity`, token budgets, or other
provider options. Those are not part of the agentcfg variant semantic.

The published schema supports additional model fields (`attachment`,
`temperature`, cost, options and more). They remain outside the IR until their
semantics are shared and required.

## MCP

IR:

```yaml
mcp:
  - id: context7
    transport: stdio
    command: [npx, -y, "@upstash/context7-mcp"]
    cwd: /var/lib/context7
    timeout_ms: 30000
    env:
      CONTEXT7_API_KEY: "ENV:CONTEXT7_API_KEY"
  - id: github
    transport: http
    url: https://api.githubcopilot.com/mcp/
    headers:
      Authorization: "Bearer ENV:GITHUB_TOKEN"
    timeout_ms: 15000
```

Native:

```json
{
  "mcp": {
    "context7": {
      "type": "local",
      "command": ["npx", "-y", "@upstash/context7-mcp"],
      "enabled": true,
      "environment": { "CONTEXT7_API_KEY": "{env:CONTEXT7_API_KEY}" },
      "cwd": "/var/lib/context7",
      "timeout": 30000
    },
    "github": {
      "type": "remote",
      "url": "https://api.githubcopilot.com/mcp/",
      "enabled": true,
      "headers": { "Authorization": "Bearer {env:GITHUB_TOKEN}" },
      "timeout": 15000
    }
  }
}
```

The schema supports `cwd`, `enabled`, and `timeout` for local and remote MCP; remote also has OAuth configuration. v1 maps stdio and HTTP headers, `cwd`, `enabled`, and `timeout_ms` (milliseconds, native `timeout`). OAuth is omitted because it contains runtime credential state. `enabled` defaults to `true` when the IR omits it.

## Defaults

OpenCode documents root `model` as `provider/model`. `defaults.model` can therefore emit the root `model` field after the provider/model reference passes validation.

## Sources

- https://opencode.ai/docs/providers/
- https://opencode.ai/docs/mcp-servers
- https://opencode.ai/config.json
