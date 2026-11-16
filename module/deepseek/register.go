// register.go：把 DeepSeek 的对话、网页搜索、网页抓取三条接口登记进注册表。

package deepseek

import (
	"github.com/kissqianfu/AI-SDK/common/protocol"
	"github.com/kissqianfu/AI-SDK/common/types"
	"github.com/kissqianfu/AI-SDK/core/provider"
)

func init() {
	provider.RegisterChat(protocol.DeepSeek, DefaultBaseURL, func(p provider.ChatParams) (types.ChatModel, error) {
		return New(Config{
			APIKey: p.APIKey,
			Model:  p.Model,
			HTTP:   p.HTTP,
			Logger: p.Logger,
		})
	})
	provider.RegisterSearch(protocol.DeepSeek, DefaultBaseURL, func(p provider.SearchParams) (types.Searcher, error) {
		return NewSearch(Config{
			APIKey: p.APIKey,
			HTTP:   p.HTTP,
		})
	})
	provider.RegisterFetch(protocol.DeepSeek, DefaultBaseURL, func(p provider.FetchParams) (types.Fetcher, error) {
		return NewFetch(Config{
			APIKey: p.APIKey,
			HTTP:   p.HTTP,
		})
	})
}
