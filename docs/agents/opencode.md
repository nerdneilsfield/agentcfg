# OpenCode

- **Target id:** `opencode`
- **Verified against:** local `opencode 1.18.20`; official schema downloaded 2026-09-07.
- **Native file:** `opencode.json` / `opencode.jsonc` according to OpenCode configuration discovery.
- **v1 artifact:** JSON fragment containing `provider` and `mcp`. v1 does not edit this file.
- **Schema snapshot:** [`opencode.config.schema.json`](opencode.config.schema.json), downloaded from `https://opencode.ai/config.json`.

## Provider route

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

The v1 emitter supports `openai-completions` only. It maps request headers to `options.headers`, using OpenCode's `{env:NAME}` interpolation. `openai-responses` and `anthropic-messages` need a verified adapter package and are rejected rather than guessed.

The published schema supports additional model fields (`attachment`, `temperature`, cost, variants, options and more). They are intentionally outside the v1 IR until their semantics are shared and required.

## MCP

```json
{
  "mcp": {
    "context7": {
      "type": "local",
      "command": ["npx", "-y", "@upstash/context7-mcp"],
      "environment": { "CONTEXT7_API_KEY": "{env:CONTEXT7_API_KEY}" }
    },
    "github": {
      "type": "remote",
      "url": "https://api.githubcopilot.com/mcp/",
      "headers": { "Authorization": "Bearer {env:GITHUB_TOKEN}" }
    }
  }
}
```

The schema supports `cwd`, `enabled`, and `timeout` for local and remote MCP; remote also has OAuth configuration. v1 supports stdio and HTTP headers, and maps `timeout_ms` / `enabled`; OAuth is omitted because it contains runtime credential state.

## Defaults

OpenCode documents root `model` as `provider/model`. `defaults.model` can therefore emit the root `model` field after the provider/model reference passes validation.

## Sources

- https://opencode.ai/docs/providers/
- https://opencode.ai/docs/mcp-servers
- https://opencode.ai/config.json
