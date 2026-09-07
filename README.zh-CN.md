# agentcfg

[English](README.md)

agentcfg 把一份 `agentcfg.yaml` 编译成多个编码 Agent CLI 的原生配置产物，
支持：Codex、OpenCode、Pi、Prime Agent、DeepSeek Harness、Grok Build、
Kimi Code、ZCode、MiMo Code、jcode、Cline CLI、Gajae Code、Hermes Agent、
OpenClaw、Goose 和 Crush。

它只管理**自定义 Provider（模型）和 MCP 服务器**。它不保存任何密钥
（只写环境变量引用），不运行 Provider，也不修改目标文件。IR 契约见
[docs/protocol.md](docs/protocol.md)，架构设计见
[docs/architecture.md](docs/architecture.md)。

## 安装

```sh
go install ./cmd/agentcfg
```

## 用法

```sh
# 校验 IR 和选定的 target
agentcfg validate --config agentcfg.yaml --to codex,opencode

# 生成原生配置到标准输出（单个产物：原始内容；
# 多个产物：稳定的 BEGIN/END 分块流）
agentcfg gen --config agentcfg.yaml --to all
```

`--to` 会覆盖 `agentcfg.yaml` 里的 `targets:` 列表。`gen` 只写标准输出；
诊断和日志走标准错误。

## 开发

```sh
CGO_ENABLED=0 go build ./...
CGO_ENABLED=0 go test ./...
CGO_ENABLED=0 go test -bench=. ./internal/ir   # YAML 解码基线
goreleaser check                                # 发布配置检查
```

发布使用 GoReleaser v2（`.goreleaser.yaml`），darwin/linux/windows 的
amd64/arm64 均使用 `CGO_ENABLED=0` 构建。
