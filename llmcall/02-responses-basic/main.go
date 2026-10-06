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

	fmt.Printf("--- 响应元信息 ---\n")
	fmt.Printf("id     = %s\n", resp.ID)
	fmt.Printf("status = %s\n", resp.Status)
	fmt.Printf("model  = %s\n", resp.Model)

	// 差异 3：文本的读取路径。
	//
	// Chat Completions 是 choices[0].message.content，3 层。
	// Responses 是 output[0].content[0].text，4 层，
	// 而且 output 数组可以放多种类型的 item，不只是文本。
	//
	// 下面把这条路径逐层走一遍。
	fmt.Printf("\n--- 逐层读取 output ---\n")
	for i, item := range resp.Output {
		fmt.Printf("output[%d].type = %s\n", i, item.Type)

		if item.Type != "message" {
			// 还可能是 reasoning、function_call 等其他类型
			continue
		}

		msg := item.AsMessage()
		fmt.Printf("  id     = %s\n", msg.ID)
		fmt.Printf("  role   = %s\n", msg.Role)
		fmt.Printf("  status = %s\n", msg.Status)

		for j, part := range msg.Content {
			fmt.Printf("  content[%d].type = %s\n", j, part.Type)
			if part.Type == "output_text" {
				fmt.Printf("  content[%d].text = %s\n", j, part.AsOutputText().Text)
			}
		}
	}

	// OutputText() 是上面那段遍历的封装：
	// 把所有 output_text 片段取出来拼接。
	fmt.Printf("\n--- OutputText() 的结果 ---\n%s\n", resp.OutputText())

	// 差异 4：token 字段名。
	// Chat Completions 用 prompt_tokens / completion_tokens，
	// Responses 用 input_tokens / output_tokens，并带明细。
	fmt.Printf("\n--- usage ---\n")
	fmt.Printf("input_tokens  = %d (其中 cached: %d)\n",
		resp.Usage.InputTokens, resp.Usage.InputTokensDetails.CachedTokens)
	fmt.Printf("output_tokens = %d (其中 reasoning: %d)\n",
		resp.Usage.OutputTokens, resp.Usage.OutputTokensDetails.ReasoningTokens)
	fmt.Printf("total_tokens  = %d\n", resp.Usage.TotalTokens)
}
