// search.go：网页搜索能力的登记与构造（agent / RAG 拿实时网页内容用）。

package provider

import (
	"net/http"
	"strings"

	"github.com/kissqianfu/AI-SDK/common/protocol"
	"github.com/kissqianfu/AI-SDK/common/types"
)

// capSearch 是网页搜索能力名（纯内部细节）。
const capSearch capability = "search"

// searchRegistry 是网页搜索能力的查表，当前只有 deepseek 实现，默认协议随之。
var searchRegistry = newRegistry[*SearchParams, types.Searcher](capSearch, protocol.DeepSeek)

// SearchParams 是构造网页搜索能力所需的参数（搜索没有模型，密钥是仅有的构造要素）。
type SearchParams struct {
	endpointBase
	APIKey string
	HTTP   *http.Client
}

// RegisterSearch 由厂商模块调用，声明「这家能搜网页」。
func RegisterSearch(proto, defaultBaseURL string, build func(SearchParams) (types.Searcher, error)) {
	searchRegistry.register(proto, defaultBaseURL, func(p *SearchParams) (types.Searcher, error) {
		return build(*p)
	})
}

// NewSearch 按 protocol 构造网页搜索客户端。
func NewSearch(p SearchParams) (types.Searcher, error) {
	if strings.TrimSpace(p.APIKey) == "" {
		return nil, errMissingKey("网页搜索")
	}
	return searchRegistry.build(&p)
}
