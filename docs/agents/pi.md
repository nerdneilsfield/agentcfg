# Pi Coding Agent

- **Target id:** `pi`
- **Verified against:** local `@earendil-works/pi-coding-agent 0.84.4`; installed documentation and current upstream documentation fetched 2026-09-07.
- **Native locations:** `~/.pi/agent/` globally; `.pi/` per project.
- **v1 artifacts:** a TypeScript provider extension and, if MCP support is requested, a standard `mcpServers` JSON document for the third-party `pi-mcp-adapter`.

## Provider route

An IR literal such as `api_key: "example-key"` is emitted in the native
`apiKey` field. The example key is a placeholder; real literals also appear in
generated output.

Pi does not use a static provider section in `settings.json`. A custom route is registered by a TypeScript extension through `pi.registerProvider()`:

```ts
import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";

export default function (pi: ExtensionAPI) {
  pi.registerProvider("volcengine", {
    name: "Volcengine",
    baseUrl: "https://example.com/v1",
    apiKey: "$VOLC_API_KEY",
    api: "openai-completions",
    models: [{
      id: "glm-5.3",
      name: "GLM-5.3",
      input: ["text", "image"],
      reasoning: true,
      cost: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0 },
      contextWindow: 128000,
      maxTokens: 8192,
    }],
  });
}
```

Pi's declarative custom-provider API requires `api`, model `input`, `cost`, `contextWindow`, and `maxTokens`; agentcfg supplies zero costs because source configuration has no pricing scope. `tool_calling` has no corresponding Pi model field. Provider header values use Pi's `$NAME` expression syntax: IR literal headers are emitted as constants and `ENV:NAME` headers as `$NAME`. IR `Bearer ENV:NAME` has no verified Pi bearer expansion and is rejected.

### Reasoning variants

For `models[].variants`, the extension emits a full model `thinkingLevelMap`.
Listed Pi levels map to themselves and every other Pi level is `null`, so the
model picker exposes precisely that set. Pi only documents `off`, `minimal`,
`low`, `medium`, `high`, `xhigh`, and `max`; unsupported names such as `ultra`
are omitted. No default thinking level is emitted.
The v1 Pi emitter supports all three IR protocols by mapping them to Pi's `openai-completions`, `openai-responses`, and `anthropic-messages` API names.

`defaultProvider` and `defaultModel` are settings fields, but v1 emits no settings mutation or standalone settings fragment because the provider extension must be installed/loaded first. Defaults remain deferred for Pi.

## MCP

Pi intentionally ships without built-in MCP support. The common `pi-mcp-adapter` package reads standard `mcpServers` JSON from `~/.config/mcp/mcp.json`, `~/.pi/agent/mcp.json`, `.mcp.json`, or `.pi/mcp.json` (precedence depends on the adapter version).

The adapter is an external dependency, not part of Pi. Thus `agentcfg gen --to pi` must fail MCP generation unless the user explicitly enables the adapter-backed target mode in a future CLI option. The artifact schema is not vendored as authoritative Pi schema.

## Sources

- Installed `docs/custom-provider.md`, `docs/providers.md`, `docs/settings.md` from Pi 0.84.4
- https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/custom-provider.md
- https://nicobailon-pi-mcp-adapter.mintlify.app/introduction (third-party adapter)
