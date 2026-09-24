# Qwen Code

- **Target:** `qwen-code`
- **Artifact:** `~/.qwen/settings.json` fragment.
- **Verified:** CLI `0.24.5` and source commit `ffea2d024e529b8345cc6d2815e51c1e2d3e0a11` on 2026-09-25.

IR providers become named `modelProviders` buckets. `providerProtocol` maps each
bucket to `openai` or `anthropic`. OpenAI entries explicitly select
`wireApi: chat-completions` or `responses`. Model IDs remain unchanged; endpoints,
names and credential references use `baseUrl`, `name` and `envKey`.

Literal credentials go into generated, provider-specific `settings.env` entries
referenced by `envKey`. Environment credentials retain the original variable
name. Headers and MCP values use `${NAME}` / `Bearer ${NAME}`. Literal values
containing `$` are skipped with warnings to avoid accidental substitution.
Unsupported auth overrides receive the shared diagnostic.

Limits map to `generationConfig.contextWindowSize` and
`generationConfig.samplingParams.max_tokens`; provider headers map to
`generationConfig.customHeaders`. Image, audio and video input flags map to
`generationConfig.modalities`. PDF, output modalities, tool/reasoning capability
flags and selectable variants are skipped with warnings.

The default sets `model.name`, the effective `security.auth.selectedType`, and
`security.auth.baseUrl` to disambiguate endpoints. Although the latter field is
deprecated for credential setup, the inspected resolver still uses it for route
selection. Qwen deduplicates routes by protocol, model ID and URL, not provider
bucket; subsequent duplicates are skipped with warnings.

MCP uses `mcpServers`: stdio `command`, `args`, `env`, `cwd`; HTTP `httpUrl` and
`headers`; millisecond `timeout`. Disabled server IDs go into `mcp.excluded`.

Isolated CLI requests reached local capture endpoints for all three protocols,
with the configured model and credential. For Anthropic, use the API root:
Qwen appends `/v1/messages`, so a base already ending in `/v1` produces a doubled
`/v1/v1/messages`. The emitter preserves the supplied base URL.

Sources: [model providers](https://github.com/QwenLM/qwen-code/blob/main/docs/users/configuration/model-providers.md),
`packages/core/src/models/modelRegistry.ts`, and `config/mcp-server-config.ts`.
