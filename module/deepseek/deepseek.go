// Package deepseek 是 DeepSeek 官方接口子模块：Chat Completions 与 Search/Fetch（web.go），
// 与标准 OpenAI 的差异见 Dialect（thinking 参数、端点、缓存 token 取法）。
package deepseek

import (
	"log/slog"
	"net/http"

	"github.com/kissqianfu/AI-SDK/core/compat"
)

const (
	// DefaultBaseURL 是 DeepSeek API 地址。
	DefaultBaseURL = "https://api.deepseek.com"
)

// Config 是适配器参数（key 由使用方注入）。
type Config struct {
	APIKey  string
	BaseURL string
	Model   string
	HTTP    *http.Client
	Logger  *slog.Logger
}

// baseURL 兜底默认端点；三条链路（对话/搜索/抓取）共用同一份真相。
func (c Config) baseURL() string {
	if c.BaseURL != "" {
		return c.BaseURL
	}
	return DefaultBaseURL
}

// New 构造适配器：校验由 compat.New 统一做，这里只补默认端点。
func New(cfg Config) (*compat.Model, error) {
	cfg.BaseURL = cfg.baseURL()
	return compat.New(compat.Config{
		Name:    "deepseek",
		APIKey:  cfg.APIKey,
		BaseURL: cfg.BaseURL,
		Model:   cfg.Model,
		HTTP:    cfg.HTTP,
		Logger:  cfg.Logger,
		Dialect: Dialect{},
	})
}

// Dialect 是 DeepSeek 的 wire 差异。
type Dialect struct{}

// Name 是错误与日志前缀。
func (Dialect) Name() string { return "deepseek" }

// EncodeThinking 只在「要思考」时写 thinking:{type:"enabled"}；思考模式下温度静默失效，故丢弃。
func (Dialect) EncodeThinking(p *compat.Payload, on bool) {
	if !on {
		return
	}
	p.Thinking = &compat.ThinkingParam{Type: "enabled"}
	p.Temperature = nil
}

// CachedTokens 取 prompt_cache_hit_tokens。
func (Dialect) CachedTokens(u compat.Usage) int { return u.PromptCacheHitTokens }
