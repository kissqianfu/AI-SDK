// web.go：网页搜索与抓取的契约；纯 JSON API，搜索不指定 model。

package types

import "context"

// SearchRequest 是一次网页搜索请求；Query 原样透传，检索算符由厂商侧解析。
type SearchRequest struct {
	// Query 是搜索词（必填，1~128 字符且不能全空白）。
	Query string
	// Count 是返回条数（1~50；0 = 用服务端默认 10）。
	Count int
	// OriQuestion 是用户原始问题（可选，≤128 字符）；空 = 不提供。
	OriQuestion string
	// Localization 是本地化结果的 IP（可选）；空 = 不本地化。
	Localization string
	// NeedContent 为 true 时结果带正文。
	NeedContent bool
}

// SearchResult 是一条搜索结果。日期等可空字段缺省为零值（空串）。
type SearchResult struct {
	// RerankScore 是综合打分（0~3），官方口径 ≥1.5 为高质量结果。
	RerankScore float64
	// Title 是结果标题。
	Title string
	// URL 是结果地址。
	URL string
	// Snippet 是短摘要（给人看的 UI 摘要）。
	Snippet string
	// Summary 是长摘要（给模型读的）。
	Summary string
	// Content 是页面正文；仅 NeedContent=true 且抓取成功时有值。
	Content string
	// SiteName 是站点名（不可得时为空）。
	SiteName string
	// SiteIcon 是站点图标地址（不可得时为空）。
	SiteIcon string
	// Date 是发布时间（RFC 3339 原文，未知时为空）。
	Date string
	// Authority 是权威等级（0~4，-1 = 未知）。
	Authority int
}

// SearchResponse 是一次搜索的返回；零结果是正常情形，不是错误。
type SearchResponse struct {
	Results []SearchResult
}

// Searcher 是网页搜索契约（协议适配器实现，不是厂商 SDK）。
type Searcher interface {
	Search(ctx context.Context, req SearchRequest) (*SearchResponse, error)
}

// FetchRequest 是一次网页抓取请求。
type FetchRequest struct {
	// URL 是要抓取的页面地址（必填，HTTP/HTTPS，≤8192 字符，不带用户名/密码）。
	URL string
}

// FetchResponse 是抓取到的页面内容。
type FetchResponse struct {
	// Title 是页面标题（不可得时为空）。
	Title string
	// Content 是页面正文的 Markdown 文本（受目标站可用性与反爬约束）。
	Content string
}

// Fetcher 是网页抓取契约（协议适配器实现，不是厂商 SDK）。
type Fetcher interface {
	Fetch(ctx context.Context, req FetchRequest) (*FetchResponse, error)
}
