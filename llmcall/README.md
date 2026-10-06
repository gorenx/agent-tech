# llmcall — 认识 LLM API

用 Go 和 openai-go SDK 直接调用 LLM，不使用 Agent 框架。

六节示例覆盖两种接口：Chat Completions 和 Responses API。每节都可以打印真实的 HTTP 请求与响应 JSON。

## 快速开始

```bash
cd llmcall
cp .env.example .env && source .env   # 填入 key

go run ./01-chat-basic
```

`LLM_DEBUG=1` 会把实际收发的 JSON 打到 stderr：

```bash
LLM_DEBUG=1 go run ./01-chat-basic
```

非流式请求打印完整 JSON：

```
>>> POST https://api.deepseek.com/chat/completions
{
  "model": "deepseek-flash",
  "messages": [
    {"role": "system", "content": "你是一个简洁的助手，回答不超过两句话。"},
    {"role": "user", "content": "用一句话解释什么是 HTTP。"}
  ]
}
<<< 200
{
  "id": "chatcmpl-...",
  "choices": [ ... ],
  "usage": { ... }
}
```

流式响应不能缓冲（缓冲后就不是流式了），所以按到达顺序逐行打印：

```
>>> POST https://api.deepseek.com/chat/completions
<<< 200 (text/event-stream)
  data: {...}
  data: {...}
  data: [DONE]
```

`wireDump`（`internal/llm/client.go`）实现这个行为：一个 `http.RoundTripper`，拦在 SDK 和网络之间。流式分支用 `sseEcho` 包装响应体，SDK 读到多少就转出多少，保持原有顺序。

## 课程地图

三组对照，每组是同一个任务用两种 API 各写一遍：

| # | 目录 | 主题 |
|---|------|------|
| 01 | `01-chat-basic` | Chat Completions 最小调用 |
| 02 | `02-responses-basic` | Responses 最小调用 |
| 03 | `03-chat-stream` | 流式输出 |
| 04 | `04-responses-stream` | 流式：事件序列 |
| 05 | `05-chat-multiturn` | 多轮：客户端维护状态 |
| 06 | `06-responses-multiturn` | 多轮：两种接续方式 |

六节都能运行。默认端点 DeepSeek 同时提供 `/chat/completions` 和 `/responses`。

## 关于「兼容 OpenAI」

以 DeepSeek 为例，Responses API 的功能它支持一部分、不支持一部分：

| Responses API 的组成部分 | DeepSeek |
|---|---|
| 协议形态（`instructions` / `input` / `output` items） | 支持 |
| 流式事件带类型（SSE + `event` 字段） | 支持 |
| 结构化输出（`text.format`） | 支持 |
| 服务端状态（`previous_response_id`） | 忽略 |
| `store` / `conversation` | 忽略 |
| 内建工具（`web_search` / `file_search` / `code_interpreter`） | 忽略 |

DeepSeek 文档说明：不支持的参数会被静默忽略，不返回错误。

所以下面两种判断方式不可靠：

- 请求返回 200 → 认为参数已生效
- SDK 编译通过 → 认为厂商支持该能力

06 节给出检测方法（读响应体里 `previous_response_id` 是否被回填）和无状态替代方案。

## 核心差异对照

### 0. 请求与响应的 JSON

两种 API 发出去的都是 JSON。下面两段可以分别用
`LLM_DEBUG=1 go run ./01-chat-basic` 和 `LLM_DEBUG=1 go run ./02-responses-basic` 打印出来。

**请求体**

```jsonc
// Chat Completions                      // Responses
{                                        {
  "model": "deepseek-flash",               "model": "deepseek-flash",
  "messages": [                            "instructions": "你是一个简洁的助手。",
    {                                      "input": "用一句话解释什么是 HTTP。"
      "role": "system",                  }
      "content": "你是一个简洁的助手。"
    },
    {
      "role": "user",
      "content": "用一句话解释什么是 HTTP。"
    }
  ]
}
```

Chat Completions 是扁平的 messages 数组，所有角色平铺。Responses 把系统提示词提成 `instructions` 字段，输入是字符串。

**响应体**

```jsonc
// Chat Completions                      // Responses
{                                        {
  "id": "chatcmpl-...",                    "id": "resp_abc123",
  "object": "chat.completion",             "object": "response",
  "choices": [                             "status": "completed",
    {                                      "output": [
      "index": 0,                            {
      "message": {                             "type": "message",
        "role": "assistant",                   "content": [
        "content": "HTTP 是..."                  {"type": "output_text", "text": "HTTP 是..."}
      },                                       ]
      "finish_reason": "stop"                  }
    }                                      ],
  ],                                       "usage": {
  "usage": {                                 "input_tokens": 20,
    "prompt_tokens": 20,                     "output_tokens": 15,
    "completion_tokens": 15,                 "total_tokens": 35
    "total_tokens": 35                     }
  }                                      }
}
```

| | Chat Completions | Responses |
|---|---|---|
| 文本路径 | `choices[0].message.content` | `output[0].content[0].text` |
| 嵌套深度 | 3 层 | 4 层，`output` 里可混放多种 item |
| id 前缀 | `chatcmpl-` | `resp_` |
| token 字段名 | `prompt_tokens` / `completion_tokens` | `input_tokens` / `output_tokens` |
| 结束标志 | `finish_reason` | `finish_reason` + `status` |

