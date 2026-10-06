// 06 Responses API：多轮对话的两种接续方式
//
//	A. previous_response_id —— 历史存服务端（OpenAI 支持，DeepSeek 忽略该参数）
//	B. 回填 output items    —— 调用方自己带历史（不依赖厂商特性）
//
// 运行： go run ./06-responses-multiturn
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/elio/agent-tech/llmcall/internal/llm"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/responses"
)

const systemPrompt = "你是一个简洁的助手，回答不超过一句话。"

const (
	firstQ  = "我叫 Elio，我养的狗叫旺财。"
	secondQ = "我的狗叫什么名字？"
)

func main() {
	client := llm.NewClient()
	ctx := context.Background()

	// ─────────────────────────────────────────────────────────
	// 方式 A：previous_response_id
	// ─────────────────────────────────────────────────────────
	fmt.Println("=== 方式 A：previous_response_id ===")

	r1, err := client.Responses.New(ctx, responses.ResponseNewParams{
		Model:        llm.Model(),
		Instructions: openai.String(systemPrompt),
		Input:        responses.ResponseNewParamsInputUnion{OfString: openai.String(firstQ)},
	})
	check(err)
	fmt.Printf("第 1 轮答: %s\n", r1.OutputText())

	// 第二轮只发新问题，历史通过 ID 由服务端接续。
	// instructions 也不重发。
	r2, err := client.Responses.New(ctx, responses.ResponseNewParams{
		Model:              llm.Model(),
		Input:              responses.ResponseNewParamsInputUnion{OfString: openai.String(secondQ)},
		PreviousResponseID: openai.String(r1.ID),
	})
	check(err)
	fmt.Printf("第 2 轮答: %s\n", r2.OutputText())

	// 检测该参数是否被服务端接受。
	//
	// 请求返回 200 不代表参数生效：不支持的参数会被静默忽略。
	// 服务端接受该参数时，会在响应体里回填 previous_response_id。
	if r2.PreviousResponseID == "" {
		fmt.Println()
		fmt.Println("服务端未回填 previous_response_id，该参数被忽略，上下文未接续。")
		fmt.Println("DeepSeek 的 Responses API 不保存服务端状态。")
	}

	// ─────────────────────────────────────────────────────────
	// 方式 B：把上一轮的 output 回填进 input
	// ─────────────────────────────────────────────────────────
	fmt.Println("\n=== 方式 B：回填 output items ===")

	// input 可以是一个 item 列表，因此可以把上一轮返回的 output 放回去。
	history := responses.ResponseInputParam{
		responses.ResponseInputItemParamOfMessage(firstQ, responses.EasyInputMessageRoleUser),
	}

	b1, err := client.Responses.New(ctx, responses.ResponseNewParams{
		Model:        llm.Model(),
		Instructions: openai.String(systemPrompt),
		Input:        responses.ResponseNewParamsInputUnion{OfInputItemList: history},
	})
	check(err)
	fmt.Printf("第 1 轮答: %s\n", b1.OutputText())

	// 回填整个 output item，而非只回填文本。
	// output 数组可以包含推理、工具调用等多种 item，只回填文本会丢失这些内容。
	for _, item := range b1.Output {
		if item.Type != "message" {
			continue // 推理、工具调用等类型同样可以回填，这里只处理文本
		}
		p := item.AsMessage().ToParam()
		history = append(history, responses.ResponseInputItemParamOfOutputMessage(p.Content, p.ID, p.Status))
	}
	history = append(history, responses.ResponseInputItemParamOfMessage(secondQ, responses.EasyInputMessageRoleUser))

	b2, err := client.Responses.New(ctx, responses.ResponseNewParams{
		Model: llm.Model(),
		// 无状态方式下每轮都要重发 instructions。
		Instructions: openai.String(systemPrompt),
		Input:        responses.ResponseNewParamsInputUnion{OfInputItemList: history},
	})
	check(err)
	fmt.Printf("第 2 轮答: %s\n", b2.OutputText())

	// ─────────────────────────────────────────────────────────
	// 两种方式的区别：
	//   方式 A 每轮只发送新问题，历史在服务端，传输量 O(1)。
	//   方式 B 每轮重发全部历史，与 05 的 Chat Completions 相同，传输量 O(n²)。
	//
	//   方式 A 依赖厂商实现 previous_response_id 与 store。
	//   方式 B 不依赖这些参数。
	fmt.Printf("\n方式 B 最后一轮发送 %d 个 item\n", len(history))
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "调用失败:", err)
		os.Exit(1)
	}
}
