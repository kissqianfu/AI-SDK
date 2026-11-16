// image.go：文生图能力的登记与构造。

package provider

import (
	"context"
	"net/http"

	"github.com/kissqianfu/AI-SDK/common/protocol"
)

// imageRegistry 是画图能力的查表，默认协议 openai。
var imageRegistry = newRegistry[*ImageParams, ImageFunc](capImage, protocol.OpenAI)

// ImageParams 是构造画图能力所需的参数；APIKey 用回调以支持运行期轮换。
type ImageParams struct {
	endpointBase
	Model  string
	Size   string
	APIKey func(ctx context.Context) (string, error)
	HTTP   *http.Client
}

// ImageFunc 生成一张图，返回 (图片字节, MIME, 错误)；这里是能力契约。
type ImageFunc func(ctx context.Context, prompt string) (data []byte, mime string, err error)

// RegisterImage 由厂商模块调用，声明「这家能画图」。
func RegisterImage(proto, defaultBaseURL string, build func(ImageParams) (ImageFunc, error)) {
	imageRegistry.register(proto, defaultBaseURL, func(p *ImageParams) (ImageFunc, error) {
		return build(*p)
	})
}

// NewImage 按 protocol 构造画图闭包。模型由使用方指定，SDK 不代选。
func NewImage(p ImageParams) (ImageFunc, error) {
	if err := requireModel("画图", p.Model); err != nil {
		return nil, err
	}
	if err := requireKeyFn(p.APIKey); err != nil {
		return nil, err
	}
	return imageRegistry.build(&p)
}
