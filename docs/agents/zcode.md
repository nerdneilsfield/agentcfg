# ZCode

- **Target id:** `zcode`
- **Verified against:** locally installed ZCode v3.11.2 bundled zod schema (research report `.research/zcode.md`, 2026-09-07). ZCode is Z.ai (智谱) closed-source Electron app + headless CLI.
- **Native file:** `~/.zcode/cli/config.json`.
- **v1 artifact:** single JSON fragment.

## Provider route

ZCode uses `provider.<id>` objects with a `kind` discriminator, `options`, and an inline `models` catalog:

```json
{
  "provider": {
    "volcengine": {
      "kind": "openai-compatible",
      "name": "Volcengine",
      "options": {
        "baseURL": "https://example.com/v1",
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
  "model": { "main": "volcengine/glm-5.3" }
}
```

`kind` is `anthropic` | `openai` | `openai-compatible`. IR `anthropic-messages` and `openai-completions` map to `anthropic` / `openai-compatible`. IR `openai-responses` has no native location for custom providers and is rejected.

`options.apiKey` is a literal string with **no** user-level `${ENV}` expansion, so IR `api_key_env` is rejected (never render a secret inline). `headers` values are literal; IR `from_env`/`bearer_from_env` provider headers are rejected.

Model `limit`, `modalities`, `reasoning`, and `tool_call` are emitted natively from the IR (`reasoning`/`tool_call` only when set).

## MCP

ZCode uses `mcp.servers.<id>` with `type` `stdio` (`command`, `args`, `env`, `cwd`) or `http`/`sse` (`url`, `headers`, `oauth`), plus shared `enabled` and `timeoutMs`.

MCP `env` and `headers` values are literal strings with no documented expansion, so IR `from_env`/`bearer_from_env` MCP references are rejected.

## Defaults

`defaults.model` maps to `model.main` verbatim — ZCode's `"provider/model"` selector syntax matches the IR string 1:1.
