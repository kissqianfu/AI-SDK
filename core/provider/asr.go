// asr.go：语音转文字能力的登记与构造。

package provider

import (
	"context"
	"net/http"

	"github.com/kissqianfu/AI-SDK/common/protocol"
)

// asrRegistry 是转写能力的查表。没写 protocol 时沿用历史默认（openai）。
var asrRegistry = newRegistry[*ASRParams, Transcriber](capASR, protocol.OpenAI)

// Transcriber 是语音转文字契约（音频字节 + MIME → 文本）。
type Transcriber interface {
	Transcribe(ctx context.Context, audio []byte, mime string) (string, error)
}

// ASRParams 是构造转写能力所需的参数；APIKey 用回调以支持运行期轮换。
type ASRParams struct {
	endpointBase
	Model    string
	Filename string // multipart 的文件名（缺省 voice；不支持 multipart 的协议忽略它）
	APIKey   func(ctx context.Context) (string, error)
	HTTP     *http.Client
}

// RegisterASR 由厂商模块调用，声明「这家能转写」。
func RegisterASR(proto, defaultBaseURL string, build func(ASRParams) (Transcriber, error)) {
	asrRegistry.register(proto, defaultBaseURL, func(p *ASRParams) (Transcriber, error) {
		return build(*p)
	})
}

// NewASR 按 protocol 构造转写器。模型由使用方指定，SDK 不代选。
func NewASR(p ASRParams) (Transcriber, error) {
	if err := requireModel("转写", p.Model); err != nil {
		return nil, err
	}
	if err := requireKeyFn(p.APIKey); err != nil {
		return nil, err
	}
	return asrRegistry.build(&p)
}
