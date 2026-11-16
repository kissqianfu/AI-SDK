// Package elevenlabs 是 ElevenLabs 官方的 tts 适配，协议实现共享在 core/ttscompat，这里只登记默认端点。

package elevenlabs

import (
	"github.com/kissqianfu/AI-SDK/common/protocol"
	"github.com/kissqianfu/AI-SDK/core/provider"
	"github.com/kissqianfu/AI-SDK/core/ttscompat"
	"github.com/kissqianfu/AI-SDK/module/tts"
)

// DefaultBaseURL 是 ElevenLabs 官方 API 根（POST {base}/text-to-speech/{voice}）。
const DefaultBaseURL = "https://api.elevenlabs.io/v1"

func init() {
	provider.RegisterTTS(protocol.ElevenLabs, DefaultBaseURL, func(p provider.TTSParams) (provider.Synthesizer, error) {
		client, err := ttscompat.New(ttscompat.Config{
			Name:    "elevenlabs",
			APIKey:  p.APIKey,
			BaseURL: DefaultBaseURL,
			Model:   p.Model,
			HTTP:    p.HTTP,
			// OutputFormat 留空 = mp3，产物直传无需本地转码。
		})
		if err != nil {
			return nil, err
		}
		return tts.NewEngine(client, p.Voice, tts.MPEGMIME, p.Logger)
	})
}
