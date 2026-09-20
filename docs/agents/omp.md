# Oh My Pi

- **Target id:** `omp`
- **Verified against:** upstream `docs/models.md`, `docs/mcp-config.md`, and
  `docs/config-usage.md` (fetched 2026-09-20). Provider/model schema and value
  resolution come from `packages/coding-agent/src/config/`.
- **Native locations:** `~/.omp/agent/`; a named profile relocates the user
  scope to `~/.omp/profiles/<name>/agent/`.
- **v1 artifacts:** `models.yml`; plus `mcp.json` when the IR declares MCP
  servers, and `config.yml` when it sets a default model.

Oh My Pi is a fork of the same codebase as Pi and Prime Agent, so the provider
document is built by the shared `internal/target/pifamily` package and differs
only in path, encoding, and credential value syntax. Its value syntax matches
Prime Agent, **not** Pi.

## Provider route

Custom providers and models live in `~/.omp/agent/models.yml`. The root object
carries `providers` only; unknown root keys fail schema validation.

```yaml
providers:
  relay:
    baseUrl: https://relay.example
    api: anthropic-messages
    apiKey: RELAY_TOKEN
    authHeader: true
    models:
      - id: claude-sonnet-4-5
        name: Claude Sonnet 4.5
        input: [text, image]
        reasoning: true
        contextWindow: 200000
        maxTokens: 8192
        cost: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0 }
```

A value is resolved by running it as a command when it starts with `!`, and
otherwise by treating the whole value as an environment variable name and
falling back to the literal string. IR `ENV:NAME` therefore renders as a bare
`NAME`: `"$NAME"` is Pi's syntax and this fork would send it as a literal token.
A literal API key starting with `$` or `!` would be read as that syntax, so it is
rejected.

`apiKey` is omitted rather than written empty. The schema requires a non-empty
string when the field is present, and an invalid `models.yml` makes the registry
run on built-in models only.

All three IR protocols map 1:1 to `api` values. Model `input` accepts `text` and
`image` only. `cost` is emitted as zeros because the IR has no pricing scope, and
`contextWindow` / `maxTokens` are emitted only when the IR sets them.

### Authentication

`auth_type` selects the credential header instead of leaving it to the protocol:

| `auth_type` | Emitted | Effect |
|---|---|---|
| `official` (default) | nothing extra | the protocol's native header |
| `bearer` | `"authHeader": true` | Oh My Pi sends `Authorization: Bearer <resolved apiKey>` |
| `none` | `apiKey` omitted | no credential, for keyless local endpoints |

An IR `Authorization: "Bearer ENV:NAME"` provider header is representable here
and renders as `"Bearer ${NAME}"`.

### Reasoning variants

`variants` becomes a model `thinking` block:

```yaml
thinking:
  mode: effort
  efforts: [low, high]
```

`mode` selects how an effort reaches the provider, and the IR carries effort
names only, so the effort mode is used and the provider dialect stays Oh My Pi's
business (its own `compat.thinkingFormat` covers that). The schema accepts
`minimal`, `low`, `medium`, `high`, `xhigh`, and `max`; names such as `ultra` are
skipped. No default level is set.

## MCP

`~/.omp/agent/mcp.json` (project: `.omp/mcp.json`) with an `mcpServers` map.
stdio and http servers are emitted:

- stdio: `command`, `args`, `env`, `cwd`
- http: `url`, `headers`
- both: `timeout` (IR `timeout_ms`, in milliseconds, unchanged) and
  `enabled: false` for a disabled server

stdio `env` references render as bare environment names, and
`Authorization: "Bearer ENV:NAME"` renders as `"Bearer ${NAME}"`, which Oh My Pi
expands while discovering an OMP-native file. The artifact is omitted when the
IR declares no MCP servers. The `$schema` line Oh My Pi's own writer adds is not
emitted; add it after merging if you want editor validation.

## Defaults

`defaults.model` becomes `modelRoles.default` in a `config.yml` fragment. Role
assignments beyond `default`, retry fallback chains, and provider discovery are
Oh My Pi configuration the IR does not model.

## Sources

- https://github.com/can1357/oh-my-pi/blob/main/docs/models.md
- https://github.com/can1357/oh-my-pi/blob/main/docs/mcp-config.md
- https://github.com/can1357/oh-my-pi/blob/main/docs/config-usage.md
