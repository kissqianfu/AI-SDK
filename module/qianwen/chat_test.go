// 千问子模块的行为测试：思考开关显式关闭、base_url 归一化、缓存 token 标准位。
package qianwen

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kissqianfu/AI-SDK/common/types"
	"github.com/kissqianfu/AI-SDK/core/compat"
)

func newTestModel(t *testing.T, handler http.HandlerFunc) *compat.Model {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	m, err := New(Config{APIKey: "sk-test", BaseURL: srv.URL, Model: "qwen-plus"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return m
}

func userReq() types.ChatRequest {
	return types.ChatRequest{Messages: []types.ChatMessage{{Role: types.ChatRoleUser, Content: "hi"}}}
}

// 不要求思考时必须显式发 false：qwen3 默认开启思考，漏发不报错但白烧 token。
func TestChatDisablesThinkingExplicitly(t *testing.T) {
	var gotBody map[string]any
	m := newTestModel(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"},"finish_reason":"stop"}]}`))
	})
	if _, err := m.Chat(context.Background(), userReq()); err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	v, has := gotBody["enable_thinking"]
	if !has {
		t.Fatal("请求里必须有 enable_thinking=false（qwen3 默认开启思考，漏发等于一直开着）")
	}
	if v != false {
		t.Fatalf("enable_thinking = %v, want false", v)
	}
}

func TestChatThinkingEnabled(t *testing.T) {
	var gotBody map[string]any
	m := newTestModel(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"},"finish_reason":"stop"}]}`))
	})
	req := userReq()
	temp := 0.7
	req.Temperature = &temp
	req.Thinking = true
	if _, err := m.Chat(context.Background(), req); err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if gotBody["enable_thinking"] != true {
		t.Fatalf("enable_thinking = %v, want true", gotBody["enable_thinking"])
	}
	// 思考模式下温度不可控，必须丢弃。
	if _, has := gotBody["temperature"]; has {
		t.Fatalf("思考模式不应携带 temperature: %v", gotBody["temperature"])
	}
	// 千问没有 DeepSeek 的 thinking 字段，不能混发。
	if _, has := gotBody["thinking"]; has {
		t.Fatalf("千问请求不应带 thinking 字段: %v", gotBody["thinking"])
	}
}

// 缓存命中走标准位 prompt_tokens_details.cached_tokens，不是 DeepSeek 的字段。
func TestChatReadsCachedTokens(t *testing.T) {
	m := newTestModel(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"},"finish_reason":"stop"}],
			"usage":{"prompt_tokens":1200,"completion_tokens":30,"total_tokens":1230,
			"prompt_tokens_details":{"cached_tokens":900}}}`))
	})
	resp, err := m.Chat(context.Background(), userReq())
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if resp.Usage.CachedTokens != 900 {
		t.Fatalf("CachedTokens = %d, want 900", resp.Usage.CachedTokens)
	}
}

// base_url 归一化：只写域名会打根路径 404。
func TestNormalizeBaseURL(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", DefaultBaseURL},
		{"  ", DefaultBaseURL},
		{"https://dashscope.aliyuncs.com", "https://dashscope.aliyuncs.com/compatible-mode/v1"},
		{"https://dashscope.aliyuncs.com/", "https://dashscope.aliyuncs.com/compatible-mode/v1"},
		// 已带完整兼容模式路径：原样保留（含结尾斜杠清理）。
		{"https://dashscope.aliyuncs.com/compatible-mode/v1", "https://dashscope.aliyuncs.com/compatible-mode/v1"},
		{"https://dashscope.aliyuncs.com/compatible-mode/v1/", "https://dashscope.aliyuncs.com/compatible-mode/v1"},
		// 业务空间专属域名：有 path，不动。
		{"https://ws123.cn-beijing.maas.aliyuncs.com/compatible-mode/v1", "https://ws123.cn-beijing.maas.aliyuncs.com/compatible-mode/v1"},
		// 自建代理：用户写了路径就按他的来。
		{"https://proxy.example.com/qwen/v1", "https://proxy.example.com/qwen/v1"},
	}
	for _, c := range cases {
		if got := NormalizeBaseURL(c.in); got != c.want {
			t.Fatalf("NormalizeBaseURL(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// 端点路径必须是 /compatible-mode/v1/chat/completions。
func TestChatPath(t *testing.T) {
	var gotPath string
	m := newTestModel(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"},"finish_reason":"stop"}]}`))
	})
	if _, err := m.Chat(context.Background(), userReq()); err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if want := "/compatible-mode/v1/chat/completions"; gotPath != want {
		t.Fatalf("path = %q, want %q", gotPath, want)
	}
}

func TestNewValidation(t *testing.T) {
	if _, err := New(Config{}); err == nil {
		t.Fatal("New(empty) error = nil, want error")
	}
}
