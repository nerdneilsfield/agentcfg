# Target contracts

This directory records the native configuration evidence used by agentcfg emitters. It is intentionally separate from [`../protocol.md`](../protocol.md): the protocol describes how to write `agentcfg.yaml`; these files describe how each CLI spells the same facts, and which IR fields it rejects.

Write the IR against the protocol. Open the contract for a target before putting that id in `targets:` or `--to`. A field that is legal in the IR can still be unrepresentable on a given CLI. Common mismatches:

- Codex accepts `openai-responses` only; OpenCode accepts `openai-completions` only.
- Cline and ZCode reject `api_key: "ENV:NAME"` (literal keys only). Kimi maps that scalar to `api_key_env`. Goose and DeepSeek Harness reject literal API keys.
- Pi rejects every MCP server in v1. jcode rejects HTTP MCP.
- Crush, MiMo Code, and jcode reject MCP `cwd`. Grok and Prime Agent reject MCP `timeout_ms`. Kimi maps `timeout_ms` to `startupTimeoutMs` and does not write `toolTimeoutMs`.
- Crush keeps reasoning-effort names such as `ultra`. Gajae, OpenClaw, Pi, Prime Agent, Grok, and DeepSeek Harness skip names outside their native ladder.

`--to all` is a CLI wildcard for every compiled-in emitter. An IR list `targets: [all]` is an unknown id. One document rarely represents faithfully on every target at once; put the CLIs you generate for in `targets:` and override with `--to`.

| Target | Provider form | MCP form | Schema snapshot |
|---|---|---|---|
| [Codex](codex.md) | TOML provider table | TOML MCP table | developers.openai.com/codex/config-reference + config.schema.json |
| [OpenCode](opencode.md) | JSON `provider` | JSON `mcp` | [`opencode.config.schema.json`](opencode.config.schema.json) |
| [Pi](pi.md) | JSON `models.json` | external adapter only | upstream docs |
| [Prime Agent](prime-agent.md) | JSON `models.json` | JSON `settings.json.mcpServers` | installed 0.9.3 source contract |
| [Oh My Pi](omp.md) | YAML `models.yml` | JSON `mcp.json` | upstream docs |
| [DeepSeek Harness](deepseek-harness.md) | YAML `llm-pi-ai` route | Cordis MCP-client patch | provider guide; MCP partial |
| [Grok Build](grok.md) | TOML `[model."<id>"]` | TOML `[mcp_servers]` | local install + upstream docs |
| [Kimi Code](kimi.md) | TOML `[providers]`/`[models]` | JSON `mcp.json` | official kimi-code docs (2026-09-20) |
| [ZCode](zcode.md) | JSON `provider` | JSON `mcp.servers` | installed v3.11.2 bundled schema |
| [MiMo Code](mimocode.md) | JSON `provider` (AI SDK npm) | JSON `mcp` local/remote | official live JSON schema |
| [jcode](jcode.md) | TOML `[providers]` | JSON `mcp.json` | official repo (Rust) |
| [Cline CLI](cline.md) | JSON `providers.json` | JSON `cline_mcp_settings.json` | official repo zod schemas |
| [Gajae Code](gajae.md) | YAML `models.yml` providers | JSON `mcp.json` | official repo zod schemas |
| [Hermes Agent](hermes.md) | YAML `providers` | YAML `mcp_servers` | official repo config |
| [OpenClaw](openclaw.md) | JSON `models.providers` | JSON `mcp.servers` | official repo zod schemas |
| [Crush](crush.md) | JSON `providers` | JSON `mcp` | live schema charm.land/crush.json |
| [Goose](goose.md) | JSON `custom_providers/<id>.json` | YAML `extensions` | official repo (block/goose) |

Every emitter change must update the applicable target contract with version, source URL/path, and tested status.
