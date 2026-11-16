// register.go：把千问的四条接口登记进注册表，各条登记带各自的默认端点。

package qianwen

import (
	"github.com/kissqianfu/AI-SDK/common/protocol"
	"github.com/kissqianfu/AI-SDK/common/types"
	"github.com/kissqianfu/AI-SDK/core/provider"
	"github.com/kissqianfu/AI-SDK/module/tts"
)

func init() {
	provider.RegisterChat(protocol.Qianwen, DefaultBaseURL, func(p provider.ChatParams) (types.ChatModel, error) {
		return New(Config{
			APIKey: p.APIKey,
			Model:  p.Model,
			HTTP:   p.HTTP,
			Logger: p.Logger,
		})
	})

	provider.RegisterImage(protocol.Qianwen, DefaultImageBaseURL, func(p provider.ImageParams) (provider.ImageFunc, error) {
		return NewImageGen(ImageConfig{
			APIKey: p.APIKey,
			Model:  p.Model,
			Size:   p.Size,
			HTTP:   p.HTTP,
		})
	})

	provider.RegisterASR(protocol.Qianwen, DefaultBaseURL, func(p provider.ASRParams) (provider.Transcriber, error) {
		return newASR(asrConfig{
			APIKey: p.APIKey,
			Model:  p.Model,
			HTTP:   p.HTTP,
		})
	})

	provider.RegisterTTS(protocol.Qianwen, DefaultTTSBaseURL, func(p provider.TTSParams) (provider.Synthesizer, error) {
		synth, err := newTTS(ttsConfig{
			APIKey: p.APIKey,
			Model:  p.Model,
			HTTP:   p.HTTP,
		})
		if err != nil {
			return nil, err
		}
		// format=mp3 直传：产物可直接经媒体发送。
		return tts.NewEngine(synth, p.Voice, tts.MPEGMIME, p.Logger)
	})
}