`OutputText()` 内部做的是遍历 `output` 数组、拼接文本片段。

### 1. 请求形态

```go
// Chat Completions
openai.ChatCompletionNewParams{
    Model: "deepseek-flash",
    Messages: []openai.ChatCompletionMessageParamUnion{
        openai.SystemMessage("你是一个简洁的助手。"),
        openai.UserMessage("用一句话解释什么是 HTTP。"),
    },
}

// Responses
responses.ResponseNewParams{
    Model:        "deepseek-flash",
    Instructions: openai.String("你是一个简洁的助手。"),
    Input: responses.ResponseNewParamsInputUnion{
        OfString: openai.String("用一句话解释什么是 HTTP。"),
    },
}
```

`Messages` 是扁平数组，所有角色都在里面。`Instructions` 和 `Input` 是分开的字段。

### 2. 读取响应

```go
// Chat Completions：3 层
chatResp.Choices[0].Message.Content

// Responses：4 层
resp.Output[0].AsMessage().Content[0].AsOutputText().Text

// OutputText() 是上面这条路径的封装：
// 遍历 output 数组，取出所有 output_text 片段并拼接。
resp.OutputText()
```

`output` 数组里可以放多种类型的 item，不只是文本。02 节把这些层级逐层打印出来：

```
output[0].type = message
  id     = msg_...
  role   = assistant
  status = completed
  content[0].type = output_text
  content[0].text = HTTP 是...
```

04 节的流式版本会打印构建过程对应的事件：

```
[事件] response.created
[事件] output_item.added   output_index=0 item.type=message
[事件] content_part.added  part.type=output_text
```

### 3. 多轮对话

Responses 有两种接续方式：

| | Chat Completions | Responses（有状态） | Responses（无状态） |
|---|---|---|---|
| 历史位置 | 进程内存 | 服务端 | 进程内存 |
| 每轮发送 | 整段历史 + 新问题 | 只发新问题 | 整段 item 列表 + 新问题 |
| 接续方式 | 重发 | `previous_response_id` | 重发 |
| 第 n 轮传输量 | O(n)，总计 O(n²) | O(1) | O(n)，总计 O(n²) |
| 改写历史 | 可以 | 不可以 | 可以 |
| 依赖厂商 | 换 base URL | 需厂商支持该参数 | 换 base URL |

05 用第一列和第三列，06 用第二列和第三列。

### 4. 流式

```go
// Chat Completions：流里只有内容增量
for stream.Next() {
    fmt.Print(stream.Current().Choices[0].Delta.Content)
}

// Responses：流是事件序列
for stream.Next() {
    event := stream.Current()
    switch event.Type {
    case "response.output_text.delta":  // 文本增量
    case "response.created":            // 响应开始
    case "response.output_item.added":  // 新增输出项
    case "response.completed":          // 结束，event.Response 是完整对象
    }
}
```

Chat Completions 的流只有文本增量。Responses 的流包含 `response.output_item.added` 一类事件，可以在流中判断模型是否开始调用工具。

03 和 04 两节的示例在每次收到增量后暂停 100ms（常量 `chunkDelay`），让渐进输出的过程可见。该延迟是本示例加的，不属于 API 行为。

两节在流结束后会打印累积的完整文本，可以和上面的渐进输出对照。

`LLM_DEBUG=1` 时还能看到服务端推送的原始数据流。这是两种流式差别最直接的体现：Chat Completions 的每行数据是文本碎片，Responses 的每行数据是带类型的事件。

### 5. 状态与隐私

在 OpenAI 上，Responses 默认把响应存在服务端（`Store` 默认为 true）：

```go
Store: openai.Bool(false)   // 关闭后不能用 previous_response_id
```

DeepSeek 的响应里 `store` 恒为 `false`，该参数不被支持。

## 两种 API 的约束

| | Chat Completions | Responses |
|---|---|---|
| 厂商覆盖 | 本教程默认端点及多数厂商提供 | 端点需实现 `/responses` |
| 服务端状态 | 无 | OpenAI 支持；DeepSeek 忽略 |
| 多轮传输量 | O(n²) | 有状态 O(1)；无状态 O(n²) |
| 内建工具 | 无 | OpenAI 提供 web search / file search / code interpreter |

## 环境变量

| 变量 | 说明 | 默认 |
|------|------|------|
| `LLM_API_KEY` | 密钥（也读 `DEEPSEEK_API_KEY` / `OPENAI_API_KEY`） | 无，必填 |
| `LLM_BASE_URL` | 端点 | `https://api.deepseek.com` |
| `LLM_MODEL` | 模型名 | `deepseek-flash` |
| `LLM_DEBUG` | 非空则打印 HTTP JSON | 关 |

## 代码结构

```
llmcall/
├── internal/llm/client.go   # client 构造 + wireDump 中间件
├── 01-chat-basic/           # 每节一个独立 main 包
├── 02-responses-basic/
├── 03-chat-stream/
├── 04-responses-stream/
├── 05-chat-multiturn/
└── 06-responses-multiturn/
```
