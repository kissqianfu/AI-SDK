// image.go：OpenAI 标准文生图（POST /images/generations）；b64_json 与 URL 两种响应都接住。

package openai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
)

// DefaultImageBaseURL 是图像生成端点前缀（与对话端点值相同但语义分开，便于单独换域名）。
const DefaultImageBaseURL = "https://api.openai.com/v1"

// GenerateImage 生成图片（/images/generations），返回图片字节与 MIME。
func (c *Client) GenerateImage(ctx context.Context, model, prompt, size string) ([]byte, string, error) {
	raw, err := c.do(ctx, "/images/generations", map[string]any{
		"model": model, "prompt": prompt, "size": size, "n": 1, "response_format": "b64_json",
	})
	if err != nil {
		return nil, "", err
	}
	var resp struct {
		Data []struct {
			B64  string `json:"b64_json"`
			URL  string `json:"url"`
			MIME string `json:"mime_type"`
		} `json:"data"`
	}
	// 解码失败与"空结果"分开报：合并写会在 err==nil 时渲染出坏错误串。
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, "", fmt.Errorf("openai: decode image: %w", err)
	}
	if len(resp.Data) == 0 {
		return nil, "", fmt.Errorf("openai: 画图响应没有 data（模型可能不支持该 prompt/size）")
	}
	item := resp.Data[0]
	if item.B64 != "" {
		data, err := base64.StdEncoding.DecodeString(item.B64)
		if err != nil {
			return nil, "", fmt.Errorf("openai: decode image b64: %w", err)
		}
		mime := item.MIME
		if mime == "" {
			mime = "image/png"
		}
		return data, mime, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, item.URL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("openai: build image download request: %w", err)
	}
	data, status, _, err := c.send(req, 16<<20)
	if err != nil {
		return nil, "", err
	}
	if status != http.StatusOK {
		return nil, "", fmt.Errorf("openai: 下载生成图 HTTP %d", status)
	}
	return data, "image/png", nil
}
