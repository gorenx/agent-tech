// Package llm 提供各示例共用的 client 构造逻辑。
package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

const (
	// DeepSeek 的 OpenAI 兼容端点。
	// 它的 /chat/completions 与 OpenAI 协议一致，因此可以直接用 openai-go SDK，
	// 只需替换 base URL。
	defaultBaseURL = "https://api.deepseek.com"
	defaultModel   = "deepseek-flash"
)

// BaseURL 返回本次运行使用的端点。
func BaseURL() string {
	if v := os.Getenv("LLM_BASE_URL"); v != "" {
		return v
	}
	return defaultBaseURL
}

// Model 返回本次运行使用的模型名。
func Model() string {
	if v := os.Getenv("LLM_MODEL"); v != "" {
		return v
	}
	return defaultModel
}

// APIKey 按优先级从环境变量读取密钥。
func APIKey() string {
	for _, name := range []string{"LLM_API_KEY", "DEEPSEEK_API_KEY", "OPENAI_API_KEY"} {
		if v := os.Getenv(name); v != "" {
			return v
		}
	}
	return ""
}

// NewClient 构造指向 BaseURL() 的 client。
//
// 设置 LLM_DEBUG=1 会把收发的 JSON 打到 stderr，
// 用于观察 SDK 生成的请求体。
func NewClient() openai.Client {
	key := APIKey()
	if key == "" {
		fmt.Fprintln(os.Stderr, "缺少 API key。请先: export LLM_API_KEY=sk-xxx")
		fmt.Fprintln(os.Stderr, "（也可用 DEEPSEEK_API_KEY / OPENAI_API_KEY）")
		os.Exit(1)
	}

	opts := []option.RequestOption{
		option.WithAPIKey(key),
		option.WithBaseURL(BaseURL()),
	}
	if os.Getenv("LLM_DEBUG") != "" {
		opts = append(opts, option.WithHTTPClient(&http.Client{
			Transport: &wireDump{base: http.DefaultTransport},
		}))
	}
	return openai.NewClient(opts...)
}

// wireDump 是一个 http.RoundTripper，把请求体和响应体打到 stderr。
type wireDump struct{ base http.RoundTripper }

func (t *wireDump) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Body != nil {
		body, err := io.ReadAll(req.Body)
		req.Body.Close()
		if err == nil {
			req.Body = io.NopCloser(bytes.NewReader(body))
			fmt.Fprintf(os.Stderr, "\n>>> %s %s\n%s\n", req.Method, req.URL.Redacted(), pretty(body))
		}
	}

	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	// SSE 是逐块到达的长连接，不能缓冲等待，否则流式就变成一次性返回了。
	// 这里把 body 包一层，SDK 读到多少就打多少，保持原有的时序。
	if strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		fmt.Fprintf(os.Stderr, "<<< %d (text/event-stream)\n", resp.StatusCode)
		resp.Body = &sseEcho{rc: resp.Body}
		return resp, nil
	}

	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}
	resp.Body = io.NopCloser(bytes.NewReader(body))
	fmt.Fprintf(os.Stderr, "<<< %d\n%s\n", resp.StatusCode, pretty(body))
	return resp, nil
}

// sseEcho 把流式响应读到的内容按行打到 stderr。
//
// SDK 每从连接读一段，这里就收到一段。因为不是一次性读完，
// 打出来的顺序和到达顺序一致，能看出服务端是分多次推送的。
type sseEcho struct {
	rc  io.ReadCloser
	buf []byte // 尚未凑满一行的残留
}

func (s *sseEcho) Read(p []byte) (int, error) {
	n, err := s.rc.Read(p)
	if n > 0 {
		s.buf = append(s.buf, p[:n]...)
		for {
			i := bytes.IndexByte(s.buf, '\n')
			if i < 0 {
				break
			}
			fmt.Fprintf(os.Stderr, "  %s\n", bytes.TrimRight(s.buf[:i], "\r"))
			s.buf = s.buf[i+1:]
		}
	}
	return n, err
}

func (s *sseEcho) Close() error { return s.rc.Close() }

func pretty(b []byte) string {
	var buf bytes.Buffer
	if err := json.Indent(&buf, b, "", "  "); err != nil {
		return string(b)
	}
	return buf.String()
}
