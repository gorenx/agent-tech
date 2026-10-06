// 05 Chat Completions：多轮对话
//
// 运行： go run ./05-chat-multiturn
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/elio/agent-tech/llmcall/internal/llm"
	"github.com/openai/openai-go/v3"
)

const systemPrompt = "你是一个简洁的助手，回答不超过一句话。"

func main() {
	client := llm.NewClient()
	ctx := context.Background()

	// 完整对话历史由调用方维护，每轮请求都发送整个切片。
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(systemPrompt),
	}

	// 后两轮的问题依赖第一轮给出的信息。
	// 历史不完整时模型没有这些信息，无法正确回答。
	questions := []string{
		"我叫 Elio，我养的狗叫旺财。",
		"我的狗叫什么名字？",
		"我叫什么名字？",
	}

	for i, q := range questions {
		messages = append(messages, openai.UserMessage(q))

		resp, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
			Model:    llm.Model(),
			Messages: messages,
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, "调用失败:", err)
			os.Exit(1)
		}

		reply := resp.Choices[0].Message
		fmt.Printf("第 %d 轮\n", i+1)
		fmt.Printf("  问: %s\n", q)
		fmt.Printf("  答: %s\n", reply.Content)
		fmt.Printf("  → 本次发送 %d 条消息，消耗 %d 输入 tokens\n\n",
			len(messages), resp.Usage.PromptTokens)

		// 把模型的回复追加进历史，供下一轮使用。
		messages = append(messages, reply.ToParam())
	}

	// 第 n 轮发送 n 条消息，输入 token 数逐轮上涨。
	// n 轮对话的累计传输量是 O(n²)。
	fmt.Printf("最终历史共 %d 条消息\n", len(messages))
}
