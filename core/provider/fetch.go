// fetch.go：网页抓取能力的登记与构造（单页 → Markdown 正文）。

package provider

import (
	"net/http"
	"strings"

	"github.com/kissqianfu/AI-SDK/common/protocol"
	"github.com/kissqianfu/AI-SDK/common/types"
)

// capFetch 是网页抓取能力名（纯内部细节）。
const capFetch capability = "fetch"

// fetchRegistry 是网页抓取能力的查表。当前只有 deepseek 实现，默认协议随之。
var fetchRegistry = newRegistry[*FetchParams, types.Fetcher](capFetch, protocol.DeepSeek)

// FetchParams 是构造网页抓取能力所需的参数。端点由 SDK 内部按 protocol 决定。
type FetchParams struct {
	endpointBase
	APIKey string
	HTTP   *http.Client
}

// RegisterFetch 由厂商模块调用，声明「这家能抓网页」。
func RegisterFetch(proto, defaultBaseURL string, build func(FetchParams) (types.Fetcher, error)) {
	fetchRegistry.register(proto, defaultBaseURL, func(p *FetchParams) (types.Fetcher, error) {
		return build(*p)
	})
}

// NewFetch 按 protocol 构造网页抓取客户端。
func NewFetch(p FetchParams) (types.Fetcher, error) {
	if strings.TrimSpace(p.APIKey) == "" {
		return nil, errMissingKey("网页抓取")
	}
	return fetchRegistry.build(&p)
}
