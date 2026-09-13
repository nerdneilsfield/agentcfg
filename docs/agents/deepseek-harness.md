# DeepSeek Harness (DSH)

- **Target id:** `deepseek-harness`
- **Verified against:** official DeepSeek Harness provider guide fetched 2026-09-07. DSH is explicitly a developer preview; revalidate on each upstream version.
- **Native location:** `$DSH_HOME/settings.yaml`, normally `~/.dsh/settings.yaml`.
- **v1 artifacts:** YAML provider fragment for `llm-pi-ai.providers`; a Cordis patch fragment for `@deepseek-ai/dsh-mcp-client`.

## Provider route

IR `api_key: "ENV:NAME"` maps to native `apiKeyEnv: NAME`. The provider
schema and credential resolver support only this environment-reference field,
not a literal `apiKey` field. Literal IR API keys are rejected. agentcfg does
not resolve the environment reference.

DSH custom routes live under the `llm-pi-ai` plugin namespace:

```yaml
llm-pi-ai:
  providers:
    volcengine:
      apiKeyEnv: VOLC_API_KEY
      api: openai-completions
      baseURL: https://example.com/v1
      models:
        - id: glm-5.3
          name: GLM-5.3
          contextWindow: 128000
          maxTokens: 8192
          input: [text, image]
          reasoning: true
```

Verified protocols are `openai-completions`, `openai-responses`, and `anthropic-messages`. A custom provider needs one protocol, base URL, and a non-empty model list. DSH's guide documents additional route/model compatibility settings, headers, retries, reasoning-effort maps, and modalities. They are deliberately outside v1 because they are target-specific semantics.

## Reasoning variants

`models[].variants` maps to a model-local `reasoningEfforts` map. Listed DSH
levels map to their own wire spelling and other documented levels are `null`.
DSH only accepts `off`, `minimal`, `low`, `medium`, `high`, `xhigh`, and `max`;
`ultra` and other unrecognized names are rejected instead of remapped. This
never sets DSH's route-level active `reasoning` field or any thinking budget.

DSH keeps default selected model in session/UI state, so `defaults.model` is not emitted.

## MCP

DSH connects MCP through the `@deepseek-ai/dsh-mcp-client` Cordis plugin in a patch list. A verified stdio example is:

```yaml
- insert:
    - id: context7
      name: '@deepseek-ai/dsh-mcp-client'
      config:
        serverName: context7
        transport: stdio
        command: npx
        args: [-y, '@upstash/context7-mcp']
```

The public evidence confirms stdio plugin configuration and that plugin configuration replaces its `config` object as a whole. The exact HTTP and environment-variable fields were not found in the official schema during this research pass. Therefore v1 supports DSH **providers and stdio MCP only**; HTTP MCP must be rejected until the upstream MCP-client catalog/schema is captured and tested.

## Sources

- https://github.com/deepseek-ai/deepseek-harness/blob/master/packages/llm/llm-pi-ai/src/config.ts (`PiAiProviderProfile` and provider-profile schema)
- https://github.com/deepseek-ai/deepseek-harness/blob/master/packages/llm/llm-pi-ai/src/index.ts (`resolveApiKey`)
- https://github.com/deepseek-ai/deepseek-harness/blob/master/docs/user/guide/providers.md
- https://github.com/deepseek-ai/deepseek-harness (official project)
- DSH MCP-client example: https://github.com/sepinetam/mcp-for-stata/blob/75680cf849facd4464bec20e7d3a69e3bca592de/docs/agents/deepseek_harness.md (integration example, not official schema)
