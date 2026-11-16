// web.go：DeepSeek 官方 Search / Fetch 接口（api.deepseek.com/v1/tools/*），共用 POST 骨架与 OpenAI 兼容错误信封。

package deepseek

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/kissqianfu/AI-SDK/common/apierr"
	"github.com/kissqianfu/AI-SDK/common/types"
	"github.com/kissqianfu/AI-SDK/internal/httpclient"
)

const (
	// searchEndpointPath / fetchEndpointPath 是官方工具接口路径。
	searchEndpointPath = "/v1/tools/search"
	fetchEndpointPath  = "/v1/tools/fetch"

	// webTimeout 是单次搜索/抓取的整体超时（抓取比对话慢）。
	webTimeout = 60 * time.Second

	// webMaxBytes 是单次响应的读取上限（截断会把响应切成非法 JSON，只防异常超大）。
	webMaxBytes = 16 << 20

	// 官方文档的请求约束（客户端先卡一遍，错误信息比远端 400 可读）。
	searchQueryMaxRunes = 128
	searchCountMax      = 50
	fetchURLMaxLen      = 8192
)

// webClient 是 Search / Fetch 共用的请求骨架（端点、密钥、HTTP client 同源）。
type webClient struct {
	endpoint string
	apiKey   string
	http     *http.Client
}

// newWebClient 构造公共骨架。
func newWebClient(cfg Config, path string) *webClient {
	client := cfg.HTTP
	if client == nil {
		client = httpclient.New(webTimeout)
	}
	return &webClient{
		endpoint: strings.TrimRight(cfg.baseURL(), "/") + path,
		apiKey:   cfg.APIKey,
		http:     client,
	}
}

// post 发一次工具接口调用（非 200 也返回体，由调用方走 apierr 解析）。
func (c *webClient) post(ctx context.Context, body any) ([]byte, int, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, 0, fmt.Errorf("deepseek: 编码请求: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(raw))
	if err != nil {
		return nil, 0, fmt.Errorf("deepseek: 构造请求: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("deepseek: 请求: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, webMaxBytes))
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("deepseek: 读响应: %w", err)
	}
	return data, resp.StatusCode, nil
}

// search 是 DeepSeek 网页搜索客户端（实现 types.Searcher）。
type search struct {
	web webClient
}

// NewSearch 构造网页搜索客户端（密钥构造期校验兜底）。
func NewSearch(cfg Config) (types.Searcher, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, errors.New("deepseek: 搜索缺少 APIKey（密钥由使用方注入）")
	}
	return &search{web: *newWebClient(cfg, searchEndpointPath)}, nil
}

// Search 实现 types.Searcher。
func (s *search) Search(ctx context.Context, req types.SearchRequest) (*types.SearchResponse, error) {
	if err := validateQuery(req.Query); err != nil {
		return nil, err
	}
	if req.Count < 0 || req.Count > searchCountMax {
		return nil, fmt.Errorf("deepseek: count 必须在 1~%d（0 = 服务端默认）", searchCountMax)
	}
	if n := utf8.RuneCountInString(req.OriQuestion); n > searchQueryMaxRunes {
		return nil, fmt.Errorf("deepseek: ori_question 超过 %d 字符上限", searchQueryMaxRunes)
	}
	if req.Localization != "" && net.ParseIP(req.Localization) == nil {
		return nil, fmt.Errorf("deepseek: localization 不是合法 IP: %q", req.Localization)
	}

	// wire body：空字段省略（官方对未知字段宽容，但少发少错）。
	body := map[string]any{"query": req.Query}
	if req.Count > 0 {
		body["count"] = req.Count
	}
	if req.OriQuestion != "" {
		body["ori_question"] = req.OriQuestion
	}
	if req.Localization != "" {
		body["localization"] = req.Localization
	}
	if req.NeedContent {
		body["need_content"] = true
	}

	data, status, err := s.web.post(ctx, body)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, apierr.FromOpenAI("deepseek", status, data)
	}
	return parseSearch(data)
}

// wireSearch 是搜索响应的线格式（空结果 results 为空数组，是正常情形）。
type wireSearch struct {
	Results []struct {
		RerankScore float64 `json:"rerank_score"`
		Title       string  `json:"title"`
		URL         string  `json:"url"`
		Snippet     string  `json:"snippet"`
		Summary     string  `json:"summary"`
		Content     string  `json:"content"`
		SiteName    string  `json:"site_name"`
		SiteIcon    string  `json:"site_icon"`
		Date        string  `json:"date"`
		Authority   int     `json:"authority"`
	} `json:"results"`
}

func parseSearch(data []byte) (*types.SearchResponse, error) {
	var wire wireSearch
	if err := json.Unmarshal(data, &wire); err != nil {
		return nil, fmt.Errorf("deepseek: 解析搜索响应: %w", err)
	}
	out := &types.SearchResponse{Results: make([]types.SearchResult, 0, len(wire.Results))}
	for _, r := range wire.Results {
		out.Results = append(out.Results, types.SearchResult{
			RerankScore: r.RerankScore,
			Title:       r.Title,
			URL:         r.URL,
			Snippet:     r.Snippet,
			Summary:     r.Summary,
			Content:     r.Content,
			SiteName:    r.SiteName,
			SiteIcon:    r.SiteIcon,
			Date:        r.Date,
			Authority:   r.Authority,
		})
	}
	return out, nil
}

// fetcher 是 DeepSeek 网页抓取客户端（实现 types.Fetcher）。
type fetcher struct {
	web webClient
}

// NewFetch 构造网页抓取客户端。
func NewFetch(cfg Config) (types.Fetcher, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, errors.New("deepseek: 抓取缺少 APIKey（密钥由使用方注入）")
	}
	return &fetcher{web: *newWebClient(cfg, fetchEndpointPath)}, nil
}

// Fetch 实现 types.Fetcher。
func (f *fetcher) Fetch(ctx context.Context, req types.FetchRequest) (*types.FetchResponse, error) {
	if err := validateURL(req.URL); err != nil {
		return nil, err
	}
	data, status, err := f.web.post(ctx, map[string]any{"url": req.URL})
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		// 404 = 抓不到可用正文，403 = 安全策略拒绝，统一进 apierr 解析。
		return nil, apierr.FromOpenAI("deepseek", status, data)
	}
	var wire struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return nil, fmt.Errorf("deepseek: 解析抓取响应: %w", err)
	}
	return &types.FetchResponse{Title: wire.Title, Content: wire.Content}, nil
}

// validateQuery 校验搜索词：1~128 字符且不能全空白。
func validateQuery(q string) error {
	n := utf8.RuneCountInString(q)
	if n == 0 || strings.TrimSpace(q) == "" {
		return errors.New("deepseek: query 不能为空")
	}
	if n > searchQueryMaxRunes {
		return fmt.Errorf("deepseek: query 超过 %d 字符上限", searchQueryMaxRunes)
	}
	return nil
}

// validateURL 校验抓取地址：HTTP/HTTPS、有主机、≤8192 字符、不带用户名密码。
func validateURL(raw string) error {
	if raw == "" {
		return errors.New("deepseek: url 不能为空")
	}
	if len(raw) > fetchURLMaxLen {
		return fmt.Errorf("deepseek: url 超过 %d 字符上限", fetchURLMaxLen)
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("deepseek: url 无法解析: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("deepseek: url 仅支持 http/https，实得 %q", u.Scheme)
	}
	if u.Host == "" {
		return errors.New("deepseek: url 缺少主机名")
	}
	if u.User != nil {
		return errors.New("deepseek: url 不允许携带用户名或密码")
	}
	return nil
}
