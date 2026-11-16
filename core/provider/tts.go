// tts.go：文本转语音能力的登记与构造。

package provider

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/kissqianfu/AI-SDK/common/protocol"
)

// ttsRegistry 是语音合成能力的查表。没写 protocol 时沿用历史默认（elevenlabs）。
var ttsRegistry = newRegistry[*TTSParams, Synthesizer](capTTS, protocol.ElevenLabs)

// Synthesizer 是语音合成契约；MIME 一并交出，供调用方填文件类型。
type Synthesizer interface {
	Synthesize(ctx context.Context, text string) ([]byte, error)
	MIME() string
}

// TTSParams 是构造语音合成能力所需的参数。端点由 SDK 内部按 protocol 决定。
type TTSParams struct {
	endpointBase
	Model  string
	Voice  string // voice_id；必填（构造期校验）
	APIKey string
	HTTP   *http.Client
	Logger *slog.Logger
}

// RegisterTTS 由厂商模块调用，声明「这家能合成语音」。
func RegisterTTS(proto, defaultBaseURL string, build func(TTSParams) (Synthesizer, error)) {
	ttsRegistry.register(proto, defaultBaseURL, func(p *TTSParams) (Synthesizer, error) {
		return build(*p)
	})
}

// NewTTS 按 protocol 构造语音合成器（模型与音色构造期必填校验）。
func NewTTS(p TTSParams) (Synthesizer, error) {
	if err := requireModel("语音合成", p.Model); err != nil {
		return nil, err
	}
	if err := requireVoice(p.Voice); err != nil {
		return nil, err
	}
	return ttsRegistry.build(&p)
}
