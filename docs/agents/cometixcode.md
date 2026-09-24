# CometixCode

- **Target id:** `cometixcode`
- **Verified against:** the repository `Haleclipse/CometixCode` (master, read 2026-09-23): `src/utils/config.rs` (config home and `~/.claude.json`), `src/utils/settings/constants.rs` and `src/utils/settings/types.rs` (settings sources, the published schema URL, `env`/`model`/`apiKeyHelper`/`modelOverrides`), and `src/services/mcp/config.rs` (`.mcp.json` and `mcpServers` parsing, entry validation). Field semantics come from the Claude Code docs CometixCode mirrors: settings, environment variables, and MCP. CometixCode has no installed binary on this machine, so the contract is source- and docs-based.
- **Native files:** `~/.claude/settings.json` (user), `.claude/settings.json` (shared project), `.claude/settings.local.json` (project local), `~/.claude.json` (CometixCode's own state, including user-scope `mcpServers`), and `.mcp.json` (project-scope MCP servers). `CLAUDE_CONFIG_DIR` relocates the `~/.claude` directory.
- **v1 artifact:** two JSON fragments: `settings.json` and `.mcp.json`. v1 does not edit these files.

CometixCode is a Rust reimplementation of Anthropic's Claude Code terminal UI,
so it reads Claude Code's configuration files. Custom model providers are
configured through **environment variables**, not a provider catalog: one
Anthropic-compatible endpoint at a time.

## Provider route

IR:

```yaml
providers:
  - id: claude-proxy
    protocol: anthropic-messages
    base_url: https://anthropic-proxy.example.com
    api_key: "sk-example-not-a-real-key"
    headers:
      X-Tenant: "engineering"
defaults:
  model: claude-proxy/glm-5.3
```

Native (`~/.claude/settings.json`):

```json
{
  "$schema": "https://json.schemastore.org/claude-code-settings.json",
  "env": {
    "ANTHROPIC_API_KEY": "sk-example-not-a-real-key",
    "ANTHROPIC_BASE_URL": "https://anthropic-proxy.example.com",
    "ANTHROPIC_CUSTOM_HEADERS": "X-Tenant: engineering",
    "ANTHROPIC_MODEL": "glm-5.3"
  }
}
```

| IR | Native | Note |
|---|---|---|
| `protocol` | — | Only `anthropic-messages` is served; another protocol is skipped with a warning. |
| `base_url` | `env.ANTHROPIC_BASE_URL` | |
| `auth_type: official` + literal `api_key` | `env.ANTHROPIC_API_KEY` | Sent as `x-api-key`. |
| `auth_type: bearer` + literal `api_key` | `env.ANTHROPIC_AUTH_TOKEN` | Sent as `Authorization: Bearer`. |
| `auth_type: none` | — | Neither variable is written. |
| literal `headers` | `env.ANTHROPIC_CUSTOM_HEADERS` | One newline-separated `Name: Value` string, sorted by name. |
| `defaults.model` | `env.ANTHROPIC_MODEL` | The model id half of the reference. |

One endpoint is configurable, so exactly one provider is emitted: the provider
the default model names, otherwise the first provider on the Anthropic wire.
Every other provider is skipped with a warning. `name` has no native field.

`env` values are literal — Claude Code writes each entry into the process
environment as given, and nothing in the apply path expands `${VAR}` — so an
`api_key: "ENV:NAME"` reference is skipped with a warning instead of being
emitted as text. Export the credential in the shell under `ANTHROPIC_AUTH_TOKEN`
(or `ANTHROPIC_API_KEY`), or use a literal key.

The one native mechanism for a dynamic credential is `apiKeyHelper`, a script
path whose stdout becomes the value. It is not used here: its output is sent as
`Authorization: Bearer`, so it cannot stand in for the `official` injection, and
its application is gated in the client, which would leave interactive sessions
without a credential. An environment-derived provider header is skipped for the
same literalness reason: `ANTHROPIC_CUSTOM_HEADERS` is a plain string.

## Models

Per-model facts have no native home: `context_window`, `max_output_tokens`,
`input`/`output`, `reasoning`, `variants`, and `tool_calling` are skipped with a
warning. The model picker is driven by CometixCode's own catalog plus
`ANTHROPIC_MODEL`, and `effortLevel` is a persisted *selection*, not an
availability list, so `variants` is not reduced to it.

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
    enabled: false
```

Native (`.mcp.json`):

```json
{
  "mcpServers": {
    "context7": {
      "command": "npx",
      "args": ["-y", "@upstash/context7-mcp"],
      "env": { "CONTEXT7_API_KEY": "${CONTEXT7_API_KEY}" }
    },
    "github": {
      "type": "http",
      "url": "https://api.githubcopilot.com/mcp/",
      "headers": { "Authorization": "Bearer ${GITHUB_TOKEN}" }
    }
  }
}
```

The suggested path is the project-scope `.mcp.json`; `~/.claude.json` holds the
user scope and `.claude/settings.json` is the shared project settings file. A
stdio entry is identified by its `command`, while a remote entry carries an
explicit `"type": "http"` and a `url`. `.mcp.json` expands `${NAME}` in values,
so IR environment references stay references.

`enabled: false` cannot live in the entry. It becomes the server id in
`disabledMcpjsonServers` in the settings fragment, which is the settings key
that rejects servers coming from `.mcp.json`.

MCP `cwd` and `timeout_ms` are skipped with warnings: a `.mcp.json` entry has no
working directory or timeout field, and `MCP_TIMEOUT` is a process variable.

## Defaults

`defaults.model` selects the emitted provider and sets `ANTHROPIC_MODEL` to the
model id half of the reference. A default naming a provider CometixCode cannot
serve is skipped with a warning, because no endpoint could carry it.

## Sources

- https://github.com/Haleclipse/CometixCode — `src/utils/config.rs`, `src/utils/settings/constants.rs`, `src/utils/settings/types.rs`, `src/services/mcp/config.rs`
- https://code.claude.com/docs/en/settings
- https://code.claude.com/docs/en/env-vars
- https://code.claude.com/docs/en/mcp
- https://json.schemastore.org/claude-code-settings.json
