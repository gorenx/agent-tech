// 04 Responses API：流式输出（事件序列）
//
// 运行： go run ./04-responses-stream
package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/elio/agent-tech/llmcall/internal/llm"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/responses"
)

// 每次收到增量后暂停，让渐进输出的过程可见。
// 仅为便于观察，不属于 API 行为；真实场景不需要这个延迟。
const chunkDelay = 100 * time.Millisecond

func main() {
	client := llm.NewClient()

	stream := client.Responses.NewStreaming(context.Background(), responses.ResponseNewParams{
		Model:        llm.Model(),
		Instructions: openai.String("你是一个简洁的助手。"),
		Input: responses.ResponseNewParamsInputUnion{
			OfString: openai.String("数到十，用逗号分隔。"),
		},
	})
	defer stream.Close()

	// 流里给的是增量，要拿到完整文本需要自己累积。
	var full strings.Builder
	var final *responses.Response

	for stream.Next() {
		event := stream.Current()

		// event.Type 是字符串枚举，常见取值：
		//   response.created            响应已创建
		//   response.output_item.added  开始一个新的输出项
		//   response.content_part.added 输出项里开始一个新片段
		//   response.output_text.delta  文本增量
		//   response.output_text.done   文本片段结束
		//   response.completed          整个响应结束
		//
		// Chat Completions 的流只有文本增量，没有这些边界事件。
		switch event.Type {
		case "response.output_text.delta":
			fmt.Print(event.Delta)
			full.WriteString(event.Delta)
			time.Sleep(chunkDelay)

		case "response.created":
			fmt.Fprintf(os.Stderr, "[事件] response.created\n")

		case "response.output_item.added":
			// 一个新的输出项开始构建。
			// item.type 就是 02 节 output 数组里那个 type。
			// Chat Completions 的流里没有对应事件。
			fmt.Fprintf(os.Stderr, "[事件] output_item.added   output_index=%d item.type=%s\n",
				event.OutputIndex, event.Item.Type)

		case "response.content_part.added":
			// 输出项里的一个片段开始。文本片段是 output_text。
			fmt.Fprintf(os.Stderr, "[事件] content_part.added  part.type=%s\n", event.Part.Type)

		case "response.completed":
			// 完结事件里带完整的 Response 对象，类型和 02 节非流式调用返回的相同。
			resp := event.Response
			final = &resp
			fmt.Fprintf(os.Stderr, "[事件] response.completed  id=%s 输入 %d / 输出 %d tokens\n",
				resp.ID, resp.Usage.InputTokens, resp.Usage.OutputTokens)
		}
	}

	if err := stream.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "\n流中断:", err)
		os.Exit(1)
	}
	fmt.Println()

	// 两个来源的完整文本：自己累积的增量，和完结事件里的 Response 对象。
	fmt.Printf("\n--- 累积的增量 ---\n%s\n", full.String())
	if final != nil {
		fmt.Printf("\n--- response.completed 里的 Response 对象 ---\n%s\n", final.OutputText())
	}
}
