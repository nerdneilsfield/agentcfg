# Target contracts

This directory records the native configuration evidence used by agentcfg emitters. It is intentionally separate from [`../protocol.md`](../protocol.md): the protocol describes agentcfg source semantics, while these files describe target-specific syntax and limits.

| Target | Provider form | MCP form | Schema snapshot |
|---|---|---|---|
| [Codex](codex.md) | TOML provider table | TOML MCP table | official reference (online) |
| [OpenCode](opencode.md) | JSON `provider` | JSON `mcp` | [`opencode.config.schema.json`](opencode.config.schema.json) |
| [Pi](pi.md) | TypeScript extension | external adapter only | upstream docs |
| [Prime Agent](prime-agent.md) | JSON `models.json` | JSON `settings.json.mcpServers` | installed 0.9.3 source contract |
| [DeepSeek Harness](deepseek-harness.md) | YAML `llm-pi-ai` route | Cordis MCP-client patch | provider guide; MCP partial |

Every emitter change must update the applicable target contract with version, source URL/path, and tested status.
