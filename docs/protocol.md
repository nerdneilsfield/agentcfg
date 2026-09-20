# agentcfg protocol (IR)

This document specifies version 1 of the `agentcfg.yaml` intermediate representation (IR). The IR describes provider routes and MCP servers only. Native field names, interpolation syntax, and file layout belong to each target contract in [`docs/agents/`](agents/README.md).

Write one YAML document. Compile it with `agentcfg validate` and `agentcfg gen`. agentcfg never writes the native config files; it prints fragments on stdout.

## IR principles

- The IR represents target-independent runtime meaning, not any CLI's field names or interpolation syntax.
- The model fields are a **minimal union**: a field exists when at least one supported target needs it to describe a valid model.
- Credentials may be string literals or environment references. agentcfg never reads the process environment during compilation; emitters translate references into supported native syntax.
- Literal secrets are copied into generated output. Keep secret-bearing input and output out of Git and logs.
- Shell commands are argv arrays. The IR has no shell-command string form.
- A provider has exactly one request protocol. A gateway that serves two protocols is two providers.
- Unknown YAML keys and a second YAML document (`---`) are decode errors. Semantic problems are diagnostics.

## Document shape

```yaml
version: 1                 # required; must be 1
providers: []              # zero or more provider routes
mcp: []                    # zero or more MCP servers
defaults:
  model: provider-id/model-id
targets: [crush, gajae]    # optional; used when --to is omitted
```

The file must contain exactly one YAML document. `ir.Load` enables `KnownFields(true)`, so a typo such as `apiKey` or `baseURL` fails at decode time rather than being ignored. YAML anchors may resolve only to values the typed IR accepts. Custom YAML tags are rejected.

`version` is required and must be `1`.

The old IR field `api_key_env` and the mapping objects `{value: ...}`, `{from_env: ...}`, and `{bearer_from_env: ...}` are not accepted. Native generated configs may still use names such as `api_key_env`; those names belong to the target, not this IR.

## A complete example

This document is the same shape as the repository [`example.yaml`](../example.yaml). It validates for Crush and Gajae. Copy it with `agentcfg gen-example -o agentcfg.yaml`.

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
        variants: [low, medium, high, xhigh, max, ultra]
        tool_calling: true

  - id: anthropic-internal
    name: Anthropic Internal
    protocol: anthropic-messages
    base_url: https://anthropic-internal.example.com
    api_key: "ENV:ANTHROPIC_INTERNAL_TOKEN"
    models:
      - id: claude-internal
        name: Claude Internal
        context_window: 200000
        max_output_tokens: 64000
        input: [text, image]
        output: [text]
        reasoning: true

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

targets: [crush, gajae]
```

`targets: [crush, gajae]` is a default selection for `validate` and `gen` when `--to` is omitted. It is not a claim that every listed field is representable on every compiled-in CLI. `--to all` is a CLI wildcard; an IR entry `targets: [all]` is an unknown target id.

Crush keeps every listed reasoning-effort name, including `ultra`. Gajae's native ladder is `minimal`, `low`, `medium`, `high`, `xhigh`, and `max`; it skips `ultra` and still emits the rest of the model.

## Scalar values

`api_key`, provider and MCP header values, and MCP `env` values are YAML string scalars (`!!str`). Quote values that YAML would otherwise treat as a boolean or a number (`true`, `no`, `1e6`). Unquoted plain words such as `engineering` are still strings.

```yaml
api_key: "ENV:PROVIDER_API_KEY"
headers:
  X-Tenant: "engineering"
  X-Token: "ENV:HEADER_VALUE_ENV"
  Authorization: "Bearer ENV:TOKEN_ENV"
