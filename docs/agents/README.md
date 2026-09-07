# Target contracts

This directory records the native configuration evidence used by agentcfg emitters. It is intentionally separate from [`../protocol.md`](../protocol.md): the protocol describes agentcfg source semantics, while these files describe target-specific syntax and limits.

| Target | Provider form | MCP form | Schema snapshot |
|---|---|---|---|
| [Codex](codex.md) | TOML provider table | TOML MCP table | official reference (online) |
| [OpenCode](opencode.md) | JSON `provider` | JSON `mcp` | [`opencode.config.schema.json`](opencode.config.schema.json) |
| [Pi](pi.md) | TypeScript extension | external adapter only | upstream docs |
| [Prime Agent](prime-agent.md) | JSON `models.json` | JSON `settings.json.mcpServers` | installed 0.9.3 source contract |
| [DeepSeek Harness](deepseek-harness.md) | YAML `llm-pi-ai` route | Cordis MCP-client patch | provider guide; MCP partial |
| [Grok Build](grok.md) | TOML `[model."<id>"]` | TOML `[mcp_servers]` | local install + upstream docs |
| [Kimi Code](kimi.md) | TOML `[providers]`/`[models]` | JSON `mcp.json` | official repo zod schemas |
| [ZCode](zcode.md) | JSON `provider` | JSON `mcp.servers` | installed v3.11.2 bundled schema |
| [MiMo Code](mimocode.md) | JSON `provider` (AI SDK npm) | JSON `mcp` local/remote | official live JSON schema |
| [jcode](jcode.md) | TOML `[providers]` | JSON `mcp.json` | official repo (Rust) |
| [Cline CLI](cline.md) | JSON `providers.json` | JSON `cline_mcp_settings.json` | official repo zod schemas |
| [Gajae Code](gajae.md) | YAML `models.yml` providers | JSON `mcp.json` | official repo zod schemas |
| [Hermes Agent](hermes.md) | YAML `providers` | YAML `mcp_servers` | official repo config |
| [OpenClaw](openclaw.md) | JSON `models.providers` | JSON `mcp.servers` | official repo zod schemas |

Every emitter change must update the applicable target contract with version, source URL/path, and tested status.
