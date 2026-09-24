# agentcfg

[![CI](https://github.com/nerdneilsfield/agentcfg/actions/workflows/ci.yml/badge.svg)](https://github.com/nerdneilsfield/agentcfg/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/nerdneilsfield/agentcfg?display_name=tag&label=release)](https://github.com/nerdneilsfield/agentcfg/releases/latest)
[![Go Reference](https://pkg.go.dev/badge/github.com/nerdneilsfield/agentcfg.svg)](https://pkg.go.dev/github.com/nerdneilsfield/agentcfg)
[![Go Report Card](https://goreportcard.com/badge/github.com/nerdneilsfield/agentcfg)](https://goreportcard.com/report/github.com/nerdneilsfield/agentcfg)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

[English](README.md) | 中文

agentcfg 把一份 `agentcfg.yaml` 编译成各编码智能体 CLI 的原生配置片段。
自定义模型供应商和 MCP 服务器只需描述一次，即可为支持的 CLI 分别渲染配置。

设计原则：

- **显式配置凭据。** 使用带引号的字符串字面量或 `ENV:NAME`。
  agentcfg 不读取进程环境变量；字面量按原值写入生成结果，引用则翻译成各 CLI
  的原生语法。
- **只写 stdout。** `gen` 只打印片段，从不写目标文件。先审阅输出，再自行合并
  进原生配置。
- **绝不瞎猜。** 当某个 target 无法表示 IR 中的字段时，会跳过该字段并给出
  warning（写明字段与原因），而不是静默丢弃，也不会因为一个字段不支持就让整个
  target 失败。

## 支持的 target

| Target | 智能体 | Provider 支持 | MCP | 说明 |
|---|---|---|---|---|
| `aider` | [Aider-AI/aider](https://github.com/Aider-AI/aider) | Chat · Anthropic | 跳过 | 模型 settings + metadata + 默认配置；LiteLLM 运行时解析环境 key |
| `continue` | [continuedev/continue](https://github.com/continuedev/continue) | Chat · Anthropic | stdio + HTTP | YAML 配置；已发布 schema 丢弃 Responses 选择；默认模型由客户端保存，跳过并 warning |
| `qwen-code` | [QwenLM/qwen-code](https://github.com/QwenLM/qwen-code) | Chat · Responses · Anthropic | stdio + HTTP | 具名模型供应商、协议映射、generation config 与 MCP exclusions |
| `kilo` | [Kilo-Org/kilo](https://github.com/Kilo-Org/kilo) | Chat · Responses · Anthropic | stdio + HTTP | Kilo CLI，非旧 IDE extension；单数 `provider` + `mcp`；无 MCP `cwd` |
| `codex` | [openai/codex](https://github.com/openai/codex) | 仅 Responses | stdio + HTTP | `[model_providers]` TOML 片段；`ENV:NAME` 写入 `env_key`；MCP `timeout_ms` 写入 `startup_timeout_ms` |
| `opencode` | [sst/opencode](https://github.com/sst/opencode) | Chat · Responses · Anthropic | stdio + HTTP | 原生 V2 形状（`providers` + `mcp.servers`）；`ENV:NAME` 写入 `env: [NAME]`；供应商 headers 与模型 reasoning 标志无法表示，跳过并 warning |
| `pi` | [earendil-works/pi](https://github.com/earendil-works/pi) | 任意（TypeScript 扩展） | v1 跳过（无内置 MCP） | 输出 provider 扩展；header 值用 Pi `$NAME` 语法；默认模型在扩展加载前保持延后 |
| `prime-agent` | [契约文档](docs/agents/prime-agent.md) | 任意（`models.json`） | stdio + HTTP | `models.json` + `settings.json` `mcpServers` 片段；跳过 `Bearer ENV:NAME` provider headers |
| `deepseek-harness` | [deepseek-ai/deepseek-harness](https://github.com/deepseek-ai/deepseek-harness) | 任意（`api` 路由字段） | stdio + HTTP（Cordis 补丁） | YAML provider 路由 + `@deepseek-ai/dsh-mcp-client` 补丁；跳过字面量 API key |
| `grok` | [xai-org/grok-build](https://github.com/xai-org/grok-build) | Chat · Responses · Anthropic | stdio + HTTP | 按模型拆 TOML 表；跨供应商重复模型 id 会被跳过；`ENV:NAME` headers 写入 `env_http_headers` |
| `kimi` | [MoonshotAI/kimi-code](https://github.com/MoonshotAI/kimi-code) | Chat · Responses · Anthropic | stdio + HTTP | 扁平 `[models]` 别名表；重复模型 id 会跳过；`ENV:NAME` API key 写入 `api_key_env`；MCP `timeout_ms` 写入 `startupTimeoutMs`；供应商 headers 仍仅字面量 |
| `zcode` | [zcode.z.ai](https://zcode.z.ai) | Chat · Anthropic | stdio + HTTP | 闭源 CLI；MCP env/headers 仅支持字面量 |
| `mimocode` | [XiaomiMiMo/MiMo-Code](https://github.com/XiaomiMiMo/MiMo-Code) | Chat · Anthropic | stdio + HTTP | 单个 JSON 片段，对齐官方线上 schema |
| `jcode` | [1jehuang/jcode](https://github.com/1jehuang/jcode) | Chat · Anthropic | stdio | 仅自定义供应商；Responses API 仅内置供应商可用；headers 仅字面量 |
| `cline` | [cline/cline](https://github.com/cline/cline) | Chat · Responses · Anthropic | stdio + HTTP | `apiKey` 仅字面量（跳过 `api_key: "ENV:NAME"`）；跳过 MCP 环境变量引用；timeout 为秒（`ms/1000`） |
| `gajae` | [Yeachan-Heo/gajae-code](https://github.com/Yeachan-Heo/gajae-code) | Chat · Responses · Anthropic | stdio + HTTP | 三种协议 1:1 映射 |
| `hermes` | [NousResearch/hermes-agent](https://github.com/NousResearch/hermes-agent) | Chat · Responses · Anthropic | stdio + HTTP | 不输出供应商显示 `name` |
| `openclaw` | [openclaw/openclaw](https://github.com/openclaw/openclaw) | Chat · Responses · Anthropic | stdio + HTTP | 原生 JSON5；绝不写 agent 本地 `models.json` |
| `crush` | [charmbracelet/crush](https://github.com/charmbracelet/crush) | Chat · Responses · Anthropic | stdio + HTTP | 每个模型必须带 `context_window` + `default_max_tokens`；不支持 MCP `cwd`；timeout 取整秒 |
| `goose` | [block/goose](https://github.com/block/goose) | Chat · Responses（经 `base_path`） · Anthropic | stdio + streamable HTTP | 无法表示的字段一律跳过：模型级 max tokens、非文本模态、`tool_calling: false`、改名的 MCP env 引用、非整秒 timeout；供应商 headers 仅字面量 |
| `commandcode` | [CommandCodeAI/command-code](https://github.com/CommandCodeAI/command-code) | Chat · Responses · Anthropic | stdio + HTTP | `providers.json` + `.mcp.json` + `config.json` 片段；字面量 API key 与环境派生的供应商 headers 跳过并 warning；内置 lane 的 provider id 跳过；无 MCP `cwd`/`timeout` |
| `fast-agent` | [evalstate/fast-agent](https://github.com/evalstate/fast-agent) | Chat · Responses · Anthropic | stdio + HTTP | 配置 + 原生 model overlays + 可选 secrets 片段；MCP timeout 仅支持整秒；禁用服务器跳过 |
| `crow` | [crow-cli/crow-cli](https://github.com/crow-cli/crow-cli) | 仅 Chat | stdio + HTTP | `~/.agents/crow/config.yaml` 片段；默认模型排首位；无供应商 headers 与 MCP `cwd`/timeout；禁用服务器跳过 |
| `cometixcode` | [Haleclipse/CometixCode](https://github.com/Haleclipse/CometixCode) | 仅 Anthropic（经 `ANTHROPIC_BASE_URL` 单端点） | stdio + HTTP | Claude Code 的 Rust 重实现；`settings.json` + `.mcp.json` 片段；`auth_type` 决定写 `ANTHROPIC_API_KEY` 还是 `ANTHROPIC_AUTH_TOKEN`；仅字面量凭据；模型级字段跳过并 warning |

"Chat" = OpenAI Chat Completions，"Responses" = OpenAI Responses API。
每个 target 逐字段的完整契约（含跳过项与原因）见
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

**用 Go 1.27+** 时先克隆再安装。Go module 路径是 `agentcfg`，
`go install github.com/nerdneilsfield/agentcfg/...` 无法解析：

```sh
git clone https://github.com/nerdneilsfield/agentcfg
cd agentcfg && go install ./cmd/agentcfg
```

**从源码构建：**

```sh
git clone https://github.com/nerdneilsfield/agentcfg
cd agentcfg && make build   # 产出 ./agentcfg
```

## 快速上手

输入中的密钥字面量也会出现在生成结果中。请勿将这类输入和输出提交到 Git 或写入日志。
目标支持环境变量引用时，使用 `ENV:NAME`。

1. 在项目旁写一份 `agentcfg.yaml`。完整字段说明见
   [`docs/protocol.md`](docs/protocol.md)。仓库自带
   [`example.yaml`](example.yaml)。`agentcfg gen-example -o agentcfg.yaml`
   会在项目旁写出一份副本；不带 `-o` 时打印到 stdout。已存在的文件不会被覆盖。

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
        # 可选的 provider/model 推理等级；它们不是默认值。
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

这份文件的含义：

- `version: 1` 必填。未知字段和第二份 YAML 文档（`---`）都会报错。
- 每个 provider 只有一种 `protocol`。`openai-completions` 是 Chat Completions，
  `openai-responses` 是 Responses API，`anthropic-messages` 是 Anthropic。
  同时提供两种协议的网关要写成两个 provider。
- `api_key` 和 header 值都是带引号的字符串：`"literal"`、`"ENV:NAME"`，
  或仅用于 `Authorization` 的 `"Bearer ENV:NAME"`。agentcfg 不会读取这些
  环境变量。
- `command` 是 argv 数组，不是 shell 字符串。
- `variants` 列出该模型可选的推理等级，并不选择默认等级。Crush 会保留
  `ultra` 这类名称。Gajae 的原生等级是 `minimal`、`low`、`medium`、`high`、
  `xhigh`、`max`，会跳过 `ultra`，其余模型字段照常输出。
- `targets: [crush, gajae]` 只在省略 `--to` 时作为默认选择，不是通配符。
  `targets: [all]` 是未知 id。

本例对 Crush 和 Gajae 合法，并不能用于所有 CLI：Codex 跳过
`openai-completions`，Cline/ZCode
跳过 `api_key` 上的 `ENV:NAME`，Kimi 跳过环境变量引用的供应商 headers，
Pi 跳过 MCP。把真正要生成的 CLI 写进 `targets:`，再用 `--to` 覆盖。

2. 校验 IR 与所有选中的 target。`--config` 默认为当前目录的 `agentcfg.yaml`：

```sh
agentcfg validate --config agentcfg.yaml
```

成功时在 stderr 打印一行摘要：

```
agentcfg.yaml: OK (2 providers, 2 MCP servers, 2 targets)
```

失败时会带上 target、IR 路径和原因，例如：

```
[crush] providers[0].models[0].context_window: error: crush Model.context_window is required
```

3. 生成原生片段：

```sh
agentcfg gen --config agentcfg.yaml
agentcfg gen --config agentcfg.yaml --to crush
agentcfg gen --config agentcfg.yaml --to all
```

`--to` 覆盖 YAML 里的 `targets:`。`--to all` 表示本二进制编译进去的全部
emitter，不是机器上已安装的全部 CLI。

单个产物直接输出原文（合法的 JSON、TOML、YAML 或 TypeScript）。
多个产物按稳定的 bundle 流输出，每个文件一个块：

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
新建文件可直接使用块内内容；已有文件需要手动合并。校验或生成失败时 stdout
保持为空，进程以退出码 `1` 结束。

## 怎么写 agentcfg.yaml

IR 与具体 CLI 的字段名无关。原生拼写见
[`docs/protocol.md`](docs/protocol.md) 和 [`docs/agents/`](docs/agents/README.md)。
下面是最常先碰到的写法。

### 凭据

```yaml
api_key: "ENV:VOLC_API_KEY"          # 环境变量引用
api_key: "sk-example-not-a-real-key" # 字面量，会进入生成结果
headers:
  X-Tenant: "engineering"            # 字面量
  X-Gateway-Key: "ENV:GATEWAY_KEY"   # 环境变量引用
  Authorization: "Bearer ENV:TOKEN"  # 仅 Authorization
```

会被 YAML 当成布尔值或数字的内容（`true`、`no`、`1e6`）必须加引号。
Cline 和 ZCode 把 API key 存成字面量，跳过 `api_key` 上的 `ENV:NAME`。
Kimi 把 `api_key: "ENV:NAME"` 写成 `api_key_env`。Goose 和 DeepSeek Harness
没有字面量密钥字段，会跳过字面量 `api_key`。

### 协议

```yaml
protocol: openai-completions   # Chat Completions
protocol: openai-responses     # Responses API；Codex 只接受这个
protocol: anthropic-messages   # Anthropic Messages
```

`base_url` 是 CLI 实际请求的地址，通常包含 `/v1`。

### 模型

Crush 和 Kimi 要求 `context_window`。Crush 还要求 `max_output_tokens`。
Goose 跳过模型级 `max_output_tokens`，也跳过非文本模态。

```yaml
models:
  - id: glm-5.3
    name: GLM-5.3
    context_window: 128000
    max_output_tokens: 8192
    input: [text, image]
    output: [text]
    reasoning: true
    variants: [low, medium, high]
    tool_calling: true
```

`id` 是上游线路上的模型名。`variants` 要求 `reasoning: true`。名称匹配
`[a-z][a-z0-9_-]*`（`Ultra` 不合法）。

### MCP

```yaml
mcp:
  - id: context7
    transport: stdio
    command: [npx, -y, "@upstash/context7-mcp"]
    cwd: /var/lib/context7          # 仅 stdio；Crush/MiMo Code/jcode 会跳过
    timeout_ms: 30000               # 毫秒；有的 target 会换成秒
    env:
      CONTEXT7_API_KEY: "ENV:CONTEXT7_API_KEY"
    enabled: true

  - id: github
    transport: http
    url: https://api.githubcopilot.com/mcp/
    headers:
      Authorization: "Bearer ENV:GITHUB_TOKEN"
```

IR 没有 `sse` 传输。会区分 SSE 与 streamable HTTP 的 target 把 `http` 映射成
streamable HTTP。Pi 在 v1 跳过全部 MCP。jcode 只加载 stdio。

`timeout_ms` 在 Gajae、Codex、Kimi（`startupTimeoutMs`）、
ZCode 等 target 上仍是毫秒。OpenCode V2 写入 `timeout` 对象，同时设置
`catalog` 与 `execution`。Crush、Goose、jcode 要求整秒（`timeout_ms`
能被 1000 整除）。Grok 和 Prime Agent 没有已验证的原生超时字段，会直接跳过。
只有所有选中的 target 都能表示时才写这个字段。

### 默认模型与 target 选择

```yaml
defaults:
  model: volcengine/glm-5.3   # provider-id/model-id；必须在本文档中存在

targets: [crush, gajae]       # 省略 --to 时使用
```

选择顺序：`--to` 参数，然后是 YAML 的 `targets:`，否则报错。把全部 16 个
id 写进 YAML 几乎没有用：同一份文档很少能同时在 Codex、OpenCode、Cline、
Kimi、Pi 上忠实表示。

## CLI

```text
agentcfg validate [--config FILE] [--to TARGETS] [--verbose|--debug]
agentcfg gen      [--config FILE] [--to TARGETS] [--verbose|--debug]
agentcfg gen-example [--output FILE]
agentcfg version
```

| 参数 | 默认值 | 含义 |
|---|---|---|
| `-c`, `--config` | `agentcfg.yaml` | IR 文档路径。 |
| `-t`, `--to` | （YAML `targets:`） | 逗号分隔的 target id，或 `all`。 |
| `-v`, `--verbose` | 关 | 在 stderr 打 info 日志。 |
| `-d`, `--debug` | 关 | 在 stderr 打 debug 日志。 |
| `-o`, `--output` | stdout | 仅 `gen-example`：把内置示例写到该路径。 |

`--config`、`--to`、`--verbose`、`--debug` 是根命令参数，因此
`agentcfg -c file.yaml gen` 和 `agentcfg gen -c file.yaml` 等价。
`gen-example` 不读 `--config`，它写出内嵌的 [`example.yaml`](example.yaml)。

stdout 只给产物（`gen`）或示例 YAML（`gen-example`）。诊断信息、`OK` 摘要
和日志走 stderr。成功退出码 `0`；用法、解码、校验或生成失败为 `1`。

## 文档

- [`docs/protocol.md`](docs/protocol.md) — IR 契约：provider、model、MCP、
  defaults 的全部字段、校验规则和工作示例。
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
   跳过项）。
2. 新建 `internal/target/<id>/`，实现 `Validate` 与 `Emit`；`Emit` 返回产物，
   且绝不静默丢弃无法表示的 IR 字段。
3. 在 `internal/target/all/all.go` 里加空导入完成注册。
4. 补 golden 测试；跑 `make check`。

## 许可

[MIT](LICENSE)
