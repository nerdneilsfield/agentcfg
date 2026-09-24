# Aider

- **Target:** `aider`
- **Source:** `Aider-AI/aider` commit `5dc9490bb35f9729ef2c95d00a19ccd30c26339c`, checked 2026-09-25.
- **Artifacts:** `.aider.conf.yml`, `.aider.model.settings.yml`, `.aider.model.metadata.json` (project-local).

Each qualified IR reference becomes a model-settings `name`. Its `extra_params`
override the wire `model` (`openai/<id>` or `anthropic/<id>`) and `api_base`, so
providers with identical upstream model IDs remain separate. `defaults.model`
selects the qualified name. Context and output limits populate model metadata;
the output limit also sets `extra_params.max_tokens`.

Literal keys pass through `extra_params.api_key`. Environment keys use LiteLLM's
`os.environ/NAME` syntax and are resolved at runtime. Literal keys beginning with
that prefix are skipped with a warning. Literal headers become `extra_headers`;
environment-derived headers are skipped because this path does not interpolate
them. Unsupported auth overrides receive the shared auth warning.

Responses providers are skipped with a warning: this emitter does not assume
that a call to `litellm.completion` selects the Responses wire. A default on a
skipped provider is also omitted. Aider has no native MCP client configuration,
so MCP entries are skipped. Modality, tool and reasoning capabilities and
variant lists are not mapped to Aider editing preferences.

A native Aider model-load and LiteLLM request test reached a local capture server
at `/v1/chat/completions`, preserving the upstream model ID and resolving the
configured environment key. The server returned `OK` successfully.

Sources: [advanced model settings](https://aider.chat/docs/config/adv-model-settings.html),
[API keys](https://aider.chat/docs/config/api-keys.html), and pinned `aider/models.py`.
