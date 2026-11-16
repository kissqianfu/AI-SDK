// imagegen.go：千问文生图（Qwen-Image），走百炼原生 multimodal-generation；响应只给图片 URL（24h 过期），故当场下载。

package qianwen

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/kissqianfu/AI-SDK/common/apierr"
	"github.com/kissqianfu/AI-SDK/internal/httpclient"
)

const (
	// DefaultImageBaseURL 是千问原生接口根域名（不带 /compatible-mode/v1）。
	DefaultImageBaseURL = "https://maas.qianwenaiapi.com"
	// imageGenPath 是原生多模态生成路径。
	imageGenPath = "/api/v1/services/aigc/multimodal-generation/generation"
	// defaultImageSize 是缺省出图尺寸（百炼默认 2048*2048 太大，收小）。
	defaultImageSize = "1024*1024"
	// imageMaxBytes 限制下载的图片体积。
	imageMaxBytes = 16 << 20
	// imageGenTimeout 是单次生成的整体超时。
	imageGenTimeout = 90 * time.Second
)

// ImageConfig 是文生图参数；APIKey 用回调是为了支持密钥轮换（每次调用时取）。
type ImageConfig struct {
	APIKey  func(ctx context.Context) (string, error)
	BaseURL string
	Model   string
	Size    string
	HTTP    *http.Client
}

// NewImageGen 构造文生图闭包，模型与 APIKey 回调构造期必填校验。
func NewImageGen(cfg ImageConfig) (func(ctx context.Context, prompt string) ([]byte, string, error), error) {
	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		return nil, errors.New("qianwen: 画图缺少 Model（模型由使用方指定，SDK 不代选）")
	}
	if cfg.APIKey == nil {
		return nil, errors.New("qianwen: 画图缺少 APIKey 回调")
	}
	size := NormalizeImageSize(cfg.Size)
	endpoint := ImageEndpoint(cfg.BaseURL)
	client := cfg.HTTP
	if client == nil {
		client = httpclient.New(imageGenTimeout)
	}
	return func(ctx context.Context, prompt string) ([]byte, string, error) {
		prompt = strings.TrimSpace(prompt)
		if prompt == "" {
			return nil, "", errors.New("qianwen: 提示词为空")
		}
		key, err := cfg.APIKey(ctx)
		if err != nil {
			return nil, "", fmt.Errorf("qianwen: 取密钥: %w", err)
		}

		// 官方限制：messages 仅支持单轮且只有一个元素，content 里只能有一个 text。
		body := map[string]any{
			"model": model,
			"input": map[string]any{
				"messages": []any{map[string]any{
					"role":    "user",
					"content": []any{map[string]any{"text": prompt}},
				}},
			},
			"parameters": map[string]any{
				"size":          size,
				"n":             1,
				"watermark":     false, // 关右下角水印
				"prompt_extend": true,  // 提示词智能改写
			},
		}
		data, status, err := postJSON(ctx, client, endpoint, key, body, 4<<20, "文生图")
		if err != nil {
			return nil, "", err
		}
		if status != http.StatusOK {
			// Vendor 已在 APIError 里（DashScope 信封解析在 apierr.FromDashScope）。
			return nil, "", apierr.FromDashScope("qianwen", status, data)
		}

		var parsed struct {
			Output struct {
				Choices []struct {
					Message struct {
						Content []struct {
							Image string `json:"image"`
							Text  string `json:"text"`
						} `json:"content"`
					} `json:"message"`
				} `json:"choices"`
			} `json:"output"`
			RequestID string `json:"request_id"`
		}
		if err := json.Unmarshal(data, &parsed); err != nil {
			return nil, "", fmt.Errorf("qianwen: 解析文生图响应: %w", err)
		}
		// 找第一个非空 image：content 顺序由模型决定，可能先 text 后 image。
		imageURL := ""
		for _, choice := range parsed.Output.Choices {
			for _, item := range choice.Message.Content {
				if item.Image != "" {
					imageURL = item.Image
					break
				}
			}
			if imageURL != "" {
				break
			}
		}
		if imageURL == "" {
			return nil, "", fmt.Errorf("qianwen: 文生图响应没有图片（request_id=%s）", parsed.RequestID)
		}
		return downloadImage(ctx, client, imageURL)
	}, nil
}

// ImageEndpoint 把「兼容模式 / 裸域名 / 完整端点」统一成原生路径；已是完整端点的原样返回。
func ImageEndpoint(base string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		return DefaultImageBaseURL + imageGenPath
	}
	base = strings.TrimRight(base, "/")
	if strings.Contains(base, "/services/aigc/") {
		return base
	}
	base = strings.TrimSuffix(base, "/compatible-mode/v1")
	base = strings.TrimSuffix(base, "/api/v1")
	return base + imageGenPath
}

// NormalizeImageSize 把 size 统一成百炼要的「宽*高」（OpenAI 风格 1024x1024 也兼容），非法值退默认。
func NormalizeImageSize(size string) string {
	size = strings.TrimSpace(size)
	if size == "" {
		return defaultImageSize
	}
	parts := strings.FieldsFunc(size, func(r rune) bool {
		return r == 'x' || r == 'X' || r == '*' || r == '×'
	})
	if len(parts) != 2 {
		return defaultImageSize
	}
	w, err1 := strconv.Atoi(parts[0])
	h, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil || w <= 0 || h <= 0 {
		return defaultImageSize
	}
	return fmt.Sprintf("%d*%d", w, h)
}

// downloadImage 取回生成结果。官方只给 URL 且 24 小时过期，所以必须当场落地。
func downloadImage(ctx context.Context, client *http.Client, imageURL string) ([]byte, string, error) {
	if !strings.HasPrefix(imageURL, "http://") && !strings.HasPrefix(imageURL, "https://") {
		return nil, "", fmt.Errorf("qianwen: 图片地址不是 URL: %q", imageURL)
	}
	data, mime, err := fetchArtifact(ctx, client, imageURL, imageMaxBytes, "生成图")
	if err != nil {
		return nil, "", err
	}
	// OSS 的 Content-Type 可靠；取不到时按 png（官方固定输出 png）。
	if !strings.HasPrefix(mime, "image/") {
		mime = "image/png"
	}
	return data, mime, nil
}
