// Package ttscompat 是 ElevenLabs 兼容 TTS 协议的共享实现，各厂商模块只登记默认端点。
package ttscompat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/kissqianfu/AI-SDK/common/apierr"
	"github.com/kissqianfu/AI-SDK/internal/httpclient"
)

// maxAudioBytes 是单次音频读取上限。
const maxAudioBytes = 20 << 20

// Config 是客户端配置。
type Config struct {
	// Name 是错误信息前缀；留空用 "ttscompat"。
	Name string
	// APIKey 是明文密钥（xi-api-key）。
	APIKey string
	// BaseURL 是协议根，必填——默认端点归厂商模块登记。
	BaseURL string
	// Model 是 model_id。
	Model string
	// HTTP 是注入的客户端；nil 时用默认（60s 超时）。
	HTTP *http.Client
	// OutputFormat 是请求的输出格式；留空 = "mp3"。
	OutputFormat string
}

// Client 是 ElevenLabs 兼容 TTS 客户端。
type Client struct {
	name    string
	apiKey  string
	baseURL string
	model   string
	http    *http.Client
	format  string
}

// New 构造客户端：APIKey/BaseURL/Model 三段构造期校验。
func New(cfg Config) (*Client, error) {
	name := cfg.Name
	if name == "" {
		name = "ttscompat"
	}
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("%s: APIKey 不能为空", name)
	}
	if strings.TrimSpace(cfg.BaseURL) == "" {
		return nil, fmt.Errorf("%s: BaseURL 不能为空（默认端点由厂商模块登记）", name)
	}
	if strings.TrimSpace(cfg.Model) == "" {
		return nil, fmt.Errorf("%s: Model 不能为空", name)
	}
	httpClient := cfg.HTTP
	if httpClient == nil {
		httpClient = httpclient.New(60 * time.Second)
	}
	format := strings.TrimSpace(cfg.OutputFormat)
	if format == "" {
		format = "mp3"
	}
	return &Client{
		name:    name,
		apiKey:  cfg.APIKey,
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		model:   cfg.Model,
		http:    httpClient,
		format:  format,
	}, nil
}

// Speech 合成语音，返回原始音频字节（默认 mp3；OutputFormat 可换其他格式）。
func (c *Client) Speech(ctx context.Context, voiceID, text string) ([]byte, error) {
	if strings.TrimSpace(voiceID) == "" {
		return nil, fmt.Errorf("%s: voice_id 不能为空", c.name)
	}
	body, err := json.Marshal(map[string]any{
		"text":     text,
		"model_id": c.model,
	})
	if err != nil {
		return nil, fmt.Errorf("%s: 编码请求: %w", c.name, err)
	}
	endpoint := c.baseURL + "/text-to-speech/" + url.PathEscape(voiceID) +
		"?output_format=" + url.QueryEscape(c.format)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("%s: 构造请求: %w", c.name, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("xi-api-key", c.apiKey)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s: 请求失败: %w", c.name, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		// Vendor 已在 APIError 里，直接返回（信封解析在 apierr.FromElevenLabs）。
		return nil, apierr.FromElevenLabs(c.name, resp.StatusCode, raw)
	}
	audio, err := io.ReadAll(io.LimitReader(resp.Body, maxAudioBytes))
	if err != nil {
		return nil, fmt.Errorf("%s: 读取音频: %w", c.name, err)
	}
	if len(audio) == 0 {
		return nil, fmt.Errorf("%s: 返回空音频", c.name)
	}
	return audio, nil
}
