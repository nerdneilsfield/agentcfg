# Kilo CLI

- **Target:** `kilo` (the CLI, not the older VS Code extension).
- **Artifact:** `~/.config/kilo/opencode.json` fragment.
- **Verified:** `@kilocode/cli 7.7.9` and `Kilo-Org/kilo` commit `6f08110d4dcc36200c084a7b4134ec040cedfd61`, 2026-09-25.

Kilo retains the OpenCode-derived singular `provider` and direct `mcp` maps.
It does not use the native OpenCode V2 format emitted by the `opencode` target.

| Protocol | Provider `npm` |
|---|---|
| Chat Completions | `@ai-sdk/openai-compatible` |
| Responses | `@ai-sdk/openai` |
| Anthropic Messages | `@ai-sdk/anthropic` |

Endpoints, credentials and headers go into provider `options`. Environment
references use `{env:NAME}`; bearer headers use `Bearer {env:NAME}`. Literal
native expressions (`{env:` or `{file:`) are skipped with warnings. Auth overrides
without a native mapping receive the shared warning.

Model metadata maps to `name`, `reasoning`, `tool_call`, `modalities`, and `limit`.
The native `limit` object requires both context and output, so incomplete pairs
are skipped with a warning. Variant lists are skipped because their request
settings depend on the selected provider. `defaults.model` becomes `model`.

MCP local entries use a command array and `environment`; remote entries use
`url` and `headers`. Both support `enabled` and millisecond `timeout`. Per-server
`cwd` is skipped with a warning because the native schema rejects it.

CLI tests in isolated homes reached local capture endpoints for Chat,
Responses and Anthropic, preserving the wire model IDs. These tests verified
routing and loading, not hosted-provider authentication.

Sources: [Kilo CLI repository](https://github.com/Kilo-Org/kilo),
`packages/opencode/src/config/config.ts`, `provider/models.ts`, and `provider/provider.ts`.
