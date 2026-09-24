# Continue

- **Target:** `continue`
- **Artifact:** `~/.continue/config.yaml` fragment.
- **Verified:** published `@continuedev/config-yaml 1.42.0`; compared with source commit `5522c6f44ca0ac3528b37244818fbfa39b5af470` on 2026-09-25.

Models use `provider: openai` for Chat Completions or `provider: anthropic` for
Messages, with `model`, `apiBase`, and a qualified `name`. Context and output
limits become `defaultCompletionOptions.contextLength` and `maxTokens`.
Credentials and request headers support literal values or Continue secret
references, `${{ secrets.NAME }}`. These resolve through Continue's secret
sources, including `.env` files and the process environment; IDE launches may
not inherit shell variables. Literal template expressions are skipped with a
warning. Auth overrides without a native mapping receive the shared warning.

Responses providers are skipped. The inspected main branch adds
`useResponsesApi`, but the published schema silently removes it; emitting it
would not reliably preserve the wire. The same published schema drops top-level
`contextLength`, which is why limits use `defaultCompletionOptions`.

Continue stores active model selection in client state. `defaults.model` is
therefore skipped with a warning. Modality, tool and reasoning capability
declarations and variant lists are also skipped rather than converted to
additive capability hints or request defaults.

MCP supports stdio `command`, `args`, `env`, `cwd`, and HTTP
`type: streamable-http`, `url`, `requestOptions.headers`. Disabled servers are
omitted with warnings. `timeout_ms` is skipped: `connectionTimeout` controls
startup, not a general tool-call timeout.

The generated config passed the published native schema and validator; checks
also confirmed that model limits survived parsing. This is schema verification,
not an IDE session or hosted model request.

Sources: [Continue repository](https://github.com/continuedev/continue),
`packages/config-yaml/src/schemas/{models,mcp/index}.ts`, and
[local secrets](https://docs.continue.dev/faqs).
