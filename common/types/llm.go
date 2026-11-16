// Package types 是厂商无关的模型契约：对话与向量化接口及消息结构，本包是依赖叶子。
package types

import (
	"context"
	"encoding/json"
)

// ChatRole 是对话消息角色。
type ChatRole string

const (
	// ChatRoleSystem 是 system 消息。
	ChatRoleSystem ChatRole = "system"
	// ChatRoleUser 是用户消息（图片只能出现在这里）。
	ChatRoleUser ChatRole = "user"
	// ChatRoleAssistant 是模型消息。
	ChatRoleAssistant ChatRole = "assistant"
	// ChatRoleTool 是工具结果消息。
	ChatRoleTool ChatRole = "tool"
)

// ChatMessage 是一条对话消息。
type ChatMessage struct {
	Role       ChatRole
	Content    string
	Name       string
	ToolCalls  []ToolCall
	ToolCallID string
	Images     []ImageRef
}

// ToolCall 是模型请求的一次工具调用。
type ToolCall struct {
	ID        string
	Name      string
	Arguments json.RawMessage
}

// TokenUsage 是一次调用的 token 用量（缓存命中的部分单独计）。
type TokenUsage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	CachedTokens     int
}

// ChatRequest 是一次模型调用请求（厂商无关；model 等在构造时指定）。
type ChatRequest struct {
	Messages    []ChatMessage
	Tools       []ToolSpec
	Temperature *float64 // nil = 用服务端默认值
	MaxTokens   int
	Thinking    bool // 思考模式是参数不是模型：各厂商由 Dialect 映射参数名
	JSONMode    bool
	Stop        []string
}

// ChatResponse 是一次模型调用的结果。
type ChatResponse struct {
	Content      string
	ToolCalls    []ToolCall
	Usage        TokenUsage
	FinishReason string
	Reasoning    string // 思考内容仅用于日志/排错，不回传给模型
}

// Delta 是流式输出的一个增量；流结束时 Recv 返回 io.EOF。
type Delta struct {
	Content      string
	ToolCalls    []ToolCall
	Usage        *TokenUsage
	FinishReason string
}

// ChatStream 是流式对话句柄；不是并发安全的，打断请用 ctx 取消而不是跨 goroutine 调 Close。
type ChatStream interface {
	Recv() (Delta, error) // 流结束返回 io.EOF
	Close() error
}

// ChatModel 是对话模型适配器契约（协议适配器实现，不是厂商 SDK）。
type ChatModel interface {
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
	Stream(ctx context.Context, req ChatRequest) (ChatStream, error)
}

// ImageRef 指向一张图片；当前多模态请求只用 URL，其余字段留给使用方自用。
type ImageRef struct {
	URL    string
	Path   string
	MIME   string
	Width  int
	Height int
}

// Embedder 是向量化契约（记忆向量回填与召回用）。
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}
