# Command Code

- **Target id:** `commandcode`
- **Verified against:** installed `Command Code v1.64.0` (`cmd`, `command-code` npm package, inspected 2026-09-23): the BYOK parser (`parseProvidersConfig`, `parseProvider`, `parseModel`) and writer (`addUserProviderEntry`, `toWrittenModel`), the MCP config store (`parseStoredMcpServer`, `getMcpConfigPath`), the reserved provider ids and the credential resolver; plus the official docs `https://commandcode.ai/docs/byok` and `https://commandcode.ai/docs/mcp` (fetched 2026-09-23).
- **Native files:** `~/.commandcode/providers.json` (BYOK providers, user scope only), `.mcp.json` at the project root (project scope) with `~/.commandcode/mcp.json` (user) and `~/.commandcode/projects/<slug>/mcp.json` (local) as the other scopes, and `~/.commandcode/config.json` (user settings).
- **v1 artifact:** three JSON fragments: `providers.json`, `.mcp.json`, and `config.json`. v1 does not edit these files.

## Provider route

`~/.commandcode/providers.json` is a single map keyed by provider id. BYOK has no
project scope: the docs state a project or repository cannot define providers.

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
        reasoning: true
        variants: [low, high, ultra]
```

Native:

```json
{
  "provider": {
    "volcengine": {
      "name": "Volcengine",
      "baseURL": "https://example.com/v1",
      "apiKey": "$VOLC_API_KEY",
      "headers": { "X-Tenant": "engineering" },
      "models": {
        "glm-5.3": {
          "name": "GLM-5.3",
          "contextWindow": 128000,
          "maxOutput": 8192,
          "reasoning": true,
          "reasoningEfforts": ["low", "high"]
        }
      }
    }
  }
}
```

`provider` is the canonical map key: the CLI's own writers present both
`provider` and `providers` in the source file and create entries under
`provider`. `models` is an object keyed by model id, not an array.

The native `api` field takes `openai-completions` (default), `openai-responses`,
and `anthropic-messages`, so all three IR protocols map 1:1. For
`openai-completions` the field is omitted, matching the native writer, which
writes `api` only when it differs from the default.

`ENV:NAME` becomes `"$NAME"`. A literal `api_key` is skipped with a warning: the
native parser refuses a raw secret in this file (`raw secrets don't belong in
providers.json`), keeps the provider, and ignores the key, so emitting it would
lose the credential anyway. Store the key with `/connect` instead, which writes
`~/.commandcode/auth.json`.

Provider `name` is emitted only when the IR sets one; Command Code falls back to
the provider id. Provider `headers` are written literally. Command Code
interpolates `{env:VAR}` / `$VAR` / `!command` in `apiKey` only, so an
environment-derived header (`ENV:NAME`, `Bearer ENV:NAME`) is skipped with a
warning rather than sent to the endpoint as written.

`auth_type: none` becomes `"apiKey": false`, the native keyless form. `bearer`
has no native field, so it is skipped with a warning. An absent `api_key` on an official
provider emits no credential field at all, because a key stored by `/connect`
takes precedence over the file.

Six provider ids are reserved: the built-in subscription lanes `anthropic`,
`github-copilot`, `codex`, and `command-code`, plus their reserved aliases
`copilot` and `openai`. An entry with one of them is skipped by the native
parser, so the emitter skips the entry too and warns, naming the `<id>-api`
rename the CLI itself suggests.

## Model fields

| IR | Native | Note |
|---|---|---|
| `id` | map key | The reference is `provider-id/model-id`. |
| `name` | `name` | Omitted when unset; the native default is the id. |
| `context_window` | `contextWindow` | `limit.context` is an accepted alias; this emitter writes the canonical name. |
| `max_output_tokens` | `maxOutput` | `limit.output` is an accepted alias. |
| `reasoning` | `reasoning` | An explicit `false` is preserved; native `true` assumes `low`/`medium`/`high`. |
| `variants` | `reasoningEfforts` | See below. |
| `input`, `output` | — | No native modality field; skipped with a warning. |
| `tool_calling` | — | No native per-model tool field; skipped with a warning. |

## Reasoning variants

`variants` emits `reasoningEfforts`. The native set is exactly `low`, `medium`,
`high`, `xhigh`, and `max`; a declared name outside that ladder (`ultra`, for
example) is skipped, never remapped, and named in a warning — the native parser
drops the same name with its own warning. The rest of the list is still written
in the native order. When no declared name is in the ladder, `reasoningEfforts`
is left out and the model's `reasoning` field is still emitted.

## MCP

IR:

```yaml
mcp:
  - id: context7
    transport: stdio
    command: [npx, -y, "@upstash/context7-mcp"]
    env:
      CONTEXT7_API_KEY: "ENV:CONTEXT7_API_KEY"
  - id: github
    transport: http
    url: https://api.githubcopilot.com/mcp/
    headers:
      Authorization: "Bearer ENV:GITHUB_TOKEN"
```

Native (`.mcp.json`):

```json
{
  "mcpServers": {
    "context7": {
      "transport": "stdio",
      "enabled": true,
      "command": "npx",
      "args": ["-y", "@upstash/context7-mcp"],
      "env": { "CONTEXT7_API_KEY": "${CONTEXT7_API_KEY}" }
    },
    "github": {
      "transport": "http",
      "enabled": true,
      "url": "https://api.githubcopilot.com/mcp/",
      "headers": { "Authorization": "Bearer ${GITHUB_TOKEN}" }
    }
  }
}
```

The suggested path is the project-scope `.mcp.json`, which the docs describe as
the file meant to be committed. `~/.commandcode/mcp.json` holds the same shape
for the user scope, and `~/.commandcode/projects/<slug>/mcp.json` for the local
scope; precedence is local, then project, then user.

The IR argv array splits into the native `command` string and `args` array.
`transport` is the canonical field (`type` is a native alias). Environment and
header values use the native `${NAME}` form, which Command Code resolves at
runtime for stdio `env` and http `headers`; `enabled` defaults to `true`.

MCP `cwd` and `timeout_ms` have no native field in the stored server shape and
are skipped with a warning.

## Defaults

`defaults.model` becomes the `model` setting in `~/.commandcode/config.json`,
which exists at user scope only, written verbatim as one qualified reference:

```json
{ "model": "volcengine/glm-5.3" }
```

That is the shape Command Code's own settings writer produces: `cmd config set
model zai-org/glm-5.3 --scope user` persists `{"model": "zai-org/GLM-5.3"}`, and
the reader takes the value verbatim (`readDiskDefaultModelFromConfig` returns
the stored `model`). `provider` is not a configuration setting (`cmd config get
provider` reports "Unknown configuration setting: provider"), so v1 does not
write one; the model picker's own pair key is `modelProvider`, which this
fragment also leaves alone.

Two cautions belong with this fragment:

- `cmd config set model` validates its value against Command Code's own catalog
  and refuses a BYOK id: declaring a provider in `providers.json` makes its
  models selectable in `/model` and listed by `--list-models`, but the setting
  writer still reports `Unknown model "volcengine/glm-5.3"` for the same id.
  Merge the fragment by hand, or select the model once with `/model`.
- The file also holds unrelated state such as `installed` and
  `reasoningEffort`, so this is a fragment to merge into an existing file.

## Sources

- https://commandcode.ai/docs/byok
- https://commandcode.ai/docs/mcp
- installed `command-code` v1.64.0 (`dist/cli.mjs`): BYOK provider and MCP config parsers, `addUserProviderEntry`, `toWrittenModel`, `getMcpConfigPath`, reserved-id sets
