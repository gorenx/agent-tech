// 02 Responses API：最小调用
//
// 运行： go run ./02-responses-basic
//
// DeepSeek 支持 Responses API，但它的实现不含服务端状态：
// previous_response_id / store / conversation 不被支持。
// 该差异在 06-responses-multiturn 展开。
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/elio/agent-tech/llmcall/internal/llm"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/responses"
)

func main() {
	client := llm.NewClient()

	resp, err := client.Responses.New(context.Background(), responses.ResponseNewParams{
		Model: llm.Model(),

		// 差异 1：系统提示词是独立的 instructions 字段，
		// 不是一条 role=system 的消息。
		Instructions: openai.String("你是一个简洁的助手，回答不超过两句话。"),

		// 差异 2：没有 messages 数组，只有 input。
		// input 可以是字符串，也可以是结构化的 item 列表。
		Input: responses.ResponseNewParamsInputUnion{
			OfString: openai.String("用一句话解释什么是 HTTP。"),
		},
	})
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			fmt.Fprintln(os.Stderr, "这个端点没有 /responses 接口。")
			fmt.Fprintln(os.Stderr, "请确认 LLM_BASE_URL 指向的厂商实现了 Responses API。")
		}
		fmt.Fprintln(os.Stderr, "调用失败:", err)
		os.Exit(1)
	}

	// 差异 3：OutputText() 遍历 output 数组，拼接其中的文本片段。
	// Responses 的 output 可以包含推理、工具调用等多种 item。
	fmt.Println(resp.OutputText())

	// 差异 4：每个响应有服务端 ID。
	fmt.Printf("\n--- 响应元信息 ---\n")
	fmt.Printf("response id: %s\n", resp.ID)
	fmt.Printf("status:      %s\n", resp.Status)
	fmt.Printf("model:       %s\n", resp.Model)
	fmt.Printf("输入 %d tokens / 输出 %d tokens\n",
		resp.Usage.InputTokens, resp.Usage.OutputTokens)
}
