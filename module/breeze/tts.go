// Package breeze 是 Breeze 语音（ElevenLabs 同形网关）的 tts 实现，协议客户端共享在 core/ttscompat。
package breeze

import (
	"github.com/kissqianfu/AI-SDK/common/protocol"
	"github.com/kissqianfu/AI-SDK/core/provider"
	"github.com/kissqianfu/AI-SDK/core/ttscompat"
	"github.com/kissqianfu/AI-SDK/module/tts"
)

// DefaultBaseURL 是 Breeze 的 API 根（ElevenLabs 同形：POST {base}/text-to-speech/{voice}）。
const DefaultBaseURL = "https://api.breeze.blue/v1"

func init() {
	provider.RegisterTTS(protocol.Breeze, DefaultBaseURL, func(p provider.TTSParams) (provider.Synthesizer, error) {
		client, err := ttscompat.New(ttscompat.Config{
			Name:    "breeze",
			APIKey:  p.APIKey,
			BaseURL: DefaultBaseURL,
			Model:   p.Model,
			HTTP:    p.HTTP,
		})
		if err != nil {
			return nil, err
		}
		return tts.NewEngine(client, p.Voice, tts.MPEGMIME, p.Logger)
	})
}
