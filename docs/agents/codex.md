# Codex CLI

- **Target id:** `codex`
- **Verified against:** local `codex-cli 0.153.4`; official configuration reference fetched 2026-09-07.
- **Native file:** `$CODEX_HOME/config.toml`, normally `~/.codex/config.toml`.
- **v1 artifact:** TOML fragment. v1 does not edit this file.

## Provider route

IR `api_key: "ENV:NAME"` maps to native `env_key`. A literal such as
`api_key: "example-key"` maps to `experimental_bearer_token`, which Codex uses
in the `Authorization: Bearer <token>` header. Codex recommends `env_key` for
security; a literal key appears in generated output.

Codex uses `[model_providers.<id>]`. The official reference documents `name`, `base_url`, `env_key`, `experimental_bearer_token`, `wire_api`, `http_headers`, `env_http_headers`, and `query_params`.

```toml
[model_providers.volcengine]
name = "Volcengine"
base_url = "https://example.com/v1"
env_key = "VOLC_API_KEY"
wire_api = "responses"

[model_providers.volcengine.env_http_headers]
X-Gateway-Key = "GATEWAY_KEY"
```

The current official reference says `wire_api` supports `responses` only. The v1 emitter therefore accepts `openai-responses` for Codex and rejects `openai-completions` and `anthropic-messages`; it must not emit the historical `chat` spelling.

Codex has no documented custom model catalog under `model_providers`; provider models in the IR are validated as references but not emitted as catalog entries.

## MCP

Codex uses `[mcp_servers.<id>]`.

IR:

```yaml
mcp:
  - id: context7
    transport: stdio
    command: [npx, -y, "@upstash/context7-mcp"]
    timeout_ms: 20000
    env:
      CONTEXT7_API_KEY: "ENV:CONTEXT7_API_KEY"
  - id: github
    transport: http
    url: https://api.githubcopilot.com/mcp/
    headers:
      Authorization: "Bearer ENV:GITHUB_TOKEN"
```

Native:

```toml
[mcp_servers.context7]
command = "npx"
args = ["-y", "@upstash/context7-mcp"]
env_vars = ["CONTEXT7_API_KEY"]
startup_timeout_ms = 20000

[mcp_servers.github]
url = "https://api.githubcopilot.com/mcp/"
bearer_token_env_var = "GITHUB_TOKEN"
```

The reference documents `command`, `args`, `url`, `enabled`, `required`, `env`, `env_vars`, `cwd`, `bearer_token_env_var`, `http_headers`, `env_http_headers`, `startup_timeout_ms`, `startup_timeout_sec`, and `tool_timeout_sec`. IR
`timeout_ms` maps losslessly to `startup_timeout_ms`. v1 maps a same-name environment reference through `env_vars`, a literal child value through `env`; a renamed environment reference is rejected because `env_vars` inherits only same-name variables. It maps `Authorization: "Bearer ENV:NAME"` through `bearer_token_env_var`; other static and environment-derived HTTP headers map to `http_headers` and `env_http_headers`. Literal values are rendered as supplied; environment references are not resolved by agentcfg.

## Defaults

Codex selects a custom default through top-level `model_provider` and `model`.
`defaults.model: "provider/model"` maps to those two fields. This is separate
from the replacement `model_catalog_json` mechanism, which agentcfg does not
generate from the common model IR.

## Sources

- https://raw.githubusercontent.com/openai/codex/main/codex-rs/core/config.schema.json (`ModelProviderInfo.experimental_bearer_token`)
- https://openai-codex.mintlify.app/configuration/reference
- Local `codex mcp add --help` (0.153.4)

## Reasoning variants

Codex keeps per-model availability in a separate model catalog and active effort
in a session/top-level setting, not in `model_providers`. Therefore a non-empty
agentcfg `models[].variants` list is omitted. A complete catalog would carry
`supported_reasoning_levels`, but generating it from the common IR would discard
required catalog metadata and replace Codex's built-in catalog, so agentcfg
does not attempt it.
