// asr.go：千问语音转文字——走兼容模式 chat/completions，音频作为 input_audio 内容块 base64 直传。

package qianwen

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/kissqianfu/AI-SDK/common/apierr"
	"github.com/kissqianfu/AI-SDK/internal/httpclient"
)

const (
	// asrMaxBytes 是单次送转写的音频上限（官方 10MB，超了必然被拒）。
	asrMaxBytes = 10 << 20
	// asrTimeout 是单次转写的整体超时：长音频解析要时间，但不能无限等。
	asrTimeout = 90 * time.Second
)

// asrConfig 是转写器参数（APIKey 用回调：转写器长期存活，密钥可能轮换）。
type asrConfig struct {
	APIKey  func(ctx context.Context) (string, error)
	BaseURL string
	Model   string
	HTTP    *http.Client
}

// asr 是千问转写器（实现 provider.Transcriber）。
type asr struct {
	endpoint string
	model    string
	apiKey   func(ctx context.Context) (string, error)
	http     *http.Client
}

// newASR 构造转写器。
func newASR(cfg asrConfig) (*asr, error) {
	if strings.TrimSpace(cfg.Model) == "" {
		return nil, errors.New("qianwen: 转写缺少模型名（ASRParams.Model 不能为空）")
	}
	client := cfg.HTTP
	if client == nil {
		client = httpclient.New(asrTimeout)
	}
	return &asr{
		endpoint: asrEndpoint(cfg.BaseURL),
		model:    strings.TrimSpace(cfg.Model),
		apiKey:   cfg.APIKey,
		http:     client,
	}, nil
}

// asrEndpoint 从 base_url 推导 chat/completions 完整地址，复用 chat 链路的归一化。
func asrEndpoint(base string) string {
	return NormalizeBaseURL(base) + "/chat/completions"
}

// Transcribe 实现 provider.Transcriber。
func (a *asr) Transcribe(ctx context.Context, audio []byte, mime string) (string, error) {
	if err := checkAudio(audio); err != nil {
		return "", err
	}
	if strings.TrimSpace(mime) == "" {
		mime = "audio/wav"
	}
	if a.apiKey == nil {
		return "", errors.New("qianwen: 转写缺少 APIKey 回调")
	}
	key, err := a.apiKey(ctx)
	if err != nil {
		return "", fmt.Errorf("qianwen: 取密钥: %w", err)
	}
	body, status, err := postJSON(ctx, a.http, a.endpoint, key, a.payload(audio, mime), 1<<20, "转写")
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", apierr.FromDashScope("qianwen", status, body)
	}
	return parseTranscript(body)
}

// checkAudio 先卡大小：送超限音频给官方只会换回一个看不出原因的 400。
func checkAudio(audio []byte) error {
	if len(audio) == 0 {
		return errors.New("qianwen: 音频内容为空")
	}
	if len(audio) > asrMaxBytes {
		return fmt.Errorf("qianwen: 音频超过 %d 字节上限", asrMaxBytes)
	}
	return nil
}

// payload 是千问转写的请求体：音频塞进一条 user 消息的 input_audio 内容块。
func (a *asr) payload(audio []byte, mime string) map[string]any {
	return map[string]any{
		"model":  a.model,
		"stream": false,
		"messages": []map[string]any{{
			"role": "user",
			"content": []map[string]any{{
				"type": "input_audio",
				"input_audio": map[string]string{
					"data": "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(audio),
				},
			}},
		}},
		"asr_options": map[string]any{"enable_itn": true},
	}
}

// parseTranscript 取 choices[0].message.content（千问把转写结果放在对话内容里）。
func parseTranscript(body []byte) (string, error) {
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", fmt.Errorf("qianwen: 解析转写响应失败: %w", err)
	}
	if len(out.Choices) == 0 {
		return "", errors.New("qianwen: 转写响应缺少 choices")
	}
	return strings.TrimSpace(out.Choices[0].Message.Content), nil
}
