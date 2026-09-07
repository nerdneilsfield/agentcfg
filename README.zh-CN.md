# agentcfg

[![CI](https://github.com/nerdneilsfield/agentcfg/actions/workflows/ci.yml/badge.svg)](https://github.com/nerdneilsfield/agentcfg/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/nerdneilsfield/agentcfg?display_name=tag&label=release)](https://github.com/nerdneilsfield/agentcfg/releases/latest)
[![Go Reference](https://pkg.go.dev/badge/github.com/nerdneilsfield/agentcfg.svg)](https://pkg.go.dev/github.com/nerdneilsfield/agentcfg)
[![Go Report Card](https://goreportcard.com/badge/github.com/nerdneilsfield/agentcfg)](https://goreportcard.com/report/github.com/nerdneilsfield/agentcfg)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

[English](README.md) | 中文

agentcfg 把一份 `agentcfg.yaml` 编译成各编码智能体 CLI 的原生配置片段。
自定义模型供应商和 MCP 服务器只需描述**一次**，即可为所有支持的 CLI 渲染对应配置。

设计原则：

- **不碰密钥。** 配置只引用环境变量（`from_env`、`api_key_env`、
  `bearer_from_env`）；agentcfg 既不读取也不写出任何密钥值。
- **只写 stdout。** `gen` 只打印片段，从不写目标文件。你先审阅，再自行合并
  进原生配置。
- **绝不瞎猜。** 当某个 target 无法表示 IR 中的字段时，emitter 会带诊断信息
  直接拒绝，而不是静默丢弃。

## 支持的 target

| Target | 智能体 | Provider 支持 | MCP | 说明 |
|---|---|---|---|---|
| `codex` | [openai/codex](https://github.com/openai/codex) | 仅 Responses | stdio | `[model_providers]` TOML 片段；Codex 只支持 Responses API |
| `opencode` | [sst/opencode](https://github.com/sst/opencode) | 仅 OpenAI 兼容 | stdio + HTTP | `opencode.json` 中的 `provider` + `mcp`；v1 拒绝 Anthropic/Responses 供应商 |
| `pi` | [earendil-works/pi](https://github.com/earendil-works/pi) | 任意（TypeScript 扩展） | 经 `pi-mcp-adapter` | 输出 provider 扩展；默认模型在扩展加载前保持延后 |
| `prime-agent` | [契约文档](docs/agents/prime-agent.md) | 任意（`models.json`） | stdio + HTTP | `models.json` + `settings.json` `mcpServers` 片段 |
| `deepseek-harness` | [deepseek-ai/deepseek-harness](https://github.com/deepseek-ai/deepseek-harness) | 任意（`api` 路由字段） | stdio（Cordis 补丁） | YAML provider 路由 + `@deepseek-ai/dsh-mcp-client` 补丁 |
| `grok` | [xai-org/grok-build](https://github.com/xai-org/grok-build) | Chat · Responses · Anthropic | stdio + HTTP | 按模型拆 TOML 表；跨供应商重复模型 id 会被拒绝；headers 仅字面量 |
| `kimi` | [MoonshotAI/kimi-code](https://github.com/MoonshotAI/kimi-code) | Chat · Responses · Anthropic | stdio + HTTP | 扁平 `[models]` 别名表；重复模型 id 与环境变量引用均拒绝 |
| `zcode` | [zcode.z.ai](https://zcode.z.ai) | Chat · Responses · Anthropic | stdio | 闭源 CLI；MCP env/headers 仅支持字面量 |
| `mimocode` | [XiaomiMiMo/MiMo-Code](https://github.com/XiaomiMiMo/MiMo-Code) | Chat · Responses · Anthropic | stdio + HTTP | 单个 JSON 片段，对齐官方线上 schema |
| `jcode` | [1jehuang/jcode](https://github.com/1jehuang/jcode) | Chat · Anthropic | stdio + HTTP | 仅自定义供应商；Responses API 仅内置供应商可用；headers 仅字面量 |
| `cline` | [cline/cline](https://github.com/cline/cline) | Chat · Responses · Anthropic | stdio + HTTP | `apiKey` 仅字面量（拒绝 `api_key_env`）；拒绝 MCP 环境变量引用；timeout 收敛到 1–3600 秒 |
| `gajae` | [Yeachan-Heo/gajae-code](https://github.com/Yeachan-Heo/gajae-code) | Chat · Responses · Anthropic | stdio + HTTP | 三种协议 1:1 映射 |
| `hermes` | [NousResearch/hermes-agent](https://github.com/NousResearch/hermes-agent) | Chat · Responses · Anthropic | stdio + HTTP | 不输出供应商显示 `name` |
| `openclaw` | [openclaw/openclaw](https://github.com/openclaw/openclaw) | Chat · Responses · Anthropic | stdio + HTTP | 原生 JSON5；绝不写 agent 本地 `models.json` |
| `crush` | [charmbracelet/crush](https://github.com/charmbracelet/crush) | Chat · Responses · Anthropic | stdio + HTTP | 每个模型必须带 `context_window` + `default_max_tokens`；不支持 MCP `cwd`；timeout 取整秒 |
| `goose` | [block/goose](https://github.com/block/goose) | Chat · Responses（经 `base_path`） · Anthropic | stdio + streamable HTTP | 严格拒绝：模型级 max tokens、非文本模态、`tool_calling: false`、MCP env 引用、非整秒 timeout；供应商 headers 仅字面量 |

"Chat" = OpenAI Chat Completions，"Responses" = OpenAI Responses API。
每个 target 逐字段的完整契约（含拒绝项与原因）见
[`docs/agents/`](docs/agents/README.md)。`--to all` 选中以上全部 target。

## 安装

每个 `v*` tag 都由 GoReleaser 自动发布静态二进制，覆盖 darwin/linux/windows
的 amd64/arm64（`CGO_ENABLED=0`）。

**下载 release**（见 [Releases 页面](https://github.com/nerdneilsfield/agentcfg/releases)）：

```sh
VERSION=vX.Y.Z        # 取最新 tag
OS=darwin             # darwin | linux
ARCH=arm64            # amd64 | arm64

curl -fsSL -o agentcfg.tar.gz \
  "https://github.com/nerdneilsfield/agentcfg/releases/download/${VERSION}/agentcfg_${VERSION}_${OS}_${ARCH}.tar.gz"
curl -fsSL -O \
  "https://github.com/nerdneilsfield/agentcfg/releases/download/${VERSION}/agentcfg_${VERSION}_checksums.txt"

grep "_${OS}_${ARCH}" agentcfg_${VERSION}_checksums.txt | shasum -a 256 -c -
tar -xzf agentcfg.tar.gz
install -m 0755 agentcfg /usr/local/bin/agentcfg
agentcfg version
```

Windows 资产是 `.zip`，校验用 `certutil -hashfile`，解压用 `expand-archive`。

**用 Go 1.27+：**

```sh
go install github.com/nerdneilsfield/agentcfg/cmd/agentcfg@latest
```

**从源码构建：**

```sh
git clone https://github.com/nerdneilsfield/agentcfg
cd agentcfg && make build   # 产出 ./agentcfg
```

## 快速上手

1. 在项目旁写一份 `agentcfg.yaml`（完整字段说明见
   [`docs/protocol.md`](docs/protocol.md)）：

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

targets: [crush, goose, codex]
```

2. 校验 IR 与所有选中的 target：

```sh
agentcfg validate --config agentcfg.yaml
```

诊断信息会带上 target、IR 路径和原因，例如：

```
[crush] providers[0].models[0].context_window: error: crush Model.context_window is required
```

3. 生成原生片段：

```sh
agentcfg gen --config agentcfg.yaml
```

单个产物直接输出原文。多个产物按稳定的 bundle 流输出，每个文件一个块：

```
===== BEGIN agentcfg artifact =====
target: crush
name: crush.json
format: json
suggested-path: ~/.config/crush/crush.json
===== artifact content begins =====
{ ... }
===== END agentcfg artifact =====
```

`suggested-path` 指明片段应放进哪个原生文件。v1 中 agentcfg 从不写目标文件：
新建文件可直接使用块内内容；已有文件需要手动合并。

## 文档

- [`docs/protocol.md`](docs/protocol.md) — IR 契约：provider、model、MCP、
  defaults 的全部字段及校验规则。
- [`docs/architecture.md`](docs/architecture.md) — 编译器的整体结构。
- [`docs/agents/`](docs/agents/README.md) — 每个 target 的已验证契约：
  原生文件布局、schema 来源、字段映射、限制。

## 开发

需要 Go 1.27+。可选工具：`gofumpt`、`goimports`、`golangci-lint`，以及仅
release 目标需要的 `goreleaser`。

```sh
make build      # ./agentcfg
make test       # CGO_ENABLED=0 go test ./...
make fmt        # gofumpt + goimports + gofmt
make lint       # golangci-lint run ./...
make vet        # go vet ./...
make check      # fmt-check + lint + vet + test（与 CI 一致）
make bench      # YAML 解码基准
make release-check      # goreleaser check（需要 goreleaser；不在 check 内）
make release-snapshot   # goreleaser release --snapshot --clean
```

目录结构：

```
cmd/agentcfg/          CLI（Cobra）
internal/ir/           agentcfg.yaml 加载与校验（IR）
internal/target/<id>/  每个 target 一个 emitter 包：Validate + Emit
internal/target/all/   空导入注册全部 emitter
internal/artifact/     stdout 渲染（原文或 bundle 流）
internal/diag/         带稳定源路径的诊断
docs/agents/           各 target 原生配置契约
```

新增 target 的步骤：

1. 调研该智能体的原生配置，写 `docs/agents/<id>.md`（schema 来源、字段映射、
   拒绝项）。
2. 新建 `internal/target/<id>/`，实现 `Validate` 与 `Emit`；`Emit` 返回产物，
   且绝不静默丢弃无法表示的 IR 字段。
3. 在 `internal/target/all/all.go` 里加空导入完成注册。
4. 补 golden 测试；跑 `make check`。

## 许可

[MIT](LICENSE)
