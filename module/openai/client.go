// Package openai 是 OpenAI 官方接口的适配：对话（chat.go）与媒体接口（asr.go/image.go）。
package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/kissqianfu/AI-SDK/internal/httpclient"
	"github.com/kissqianfu/AI-SDK/internal/text"
)

// Config 是媒体接口的配置（对话的配置在 chat.go 的 ChatConfig：两者字段含义不同）。
type Config struct {
	BaseURL string
	APIKey  func(ctx context.Context) (string, error)
	HTTP    *http.Client
}

// Client 是 OpenAI 兼容的媒体客户端。
type Client struct {
	baseURL string
	apiKey  func(ctx context.Context) (string, error)
	http    *http.Client
}

// New 构造媒体客户端（baseURL 为空报错，端点由 SDK 内部登记）。
func New(cfg Config) (*Client, error) {
	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if base == "" {
		return nil, errors.New("openai: 端点为空")
	}
	client := cfg.HTTP
	if client == nil {
		client = httpclient.New(60 * time.Second)
	}
	return &Client{baseURL: base, apiKey: cfg.APIKey, http: client}, nil
}

// do 发 JSON 请求并返回原始响应。
func (c *Client) do(ctx context.Context, path string, body any) ([]byte, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("openai: encode request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("openai: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	data, status, _, err := c.send(req, 4<<20)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("openai: HTTP %d: %s", status, text.TruncateHead(data, 200))
	}
	return data, nil
}

// send 带认证发请求，返回响应体、状态码与响应头（非 200 也返回体，由各接口自己报错）。
func (c *Client) send(req *http.Request, limit int64) (body []byte, status int, header http.Header, err error) {
	if c.apiKey != nil {
		key, keyErr := c.apiKey(req.Context())
		if keyErr != nil {
			return nil, 0, nil, fmt.Errorf("openai: 取 APIKey: %w", keyErr)
		}
		if key != "" {
			req.Header.Set("Authorization", "Bearer "+key)
		}
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, nil, fmt.Errorf("openai: request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit))
	if err != nil {
		return nil, resp.StatusCode, resp.Header, fmt.Errorf("openai: read response: %w", err)
	}
	return data, resp.StatusCode, resp.Header, nil
}
