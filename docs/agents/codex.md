# Codex CLI

- **Target id:** `codex`
- **Verified against:** local `codex-cli 0.153.4`; live `config.schema.json` and Configuration Reference fetched 2026-09-20.
- **Native file:** `$CODEX_HOME/config.toml`, normally `~/.codex/config.toml`.
- **v1 artifact:** TOML fragment. v1 does not edit this file.

## Provider route

IR `api_key: "ENV:NAME"` maps to native `env_key`. A literal such as
`api_key: "example-key"` maps to `experimental_bearer_token`, which Codex uses
in the `Authorization: Bearer <token>` header. Codex recommends `env_key` for
security; a literal key appears in generated output.

Codex uses `[model_providers.<id>]`. The official reference documents `name`, `base_url`, `env_key`, `experimental_bearer_token`, `wire_api`, `http_headers`, `env_http_headers`, and `query_params`. A `Bearer ENV:NAME` provider header is skipped with a warning, because `model_providers` has no bearer field for header names.

```toml
[model_providers.volcengine]
name = "Volcengine"
base_url = "https://example.com/v1"
env_key = "VOLC_API_KEY"
wire_api = "responses"

[model_providers.volcengine.env_http_headers]
X-Gateway-Key = "GATEWAY_KEY"
```

The current official reference says `wire_api` supports `responses` only. The v1 emitter therefore accepts `openai-responses` for Codex; a provider on any other protocol is dropped with a warning, and it must not emit the historical `chat` spelling.

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
`timeout_ms` maps losslessly to `startup_timeout_ms`. A stdio server with no command is dropped with a warning. v1 maps a same-name environment reference through `env_vars` as a string name (`source = "local"`), a literal child value through `env`; a renamed environment reference is skipped with a warning because `env_vars` inherits only same-name variables. It maps `Authorization: "Bearer ENV:NAME"` through `bearer_token_env_var`; a bearer reference on any other header name is skipped, since that field applies to `Authorization` only, while other static and environment-derived HTTP headers map to `http_headers` and `env_http_headers`. Literal values are rendered as supplied; environment references are not resolved by agentcfg.

v1 does not emit `startup_timeout_sec`, `tool_timeout_sec`, MCP `auth` (`oauth` | `chatgpt`), `http_headers_helper`, `experimental_environment`, or OAuth callback/store keys. Those are runtime enrollment, a second unit for the same timeout, or have no IR field.

## Defaults

Codex selects a custom default through top-level `model_provider` and `model`.
`defaults.model: "provider/model"` maps to those two fields; a default that
names a provider that is not emitted is skipped with a warning. This is separate
from the replacement `model_catalog_json` mechanism, which agentcfg does not
generate from the common model IR.

## Sources

- https://developers.openai.com/codex/config-reference (replaces the retired mintlify URL)
- https://raw.githubusercontent.com/openai/codex/main/codex-rs/core/config.schema.json (`WireApi` is `responses` only; `ModelProviderInfo.experimental_bearer_token`; `RawMcpServerConfig`)
- Local `codex mcp add --help` (0.153.4)

The schema also documents provider `auth.command` (command-backed bearer tokens; mutually exclusive with `env_key`, `experimental_bearer_token`, and `requires_openai_auth`), `query_params`, AWS signing helpers, websocket flags, and MCP `auth` (`oauth` | `chatgpt`). Those are runtime enrollment or have no IR field, and are not emitted.

## Reasoning variants

Codex keeps per-model availability in a separate model catalog and active effort
in a session/top-level setting, not in `model_providers`. Therefore a non-empty
agentcfg `models[].variants` list is omitted. A complete catalog would carry
`supported_reasoning_levels`, but generating it from the common IR would discard
required catalog metadata and replace Codex's built-in catalog, so agentcfg
does not attempt it.
