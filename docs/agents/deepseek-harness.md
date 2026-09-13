# DeepSeek Harness (DSH)

- **Target id:** `deepseek-harness`
- **Verified against:** official DeepSeek Harness provider guide fetched 2026-09-07. DSH is explicitly a developer preview; revalidate on each upstream version.
- **Native location:** `$DSH_HOME/settings.yaml`, normally `~/.dsh/settings.yaml`.
- **v1 artifacts:** YAML provider fragment for `llm-pi-ai.providers`; a Cordis patch fragment for `@deepseek-ai/dsh-mcp-client`.

## Provider route

IR `api_key: "ENV:NAME"` maps to native `apiKeyEnv: NAME`. The provider
credential resolver supports this reference rather than a literal key, so literal
IR API keys are rejected. A provider `name` maps to `displayName`. Literal
provider headers map to native `headers`; environment and bearer header
references are rejected because this target's provider expression form is not
verified here.

Custom routes are written to `$DSH_HOME/settings.yaml` under `llm-pi-ai`:

```yaml
llm-pi-ai:
  providers:
    volcengine:
      displayName: Volcengine
      apiKeyEnv: VOLC_API_KEY
      api: openai-completions
      baseURL: https://example.com/v1
      headers:
        X-Route: stable
      models:
        - id: glm-5.3
          name: GLM-5.3
          contextWindow: 128000
          maxTokens: 8192
          input: [text, image]
```

Verified protocols are `openai-completions`, `openai-responses`, and
`anthropic-messages`. Model input is limited to `text` and `image`. The emitter
does not write the obsolete model `reasoning` or `cost` fields.

## Reasoning variants

`models[].variants` maps to a model-local `reasoningEfforts` map. Only declared
DSH levels are emitted: `off` maps to a null value, which means no effort field
on the wire; other declared levels map to their own wire spelling. DSH supports
`off`, `minimal`, `low`, `medium`, `high`, `xhigh`, and `max`; `ultra` and other
unknown values are skipped. No route-level active effort/default is selected.

## Default model and MCP

The target writes an optional `$DSH_HOME/cordis.patch.yml` overlay. A default
model becomes the existing `agent-default-model` Cordis row:

```yaml
- insert:
    - id: agent-default-model
      name: '@deepseek-ai/dsh-agent-default-model'
      config:
        provider: volcengine
        model: glm-5.3
```

Each MCP server is an `@deepseek-ai/dsh-mcp-client` Cordis row, with an ID
prefixed `mcp-`. Stdio maps command, args, environment, cwd, and `timeout_ms`
to `toolCallTimeoutMs`. HTTP maps to native `streamable-http`, URL, headers,
and the same tool-call timeout. `enabled: false` maps to the Cordis row's
`disabled: true`. Environment values use the documented `!!js process.env.NAME`
form; bearer values use the documented JavaScript template expression.

```yaml
- insert:
    - id: mcp-context7
      name: '@deepseek-ai/dsh-mcp-client'
      config:
        serverName: context7
        transport: stdio
        command: npx
        args: [-y, '@upstash/context7-mcp']
        env:
          CONTEXT7_API_KEY: !!js process.env.CONTEXT7_API_KEY
        toolCallTimeoutMs: 60000
```

## Sources

- https://github.com/deepseek-ai/deepseek-harness/blob/c291e7961a515f6d7af9304e7fd1d257929aef26/packages/llm/llm-pi-ai/src/config.ts
  (`PiAiProviderProfile` and model schema)
- https://github.com/deepseek-ai/deepseek-harness/blob/c291e7961a515f6d7af9304e7fd1d257929aef26/docs/user/guide/providers.md
- https://github.com/deepseek-ai/deepseek-harness/blob/c291e7961a515f6d7af9304e7fd1d257929aef26/packages/mcp/mcp-client/src/index.ts
  and `README.md` (current MCP schema and examples)
- https://github.com/deepseek-ai/deepseek-harness/blob/c291e7961a515f6d7af9304e7fd1d257929aef26/packages/core/agent-default-model/README.md
