// Package tts 把文本合成语音：产物（mp3 等）原样返回不做转码，mime 声明产物类型。
package tts

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

const (
	// maxSynthRunes 是单次合成文本上限。
	maxSynthRunes = 200
	// synthTimeout 是合成超时。
	synthTimeout = 40 * time.Second
)

// MPEGMIME 是 mp3 直传产物的 MIME（各合成端的默认产物格式）。
const MPEGMIME = "audio/mpeg"

// Synth 是语音合成（ttscompat.Client 等实现）。
type Synth interface {
	Speech(ctx context.Context, voiceID, text string) ([]byte, error)
}

// Engine 合成语音：产物原样返回，mime 声明产物类型。
type Engine struct {
	synth   Synth
	voiceID string
	mime    string // 产物 MIME（audio/mpeg 等），由构造方给出
	log     *slog.Logger
}

// NewEngine 构造语音引擎；mime 是产物类型，必须以 audio/ 开头。
func NewEngine(synth Synth, voiceID, mime string, logger *slog.Logger) (*Engine, error) {
	if synth == nil {
		return nil, fmt.Errorf("tts: synth 不能为空")
	}
	mime = strings.ToLower(strings.TrimSpace(mime))
	if !strings.HasPrefix(mime, "audio/") {
		return nil, fmt.Errorf("tts: mime 必须以 audio/ 开头，当前 %q", mime)
	}
	if strings.TrimSpace(voiceID) == "" {
		return nil, fmt.Errorf("tts: voice_id 不能为空")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Engine{synth: synth, voiceID: voiceID, mime: mime, log: logger}, nil
}

// MIME 返回产物 MIME（调用方据此填语音消息的 file_type/文件名）。
func (e *Engine) MIME() string { return e.mime }

// Synthesize 返回语音字节（可直接走媒体发送）。
func (e *Engine) Synthesize(ctx context.Context, text string) ([]byte, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("tts: 文本为空")
	}
	if len([]rune(text)) > maxSynthRunes {
		return nil, fmt.Errorf("tts: 文本超过 %d 字", maxSynthRunes)
	}
	ctx, cancel := context.WithTimeout(ctx, synthTimeout)
	defer cancel()
	start := time.Now()
	audio, err := e.synth.Speech(ctx, e.voiceID, text)
	if err != nil {
		return nil, fmt.Errorf("tts: 合成失败: %w", err)
	}
	if len(audio) == 0 {
		return nil, fmt.Errorf("tts: 合成返回空音频")
	}
	e.log.Debug("tts 合成完成", "voice", e.voiceID, "bytes", len(audio), "elapsed", time.Since(start).Round(time.Millisecond))
	return audio, nil
}
