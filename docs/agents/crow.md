# Crow

- **Target id:** `crow`
- **Verified against:** `crow-cli/crow-cli` commit `07f4ec9b61ffc075b11027657f5d31feb2ad3e60` (2026-09-25), including its config loader, OpenAI client, and ACP MCP conversion.
- **Artifact:** a `config.yaml` fragment for `~/.agents/crow/config.yaml`.

## Providers and models

Crow uses OpenAI Chat Completions. Responses and Anthropic providers are skipped
with warnings; supported providers and MCP entries still generate.

Provider entries contain `base_url` and `api_key`. Environment credentials use
`${NAME}`; agentcfg does not resolve them. Crow has no configurable provider
headers or explicit keyless-auth switch. Unsupported auth overrides produce the
shared auth diagnostic. `auth_type: none` omits the credential, but does not
guarantee that Crow's SDK sends no authorization header.

Model keys use the qualified IR reference `provider/model`; their `provider`
and `model` fields retain the route and upstream model ID. Crow chooses its first
model as the default, so the emitter writes the selected default first. If its
provider is unsupported, a warning explains that Crow will use the first
remaining model.

Input modalities map to `modality`, accepting text, image, audio, and video.
PDF is skipped. The website's older `capabilities` example does not match the
pinned loader. Model limits, output modalities, tool-calling and reasoning
capabilities, and variant lists are skipped with warnings. A context-window
size is not substituted for Crow's compaction threshold.

## MCP

`mcpServers` accepts stdio commands, arguments and environment values, plus HTTP
URLs and headers. `${NAME}` and `Bearer ${NAME}` expand at Crow load time.
Literal credentials, environment values, or headers containing `${` are skipped
with warnings because Crow would interpolate them.

Disabled MCP entries are omitted: Crow's ACP conversion does not honor an
enabled flag. Per-server `cwd` and timeout values are skipped with warnings;
the MCP client uses the session working directory.

Merge the fragment into the existing config to preserve Crow's agent, memory,
and service settings. The generated fragment was checked with the upstream
config loader and dataclasses extracted from the pinned source; this did not
start Crow's memory services or make a hosted model request.

## Sources

- https://crow-ai.dev/docs/configuration/
- https://github.com/crow-cli/crow-cli/blob/07f4ec9b61ffc075b11027657f5d31feb2ad3e60/src/crow_cli/config/config.py
- https://github.com/crow-cli/crow-cli/blob/07f4ec9b61ffc075b11027657f5d31feb2ad3e60/src/crow_cli/agent/mcp_client.py
