// 01 Chat Completions：最小调用
//
// 运行： go run ./01-chat-basic
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/elio/agent-tech/llmcall/internal/llm"
	"github.com/openai/openai-go/v3"
)

func main() {
	client := llm.NewClient()

	resp, err := client.Chat.Completions.New(context.Background(), openai.ChatCompletionNewParams{
		Model: llm.Model(),

		// LLM 不保存对话状态。每次请求都要把完整的消息列表传进来。
		// 每条消息带 role：system、user、assistant。
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage("你是一个简洁的助手，回答不超过两句话。"),
			openai.UserMessage("用一句话解释什么是 HTTP。"),
		},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "调用失败:", err)
		os.Exit(1)
	}

	// 返回值是 choices 数组，常规用法取第 0 条。
	fmt.Println(resp.Choices[0].Message.Content)

	// token 用量：计费依据，也用于判断上下文占用。
	fmt.Printf("\n--- usage ---\n")
	fmt.Printf("输入 %d tokens / 输出 %d tokens / 合计 %d\n",
		resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.TotalTokens)

	// LLM_DEBUG=1 时可以看到这段请求对应的 JSON：
	//   {"model":"deepseek-flash","messages":[{"role":"system",...},{"role":"user",...}]}
}
