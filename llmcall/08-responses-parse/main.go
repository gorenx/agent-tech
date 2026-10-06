// 08 Responses API：读取结构化输出
//
// 任务和 07 相同，两点区别：
//
//  1. 响应本身带类型信息，按 item.type 分发；
//  2. 输出格式的约束能力不同。DeepSeek 的 Chat Completions 只支持
//     json_object（07 节），而 Responses 的 text.format 支持 json_schema，
//     可以对字段做约束。
//
// 运行： go run ./08-responses-parse
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/elio/agent-tech/llmcall/internal/llm"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/responses"
)

var schema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"name": map[string]any{"type": "string"},
		"pet":  map[string]any{"type": "string"},
	},
	"required":             []string{"name", "pet"},
	"additionalProperties": false,
}

func main() {
	client := llm.NewClient()

	resp, err := client.Responses.New(context.Background(), responses.ResponseNewParams{
		Model:        llm.Model(),
		Instructions: openai.String("从用户的话里抽取信息。"),
		Input: responses.ResponseNewParamsInputUnion{
			OfString: openai.String("我叫 Elio，我养的狗叫旺财。"),
		},

		// 约定输出格式。对应 07 节的 response_format。
		Text: responses.ResponseTextConfigParam{
			Format: responses.ResponseFormatTextConfigParamOfJSONSchema("person", schema),
		},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "调用失败:", err)
		os.Exit(1)
	}

	// 判断这次拿到了什么，先看 item 的类型。
	//
	// 所有结果都放进 output 这一个列表里，每项带 type 字段：
	//   message       模型输出的消息
	//   reasoning     推理内容
	//   function_call 工具调用
	//
	// 协议直接标出了每项是什么，不需要去猜哪个字段非空。
	fmt.Println("--- output 各项的类型 ---")
	var text string
	for i, item := range resp.Output {
		fmt.Printf("output[%d].type = %s\n", i, item.Type)

		switch item.Type {
		case "message":
			// 到这一层只知道是消息。消息内部的片段同样带类型。
			for j, part := range item.AsMessage().Content {
				fmt.Printf("  content[%d].type = %s\n", j, part.Type)

				switch part.Type {
				case "output_text":
					text = part.AsOutputText().Text
				case "refusal":
					fmt.Fprintf(os.Stderr, "模型拒答: %s\n", part.Refusal)
					os.Exit(1)
				}
			}

		case "reasoning":
			fmt.Println("  （推理内容，不是最终答案）")

		case "function_call":
			fmt.Println("  （工具调用，不是最终答案）")
		}
	}

	// 文本仍然是一段 JSON 字符串。schema 约束的是模型的生成过程，
	// 不改变它在协议里的形态。这一点两种 API 相同。
	fmt.Printf("\n--- text 原文 ---\n%s\n", text)

	var person struct {
		Name string `json:"name"`
		Pet  string `json:"pet"`
	}
	if err := json.Unmarshal([]byte(text), &person); err != nil {
		fmt.Fprintln(os.Stderr, "\n解析 text 失败:", err)
		os.Exit(1)
	}
	fmt.Printf("\n--- 解析后 ---\nname = %s\npet  = %s\n", person.Name, person.Pet)
}
