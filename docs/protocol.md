# agentcfg protocol (IR)

This document specifies version 1 of the `agentcfg.yaml` intermediate representation (IR). It models provider routes and MCP servers only.

## IR principles

- The IR represents target-independent runtime meaning, not any CLI's field names or interpolation syntax.
- The model fields are a **minimal union**: a field exists when at least one supported target needs it to describe a valid model.
- Credentials may be string literals or environment references. agentcfg never resolves references during compilation; emitters translate them to supported native syntax.
- Literal secrets are emitted into generated output. Keep inputs and outputs containing secrets out of Git and logs.
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
    api_key: "ENV:VOLC_API_KEY"
    headers:
      X-Tenant: "engineering"
      X-Gateway-Key: "ENV:GATEWAY_KEY"
    models:
      - id: glm-5.3
        name: GLM-5.3
        context_window: 128000
        max_output_tokens: 8192
        input: [text, image]
        output: [text]
        reasoning: true
        variants: [low, high, max]
        tool_calling: true

mcp:
  - id: context7
    transport: stdio
    command: [npx, -y, "@upstash/context7-mcp"]
    env:
      CONTEXT7_API_KEY: "ENV:CONTEXT7_API_KEY"

  - id: github
    transport: http
    url: https://api.githubcopilot.com/mcp/
    headers:
      Authorization: "Bearer ENV:GITHUB_TOKEN"

defaults:
  model: volcengine/glm-5.3

targets: [codex, opencode, pi, prime-agent, deepseek-harness, grok, kimi, zcode, mimocode, jcode, cline, gajae, hermes, openclaw, crush, goose]
```

## Provider

| Field | Required | Meaning |
|---|---:|---|
| `id` | yes | Stable target-safe identifier: `[a-z][a-z0-9_-]*`. |
| `name` | no | Human-facing display name; defaults to `id`. |
| `protocol` | yes | `openai-completions`, `openai-responses`, or `anthropic-messages`. |
| `base_url` | yes | Provider endpoint base URL. |
| `api_key` | no | String literal API key or `ENV:NAME` environment reference. |
| `headers` | no | Map of header names to string literals or environment references. |
| `models` | yes | Non-empty list of models served by this provider. |

### Scalar values

`api_key`, provider and MCP header values, and MCP `env` values use string scalars:

```yaml
api_key: "ENV:PROVIDER_API_KEY"
headers:
  X-Tenant: "engineering"
  X-Token: "ENV:HEADER_VALUE_ENV"
  Authorization: "Bearer ENV:TOKEN_ENV"
```

A plain string is a literal. `ENV:NAME` references an environment variable;
`Bearer ENV:NAME` adds the `Bearer ` prefix to its runtime value and is valid only
for an `Authorization` header. These are IR semantics, not native interpolation
syntax. `NAME` must be non-empty and match `[A-Za-z_][A-Za-z0-9_]*`.
The `ENV:` and `Bearer ENV:` prefixes are case-sensitive. agentcfg does not
read the referenced environment variables.

For a literal API key, use `api_key: "example-key"` (the value shown is a
placeholder). The target must have a supported native literal credential field.
If a target cannot represent a literal or reference, validation reports a
diagnostic instead of resolving the reference or silently dropping the value.
Literal API keys are also rejected when the target would interpret them as
native expressions rather than literal text:

| Target | Rejected literal API-key content |
|---|---|
| Crush | Contains `$` or a backtick. |
| Pi, Prime Agent | Starts with `$` or `!`. |
| OpenCode, MiMo Code | Contains `{env:` or `{file:`. |
| Hermes, OpenClaw | Contains `${`. |

Gajae uses native environment-name-or-literal semantics: a literal matching a
set environment variable's name can be resolved by Gajae at runtime. agentcfg
does not inspect the runtime environment to disambiguate it.

The old IR `api_key_env` field and `{value: ...}`, `{from_env: ...}`, and
`{bearer_from_env: ...}` value objects are not accepted. Native generated configs
may still use fields such as `api_key_env`; those names belong to the target,
not this IR.

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
| `variants` | no | Ordered, selectable reasoning-effort variants for this model. Requires `reasoning: true`. |
| `tool_calling` | no | Model supports tool calls. |

### Reasoning variants

`variants` belongs to a model. It declares the reasoning efforts that a user can
select for that same model; it does not create more model IDs and it does not
select a default effort. The names are provider/model-supplied lowercase identifiers. Common values include
`low`, `medium`, `high`, `xhigh`, `max`, and `ultra`; agentcfg preserves the
configured name instead of mapping it to a fixed global enum:

```yaml
models:
  - id: glm-5.3
    reasoning: true
    variants: [low, high, max]
```

The list must be non-empty when present, cannot repeat an identifier, and
requires `reasoning: true`. A variant's only IR meaning is its reasoning effort.
It does not carry output verbosity, reasoning summaries, token budgets, or
arbitrary native request options. Targets render this model-level availability
where their documented contract supports it. A target with a fixed native set
skips unrecognized effort names; a target with no faithful model-level field
skips `variants` and still emits its remaining configuration.

The v1 IR intentionally excludes costs, per-level provider wire-value overrides,
per-model provider options, and compatibility toggles. These differ too much by
runtime. A target needing one must reject the route or a later version must add
a named, cross-target semantic field; there is no arbitrary native-config escape
hatch.

## MCP

Every MCP server has `id`, `transport`, and optional `enabled`, `cwd`, and `timeout_ms`.

### Stdio

```yaml
transport: stdio
command: [executable, arg1, arg2]
env:
  CHILD_VAR: "non-secret constant"
  TOKEN: "ENV:TOKEN_ENV"
```

`command` is required and non-empty. `url` and HTTP headers are invalid.

### HTTP

```yaml
transport: http
url: https://example.com/mcp
headers:
  Authorization: "Bearer ENV:TOKEN_ENV"
```

`url` is required. `command`, `cwd`, and `env` are invalid. OAuth enrollment is deliberately out of v1: it is runtime-owned state, not source configuration.

## Defaults

`defaults.model` uses `provider-id/model-id` and selects the intended default route/model. Whether a target can represent this intent is a target-contract concern, not an IR concern.

## Targets

`targets` is an optional list of target identifiers for which this IR is intended. It has no effect on the meaning of providers, models, MCP servers, or defaults.

## IR validation

IR validation checks document version, field types, identifiers, scalar value and environment-reference forms, duplicate IDs, provider/model references, protocol constraints, and transport constraints. It does not read environment-variable values, contact endpoints, or validate target support.
