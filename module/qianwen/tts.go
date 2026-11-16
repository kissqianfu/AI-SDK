// tts.go：千问语音合成（非实时端点，format=mp3，音频在 output.audio 的 base64 或 OSS URL，二者都接住）。

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
	// DefaultTTSBaseURL 是千问非实时合成的 API 根；业务空间专属域名路径相同，用 base_url 覆盖即可。
	DefaultTTSBaseURL = "https://maas.qianwenaiapi.com/api/v1"
	// ttsEndpointPath 是合成接口在 API 根之后的路径。
	ttsEndpointPath = "/services/audio/tts/SpeechSynthesizer"
	// ttsTimeout 是单次合成的整体超时（含音频下载）。
	ttsTimeout = 60 * time.Second
	// ttsMaxBytes 是单次音频下载上限。
	ttsMaxBytes = 10 << 20
)

// ttsConfig 是合成器参数（TTSParams 的 APIKey 是明文串，非回调）。
type ttsConfig struct {
	APIKey  string
	BaseURL string
	Model   string
	HTTP    *http.Client
}

// qwenTTS 是千问合成器（实现 tts.Synth，经 NewEngine 接成 provider.Synthesizer）。
type qwenTTS struct {
	endpoint string
	model    string
	apiKey   string
	http     *http.Client
}

// newTTS 构造合成器。
func newTTS(cfg ttsConfig) (*qwenTTS, error) {
	if strings.TrimSpace(cfg.Model) == "" {
		return nil, errors.New("qianwen: 合成缺少模型名（TTSParams.Model 不能为空）")
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, errors.New("qianwen: 合成缺少 API Key")
	}
	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if base == "" {
		base = DefaultTTSBaseURL
	}
	client := cfg.HTTP
	if client == nil {
		client = httpclient.New(ttsTimeout)
	}
	return &qwenTTS{
		endpoint: base + ttsEndpointPath,
		model:    strings.TrimSpace(cfg.Model),
		apiKey:   cfg.APIKey,
		http:     client,
	}, nil
}

// Speech 实现 tts.Synth：合成并返回 mp3 字节（voiceID 必传，千问按音色名选人）。
func (t *qwenTTS) Speech(ctx context.Context, voiceID, text string) ([]byte, error) {
	if strings.TrimSpace(voiceID) == "" {
		return nil, errors.New("qianwen: 合成缺少 voice（音色由使用方指定，必填）")
	}
	body := map[string]any{
		"model": t.model,
		"input": map[string]any{"text": text, "voice": voiceID, "format": "mp3"},
	}
	raw, status, err := postJSON(ctx, t.http, t.endpoint, t.apiKey, body, ttsMaxBytes, "合成")
	if err != nil {
		return nil, err
	}
	// DashScope 标准信封：业务失败也可能 HTTP 200，按 status_code/code 判定。
	var envelope struct {
		StatusCode int    `json:"status_code"`
		Code       string `json:"code"`
		Message    string `json:"message"`
		RequestID  string `json:"request_id"`
		Output     struct {
			Audio struct {
				Data string `json:"data"`
				URL  string `json:"url"`
			} `json:"audio"`
		} `json:"output"`
	}
	if status != http.StatusOK {
		// 非 200 先走统一错误解析，不把 HTML 喂给 JSON 解码器。
		return nil, apierr.FromDashScope("qianwen", status, raw)
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("qianwen: 解析合成响应（HTTP %d）: %w", status, err)
	}
	if envelope.StatusCode != 0 && envelope.StatusCode != http.StatusOK || envelope.Code != "" {
		// DashScope 业务失败可能 HTTP 200；Vendor 已在 APIError 里，不套外层前缀。
		return nil, &apierr.APIError{
			Vendor:    "qianwen",
			Code:      envelope.Code,
			Message:   envelope.Message,
			RequestID: envelope.RequestID,
			HTTP:      status,
		}
	}
	if data := envelope.Output.Audio.Data; data != "" {
		audio, err := base64.StdEncoding.DecodeString(data)
		if err != nil {
			// 个别 OSS 回源链路给的是缺 padding 的 base64：Raw 兜底一次。
			if audio, err = base64.RawStdEncoding.DecodeString(data); err != nil {
				return nil, fmt.Errorf("qianwen: 解码合成音频: %w", err)
			}
		}
		return audio, nil
	}
	if url := envelope.Output.Audio.URL; url != "" {
		return t.download(ctx, url)
	}
	return nil, fmt.Errorf("qianwen: 合成响应无音频（request_id=%s）", envelope.RequestID)
}

// download 下载音频产物（OSS 临时地址，24h 有效）。
func (t *qwenTTS) download(ctx context.Context, url string) ([]byte, error) {
	audio, _, err := fetchArtifact(ctx, t.http, url, ttsMaxBytes, "音频")
	return audio, err
}
