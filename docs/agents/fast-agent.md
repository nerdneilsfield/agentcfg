# fast-agent

- **Target id:** `fast-agent`
- **Verified against:** `evalstate/fast-agent` commit `77b1b517318f0a21f94b7c924c1053fb5ee5e040` and the native settings/overlay loader in `fast-agent-mcp 0.10.33` (2026-09-25).
- **Artifacts:** `fastagent.config.yaml`, one manifest per model under `.fast-agent/model-overlays/`, and `.fast-agent/model-overlays.secrets.yaml` when literal keys are present.

## Providers and models

Each model becomes a native model overlay. This preserves separate endpoints
and credentials even when several IR providers use the same protocol or model ID.

| IR protocol | Overlay provider |
|---|---|
| `openai-completions` | `generic` |
| `openai-responses` | `openresponses` |
| `anthropic-messages` | `anthropic` |

The overlay carries `connection.base_url`, the unchanged wire model ID, and a
picker label from the model name (or its qualified IR reference). Overlay names
are `agentcfg-` followed by the hex-encoded `provider/model` reference. Filenames
use a SHA-256 digest of that name to stay bounded and stable. `defaults.model`
selects the corresponding overlay through `default_model`.

Environment credentials use `connection.auth: env` and `api_key_env`. Literal
keys use `auth: secret_ref` with the key in the companion secrets fragment.
`auth_type: none` sets `auth: none`. Provider headers use `default_headers`;
environment references become `${NAME}` or `Bearer ${NAME}`. Native YAML loading
expands these expressions, including in overlay and secrets files. Literal
credentials or headers containing `${` are skipped with a warning.

Context and output limits become `metadata.context_window` and
`metadata.max_output_tokens`; the output limit also sets `defaults.max_tokens`.
Modality, tool-calling, reasoning-capability, and selectable variant metadata
have no direct mapping and are skipped with warnings.

## MCP

`mcp.servers` supports stdio commands, arguments, environment values, working
directories, and HTTP URLs and headers. Environment references use `${NAME}`.
Whole-second IR timeouts become `read_timeout_seconds`; fractional-second values
are skipped rather than rounded. Disabled servers are omitted with a warning:
`load_on_start: false` only defers connection and does not disable a server.

## Installing the fragments

Merge the config and secrets fragments into their existing files, and place the
overlay manifests under the active fast-agent home. Suggested paths assume the
default project-local `.fast-agent` home; adjust them when using `--home`.
Generation writes only stdout. When removing models from the IR, remove their
old generated overlay files separately so fast-agent stops discovering them.

## Sources

- https://github.com/evalstate/fast-agent/blob/77b1b517318f0a21f94b7c924c1053fb5ee5e040/src/fast_agent/llm/model_overlays.py
- https://github.com/evalstate/fast-agent/blob/77b1b517318f0a21f94b7c924c1053fb5ee5e040/src/fast_agent/config.py
- https://github.com/evalstate/fast-agent/blob/77b1b517318f0a21f94b7c924c1053fb5ee5e040/docs/docs/models/model_overlays.md
