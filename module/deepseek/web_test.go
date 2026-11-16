// web_test.go：Search / Fetch 的线格式、校验与错误映射。
// 服务端用 httptest 假装：路径、鉴权头、请求体逐项断言。

package deepseek

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kissqianfu/AI-SDK/common/apierr"
	"github.com/kissqianfu/AI-SDK/common/types"
)

func newWebClients(t *testing.T, handler http.HandlerFunc) (types.Searcher, types.Fetcher) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	s, err := NewSearch(Config{APIKey: "sk-test", BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("NewSearch() error = %v", err)
	}
	f, err := NewFetch(Config{APIKey: "sk-test", BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("NewFetch() error = %v", err)
	}
	return s, f
}

// TestSearchSendsAndParses 正常流：请求体字段逐项核对，响应映射到契约。
func TestSearchSendsAndParses(t *testing.T) {
	var gotBody map[string]any
	s, _ := newWebClients(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/tools/search" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer sk-test" {
			t.Errorf("Authorization = %q", auth)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"results":[{
			"rerank_score":2.4,"title":"标题","url":"https://example.com/a",
			"snippet":"短摘要","summary":"长摘要","content":"正文",
			"site_name":"Example","site_icon":"https://example.com/f.ico",
			"date":"2026-06-01T00:00:00+00:00","authority":3}]}`))
	})

	resp, err := s.Search(context.Background(), types.SearchRequest{
		Query: "DeepSeek site:github.com", Count: 5,
		OriQuestion: "原始问题", Localization: "203.0.113.9", NeedContent: true,
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if gotBody["query"] != "DeepSeek site:github.com" || gotBody["count"] != float64(5) ||
		gotBody["ori_question"] != "原始问题" || gotBody["localization"] != "203.0.113.9" ||
		gotBody["need_content"] != true {
		t.Fatalf("请求体 = %v", gotBody)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("results = %d 条", len(resp.Results))
	}
	r := resp.Results[0]
	if r.Title != "标题" || r.URL != "https://example.com/a" || r.RerankScore != 2.4 ||
		r.Authority != 3 || r.Content != "正文" || r.Date != "2026-06-01T00:00:00+00:00" {
		t.Fatalf("result = %+v", r)
	}
}

// TestSearchOmitsEmptyFields 未提供的可选字段不发（少发少错）。
func TestSearchOmitsEmptyFields(t *testing.T) {
	var gotBody map[string]any
	s, _ := newWebClients(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"results":[]}`))
	})
	if _, err := s.Search(context.Background(), types.SearchRequest{Query: "x"}); err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	for _, k := range []string{"count", "ori_question", "localization", "need_content"} {
		if _, ok := gotBody[k]; ok {
			t.Errorf("未设置的字段 %q 不应出现在请求体: %v", k, gotBody)
		}
	}
}

