// chat.go：OpenAI 官方对话子模块，走标准 Chat Completions；差异是 reasoning_effort 思考开关与默认端点（带 /v1）。

package openai

import (
	"log/slog"
	"net/http"

	"github.com/kissqianfu/AI-SDK/core/compat"
)

const (
	// DefaultChatBaseURL 是 OpenAI 对话端点（带 /v1，compat 只补 /chat/completions）。
	DefaultChatBaseURL = "https://api.openai.com/v1"
)

// ChatConfig 是对话适配器参数（与媒体的 Config 分开：两者的字段含义不同）。
type ChatConfig struct {
	APIKey  string
	BaseURL string
	Model   string
	HTTP    *http.Client
	Logger  *slog.Logger
}

// NewChat 构造对话适配器：校验由 compat.New 统一做，这里只补默认端点。
func NewChat(cfg ChatConfig) (*compat.Model, error) {
	base := cfg.BaseURL
	if base == "" {
		base = DefaultChatBaseURL
	}
	return compat.New(compat.Config{
		Name:    "openai",
		APIKey:  cfg.APIKey,
		BaseURL: base,
		Model:   cfg.Model,
		HTTP:    cfg.HTTP,
		Logger:  cfg.Logger,
		Dialect: ChatDialect{},
	})
}

// ChatDialect 是 OpenAI 的 wire 差异；标准位行为（CachedTokens 等）继承 BaseDialect。
type ChatDialect struct {
	compat.BaseDialect
}

// EncodeThinking 用 reasoning_effort="high" 表达思考，只在开启时发（非推理模型收到会 400）。
func (ChatDialect) EncodeThinking(p *compat.Payload, on bool) {
	if !on {
		return
	}
	p.ReasoningEffort = "high"
	p.Temperature = nil
}
