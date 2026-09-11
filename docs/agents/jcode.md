# jcode

- **Target id:** `jcode`
- **Verified against:** official repo `1jehuang/jcode` (Rust, MIT; research report `.research/jcode.md`, fetched 2026-09-07).
- **Native files:** `~/.jcode/config.toml` (+ env files `~/.config/jcode/<profile>.env`) and `~/.jcode/mcp.json`. `$JCODE_HOME` overrides the config root.
- **v1 artifacts:** TOML fragment (`config.toml`) and a separate `mcp.json` fragment. v1 does not edit these files.

## Provider route

An IR literal such as `api_key: "example-key"` is emitted in the native
`api_key` field. The example key is a placeholder; real literals also appear in
generated output.

jcode uses `[providers.<id>]` tables with an `api_key_env` native field:

```toml
[provider]
default_provider = "volcengine"
default_model = "glm-5.3"

[providers.volcengine]
type = "openai-compatible"
base_url = "https://example.com/v1"
api_key_env = "VOLC_API_KEY"
default_model = "glm-5.3"

[[providers.volcengine.models]]
id = "glm-5.3"
context_window = 128000
reasoning = true
input = ["image"]
```

- `type` is `openai-compatible` / `anthropic-compatible` / `openrouter`. The IR's two OpenAI protocol families map to `openai-compatible`, `anthropic-messages` maps to `anthropic-compatible`.
- The built-in OpenAI provider is the only one wired for the OpenAI **Responses** API, so IR `openai-responses` on a custom provider is rejected.
- `api_key_env` is native: the IR `api_key: "ENV:NAME"` maps directly without resolving the reference.
- Provider `headers` are literal-only; IR `ENV:NAME` / `Bearer ENV:NAME` provider headers are rejected. Literal headers are emitted.
- `[[providers.<id>.models]]` supports `id`, `context_window` (plus aliases), `reasoning`, `reasoning_effort`, and `input` (only `"image"` is meaningful; text is implicit). IR `max_output_tokens`, `output` modalities, and `tool_calling` have no jcode model field and are not emitted.

## Defaults

`defaults.model "provider/model"` maps to the `[provider]` table's
`default_provider` + `default_model` pair. Per-provider `default_model` is set
to the defaults match when the IR default targets that provider, else the
provider's first model.

## MCP

`~/.jcode/mcp.json` holds `[server].<id>`-style entries with stdio-only support (`command`, `args`, `env`, `timeout_secs`, `enabled|disabled`). HTTP/SSE entries are parsed but skipped at load, so IR `http` transport is rejected. `${VAR}` / `${VAR:-default}` expansion is native for MCP string fields, so IR `ENV:NAME` renders as `"${VAR}"`. `Authorization: "Bearer ENV:NAME"` renders as `Bearer ${VAR}` inside headers. IR `timeout_ms` is emitted as `timeout_secs` (seconds, `ms/1000`); IR `cwd` has no jcode field and is rejected for MCP servers.