```

| YAML form | IR meaning |
|---|---|
| `"engineering"` | Constant string. Emitted as a literal. |
| `"ENV:VOLC_API_KEY"` | Environment reference. `NAME` must match `[A-Za-z_][A-Za-z0-9_]*`. |
| `"Bearer ENV:GITHUB_TOKEN"` | Environment reference with a `Bearer ` prefix. Valid only on an HTTP `Authorization` header. |

The prefixes `ENV:` and `Bearer ENV:` are case-sensitive. `env:VOLC_API_KEY` is a literal that happens to start with `env:`. `Bearer ENV:` is valid only on a header whose name is exactly `Authorization` (not `authorization` or `X-Authorization`). It is an IR error on `api_key`, on MCP `env`, and on any other header name.

agentcfg does not read the referenced variables. A target that cannot represent a literal or a reference reports a diagnostic instead of resolving the variable or dropping the field.

Literal API keys are also rejected when the target would treat the text as a native expression:

| Target | Rejected literal API-key content |
|---|---|
| Crush | Contains `$` or a backtick. |
| Pi, Prime Agent | Starts with `$` or `!`. |
| OpenCode, MiMo Code | Contains `{env:` or `{file:`. |
| Hermes, OpenClaw | Contains `${`. |

Gajae uses native environment-name-or-literal semantics: a literal that equals a set environment variable's name can be resolved by Gajae at runtime. agentcfg does not inspect the runtime environment to disambiguate it.

Prefer `ENV:NAME` when the selected target supports environment references. Use a quoted literal when the selected target has no implemented reference form (Cline and ZCode store inline API keys).

## Provider

| Field | Required | Meaning |
|---|---:|---|
| `id` | yes | Stable target-safe identifier: `[a-z][a-z0-9_-]*`. Unique across the document. |
| `name` | no | Human-facing display name. Emitters that have a display field default it to `id`. |
| `protocol` | yes | `openai-completions`, `openai-responses`, or `anthropic-messages`. |
| `base_url` | yes | Provider endpoint base URL, including the path the CLI should call (often `/v1`). |
| `api_key` | no | String literal or `ENV:NAME`. `Bearer ENV:NAME` is invalid here. |
| `auth_type` | no | How the credential is presented: `official` (default), `bearer`, `x-api-key`, or `none`. See [Choosing `auth_type`](#choosing-auth_type). |
| `headers` | no | Extra HTTP headers for provider requests. Same scalar forms as above. |
| `models` | yes | Non-empty list of models this route serves. Model `id` values must be unique inside the provider. |

`id` is the key targets use in native maps (`providers.volcengine`, `[model_providers.volcengine]`). It is not the upstream vendor name.

### Choosing `protocol`

The three values are request shapes, not vendor names:

| `protocol` | Request shape | Typical `base_url` |
|---|---|---|
| `openai-completions` | OpenAI Chat Completions (`/v1/chat/completions`) | `https://gateway.example/v1` |
| `openai-responses` | OpenAI Responses API (`/v1/responses`) | `https://gateway.example/v1` |
| `anthropic-messages` | Anthropic Messages API | `https://gateway.example` (no `/v1` suffix unless the gateway expects it) |

A single gateway that exposes both Chat Completions and Anthropic Messages is two providers:

```yaml
providers:
  - id: volc-chat
    protocol: openai-completions
    base_url: https://example.com/v1
    api_key: "ENV:VOLC_API_KEY"
    models:
      - id: glm-5.3
  - id: volc-anthropic
    protocol: anthropic-messages
    base_url: https://example.com/anthropic
    api_key: "ENV:VOLC_API_KEY"
    models:
      - id: claude-internal
```

Not every target accepts every protocol. Codex accepts `openai-responses` only. OpenCode accepts `openai-completions` only. ZCode, MiMo Code, and jcode reject `openai-responses` on custom providers. The other compiled-in targets accept all three. See [`docs/agents/README.md`](agents/README.md).

### Choosing `auth_type`

`auth_type` is orthogonal to `protocol`. The protocol chooses the request shape,
and with it the header its transport sends by default; `auth_type` overrides that
choice. Omitting it means `official`.

| `auth_type` | Credential presentation | Requires `api_key` |
|---|---|---|
| `official` | Whatever the protocol's transport sends natively | no |
| `bearer` | `Authorization: Bearer <credential>` | yes |
| `x-api-key` | `x-api-key: <credential>` | yes |
| `none` | No credential at all | no, and `api_key` must be absent |

The protocols do not agree on their default, which is what makes the override
necessary:

| `protocol` | Default credential header |
|---|---|
| `openai-completions`, `openai-responses` | `Authorization: Bearer <key>` (the OpenAI client is built with the key) |
| `anthropic-messages` | `x-api-key: <key>`; `Bearer` is reserved for built-in OAuth and GitHub Copilot routes |

Which value a given upstream wants:

| Upstream | `protocol` | Default | Use |
|---|---|---|---|
| Anthropic | `anthropic-messages` | `x-api-key` | `official` |
| OpenAI | `openai-completions` | `Authorization: Bearer` | `official` |
| OpenRouter | `openai-completions` | `Authorization: Bearer` | `official` |
| new-api / one-api relaying Claude | `anthropic-messages` | gateway wants `Authorization: Bearer` | `bearer` |
| new-api / one-api relaying GPT | `openai-completions` | `Authorization: Bearer` | `official` |
| LiteLLM, Vercel AI Gateway | `openai-completions` | `Authorization: Bearer` | `official` |
| Ollama, LM Studio, llama.cpp, vLLM | any | none | `none` |

`bearer` is needed only when the wire shape and the gateway's expected header
disagree. A relay that fronts an Anthropic-shaped route is the common case:

```yaml
providers:
  - id: relay-anthropic
    protocol: anthropic-messages
    base_url: https://relay.example
    api_key: "ENV:RELAY_TOKEN"
    auth_type: bearer
    models:
      - id: claude-sonnet-4-5
        context_window: 200000
```

Not every target implements every value, and a target may accept a value only
for some protocols. A target that cannot present the credential as asked reports
a diagnostic instead of emitting a different header, so `validate` and `gen`
fail until the document is changed.

## Model

| Field | Required | Meaning |
|---|---:|---|
| `id` | yes | Upstream model identifier as the provider expects it on the wire. |
| `name` | no | Display name. Defaults to `id` in emitters that have a display field. |
| `context_window` | no | Maximum accepted context tokens. Crush and Kimi require it. |
| `max_output_tokens` | no | Maximum generated tokens. Crush requires it (`default_max_tokens`). Goose rejects it (global max tokens only). |
| `input` | no | Accepted input modalities: `text`, `image`, `audio`, `video`, `pdf`. The IR does not default an omitted list; emitters that have a modality field typically treat empty as text-only. |
| `output` | no | Produced modalities. Same omit/empty rule as `input`. |
| `reasoning` | no | Model supports a reasoning/thinking mode. |
| `variants` | no | Ordered, selectable reasoning-effort names for this model. Requires `reasoning: true`. |
| `tool_calling` | no | Model supports tool calls. Cline, Crush, and Goose reject `false` (those CLIs cannot turn tools off for a custom model). |

`id` may contain dots, colons, or slashes when the upstream model name does. Provider `id` may not.

Several targets restrict modalities. Pi, Prime Agent, and DeepSeek Harness accept `text` and `image` input only. Crush accepts `text` and `image` input and `text` output only. Gajae accepts `text` and `image` on both. Goose accepts `text` only. OpenClaw rejects `pdf` input.

### Reasoning variants

`variants` belongs to a model. It lists reasoning efforts a user can select for that same model. It does not create extra model IDs and it does not choose a default effort.

```yaml
models:
  - id: glm-5.3
    reasoning: true
    variants: [low, high, max]
```

Rules:

- The list must be non-empty when present.
- Names cannot repeat.
- Each name must match `[a-z][a-z0-9_-]*` (`Ultra` is invalid; `ultra` is valid).
- `reasoning: true` is required.

A variant's only IR meaning is its effort name. It does not carry output verbosity, reasoning summaries, token budgets, or arbitrary native request options.

agentcfg preserves the configured name. Common values include `low`, `medium`, `high`, `xhigh`, `max`, and `ultra`. A target with a fixed native set skips unrecognized names without a diagnostic (OpenClaw, Pi, Prime Agent, Grok, DeepSeek Harness, and Gajae skip `ultra`). A target with no faithful model-level field omits the list and still emits the rest of the configuration (Codex, Cline, jcode, Goose, Hermes). Crush, OpenCode, Kimi, ZCode, and MiMo Code keep provider/model names such as `ultra`.

The v1 IR excludes costs, per-level provider wire-value overrides, per-model provider options, and compatibility toggles. A target that needs one of those must reject the route, or a later IR version must add a named cross-target field. There is no arbitrary native-config escape hatch.

## MCP

Every MCP server has `id` and `transport`. Optional shared fields are `enabled`, `cwd`, and `timeout_ms`.

| Field | Required | Meaning |
|---|---:|---|
| `id` | yes | Target-safe identifier: `[a-z][a-z0-9_-]*`. Unique across MCP servers. |
| `transport` | yes | `stdio` or `http`. There is no `sse` transport in the IR; targets that distinguish SSE from streamable HTTP map `http` to their streamable-HTTP form. |
| `enabled` | no | When `false`, emitters that have a disable flag set it (`disabled: true` or `enabled: false`). Omitted means enabled. |
| `cwd` | no | Working directory for a stdio process. Invalid on `http`. Crush, MiMo Code, and jcode have no native field and reject it. |
| `timeout_ms` | no | Timeout in milliseconds. Mapping is target-specific (see below). |

Pi rejects every MCP server in v1 (no built-in MCP). jcode loads stdio only and rejects `http`.

### Stdio

```yaml
mcp:
  - id: context7
    transport: stdio
    command: [npx, -y, "@upstash/context7-mcp"]
    env:
      CONTEXT7_API_KEY: "ENV:CONTEXT7_API_KEY"
      LOG_LEVEL: "info"
    cwd: /var/lib/context7
    timeout_ms: 30000
    enabled: true
```

`command` is required and non-empty. The first element is the executable; the rest are arguments. `url` and HTTP `headers` are invalid on stdio.

`env` values use the same scalar forms as provider headers, except `Bearer ENV:NAME` is invalid on `env`. Some targets can only pass through a same-name environment variable:

- Codex `env_vars` and Goose `env_keys` inherit `NAME` from the host. `CHILD: "ENV:SOURCE"` with `CHILD != SOURCE` is rejected.
- Prime Agent stdio `env` entries are environment references only; a literal child value is rejected.
- Cline, Kimi, and ZCode MCP env values are literals; `ENV:NAME` is rejected. Kimi HTTP MCP still maps `Authorization: "Bearer ENV:NAME"` to `bearerTokenEnvVar`.

### HTTP

```yaml
mcp:
  - id: github
    transport: http
    url: https://api.githubcopilot.com/mcp/
    headers:
      Authorization: "Bearer ENV:GITHUB_TOKEN"
      X-Tenant: "engineering"
    timeout_ms: 15000
```

`url` is required. `command`, `cwd`, and `env` are invalid. OAuth enrollment is out of v1: it is runtime-owned state, not source configuration.

`Authorization: "Bearer ENV:NAME"` is the IR form for a bearer token taken from the environment. Several targets map that one header to a dedicated native field (`bearer_token_env_var`, `bearerTokenEnvVar`) instead of a literal `Authorization` header.

### `timeout_ms` mapping

The IR unit is always milliseconds. Emitters do not guess a field that the native schema does not have.

| Behavior | Targets |
|---|---|
| Milliseconds unchanged | OpenCode `timeout`, MiMo Code `timeout`, Gajae `timeout`, ZCode `timeoutMs`, DeepSeek Harness `toolCallTimeoutMs`, OpenClaw `connectionTimeoutMs` and `requestTimeoutMs` (same value on both), Codex `startup_timeout_ms`, Kimi `startupTimeoutMs` (range 1–2147483647; `toolTimeoutMs` is not written) |
| Converted to seconds (`ms / 1000`) | Crush (integer seconds; not divisible by 1000 is rejected), Goose (positive integer seconds), jcode `timeout_secs` (integer seconds), Hermes `timeout` (positive; fractional seconds allowed), Cline `timeout` (fractional seconds allowed, so `1500` becomes `1.5`) |
| Rejected | Grok, Prime Agent |

Omit `timeout_ms` unless every selected target can represent it.

### `cwd` mapping

`cwd` is stdio-only. OpenCode, Codex, Grok, Kimi, ZCode, Cline, Gajae, Hermes, OpenClaw, Goose, DeepSeek Harness, and Prime Agent emit it. Crush, MiMo Code, and jcode reject it.

## Defaults

```yaml
defaults:
  model: volcengine/glm-5.3
```

`defaults.model` uses `provider-id/model-id` and must name a model that exists in this document. The first `/` splits the two ids, so a model id may itself contain `/`.

Whether a target can store a default is a target-contract concern. Codex writes `model_provider` and `model`. OpenCode and MiMo Code write a root `model` string. Crush writes both `models.large` and `models.small`. Pi emits no settings fragment in v1, so the default is deferred until the provider extension is loaded.

## Targets

`targets` is an optional list of compiled-in target identifiers. It does not change the meaning of providers, models, MCP servers, or defaults. It is only the default selection for `validate` and `gen`.

Selection order:

```text
--to > targets in agentcfg.yaml > error
```

`--to` is a comma-separated list, or the CLI wildcard `all`:

```sh
agentcfg gen --config agentcfg.yaml --to crush
agentcfg gen --config agentcfg.yaml --to crush,gajae
agentcfg gen --config agentcfg.yaml --to all
```

`--to all` means every emitter compiled into this binary, not every CLI installed on the machine. An IR list `targets: [all]` is not that wildcard; it is an unknown target id.

If both `--to` and `targets:` are omitted, agentcfg exits with `no targets: pass --to <id|all> or set targets: in agentcfg.yaml`.

Listing every compiled-in id in YAML is almost never useful. One document rarely represents faithfully on Codex (Responses only), OpenCode (Completions only), Cline/ZCode (literal keys), Kimi (literal provider headers), and Pi (no MCP) at the same time. Put the CLIs you actually generate for in `targets:`, and use `--to` to override.

Current compiled-in ids: `cline`, `codex`, `crush`, `deepseek-harness`, `gajae`, `goose`, `grok`, `hermes`, `jcode`, `kimi`, `mimocode`, `openclaw`, `opencode`, `pi`, `prime-agent`, `zcode`.

## IR validation

IR validation checks document version, field types, identifiers, scalar value and environment-reference forms, `auth_type` values and their `api_key` requirement, duplicate IDs, provider/model references, protocol names, and transport constraints. It does not read environment-variable values, contact endpoints, or decide whether a target can represent a field.

Target validation runs after IR validation, once per selected target. A field that is legal in the IR can still be rejected for a target. Diagnostics name the target, the IR path, and the reason:

```
[crush] providers[0].models[0].context_window: error: crush Model.context_window is required
```

`validate` and `gen` both run target validation. `gen` writes stdout only after every selected target emits successfully.

## Worked examples

### OpenAI Chat Completions for OpenCode

OpenCode accepts `openai-completions` only. Environment references become `{env:NAME}`.

```yaml
version: 1
providers:
  - id: volcengine
    name: Volcengine
    protocol: openai-completions
    base_url: https://example.com/v1
    api_key: "ENV:VOLC_API_KEY"
    models:
      - id: glm-5.3
        name: GLM-5.3
        context_window: 128000
        max_output_tokens: 8192
        reasoning: true
        tool_calling: true
defaults:
  model: volcengine/glm-5.3
targets: [opencode]
```

```sh
agentcfg gen --config agentcfg.yaml --to opencode
```

A single OpenCode artifact is printed as raw JSON. Merge the `provider` and `model` objects into `opencode.json`.

### OpenAI Responses for Codex

Codex accepts `openai-responses` only. `api_key: "ENV:NAME"` becomes `env_key`. MCP `timeout_ms` becomes `startup_timeout_ms`. Model catalog fields are validated as references but not written under `[model_providers]`. Command-backed `auth.command`, MCP `auth` (`oauth` | `chatgpt`), and `startup_timeout_sec` are not IR fields and are not emitted.

```yaml
version: 1
providers:
  - id: volcengine
    name: Volcengine
    protocol: openai-responses
    base_url: https://example.com/v1
    api_key: "ENV:VOLC_API_KEY"
    headers:
      X-Gateway-Key: "ENV:GATEWAY_KEY"
    models:
      - id: glm-5.3
mcp:
  - id: context7
    transport: stdio
    command: [npx, -y, "@upstash/context7-mcp"]
    timeout_ms: 20000
    env:
      CONTEXT7_API_KEY: "ENV:CONTEXT7_API_KEY"
defaults:
  model: volcengine/glm-5.3
targets: [codex]
```

`agentcfg gen --to codex` prints a TOML fragment for `~/.codex/config.toml`.

### Anthropic Messages

```yaml
version: 1
providers:
  - id: anthropic-internal
    protocol: anthropic-messages
    base_url: https://anthropic-internal.example.com
    api_key: "ENV:ANTHROPIC_INTERNAL_TOKEN"
    models:
      - id: claude-internal
        context_window: 200000
        max_output_tokens: 64000
        reasoning: true
targets: [gajae, crush, openclaw]
```

Do not send this document to OpenCode, Codex, or a Completions-only adapter. The protocol is `anthropic-messages`, not an OpenAI route with a Claude model id.

### Literal API keys (Cline, ZCode)

Cline and ZCode store inline key strings and have no custom-provider environment fallback. `ENV:NAME` is rejected:

```yaml
version: 1
providers:
  - id: volcengine
    protocol: openai-completions
    base_url: https://example.com/v1
    api_key: "sk-example-not-a-real-key"
    headers:
      X-Tenant: "engineering"
    models:
      - id: glm-5.3
        context_window: 128000
        max_output_tokens: 8192
targets: [cline, zcode]
```

The literal appears in generated output. Keep the YAML and the fragments out of Git.

### Environment API keys on Kimi

Kimi maps `api_key: "ENV:NAME"` to native `api_key_env` and a quoted literal to `api_key`. Provider `custom_headers` stay literal, so env-derived provider headers are rejected. MCP env values are also literals; HTTP `Authorization: "Bearer ENV:NAME"` still maps to `bearerTokenEnvVar`. `timeout_ms` maps to `startupTimeoutMs`.

```yaml
version: 1
providers:
  - id: volcengine
    protocol: openai-responses
    base_url: https://example.com/v1
    api_key: "ENV:VOLC_API_KEY"
    headers:
      X-Tenant: "engineering"
    models:
      - id: glm-5.3
        context_window: 128000
        max_output_tokens: 8192
mcp:
  - id: context7
    transport: stdio
    command: [npx, -y, "@upstash/context7-mcp"]
    timeout_ms: 20000
    env:
      CONTEXT7_API_KEY: "literal-key"
targets: [kimi]
```

`agentcfg gen --to kimi` prints a `config.toml` fragment and an `mcp.json` fragment.

### Stdio MCP with working directory and timeout

```yaml
version: 1
providers:
  - id: volcengine
    protocol: openai-completions
    base_url: https://example.com/v1
    api_key: "ENV:VOLC_API_KEY"
    models:
      - id: glm-5.3
        context_window: 128000
        max_output_tokens: 8192
mcp:
  - id: context7
    transport: stdio
    command: [npx, -y, "@upstash/context7-mcp"]
    cwd: /var/lib/context7
    timeout_ms: 30000
    env:
      CONTEXT7_API_KEY: "ENV:CONTEXT7_API_KEY"
targets: [opencode, gajae]
```

`timeout_ms: 30000` is 30 seconds. Crush would accept the timeout (integer seconds) but reject `cwd`. Grok and Prime Agent would reject the timeout. Kimi would accept the timeout as `startupTimeoutMs` but reject the `ENV:NAME` MCP env value. Omit those fields when the selected target cannot represent them.

### HTTP MCP with a bearer token

```yaml
mcp:
  - id: github
    transport: http
    url: https://api.githubcopilot.com/mcp/
    headers:
      Authorization: "Bearer ENV:GITHUB_TOKEN"
```

OpenCode emits `"Authorization": "Bearer {env:GITHUB_TOKEN}"`. Codex and Prime Agent emit `bearer_token_env_var` / `bearerTokenEnvVar` instead of an `Authorization` header. Cline and ZCode reject the environment reference; they need a literal header value.

### Disable an MCP server without deleting it

```yaml
mcp:
  - id: context7
    transport: stdio
    command: [npx, -y, "@upstash/context7-mcp"]
    enabled: false
```

Crush and Cline map this to `disabled: true`. OpenCode, Gajae, Hermes, OpenClaw, Kimi, ZCode, and MiMo Code map it to `enabled: false`.

## What the IR does not include

v1 does not model pricing, per-model request option bags, OAuth client enrollment, mTLS, tool allow/deny lists, agent roles beyond a single default model, or native file merge. If a target needs one of those to be correct, the emitter rejects the route rather than inventing a value.