// TestSearchEmptyResultsIsOK 空结果（results 空数组）是正常情形，不是错误。
func TestSearchEmptyResultsIsOK(t *testing.T) {
	s, _ := newWebClients(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"results":[]}`))
	})
	resp, err := s.Search(context.Background(), types.SearchRequest{Query: "不存在的东西"})
	if err != nil {
		t.Fatalf("空结果不应报错: %v", err)
	}
	if len(resp.Results) != 0 {
		t.Fatalf("results = %d 条", len(resp.Results))
	}
}

// TestSearchValidation 构造请求的客户端校验（query 长度/空白）。
func TestSearchValidation(t *testing.T) {
	s, _ := newWebClients(t, func(w http.ResponseWriter, _ *http.Request) {
		t.Error("校验失败的请求不应发出")
	})
	cases := []struct {
		name  string
		query string
		count int
	}{
		{"空 query", "", 0},
		{"全空白 query", "   ", 0},
		{"超长 query", strings.Repeat("长", 129), 0},
		{"count 超上限", "x", searchCountMax + 1},
		{"count 负数", "x", -1},
	}
	for _, c := range cases {
		if _, err := s.Search(context.Background(), types.SearchRequest{Query: c.query, Count: c.count}); err == nil {
			t.Errorf("%s: 应报错", c.name)
		}
	}
	// localization 语义上是 IP：非 IP 地址直接拦下，不发出去换远端 400。
	if _, err := s.Search(context.Background(), types.SearchRequest{Query: "x", Localization: "not-an-ip"}); err == nil {
		t.Error("localization 非 IP 应报错")
	}
}

// TestSearchErrorEnvelope 非 200 走 OpenAI 兼容错误信封解析，判定可直接用。
func TestSearchErrorEnvelope(t *testing.T) {
	cases := []struct {
		status int
		body   string
		check  func(*apierr.APIError) bool
		want   string
	}{
		{http.StatusUnauthorized, `{"error":{"message":"bad key","type":"invalid_request_error","param":null,"code":"invalid_request_error"}}`, (*apierr.APIError).IsAuth, "401→IsAuth"},
		{http.StatusTooManyRequests, `{"error":{"message":"qps","type":"invalid_request_error","param":null,"code":"rate_limit_error"}}`, (*apierr.APIError).IsRateLimited, "429→IsRateLimited"},
		{http.StatusPaymentRequired, `{"error":{"message":"no balance","type":"invalid_request_error","param":null,"code":"insufficient_balance"}}`, (*apierr.APIError).IsQuota, "402→IsQuota"},
	}
	for _, c := range cases {
		s, _ := newWebClients(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(c.status)
			_, _ = w.Write([]byte(c.body))
		})
		_, err := s.Search(context.Background(), types.SearchRequest{Query: "x"})
		ae, ok := err.(*apierr.APIError)
		if !ok {
			t.Errorf("%s: 应返回 *apierr.APIError，实得 %v", c.want, err)
			continue
		}
		if !c.check(ae) {
			t.Errorf("%s: 判定未命中（HTTP %d, code=%q）", c.want, ae.HTTP, ae.Code)
		}
	}
}

// TestFetchSendsAndParses 正常流：url 入体，title/content 出响应。
func TestFetchSendsAndParses(t *testing.T) {
	var gotBody map[string]any
	_, f := newWebClients(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/tools/fetch" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"title":"页面标题","content":"# 正文\n\n段落。"}`))
	})
	resp, err := f.Fetch(context.Background(), types.FetchRequest{URL: "https://example.com/article"})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if gotBody["url"] != "https://example.com/article" {
		t.Fatalf("请求体 = %v", gotBody)
	}
	if resp.Title != "页面标题" || resp.Content != "# 正文\n\n段落。" {
		t.Fatalf("resp = %+v", resp)
	}
}

// TestFetchValidation 构造请求的客户端校验（官方约束原样落地）。
func TestFetchValidation(t *testing.T) {
	_, f := newWebClients(t, func(w http.ResponseWriter, _ *http.Request) {
		t.Error("校验失败的请求不应发出")
	})
	cases := []struct {
		name string
		url  string
	}{
		{"空 url", ""},
		{"超长 url", "https://example.com/" + strings.Repeat("a", fetchURLMaxLen)},
		{"非 http 协议", "ftp://example.com/a"},
		{"缺主机", "/only/path"},
		{"带用户名密码", "https://user:pass@example.com/a"},
	}
	for _, c := range cases {
		if _, err := f.Fetch(context.Background(), types.FetchRequest{URL: c.url}); err == nil {
			t.Errorf("%s: 应报错", c.name)
		}
	}
}

// TestFetch404IsAPIError 抓不到正文（404）走错误信封而非 panic/裸错误。
func TestFetch404IsAPIError(t *testing.T) {
	_, f := newWebClients(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"message":"no content","type":"invalid_request_error","param":null,"code":"invalid_request_error"}}`))
	})
	_, err := f.Fetch(context.Background(), types.FetchRequest{URL: "https://example.com/x"})
	ae, ok := err.(*apierr.APIError)
	if !ok {
		t.Fatalf("应返回 *apierr.APIError，实得 %v", err)
	}
	if ae.HTTP != http.StatusNotFound {
		t.Fatalf("HTTP = %d", ae.HTTP)
	}
}

// TestNewSearchFetchRequireKey 密钥缺失在构造期报错（provider 与模块两层都拦）。
func TestNewSearchFetchRequireKey(t *testing.T) {
	if _, err := NewSearch(Config{}); err == nil {
		t.Error("模块层：缺 APIKey 应报错")
	}
	if _, err := NewFetch(Config{}); err == nil {
		t.Error("模块层：缺 APIKey 应报错")
	}
}
