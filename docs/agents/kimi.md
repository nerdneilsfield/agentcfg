# Kimi Code

- **Target id:** `kimi`
- **Verified against:** official `@moonshot-ai/kimi-code` repository zod schemas and docs (research report `.research/kimi.md`, fetched 2026-09-07; npm `@moonshot-ai/kimi-code` v0.41.0).
- **Native files:** `~/.kimi-code/config.toml` and `~/.kimi-code/mcp.json` (relocatable via `KIMI_CODE_HOME`).
- **v1 artifacts:** two artifacts — TOML fragment + JSON `mcp.json` fragment.

## Provider route

Kimi Code uses `[providers.<id>]` with a `type` discriminator and `[models.<alias>]` model entries:

```toml
[providers.volcengine]
type = "openai_responses" # kimi | anthropic | openai | openai_responses | google-genai | vertexai
base_url = "https://example.com/v1"
custom_headers = { X-Tenant = "engineering" }

[models.glm-5.3]
provider = "volcengine"
model = "glm-5.3"
max_context_size = 128000
max_output_size = 8192
display_name = "GLM-5.3"
capabilities = ["thinking", "tool_use", "image_in"]
```

All three IR protocols map 1:1 (`openai`, `openai_responses`, `anthropic`).

Kimi reads literal `api_key` values only and has **no** environment-variable fallback anywhere in `config.toml`. The IR `api_key_env` is therefore rejected for this target — never render a secret inline. `custom_headers` values are literal strings with no interpolation, so `from_env`/`bearer_from_env` provider headers are rejected.

`max_context_size` is required for every model; an IR model without `context_window` is rejected. `capabilities` is emitted natively: `thinking` (IR `reasoning`), `tool_use` (IR `tool_calling`), `image_in`/`video_in`/`audio_in` (IR `input` modalities).

Model aliases are a flat `[models]` table, so duplicate model IDs across IR providers are rejected.

## MCP

Kimi reads MCP servers from a separate `mcp.json`:

```json
{
  "mcpServers": {
    "context7": {
      "transport": "stdio",
      "command": "npx",
      "args": ["-y", "@upstash/context7-mcp"],
      "env": { "CONTEXT7_API_KEY": "literal-key" },
      "enabled": true
    },
    "github": {
      "transport": "http",
      "url": "https://api.githubcopilot.com/mcp/",
      "bearerTokenEnvVar": "GITHUB_TOKEN",
      "enabled": true
    }
  }
}
```

MCP `env` and `headers` values are literal strings: IR `from_env` MCP env/header references are rejected. `Authorization.bearer_from_env` maps to the native `bearerTokenEnvVar`.

## Defaults

`defaults.model` maps to top-level `default_model = "<alias>"` (the model ID part after the provider prefix). Validation rejects a default that does not resolve to an emitted alias.

When the IR has no MCP servers, the `mcp.json` artifact is omitted entirely.
