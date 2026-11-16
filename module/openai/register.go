// register.go：把 OpenAI 的对话、画图、转写、向量化四条接口登记进注册表。

package openai

import (
	"context"
	"strings"

	"github.com/kissqianfu/AI-SDK/common/protocol"
	"github.com/kissqianfu/AI-SDK/common/types"
	"github.com/kissqianfu/AI-SDK/core/provider"
)

func init() {
	provider.RegisterChat(protocol.OpenAI, DefaultChatBaseURL, func(p provider.ChatParams) (types.ChatModel, error) {
		return NewChat(ChatConfig{
			APIKey: p.APIKey,
			Model:  p.Model,
			HTTP:   p.HTTP,
			Logger: p.Logger,
		})
	})

	provider.RegisterImage(protocol.OpenAI, DefaultImageBaseURL, func(p provider.ImageParams) (provider.ImageFunc, error) {
		client, err := New(Config{BaseURL: DefaultImageBaseURL, APIKey: p.APIKey, HTTP: p.HTTP})
		if err != nil {
			return nil, err
		}
		return func(ctx context.Context, prompt string) ([]byte, string, error) {
			return client.GenerateImage(ctx, p.Model, prompt, p.Size)
		}, nil
	})

	provider.RegisterASR(protocol.OpenAI, DefaultASRBaseURL, func(p provider.ASRParams) (provider.Transcriber, error) {
		client, err := New(Config{BaseURL: DefaultASRBaseURL, APIKey: p.APIKey, HTTP: p.HTTP})
		if err != nil {
			return nil, err
		}
		filename := strings.TrimSpace(p.Filename)
		if filename == "" {
			filename = "voice"
		}
		return &transcriber{client: client, model: p.Model, filename: filename}, nil
	})

	provider.RegisterEmbed(protocol.OpenAI, DefaultEmbedBaseURL, func(p provider.EmbedParams) (types.Embedder, error) {
		client, err := New(Config{BaseURL: DefaultEmbedBaseURL, APIKey: p.APIKey, HTTP: p.HTTP})
		if err != nil {
			return nil, err
		}
		return &embedder{client: client, model: p.Model}, nil
	})
}
