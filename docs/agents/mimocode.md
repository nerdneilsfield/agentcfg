# MiMo Code

- **Target id:** `mimocode`
- **Verified against:** official Xiaomi MiMo Code live JSON schema (`https://mimo.xiaomi.com/mimocode/config.json`, 268 KB), official docs, and repo source `config/variable.ts` (research report `.research/mimocode.md`, fetched 2026-09-07; repo `github.com/XiaomiMiMo/MiMo-Code`, MIT; binary `mimo`, npm `@mimo-ai/cli`).
- **Native file:** `~/.config/mimocode/mimocode.jsonc` (or project `mimocode.json(c)`; relocatable via `$MIMOCODE_HOME`).
- **v1 artifact:** single JSON fragment (valid JSON, safe to store as `.jsonc`).

Note: the npm package `mimocode` by `eurocybersecurite` is an unrelated name collision and is not this target.

## Provider route

An IR literal such as `api_key: "example-key"` is emitted in the native
`options.apiKey` field. The example key is a placeholder; real literals also appear in
generated output.

MiMo Code (an opencode fork) uses AI SDK provider packages selected by an `npm` field:

```json
{
  "provider": {
    "volcengine": {
      "npm": "@ai-sdk/openai-compatible",
      "name": "Volcengine",
      "options": {
        "baseURL": "https://example.com/v1",
        "apiKey": "{env:VOLC_API_KEY}",
        "headers": { "X-Tenant": "engineering" }
      },
      "models": {
        "glm-5.3": {
          "name": "GLM-5.3",
          "limit": { "context": 128000, "output": 8192 },
          "modalities": { "input": ["text"], "output": ["text"] },
          "reasoning": true,
          "tool_call": true
        }
      }
    }
  },
  "model": "volcengine/glm-5.3"
}
```

Protocol mapping: `openai-completions` → `@ai-sdk/openai-compatible`, `anthropic-messages` → `@ai-sdk/anthropic`. There is no protocol enum — the `api` wire protocol is implied by the npm package. IR `openai-responses` is not representable for custom providers (built-in loaders only) and is rejected.

**Environment interpolation is the key advantage of this target:** MiMo Code substitutes `{env:VAR}` and `{file:path}` across the entire raw config text before parsing (verified in `config/variable.ts`). Therefore:

- IR `api_key: "ENV:NAME"` → `"apiKey": "{env:VAR}"` (native environment reference).
- IR `ENV:NAME` headers → `"{env:VAR}"` values.
- IR `Bearer ENV:NAME` → `"Bearer {env:VAR}"`.
- MCP env `ENV:NAME` → `"{env:VAR}"` values.

An unset variable expands to the empty string.

## MCP

MiMo Code uses native `type` values `local` and `remote` (the `stdio`/`http` spellings exist only in the Claude-import compatibility layer):

```json
{
  "mcp": {
    "context7": {
      "type": "local",
      "command": ["npx", "-y", "@upstash/context7-mcp"],
      "enabled": true,
      "environment": { "CONTEXT7_API_KEY": "{env:CONTEXT7_API_KEY}" }
    },
    "github": {
      "type": "remote",
      "url": "https://api.githubcopilot.com/mcp/",
      "enabled": true,
      "headers": { "Authorization": "Bearer {env:GITHUB_TOKEN}" }
    }
  }
}
```

### MCP timeout

IR `timeout_ms` maps unchanged to native `mcp.<id>.timeout` for both `local`
and `remote` servers. IR `cwd` has no MiMo Code MCP field and is rejected.
A stdio server that sets `cwd: /var/lib/context7` validates for OpenCode and
fails for MiMo Code.

## Defaults

`defaults.model` maps to the top-level `"model": "provider/model"` string verbatim. MiMo Code's `small_model` / `vision_model` selectors have no IR equivalent and are not emitted.

## Reasoning variants

MiMo Code directly supports `models.<id>.variants`. agentcfg emits one native
variant per source name with only `reasoningEffort` set to that name. It does
not copy unrelated native fields such as `textVerbosity`, `reasoningSummary`,
or `include`. Provider/model names such as `ultra` are preserved.
