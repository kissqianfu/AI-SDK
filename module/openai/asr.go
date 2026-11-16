// asr.go：OpenAI 标准语音转写（POST /audio/transcriptions，multipart 上传文件）。

package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/kissqianfu/AI-SDK/internal/text"
)

// DefaultASRBaseURL 是转写端点前缀（与对话端点值相同但语义分开，便于单独换域名）。
const DefaultASRBaseURL = "https://api.openai.com/v1"

// transcriber 把 Client 收成 provider.Transcriber 的形状；构造时定死 model 与文件名。
type transcriber struct {
	client   *Client
	model    string
	filename string
}

// Transcribe 实现 provider.Transcriber。
func (t *transcriber) Transcribe(ctx context.Context, audio []byte, mime string) (string, error) {
	return t.client.Transcribe(ctx, t.model, audio, t.filename, mime)
}

// Transcribe 语音转文字（/audio/transcriptions，multipart，boundary 由标准库随机生成）。
func (c *Client) Transcribe(ctx context.Context, model string, audio []byte, filename, mime string) (string, error) {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	if err := mw.WriteField("model", model); err != nil {
		return "", fmt.Errorf("openai: encode ASR form: %w", err)
	}
	fw, err := mw.CreateFormFile("file", filename)
	if err != nil {
		return "", fmt.Errorf("openai: encode ASR form: %w", err)
	}
	if _, err := fw.Write(audio); err != nil {
		return "", fmt.Errorf("openai: encode ASR form: %w", err)
	}
	if err := mw.Close(); err != nil {
		return "", fmt.Errorf("openai: encode ASR form: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/audio/transcriptions", &body)
	if err != nil {
		return "", fmt.Errorf("openai: build ASR request: %w", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	data, status, _, err := c.send(req, 1<<20)
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("openai: ASR HTTP %d: %s", status, text.TruncateHead(data, 200))
	}
	var out struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		// 有的兼容网关直接回纯文本，按原文用；形状像 JSON 却解析失败必须报错。
		trimmed := strings.TrimSpace(string(data))
		if strings.HasPrefix(trimmed, "{") {
			return "", fmt.Errorf("openai: decode ASR response: %w", err)
		}
		return trimmed, nil
	}
	return strings.TrimSpace(out.Text), nil
}
