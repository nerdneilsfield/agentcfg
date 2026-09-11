# Grok Build

- **Target id:** `grok`
- **Verified against:** Grok Build local install + upstream docs (research report `.research/grok.md`, fetched 2026-09-07).
- **Native file:** `~/.grok/config.toml`.
- **v1 artifact:** TOML fragment. v1 does not edit this file.

## Provider route

An IR literal such as `api_key: "example-key"` is emitted in the native
`api_key` field. The example key is a placeholder; real literals also appear in
generated output.

Grok Build uses per-model TOML tables `[model."<model-id>"]` plus a `[models]` default selector. Each model table carries its own connection fields:

```toml
[models]
default = "glm-5.3"

[model."glm-5.3"]
model = "glm-5.3"
base_url = "https://example.com/v1"
name = "GLM-5.3"
env_key = "VOLC_API_KEY"
api_backend = "responses"
context_window = 128000
max_completion_tokens = 8192
```

`api_backend` is one of `chat_completions`, `responses`, `messages`, so all three IR protocols map 1:1. `env_key` is a native environment-variable name — the IR `api_key: "ENV:NAME"` maps directly without resolving the reference.

Grok has no provider table: every model repeats `base_url`/`env_key`. The emitter therefore rejects duplicate model IDs across IR providers, because both would emit the same `[model."<id>"]` table.

`extra_headers` is a literal string map with no documented environment expansion, so IR `ENV:NAME` / `Bearer ENV:NAME` provider headers are rejected; literal headers are emitted.

## Capabilities

Grok model tables have no capability/modality fields. IR `input`/`output`/`reasoning`/`tool_calling` are validated as references but not emitted (same precedent as Codex). This is documented here so the omission is explicit.

## MCP

Grok Build uses `[mcp_servers.<id>]` with either stdio fields (`command`, `args`, `env`, `cwd`) or http fields (`url`, `headers`, `bearer_token_env_var`).

Upstream documents `${VAR}` expansion for MCP string fields, so the emitter renders IR `ENV:NAME` references as `"${VAR}"` literals in MCP `env`/`headers`. `Authorization: "Bearer ENV:NAME"` maps to the native `bearer_token_env_var`.

## Defaults

`defaults.model` maps to `[models] default = "<model-id>"` (the model ID part after the provider prefix). Validation rejects a default that does not resolve to an emitted model table.
