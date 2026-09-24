# OpenCode

- **Target id:** `opencode`
- **Verified against:** installed `opencode v2.0.12` on 2026-09-23 — `opencode debug config` (discovers, parses, and normalizes the emitted fragment) plus a local capture server for wire behavior: all three provider packages load and honor `settings.baseURL`, `env: [NAME]` reaches the endpoint as `Authorization: Bearer <value>`, and a provider `headers` value of `{env:NAME}` arrives expanded. Docs: `https://opencode.ai/v2/docs/{config,mcp-servers,providers,models,migrate-v1,instructions}` (fetched 2026-09-23).
- **Native file:** `opencode.json` / `opencode.jsonc`. V2 also reads `~/.config/opencode/opencode.json(c)`, `<project>/opencode.json(c)`, and `<project>/.opencode/opencode.json(c)`.
- **v1 artifact:** one JSON fragment in the native **V2** shape. v1 does not edit this file.
- **Schema snapshot:** [`opencode.config.schema.json`](opencode.config.schema.json), downloaded from `https://opencode.ai/config.json`.

## Version scope

This target emits the native V2 shape only. OpenCode V2 keeps reading supported
V1 configuration by normalizing it in memory, but this emitter no longer writes
the V1 spelling: `provider`/`npm`/`options` and a flat `mcp.<name>` map are not
emitted.

The published schema at `https://opencode.ai/config.json` was re-fetched on
2026-09-23 and is still byte-identical to the vendored snapshot, which describes
the **V1** shape (`provider`, `mcp.<name>`, `enabled`, `additionalProperties:
false`). The V2 docs still name that URL as the schema, so the fragment keeps
`$schema`; expect an editor to flag native V2 keys until the schema is updated.

## Provider route

IR:

```yaml
providers:
  - id: volcengine
    name: Volcengine
    protocol: openai-completions
    base_url: https://example.com/v1
    api_key: "ENV:VOLC_API_KEY"
    headers:
      X-Tenant: "engineering"
    models:
      - id: glm-5.3
        name: GLM-5.3
        context_window: 128000
        max_output_tokens: 8192
        input: [text, image]
        output: [text]
        tool_calling: true
        variants: [low, high]
```

Native:

```json
{
  "providers": {
    "volcengine": {
      "name": "Volcengine",
      "env": ["VOLC_API_KEY"],
      "package": "@opencode/ai/providers/openai-compatible",
      "settings": { "baseURL": "https://example.com/v1" },
      "headers": { "X-Tenant": "engineering" },
      "models": {
        "glm-5.3": {
          "name": "GLM-5.3",
          "capabilities": {
            "tools": true,
            "input": ["text", "image"],
            "output": ["text"]
          },
          "limit": { "context": 128000, "output": 8192 },
          "variants": [
            { "id": "low", "settings": { "reasoningEffort": "low" } },
            { "id": "high", "settings": { "reasoningEffort": "high" } }
          ]
        }
      }
    }
  }
}
```

Every IR protocol maps to its native runtime package, so no protocol is
rejected:

| IR `protocol` | Native `package` |
|---|---|
| `openai-completions` | `@opencode/ai/providers/openai-compatible` |
| `openai-responses` | `@opencode/ai/providers/openai` |
| `anthropic-messages` | `@opencode/ai/providers/anthropic` |

The V2 docs also list `@opencode/ai/providers/openai-compatible/responses` and
`@opencode/ai/providers/anthropic-compatible`, and the SDK carries both. The
pinned v2.0.12 bundle does not: either name fails with `Cannot initialize
<provider>/<model>: Cannot find package '@opencode/ai'`, while `openai` and
`anthropic` load and send to `settings.baseURL` on `/v1/responses` and
`/v1/messages` respectively. Revisit the two names once a bundle ships that
resolves them. A request-level recheck on v2.0.16 (2026-09-25) reproduced
the same two package-loading failures. The emitted `openai` and `anthropic`
packages still reached a local capture server at `/v1/responses` and
`/v1/messages`, respectively, with the configured model ID.

