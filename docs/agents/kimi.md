# Kimi Code

- **Target id:** `kimi`
- **Verified against:** official Kimi Code docs and `MoonshotAI/kimi-code` repository (config-files + MCP pages fetched 2026-09-20; npm `@moonshot-ai/kimi-code`). The 2026-09-07 research snapshot is superseded for credentials and MCP timeouts.
- **Native files:** `~/.kimi-code/config.toml` and `~/.kimi-code/mcp.json` (relocatable via `KIMI_CODE_HOME`).
- **v1 artifacts:** two artifacts — TOML fragment + JSON `mcp.json` fragment.

## Provider route

An IR literal such as `api_key: "example-key"` is emitted in the native
`api_key` field. The example key is a placeholder; real literals also appear in
generated output. IR `api_key: "ENV:NAME"` maps to native `api_key_env`.

Kimi Code uses `[providers.<id>]` with a `type` discriminator and `[models.<alias>]` model entries:

```toml
[providers.volcengine]
type = "openai_responses" # kimi | anthropic | openai | openai_responses | google-genai | vertexai
base_url = "https://example.com/v1"
api_key_env = "VOLC_API_KEY"
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

Official Kimi Code docs (2026-09-20) document `api_key` and `api_key_env` as mutually exclusive provider credentials. `api_key_env` is the name of a shell variable re-read on every request; it is not an automatic fallback from `export KIMI_API_KEY`. The emitter writes exactly one of those two fields from the IR scalar. It does not emit the `[providers.<id>.env]` fallback table or OAuth `storage`/`key` objects: those are CLI-owned credential sources, not IR fields.

`custom_headers` values remain literal strings with no interpolation, so `ENV:NAME`/`Bearer ENV:NAME` provider headers are skipped; only constant values are written.

`max_context_size` is required for every model; an IR model without `context_window` is skipped and gets no `[models]` entry. `capabilities` is emitted natively: `thinking` (IR `reasoning`), `tool_use` (IR `tool_calling`), `image_in`/`video_in`/`audio_in` (IR `input` modalities).

Model aliases are a flat `[models]` table, so duplicate model IDs across IR providers collapse to one alias; the first provider's model wins.

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
      "enabled": true,
      "startupTimeoutMs": 20000
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

MCP `env` and `headers` values are literal strings: IR `ENV:NAME` MCP env/header references are skipped (the entry is kept, the value omitted). `Authorization: "Bearer ENV:NAME"` maps to the native `bearerTokenEnvVar`. Stdio `cwd` is documented and emitted.

IR `timeout_ms` maps to per-server `startupTimeoutMs` (milliseconds; native range 1–2147483647, default 30000). Values outside that range are skipped, so no `startupTimeoutMs` is written. Official MCP docs also list `toolTimeoutMs` for a single tool call, plus global `[mcp] startup_timeout_ms` / `tool_timeout_ms` in `config.toml`. v1 does not emit `toolTimeoutMs` or the global table: the IR has one timeout, and Codex/OpenCode already treat that field as connection/startup, not per-tool. Deferred loading (`deferred`), tool allow/deny lists, and `/mcp-config` OAuth login are omitted for the same reason.

A Kimi-valid MCP pair uses a literal stdio env value, an optional startup timeout, and a bearer env var on HTTP:

```yaml
mcp:
  - id: context7
    transport: stdio
    command: [npx, -y, "@upstash/context7-mcp"]
    timeout_ms: 20000
    env:
      CONTEXT7_API_KEY: "literal-key"
  - id: github
    transport: http
    url: https://api.githubcopilot.com/mcp/
    headers:
      Authorization: "Bearer ENV:GITHUB_TOKEN"
```

## Defaults

`defaults.model` maps to top-level `default_model = "<alias>"` (the model ID part after the provider prefix). A default that does not resolve to an emitted alias is skipped with a warning and `default_model` is omitted.

When the IR has no MCP servers, the `mcp.json` artifact is omitted entirely.

## Reasoning variants

`models[].variants` maps to Kimi's string-list `support_efforts` and preserves
provider/model names such as `ultra`. It never writes `default_effort`.

## Sources

- https://moonshotai.github.io/kimi-code/en/configuration/config-files.html (`api_key` / `api_key_env`)
- https://github.com/MoonshotAI/kimi-code/blob/master/docs/en/configuration/providers.md
- https://github.com/MoonshotAI/kimi-code/blob/master/docs/en/customization/mcp.md (`mcp.json` fields including `cwd`, `bearerTokenEnvVar`, `startupTimeoutMs`, `toolTimeoutMs`)
