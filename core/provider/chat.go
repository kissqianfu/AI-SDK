// chat.go：对话能力（含视觉）的登记与构造。

package provider

import (
	"log/slog"
	"net/http"

	"github.com/kissqianfu/AI-SDK/common/protocol"
	"github.com/kissqianfu/AI-SDK/common/types"
)

// chatRegistry 是对话能力的查表，默认协议 deepseek。
var chatRegistry = newRegistry[*ChatParams, types.ChatModel](capChat, protocol.DeepSeek)

// ChatParams 是构造对话模型所需的全部参数；端点由 SDK 内部按 protocol 决定。
type ChatParams struct {
	endpointBase
	APIKey string
	Model  string
	HTTP   *http.Client
	Logger *slog.Logger
}

// RegisterChat 由厂商模块在自己的 register.go 里调用，声明「这家能对话」。
func RegisterChat(proto, defaultBaseURL string, build func(ChatParams) (types.ChatModel, error)) {
	chatRegistry.register(proto, defaultBaseURL, func(p *ChatParams) (types.ChatModel, error) {
		return build(*p)
	})
}

// NewChat 按 protocol 构造对话模型（模型由使用方指定，SDK 不代选）。
func NewChat(p ChatParams) (types.ChatModel, error) {
	if err := requireModel("对话", p.Model); err != nil {
		return nil, err
	}
	return chatRegistry.build(&p)
}
