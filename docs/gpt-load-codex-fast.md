# 用 GPT-Load 为 Pi、OpenCode、Grok 和 Kimi 启用 Codex fast

GPT-Load 可以为请求注入 `service_tier: priority`，让四个客户端通过选择 fast
模型切换服务档位。本文分别说明 **2.x** 和 **1.x** 的自带 Web 界面配置；如果
使用 2.x，按下一节操作即可。

示例使用 `gpt-5.5` 和 `gpt-5.5-fast`，请替换为账户实际可用且支持 fast 的模型
和对应别名。Fast 与 high/xhigh 推理强度独立，会增加费用或订阅额度消耗，具体
规则见 [OpenAI 的速度说明](https://learn.chatgpt.com/docs/agent-configuration/speed)。

## GPT-Load 2.x：同一分组保留普通模型和 fast 别名

本节按 **v2.0.0-rc.51** 的源码核对。2.x 提供 Codex 订阅渠道，也支持通过
OpenAI Compatible 渠道连接 CLIProxy 等中转。先确认原模型已能请求，再添加别名
和参数规则。2.x 的统一客户端入口是 `/v1`，不使用 1.x 的 `/proxy/<分组名>/v1`。

### 1. 添加模型映射

在自带 GUI 中打开对应分组的 **模型** 或 **管理模型**，保留原条目，并添加一条
相同上游模型 ID 的映射：

| 上游模型 ID | 别名设置 | 客户端请求的模型名 |
| --- | --- | --- |
| `gpt-5.5` | 不启用别名 | `gpt-5.5` |
| `gpt-5.5` | 启用对外别名，填写 `gpt-5.5-fast` | `gpt-5.5-fast` |

保存模型列表。第二行的上游模型 ID 仍是 `gpt-5.5`，只将对外别名设为
`gpt-5.5-fast`。同一上游模型可使用不同对外名称，两个条目的客户端模型名需不同。

### 2. 添加参数覆盖规则

新版界面从分组详情打开 **高级配置**，找到 **参数覆盖**；经典界面从分组的
**分组设置** 打开 **参数覆盖**。两种界面使用相同的规则格式。

1. 选择 **添加规则**，将 **协议** 设为 **全部协议**。
2. 将 **模型匹配**（经典界面为 **模型**）填为 `gpt-5.5-fast`。
3. 在 **逐项** 编辑方式中添加一个参数，填写下表。

   | 字段 | 值 |
   | --- | --- |
   | 操作（经典界面为动作） | 设置 |
   | 参数路径 | `service_tier` |
   | 值类型（经典界面为类型） | 文本 |
   | 参数值 | `priority`，不加引号 |

4. 保存分组配置，重新打开规则，确认模型匹配条件和参数仍在。

规则只修改服务档位，不设置 `reasoning/effort`。留空协议可覆盖四个客户端所用的
OpenAI Responses 请求；如果要限定为 Responses，协议值为 `openai-responses`，
不是 CLIProxy Payload 规则中的 `codex`。

如果切换规则的 **设置参数 · JSON**（经典界面为 **转 JSON**），文本框只编辑
`set` 对象，填写：

```json
{
  "service_tier": "priority"
}
```

**协议** 和 **模型匹配** 仍在文本框外设置。不要把下面的完整规则数组粘贴到
这个只接收 `set` 对象的文本框中。

完整的 `parameter_overrides` 配置值如下，供核对配置结构：

```json
[
  {
    "match": {
      "model": "gpt-5.5-fast"
    },
    "set": {
      "service_tier": "priority"
    }
  }
]
```

规则匹配客户端请求的模型名，普通模型 `gpt-5.5` 不会命中这条规则。规则按顺序
应用，后面的规则可能覆盖前面的值；检查已有规则是否再次设置或删除
`service_tier`。模型资料中的 `service_tiers` 是能力声明，不能替代这里的请求
参数覆盖。

### 3. 生成四个客户端的配置

将下面的示例保存为 `agentcfg.gpt-load.yaml`。能力、限额和推理档位是示例配置，
请换成原模型已验证的值；YAML 锚点让普通模型和 fast 条目共享这些字段。

```yaml
version: 1

providers:
  - id: gpt-load
    name: GPT-Load
    protocol: openai-responses
    base_url: http://127.0.0.1:3001/v1
    api_key: "ENV:GPTLOAD_API_KEY"
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

将 GPT-Load 的客户端访问密钥设置在环境变量 `GPTLOAD_API_KEY` 中，并确保密钥
具有该分组的访问权限。替换代理地址后校验并生成：

```sh
rtk agentcfg validate --config agentcfg.gpt-load.yaml
rtk agentcfg gen --config agentcfg.gpt-load.yaml --output ./generated-gpt-load/
```

OpenCode V2 提示 `reasoning` 标记被跳过时，`variants` 仍会生成推理档位。
Grok/Kimi 要求模型包含 `context_window`，复制条目时保留这个字段。
原生文件位置和合并注意事项见 [CLIProxy 指南的客户端接入部分](cliproxy-codex-fast.md#通过-agentcfg-接入四个客户端)。

## GPT-Load 1.x：使用 fast 专用分组

本节按 **v1.4.12** 核对。1.x 的 **参数覆盖** 会修改该分组的所有请求，不能在
同一分组中直接使用上面的 `match` / `set` 数组。需要由 GPT-Load 注入档位时，
复制现有分组，将普通模式和 fast 分开。

1. 在分组管理中保留原分组 `codex`，复制出 `codex-fast`，沿用原分组的上游和
   凭据；使用 Responses 的客户端选择 `openai-response` 渠道类型。
2. 编辑 `codex-fast`，在 **参数覆盖** 中填写以下对象：

   ```json
   {
     "service_tier": "priority"
   }
   ```

3. 在同一分组的 **模型重定向规则** 中填写：

   ```json
   {
     "gpt-5.5-fast": "gpt-5.5"
   }
   ```

4. 保存分组配置。原分组不添加这项覆盖，两个客户端地址分别为：

   | 用途 | agentcfg 的 `base_url` | 模型 ID |
   | --- | --- | --- |
   | 普通 | `http://127.0.0.1:3001/proxy/codex/v1` | `gpt-5.5` |
   | Fast | `http://127.0.0.1:3001/proxy/codex-fast/v1` | `gpt-5.5-fast` |

agentcfg 声明两个 provider，分别使用表中的地址；将原模型的能力配置复制到
fast 条目，并使用不同的模型 ID，避免 Grok/Kimi 的扁平模型表发生名称冲突。
1.x 分组中的所有请求都会应用其覆盖；模型重定向只改模型名，不会给参数覆盖
增加按模型匹配的条件。

## GPT-Load 的上游仍然是 CLIProxy 时

如果已按 [CLIProxy 指南](cliproxy-codex-fast.md)配置了 fast 别名和 Payload
规则，可以让 GPT-Load 保留客户端的 `gpt-5.5-fast`，由 CLIProxy 统一注入档位。
此时 GPT-Load 不再添加同一项覆盖，也不要提前把 fast 别名改写成 `gpt-5.5`，
否则 CLIProxy 的别名规则无法命中。

2.x 在模型列表中将 `gpt-5.5-fast` 作为上游模型 ID 透传；1.x 检查模型重定向和
未配置模型策略，确保 fast 别名能到达 CLIProxy。客户端的地址和访问密钥仍使用
GPT-Load 的入口与客户端密钥。

如果改为在 GPT-Load 注入 `priority`，则使用前面的模型映射，由 GPT-Load 将
fast 别名解析为真实模型，再把带档位的请求发给 CLIProxy。选择一个代理负责
档位设置，可以减少两边规则相互覆盖的问题。

## 验证与排错

配置生成成功只能证明客户端配置可生成。加载生成的模型条目后，在四个客户端
各发送一个短请求，检查以下内容；请求会产生实际模型用量。

| 检查位置 | 预期结果 |
| --- | --- |
| GPT-Load 收到的请求 | 模型名为 `gpt-5.5-fast`。 |
| 最终发给模型后端的请求 | 模型名为 `gpt-5.5`，`service_tier` 为 `priority`，推理强度保持客户端所选值。 |
| 上游响应或 SSE 的 response 对象 | 检查实际 `service_tier`；返回 `default` 表示该次请求按默认档位处理。 |
| 改用普通模型请求 | 2.x 不命中 fast 规则；1.x 使用普通分组，避免误用 fast 分组入口。 |

2.x 使用 `/v1/models` 和 `/v1/responses`，1.x 使用对应分组前缀下的接口。
可参考 [CLIProxy 指南的请求示例](cliproxy-codex-fast.md#验证-fast-是否生效)，将
URL 和客户端 API Key 换成 GPT-Load 的值。串联两层代理时，检查最终出站请求，
而不仅是 GPT-Load 的入站记录。下游没有暴露实际档位时，需查上游请求/响应记录；
仅凭别名或响应变快无法确认 fast 生效。

## 来源与验证范围

- [GPT-Load 2.x README](https://github.com/tbphp/gpt-load/blob/v2.0.0-rc.51/README_CN.md)：客户端入口和 Codex 订阅渠道。
- [2.x 模型映射测试](https://github.com/tbphp/gpt-load/blob/v2.0.0-rc.51/internal/control/group_models_test.go)：同一上游模型可配置不同对外名称。
- [2.x 参数规则](https://github.com/tbphp/gpt-load/blob/v2.0.0-rc.51/internal/parameteroverride/rules.go)与[网关应用顺序](https://github.com/tbphp/gpt-load/blob/v2.0.0-rc.51/internal/gateway/handler.go)：规则格式及客户端模型名匹配。
- [2.x 新版 GUI 文本](https://github.com/tbphp/gpt-load/blob/v2.0.0-rc.51/web/src/frontends/modern/i18n/locales/parameter-rules.ts)与[经典 GUI 编辑器](https://github.com/tbphp/gpt-load/blob/v2.0.0-rc.51/web/src/frontends/classic/components/config/ParameterOverrideRulesEditor.vue)：逐项与 JSON 编辑的范围。
- [1.x 覆盖实现](https://github.com/tbphp/gpt-load/blob/v1.4.12/internal/proxy/request_helpers.go)、[GUI 字段](https://github.com/tbphp/gpt-load/blob/v1.4.12/web/src/locales/zh-CN.ts)及[入口说明](https://github.com/tbphp/gpt-load/blob/v1.4.12/README_CN.md#1-代理端点格式)。

GUI 步骤和代理行为经上述版本的源码核对，示例经 JSON/YAML 校验及四端配置生成
验证。未连接实际 GPT-Load 部署进行推理测试，账户的 fast 权限与实际处理档位
仍需按上一节检查。
