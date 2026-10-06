// 07 Chat Completions：读取结构化输出
//
// 任务：从一句话里抽出姓名和宠物名。
//
// 本节和 08 节做同一件事，区别在于拿到响应之后如何判断
// 返回的是什么，以及如何取出数据。
//
// 关于 response_format：DeepSeek 的 Chat Completions 只支持 json_object，
// 不支持 json_schema。带 json_schema 请求会返回 400：
//
//	This response_format type is unavailable now
//
// 所以这里只能约定「输出 JSON」，无法约束字段结构。
//
// 运行： go run ./07-chat-parse
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/elio/agent-tech/llmcall/internal/llm"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/shared"
)

func main() {
	client := llm.NewClient()

	resp, err := client.Chat.Completions.New(context.Background(), openai.ChatCompletionNewParams{
		Model: llm.Model(),
		Messages: []openai.ChatCompletionMessageParamUnion{
			// json_object 模式要求 prompt 里出现 "json" 字样并给出格式样例，
			// 否则请求会被拒绝。
			openai.SystemMessage(`从用户的话里抽取信息，用 json 返回，格式：
{"name": "...", "pet": "..."}`),
			openai.UserMessage("我叫 Elio，我养的狗叫旺财。"),
		},

		// 只约定「输出 JSON」，不指定字段结构。
		ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONObject: &shared.ResponseFormatJSONObjectParam{},
		},

		// 防止 JSON 字符串被中途截断。
		MaxTokens: openai.Int(512),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "调用失败:", err)
		os.Exit(1)
	}

	choice := resp.Choices[0]
	msg := choice.Message

	// 判断这次拿到了什么，需要逐个检查 message 上的固定字段。
	//
	// 不同种类的结果摊在这些字段里，各自独立：
	//   content    模型输出的文本
	//   refusal    模型拒答时的说明
	//   tool_calls 工具调用
	//
	// 协议没有标注本次是哪一种，只能看哪个非空。
	fmt.Println("--- message 上的字段 ---")
	fmt.Printf("finish_reason = %s\n", choice.FinishReason)
	fmt.Printf("content       = %d 字符\n", len(msg.Content))
	fmt.Printf("refusal       = %d 字符\n", len(msg.Refusal))
	fmt.Printf("tool_calls    = %d 个\n", len(msg.ToolCalls))

	// DeepSeek 文档列出的已知问题：json_object 模式有概率返回空 content。
	if msg.Content == "" {
		fmt.Fprintln(os.Stderr, "\ncontent 为空。DeepSeek 文档说明 json_object 模式存在该问题，可调整 prompt 缓解。")
		os.Exit(1)
	}

	// content 是不透明字符串。json_object 只是输出格式的约定，
	// 拿到手仍是一段文本，要自己解析成结构体。
	fmt.Printf("\n--- content 原文 ---\n%s\n", msg.Content)

	var person struct {
		Name string `json:"name"`
		Pet  string `json:"pet"`
	}
	if err := json.Unmarshal([]byte(msg.Content), &person); err != nil {
		fmt.Fprintln(os.Stderr, "\n解析 content 失败:", err)
		os.Exit(1)
	}

	// 没有 schema 约束，字段是否齐全要自己检查。
	if person.Name == "" || person.Pet == "" {
		fmt.Fprintf(os.Stderr, "\n返回的 JSON 缺少字段: %s\n", msg.Content)
		os.Exit(1)
	}

	fmt.Printf("\n--- 解析后 ---\nname = %s\npet  = %s\n", person.Name, person.Pet)
}
