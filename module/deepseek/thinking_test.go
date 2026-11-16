// 思考模式测试：deepseek 适配器透传 thinking 参数，默认不发。
package deepseek

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/kissqianfu/AI-SDK/common/types"
)

func TestChatThinkingParam(t *testing.T) {
	var gotBody map[string]any
	m := newTestModel(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok","reasoning_content":"推理"},"finish_reason":"stop"}]}`))
	})

	resp, err := m.Chat(context.Background(), types.ChatRequest{
		Messages: []types.ChatMessage{{Role: types.ChatRoleUser, Content: "hi"}},
		Thinking: true,
	})
	if err != nil || resp.Content != "ok" {
		t.Fatalf("Chat() = %v/%v", resp, err)
	}
	th, ok := gotBody["thinking"].(map[string]any)
	if !ok || th["type"] != "enabled" {
		t.Fatalf("thinking 参数缺失或格式错: %v", gotBody["thinking"])
	}
	// 思考模式下温度必须被丢弃（静默失效陷阱）。
	if _, has := gotBody["temperature"]; has {
		t.Fatalf("思考模式不应携带 temperature: %v", gotBody["temperature"])
	}
}

func TestChatNoThinkingByDefault(t *testing.T) {
	var gotBody map[string]any
	m := newTestModel(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"},"finish_reason":"stop"}]}`))
	})
	if _, err := m.Chat(context.Background(), types.ChatRequest{
		Messages: []types.ChatMessage{{Role: types.ChatRoleUser, Content: "hi"}},
	}); err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if _, has := gotBody["thinking"]; has {
		t.Fatalf("非思考请求不应携带 thinking: %v", gotBody["thinking"])
	}
}
