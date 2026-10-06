// 03 Chat Completions：流式输出
//
// 与非流式的区别是返回时机：非流式等生成完毕后一次性返回，
// 流式在生成过程中持续推送。
//
// 运行： go run ./03-chat-stream
package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/elio/agent-tech/llmcall/internal/llm"
	"github.com/openai/openai-go/v3"
)

// 每次收到增量后暂停，让渐进输出的过程可见。
// 仅为便于观察，不属于 API 行为；真实场景不需要这个延迟。
const chunkDelay = 100 * time.Millisecond

func main() {
	client := llm.NewClient()

	// 与非流式唯一的区别：New → NewStreaming。
	stream := client.Chat.Completions.NewStreaming(context.Background(), openai.ChatCompletionNewParams{
		Model: llm.Model(),
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage("你是一个简洁的助手。"),
			openai.UserMessage("数到十，用逗号分隔。"),
		},
	})
	defer stream.Close()

	// 流里给的是增量，要拿到完整文本需要自己累积。
	var full strings.Builder

	for stream.Next() {
		chunk := stream.Current()

		// Chunk 里是 Delta（增量），不是完整 Message。
		// 首个 chunk 通常只带 role、不带 content，所以要判空。
		if len(chunk.Choices) > 0 {
			delta := chunk.Choices[0].Delta.Content
			fmt.Print(delta)
			full.WriteString(delta)
			time.Sleep(chunkDelay)
		}
	}

	// Next() 返回 false 有两种原因：正常结束或连接中断。
	// 必须检查 Err() 才能区分。
	if err := stream.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "\n流中断:", err)
		os.Exit(1)
	}
	fmt.Println()

	// 把累积的结果打出来，和上面的渐进输出对照。
	fmt.Printf("\n--- 完整结果 ---\n%s\n", full.String())
}
