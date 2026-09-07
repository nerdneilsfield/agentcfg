# Codex CLI

- **Target id:** `codex`
- **Verified against:** local `codex-cli 0.153.4`; official configuration reference fetched 2026-09-07.
- **Native file:** `$CODEX_HOME/config.toml`, normally `~/.codex/config.toml`.
- **v1 artifact:** TOML fragment. v1 does not edit this file.

## Provider route

Codex uses `[model_providers.<id>]`. The official reference documents `name`, `base_url`, `env_key`, `wire_api`, `http_headers`, `env_http_headers`, and `query_params`.

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

```toml
[mcp_servers.context7]
command = "npx"
args = ["-y", "@upstash/context7-mcp"]
env_vars = ["CONTEXT7_API_KEY"]

[mcp_servers.github]
url = "https://api.githubcopilot.com/mcp/"
bearer_token_env_var = "GITHUB_TOKEN"
```

The reference documents `command`, `args`, `url`, `enabled`, `required`, `env`, `env_vars`, `cwd`, `bearer_token_env_var`, `http_headers`, `env_http_headers`, `startup_timeout_sec`, and `tool_timeout_sec`. v1 maps a same-name environment reference through `env_vars`, a non-secret child value through `env`; a renamed environment reference is rejected because `env_vars` inherits only same-name variables. It maps `Authorization.bearer_from_env` through `bearer_token_env_var`; other static and environment-derived HTTP headers map to `http_headers` and `env_http_headers`. No secret value is ever rendered.

## Defaults

Codex's top-level `model` selects a model string, but a custom provider also requires the top-level `model_provider`. This mapping needs a live custom-provider smoke test before `defaults.model` is enabled for this target. v1 validates but does not emit defaults for Codex.

## Sources

- https://openai-codex.mintlify.app/configuration/reference
- Local `codex mcp add --help` (0.153.4)
