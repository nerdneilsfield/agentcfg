# 用 CLIProxy 为 Pi、OpenCode、Grok 和 Kimi 启用 Codex fast

在 CLIProxy 中为同一个 Codex 模型增加 `-fast` 别名，并只对这个别名注入
`service_tier: priority`。四个客户端通过模型选择切换普通模式和 fast，agentcfg
沿用现有模型字段，不需要新增 IR 字段或修改 emitter。

如果入口使用 GPT-Load，参见 [GPT-Load 2.x / 1.x 配置指南](gpt-load-codex-fast.md)，
其中也说明了 GPT-Load 与 CLIProxy 串联时的配置。

Fast 是服务档位，与推理强度独立；选择 fast 模型后仍可使用 high/xhigh。
Fast 会增加费用或订阅额度消耗，具体规则见 [OpenAI 的速度说明](https://learn.chatgpt.com/docs/agent-configuration/speed)。

## 适用条件

本文以 **CLIProxyAPI v8.0.32、Codex OAuth 后端**为基准，GUI 指服务自带的
Web 管理中心。界面名称按 2026-10-18 的官方面板源码核对；面板独立更新，旧版
界面的布局可能不同。

示例使用 `gpt-5.5` 和 `gpt-5.5-fast`。操作前，将两者替换为你账户可用且支持
fast 的真实模型 ID 和对应别名。`-fast` 只是本方案约定的名字，创建别名本身
不会开启 fast，还要配置 Payload 规则。

代理应已能通过 Codex OAuth 请求原模型。如果使用的是上游 API Key 分组，本文的
`oauth.model-alias` 不适用，模型映射应配置在对应的上游分组中。

## 在自带 GUI 中配置

### 1. 打开管理中心

打开 `http://127.0.0.1:8317/management.html`，输入**管理密钥**连接。
如果代理使用其他地址或端口，替换 URL。管理密钥与请求 `/v1` 接口使用的
客户端 API Key 是两套凭据。

如果管理接口返回 `404`，检查服务端 `management.secret-key` 是否为空；为空会
关闭管理接口。页面本身不可用时，检查 `management.disable-control-panel`。
这些设置见 [CLIProxy 官方配置示例](https://github.com/router-for-me/CLIProxyAPI/blob/v8.0.32/config.example.yaml)。

### 2. 增加 fast 别名

1. 打开 **认证文件** 页，找到 **OAuth 模型别名**。
2. 如果已有 `codex` 配置，选择该项的 **编辑**，保留原有映射并添加一行；
   否则选择 **新增别名**，将 **提供商** 设为 `codex`。
3. 将 **原模型名称** 设为 `gpt-5.5`，将 **别名 (必填)** 设为 `gpt-5.5-fast`。
4. 开启 **保留原名**，使原模型和 fast 别名同时出现在模型列表中。
5. 选择 **保存/更新**，确认提示模型别名已更新。

### 3. 为 fast 别名添加覆盖规则

1. 打开 **配置面板**，将编辑模式切换为 **可视化**。
2. 打开 **Payload 配置**，展开 **覆盖规则**，选择 **添加规则**。
3. 在该规则的 **适用模型** 中选择 **添加模型**，填写下表。

   | 字段 | 值 |
   | --- | --- |
   | 供应商类型 | `Codex`（配置值为 `codex`） |
   | 模型名称 | `gpt-5.5-fast` |

4. 在同一规则的 **参数设置** 中选择 **添加参数**，填写下表。

   | 字段 | 值 |
   | --- | --- |
   | JSON 路径 (如 temperature) | `service_tier` |
   | 参数类型 | 字符串 |
   | 参数值 | `priority`，不加引号 |

5. 保持 **来源协议 (from-protocol)** 和其他高级匹配条件为空，避免限制为某一种
   客户端协议。这里的 `codex` 指代理调用的后端，与客户端的 `openai-responses`
   配置不是同一个字段。
6. 选择 **保存更改**，在 **确认变更** 中核对差异后选择 **确认保存**。
   保存后重新加载配置，确认规则仍在。

使用普通的 **覆盖规则** 即可，不需要 **覆盖 Raw 规则**。规则只修改服务档位，
不设置 `reasoning.effort`。如果已有其他规则或插件设置 `service_tier`，先核对
它们的匹配范围；后续覆盖规则可能改写 `priority`，过滤规则也可能删除该字段。

## 核对保存后的 YAML

在 **配置面板** 切换为 **源码**，核对以下两部分。也可以直接在源码模式编辑，
保存并确认差异。合并到已有的 `oauth`、`requests` 节点中，保留其他配置；同一层
不要重复创建同名 YAML 键。

```yaml
oauth:
  model-alias:
    codex:
      - name: gpt-5.5
        alias: gpt-5.5-fast
        fork: true

requests:
  payload:
    override:
      - models:
          - name: gpt-5.5-fast
            protocol: codex
        params:
          service_tier: priority
```

`fork: true` 对应 GUI 的 **保留原名**。CLIProxy 会在路由时把别名解析为真实模型，
Payload 匹配仍能使用客户端请求的别名，所以原模型不会因这条规则被强制开启 fast。
原模型继续使用已有的档位策略，不保证其他全局规则或上游默认值一定是普通档位。

以上是 v8 配置结构。旧版教程常用顶层 `oauth-model-alias`、`payload` 和
`remote-management`，不能直接当作本节的 v8 字段路径。旧版部署请先核对服务和
面板版本，再参考对应版本的官方配置。

## 通过 agentcfg 接入四个客户端

在现有 CLIProxy provider 下复制原模型条目，只改新条目的 `id` 和 `name`。
上下文窗口、模态、工具能力与推理档位继续沿用原模型。Grok 和 Kimi 都要求
`context_window`，遗漏时 agentcfg 会跳过该模型。

下面是一份可单独保存为 `agentcfg.cliproxy.yaml` 的示例。YAML 锚点让两个模型
共享能力配置；`context_window`、输出上限和 `variants` 是示例配置值，使用时
替换为原模型已验证的值，不将它们视为官方模型规格。

```yaml
version: 1

providers:
  - id: cliproxy
    name: CLIProxy
    protocol: openai-responses
    base_url: http://127.0.0.1:8317/v1
    api_key: "ENV:CLIPROXY_API_KEY"
    models:
      - &codex_model
        id: gpt-5.5
        name: GPT-5.5
        context_window: 128000
        max_output_tokens: 16384
        input: [text, image]
        output: [text]
        reasoning: true
        variants: [low, medium, high, xhigh]
        tool_calling: true
      - <<: *codex_model
        id: gpt-5.5-fast
        name: GPT-5.5 Fast

targets: [pi, opencode, grok, kimi]
```

将代理的**客户端 API Key** 设置在环境变量 `CLIPROXY_API_KEY` 中。不要使用
管理密钥或 Codex OAuth token。先校验并生成到独立目录：

```sh
rtk agentcfg validate --config agentcfg.cliproxy.yaml
rtk agentcfg gen --config agentcfg.cliproxy.yaml --output ./generated-cliproxy/
```

OpenCode V2 可能提示 `reasoning` 标记被跳过；推理档位仍由 `variants` 生成。
检查生成的四组配置都包含原模型和 fast 别名，且 fast 条目的能力、限额和推理
档位与原条目一致。各端文件位置见 [Pi](agents/pi.md)、[OpenCode](agents/opencode.md)、
[Grok](agents/grok.md) 和 [Kimi](agents/kimi.md)。

将条目合并到现有 agentcfg 输入后，也可以使用 `--in-place` 更新客户端原生配置。
该操作会替换 agentcfg 管理的 provider/model 和 MCP 区块；先将需要保留的条目
纳入输入并备份原文件。仅新增 fast 模型时，手工合并生成目录中的模型条目即可。

加载配置后，在四个客户端各自的模型选择器中选择 `gpt-5.5-fast`。选择原模型
则恢复原有档位策略，无需调整客户端的推理强度设置。

## 验证 fast 是否生效

先查询模型列表。以下命令读取环境变量 `CLIPROXY_API_KEY`；如果代理不在本机
默认端口，将 `CLIPROXY_URL` 设置为代理根地址，不带 `/v1`。

```sh
rtk proxy curl --fail-with-body -sS \
  "${CLIPROXY_URL:-http://127.0.0.1:8317}/v1/models" \
  -H "Authorization: Bearer ${CLIPROXY_API_KEY:?请先设置客户端 API Key}"
```

模型列表应同时包含 `gpt-5.5` 和 `gpt-5.5-fast`。再用 fast 别名发一个
Responses 请求；这个步骤会产生实际模型用量：

```sh
rtk proxy curl --fail-with-body -sS -N --max-time 120 \
  "${CLIPROXY_URL:-http://127.0.0.1:8317}/v1/responses" \
  -H "Authorization: Bearer ${CLIPROXY_API_KEY:?请先设置客户端 API Key}" \
  -H 'Content-Type: application/json' \
  --data '{"model":"gpt-5.5-fast","input":"Reply with OK.","reasoning":{"effort":"high"},"stream":true}'
```

按下面的证据区分“别名可用”和“上游实际使用 fast”：

| 检查位置 | 预期结果 |
| --- | --- |
| 代理收到的请求 | `model` 是 `gpt-5.5-fast`。 |
| 代理发给 Codex 的最终请求 | `model` 是 `gpt-5.5`，`service_tier` 是 `priority`，推理强度仍为 `high`。 |
| 上游响应或 SSE 的 response 对象 | 检查实际 `service_tier`；`priority` 或 `fast` 表示使用相应服务档位。 |
| 改用原模型再次请求 | 本文的别名规则不注入 `priority`；其他既有策略仍可能设置服务档位。 |

若响应返回 `service_tier: default`，该请求按默认档位处理，不能仅凭模型名称
带有 `fast` 或体感变快判断成功。如果下游响应未暴露档位，检查代理可提供的上游
请求/响应记录；没有这些证据时，只能确认别名可用和请求成功。

代理侧验证通过后，再在四个客户端各发一个短请求，检查其入站模型 ID 和代理
最终发出的服务档位。这样可以发现客户端未加载新配置、仍在使用原模型等问题。

## 常见问题

| 症状 | 检查方法 |
| --- | --- |
| GUI 中找不到相关控件 | 核对后端与面板版本；在支持 v8 的面板中使用源码模式核对 YAML。 |
| 增加别名后原模型消失 | 检查是否开启 **保留原名**，以及是否存在排除规则或其他同名映射。 |
| 能选择 fast 模型，但上游没有 `priority` | 检查是否只创建了别名；确认覆盖规则匹配 fast 别名、供应商为 `codex`，且未被后续规则或插件改写。 |
| 所有模型都使用 fast | 检查已有规则是否用 `gpt-*` 等范围设置服务档位，以及是否启用了全局服务档位插件。 |
| Grok/Kimi 看不到 fast 模型 | 检查 agentcfg 输入是否保留 `context_window`，并确认客户端加载了更新后的文件。 |
| 原模型可用，但 fast 报错或返回默认档位 | 检查模型和账户是否支持 fast，并查看上游错误或返回档位；代理注入参数不会获得额外账户权限。 |

## 来源与验证范围

- [CLIProxyAPI v8.0.32 配置示例](https://github.com/router-for-me/CLIProxyAPI/blob/v8.0.32/config.example.yaml)：别名、Payload 和管理接口字段。
- [CLIProxy Payload 匹配实现](https://github.com/router-for-me/CLIProxyAPI/blob/v8.0.32/internal/runtime/executor/helps/payload_helpers.go)：匹配时保留客户端请求的别名。
- [官方 Web 管理中心说明](https://github.com/router-for-me/Cli-Proxy-API-Management-Center/blob/fe2963330f32a933f562282165f62ae52cf57145/README_CN.md)与[简体中文界面文本](https://github.com/router-for-me/Cli-Proxy-API-Management-Center/blob/fe2963330f32a933f562282165f62ae52cf57145/src/i18n/locales/zh-CN.json)：GUI 入口、控件名称和密钥用途。
- [OpenAI Fast mode](https://developers.openai.com/api/docs/guides/fast-mode)：服务档位参数与实际响应档位。

本文的 GUI 步骤经上游源码核对，agentcfg 示例经本地校验与四端配置生成验证。
未连接读者的代理或账户进行真实推理测试；实际 fast 权限与处理档位按上节确认。
