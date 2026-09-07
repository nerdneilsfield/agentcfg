# agentcfg protocol (IR)

This document specifies version 1 of the `agentcfg.yaml` intermediate representation (IR). It models provider routes and MCP servers only.

## IR principles

- The IR represents target-independent runtime meaning, not any CLI's field names or interpolation syntax.
- The model fields are a **minimal union**: a field exists when at least one supported target needs it to describe a valid model.
- Credentials are references only. No scalar in this IR may contain a secret value.
- Shell commands are argv arrays. The IR has no shell-command form.
- A provider has exactly one request protocol. A gateway serving two protocols is represented by two providers.

## YAML document

```yaml
version: 1

providers:
  - id: volcengine
    name: Volcengine
    protocol: openai-completions
    base_url: https://example.com/v1
    api_key_env: VOLC_API_KEY
    headers:
      X-Tenant:
        value: engineering
      X-Gateway-Key:
        from_env: GATEWAY_KEY
    models:
      - id: glm-5.3
        name: GLM-5.3
        context_window: 128000
        max_output_tokens: 8192
        input: [text, image]
        output: [text]
        reasoning: true
        tool_calling: true

mcp:
  - id: context7
    transport: stdio
    command: [npx, -y, "@upstash/context7-mcp"]
    env:
      CONTEXT7_API_KEY:
        from_env: CONTEXT7_API_KEY

  - id: github
    transport: http
    url: https://api.githubcopilot.com/mcp/
    headers:
      Authorization:
        bearer_from_env: GITHUB_TOKEN

defaults:
  model: volcengine/glm-5.3

targets: [codex, opencode, pi, prime-agent, deepseek-harness, grok, kimi, zcode, mimocode, jcode, cline, gajae, hermes, openclaw]
```

## Provider

| Field | Required | Meaning |
|---|---:|---|
| `id` | yes | Stable target-safe identifier: `[a-z][a-z0-9_-]*`. |
| `name` | no | Human-facing display name; defaults to `id`. |
| `protocol` | yes | `openai-completions`, `openai-responses`, or `anthropic-messages`. |
| `base_url` | yes | Provider endpoint base URL. |
| `api_key_env` | no | Environment-variable name containing the API key. |
| `headers` | no | Non-secret values or environment-derived request headers. |
| `models` | yes | Non-empty list of models served by this provider. |

A header value is exactly one of:

```yaml
value: non-secret constant
# or
from_env: HEADER_VALUE_ENV
# or
bearer_from_env: TOKEN_ENV
```

`bearer_from_env` means `Authorization: Bearer <value>` and is only valid for the `Authorization` header.

## Model

| Field | Required | Meaning |
|---|---:|---|
| `id` | yes | Upstream model identifier. |
| `name` | no | Display name; defaults to `id`. |
| `context_window` | no | Maximum accepted context tokens. |
| `max_output_tokens` | no | Maximum generated tokens. |
| `input` | no | Accepted input modalities: `text`, `image`, `audio`, `video`, `pdf`. Default: `[text]`. |
| `output` | no | Produced modalities. Default: `[text]`. |
| `reasoning` | no | Model supports a reasoning/thinking mode. |
| `tool_calling` | no | Model supports tool calls. |

The v1 IR intentionally excludes costs, thinking-level mappings, per-model provider options, and compatibility toggles. These differ too much by runtime. A target needing one must reject the route or a later version must add a named, cross-target semantic field; there is no arbitrary native-config escape hatch.

## MCP

Every MCP server has `id`, `transport`, and optional `enabled`, `cwd`, and `timeout_ms`.

### Stdio

```yaml
transport: stdio
command: [executable, arg1, arg2]
env:
  CHILD_VAR:
    value: non-secret constant
  TOKEN:
    from_env: TOKEN_ENV
```

`command` is required and non-empty. `url` and HTTP headers are invalid.

### HTTP

```yaml
transport: http
url: https://example.com/mcp
headers:
  Authorization:
    bearer_from_env: TOKEN_ENV
```

`url` is required. `command`, `cwd`, and `env` are invalid. OAuth enrollment is deliberately out of v1: it is runtime-owned state, not source configuration.

## Defaults

`defaults.model` uses `provider-id/model-id` and selects the intended default route/model. Whether a target can represent this intent is a target-contract concern, not an IR concern.

## Targets

`targets` is an optional list of target identifiers for which this IR is intended. It has no effect on the meaning of providers, models, MCP servers, or defaults.

## IR validation

IR validation checks document version, field types, identifiers, safe credential-reference forms, duplicate IDs, provider/model references, protocol constraints, and transport constraints. It does not read environment-variable values, contact endpoints, or validate target support.