`package` is load-bearing and unvalidated: OpenCode accepts any string, lists the
provider's models, and fails only when a request is made. Keep the values above
verbatim.

`base_url` becomes `settings.baseURL`. `api_key: "ENV:NAME"` becomes the native
credential reference `"env": ["NAME"]`; a literal API key becomes
`settings.apiKey`. `auth_type: none` writes neither, leaving the provider
keyless. A literal `api_key` that spells a native expression (`{env:...}`,
`{file:...}`) is skipped with a warning instead of being emitted, because
OpenCode would resolve it again as a reference.

Provider `headers` string values, including environment references, are written
as given. V2 expands `{env:NAME}` in a header value before sending it (verified
against a capture server on v2.0.12: `"X-Gateway-Key": "{env:GATEWAY_KEY}"`
arrived as the variable's value, and as an empty string when the variable was
unset). The closed request `anomalyco/opencode#28527` concerned the V1
`options.headers` field only.

## Models

| IR | Native | Note |
|---|---|---|
| `id` | map key | `modelID` is omitted because the selectable id and the wire id are the same. |
| `name` | `name` | |
| `context_window` | `limit.context` | |
| `max_output_tokens` | `limit.output` | |
| `input`, `output` | `capabilities.input`, `capabilities.output` | Emitted only when the IR sets them; V2's documented fallback is text and image input, text output. |
| `tool_calling` | `capabilities.tools` | Emitted only when the IR sets it. |
| `variants` | `variants[].id` + `variants[].settings.reasoningEffort` | Names pass through unchanged; V2 has no fixed ladder. |
| `reasoning` | — | Skipped with a warning: V2 has no model reasoning flag and lists the V1 field as accepted but unsupported. Reasoning is declared through `variants`. |

## MCP

IR:

```yaml
mcp:
  - id: context7
    transport: stdio
    command: [npx, -y, "@upstash/context7-mcp"]
    env:
      CONTEXT7_API_KEY: "ENV:CONTEXT7_API_KEY"
    timeout_ms: 30000
  - id: github
    transport: http
    url: https://api.githubcopilot.com/mcp/
    headers:
      Authorization: "Bearer ENV:GITHUB_TOKEN"
    enabled: false
```

Native:

```json
{
  "mcp": {
    "servers": {
      "context7": {
        "type": "local",
        "command": ["npx", "-y", "@upstash/context7-mcp"],
        "environment": { "CONTEXT7_API_KEY": "{env:CONTEXT7_API_KEY}" },
        "timeout": { "catalog": 30000, "execution": 30000 }
      },
      "github": {
        "type": "remote",
        "url": "https://api.githubcopilot.com/mcp/",
        "headers": { "Authorization": "Bearer {env:GITHUB_TOKEN}" },
        "disabled": true
      }
    }
  }
}
```

Servers live under `mcp.servers`. `stdio` becomes `local` with the argv array,
the working directory, and `environment`; `http` becomes `remote` with `url` and
`headers`. Both keep the documented `{env:NAME}` substitution. `enabled: false`
becomes `disabled: true`. `timeout_ms` becomes the V2 timeout object with both
`catalog` and `execution` set, the mapping the official V1 migration uses.

## Defaults

`defaults.model` becomes the root `model` field, verbatim as `provider/model`.
The root selection does not retain a `#variant` in V2; select a variant per
session, agent, or command.

## Sources

- https://opencode.ai/v2/docs/config
- https://opencode.ai/v2/docs/mcp-servers
- https://opencode.ai/v2/docs/providers
- https://opencode.ai/v2/docs/models
- https://opencode.ai/v2/docs/migrate-v1
- https://github.com/anomalyco/opencode/issues/28527 (V1 `options.headers` never expanded `{env:VAR}`; V2 provider headers do)
- installed `opencode v2.0.12`: `opencode debug config`, a local capture server for request headers and provider packages
