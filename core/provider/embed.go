// embed.go：文本向量化能力的登记与构造（记忆回填与召回 query 向量化用）。

package provider

import (
	"context"
	"net/http"

	"github.com/kissqianfu/AI-SDK/common/protocol"
	"github.com/kissqianfu/AI-SDK/common/types"
)

// embedRegistry 是向量化能力的查表。没写 protocol 时沿用历史默认（openai）。
var embedRegistry = newRegistry[*EmbedParams, types.Embedder](capEmbed, protocol.OpenAI)

// EmbedParams 是构造向量化能力所需的参数；APIKey 用回调以支持运行期轮换。
type EmbedParams struct {
	endpointBase
	Model  string
	APIKey func(ctx context.Context) (string, error)
	HTTP   *http.Client
}

// RegisterEmbed 由厂商模块调用，声明「这家能向量化」。
func RegisterEmbed(proto, defaultBaseURL string, build func(EmbedParams) (types.Embedder, error)) {
	embedRegistry.register(proto, defaultBaseURL, func(p *EmbedParams) (types.Embedder, error) {
		return build(*p)
	})
}

// NewEmbedder 按 protocol 构造向量化客户端。模型由使用方指定，SDK 不代选。
func NewEmbedder(p EmbedParams) (types.Embedder, error) {
	if err := requireModel("向量化", p.Model); err != nil {
		return nil, err
	}
	if err := requireKeyFn(p.APIKey); err != nil {
		return nil, err
	}
	return embedRegistry.build(&p)
}
