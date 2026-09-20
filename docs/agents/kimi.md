# Kimi Code

- **Target id:** `kimi`
- **Verified against:** official Kimi Code docs and `MoonshotAI/kimi-code` repository (config-files + MCP pages fetched 2026-09-20; npm `@moonshot-ai/kimi-code`). The 2026-09-07 research snapshot is superseded for credentials and MCP timeouts.
- **Native files:** `~/.kimi-code/config.toml` and `~/.kimi-code/mcp.json` (relocatable via `KIMI_CODE_HOME`).
- **v1 artifacts:** two artifacts — TOML fragment + JSON `mcp.json` fragment.

## Provider route

An IR literal such as `api_key: "example-key"` is emitted in the native
`api_key` field. The example key is a placeholder; real literals also appear in
generated output.

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

Official Kimi Code docs (2026-09-20) document `api_key` and `api_key_env` as mutually exclusive provider credentials. `api_key_env` is the name of a shell variable re-read on every request; it is not an automatic fallback from `export KIMI_API_KEY`. The v1 emitter still rejects `api_key: "ENV:NAME"` and writes only a literal `api_key`. Use a quoted literal for this target.

`custom_headers` values remain literal strings with no interpolation, so `ENV:NAME`/`Bearer ENV:NAME` provider headers are still rejected.

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

MCP `env` and `headers` values are literal strings: IR `ENV:NAME` MCP env/header references are rejected. `Authorization: "Bearer ENV:NAME"` maps to the native `bearerTokenEnvVar`. Stdio `cwd` is documented and emitted.

Official MCP docs also list per-server `startupTimeoutMs` and `toolTimeoutMs` (milliseconds; default startup timeout 30000), plus global `[mcp]` defaults in `config.toml`. The v1 emitter still rejects IR `timeout_ms` because it does not choose between those two native fields. Omit `timeout_ms` when `--to` includes `kimi`.

A Kimi-valid MCP pair uses a literal stdio env value and a bearer env var on HTTP:

```yaml
mcp:
  - id: context7
    transport: stdio
    command: [npx, -y, "@upstash/context7-mcp"]
    env:
      CONTEXT7_API_KEY: "literal-key"
  - id: github
    transport: http
    url: https://api.githubcopilot.com/mcp/
    headers:
      Authorization: "Bearer ENV:GITHUB_TOKEN"
```

## Defaults

`defaults.model` maps to top-level `default_model = "<alias>"` (the model ID part after the provider prefix). Validation rejects a default that does not resolve to an emitted alias.

When the IR has no MCP servers, the `mcp.json` artifact is omitted entirely.

## Reasoning variants

`models[].variants` maps to Kimi's string-list `support_efforts` and preserves
provider/model names such as `ultra`. It never writes `default_effort`.

## Sources

- https://moonshotai.github.io/kimi-code/en/configuration/config-files.html (`api_key` / `api_key_env`)
- https://github.com/MoonshotAI/kimi-code/blob/master/docs/en/configuration/providers.md
- https://github.com/MoonshotAI/kimi-code/blob/master/docs/en/customization/mcp.md (`mcp.json` fields including `cwd`, `bearerTokenEnvVar`, `startupTimeoutMs`, `toolTimeoutMs`)
