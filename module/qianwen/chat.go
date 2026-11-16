// Package qianwen 是通义千问适配：chat 走兼容模式 Chat Completions，
// image 走原生 multimodal-generation，asr 走兼容模式 input_audio，均在 register.go 分别登记。
package qianwen

import (
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/kissqianfu/AI-SDK/core/compat"
)

const (
	// DefaultBaseURL 是千问兼容模式默认端点，其他可用域名路径相同。
	DefaultBaseURL = "https://maas.qianwenaiapi.com/compatible-mode/v1"
	// compatiblePath 是兼容模式路径（base_url 缺它必 404）。
	compatiblePath = "/compatible-mode/v1"
)

// Config 是适配器参数（key 由使用方注入）。
type Config struct {
	APIKey  string
	BaseURL string
	Model   string
	HTTP    *http.Client
	Logger  *slog.Logger
}

// New 构造适配器：校验由 compat.New 统一做，这里只归一化 base_url。
func New(cfg Config) (*compat.Model, error) {
	cfg.BaseURL = NormalizeBaseURL(cfg.BaseURL)
	return compat.New(compat.Config{
		Name:    "qianwen",
		APIKey:  cfg.APIKey,
		BaseURL: cfg.BaseURL,
		Model:   cfg.Model,
		HTTP:    cfg.HTTP,
		Logger:  cfg.Logger,
		Dialect: Dialect{},
	})
}

// NormalizeBaseURL 把 base_url 补成带 /compatible-mode/v1 的完整前缀；已带路径或留空则不动/用默认端点。
func NormalizeBaseURL(base string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		return DefaultBaseURL
	}
	base = strings.TrimRight(base, "/")
	u, err := url.Parse(base)
	if err != nil {
		return base
	}
	// 只补路径：已有 path 的（自建网关、代理前缀）原样使用。
	if u.Path != "" && u.Path != "/" {
		return base
	}
	return base + compatiblePath
}

// Dialect 是千问的 wire 差异：只覆写 Name 与 enable_thinking，其余继承 BaseDialect。
type Dialect struct {
	compat.BaseDialect
}

// Name 是错误与日志前缀。
func (Dialect) Name() string { return "qianwen" }

// EncodeThinking 显式发送 enable_thinking（qwen3 默认开思考，两个方向都必须写）。
func (Dialect) EncodeThinking(p *compat.Payload, on bool) {
	v := on
	p.EnableThinking = &v
	if on {
		// 思考模式下温度不可控，丢弃以免设了不生效。
		p.Temperature = nil
	}
}
