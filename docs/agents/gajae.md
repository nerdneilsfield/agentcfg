# Gajae Code (gjc)

- **Target id:** `gajae`
- **Verified against:** official repo `Yeachan-Heo/gajae-code` (HEAD f238c66, v0.16.6; research report `.research/gajae.md`, fetched 2026-09-07). The npm package is `gajae-code` (bin `gjc` / Korean `가재씨`). Gajae is pre-1.0 with rapid releases — schema facts here are pinned to v0.16.6.
- **Native files:** `~/.gjc/agent/models.yml` (strict zod), `~/.gjc/agent/mcp.json`, `~/.gjc/agent/config.yml`. `GJC_CONFIG_DIR` overrides `~/.gjc`.
- **v1 artifacts:** YAML fragment (`models.yml`), a small `config.yml` fragment carrying only `modelRoles.default`, and `mcp.json` when MCP servers exist. v1 does not edit these files.

## Provider route

`models.yml` uses `providers.<id>` with camelCase keys:

```yaml
providers:
  volcengine:
    baseUrl: https://example.com/v1
    api: openai-completions
    apiKeyEnv: VOLC_API_KEY
    headers:
      X-Gateway-Key: GATEWAY_KEY
    models:
      - id: glm-5.3
        name: GLM-5.3
        contextWindow: 128000
        maxTokens: 8192
        input: [text, image]
        output: [text]
        reasoning: true
```

- `api` is a 12-value enum including `openai-completions`, `openai-responses`, and `anthropic-messages`, so all three IR protocols map 1:1 (no protocol rejection).
- `apiKeyEnv` is native; IR `api_key_env` maps directly.
- Gajae resolves header and credential values with **env-name-or-literal** semantics: a value equal to a set environment variable's name is replaced by that variable's value, otherwise kept as a literal. The emitter renders IR `from_env` as the bare env var name and documents the ambiguity risk. IR `bearer_from_env` has no native equivalent for providers (no prefix is added), so it is rejected.
- `models` is an array of `{id, name, contextWindow, maxTokens, input, output, reasoning, thinking, compat}`; modalities are limited to `text` and `image`, so IR models with other input/output modalities are rejected. Provider display `name` is not in the strict provider schema and is not emitted. IR `tool_calling` has no direct field (only `compat` flags) and is not emitted.
- `models.yml` has no `${VAR}` interpolation; env references ride on env-name-or-literal semantics.

## Defaults

`defaults.model "provider/model"` maps to `config.yml`'s
`modelRoles.default` string. Gajae also supports role entries (`executor`,
`architect`, ...) which the IR does not model.

## MCP

`mcp.json` uses `mcpServers.<id>` with `type: stdio|http|sse`, stdio fields
(`command`, `args`, `env`, `cwd`, `timeout` in **ms**), http fields (`url`,
`headers`), `enabled`, and `protocol`. `${VAR}` expansion is native at
discovery, so IR `from_env` renders as `"${VAR}"` and
`Authorization.bearer_from_env` renders as `Bearer ${VAR}`. IR `timeout_ms`
maps 1:1 (no conversion).
