package apierr

import "testing"

func TestPredicates(t *testing.T) {
	cases := []struct {
		name  string
		err   APIError
		check func(*APIError) bool
		want  bool
	}{
		// IsAuth
		{"401→auth", APIError{HTTP: 401}, (*APIError).IsAuth, true},
		{"openai invalid key→auth", APIError{Code: "invalid_api_key"}, (*APIError).IsAuth, true},
		{"dashscope key→auth", APIError{Code: "InvalidApiKey.AccessKeyNotFound"}, (*APIError).IsAuth, true},
		{"camel key→auth", APIError{Code: "InvalidAPIKey"}, (*APIError).IsAuth, true},
		{"authentication_error→auth", APIError{Type: "authentication_error"}, (*APIError).IsAuth, true},
		{"invalid_request→not auth", APIError{Code: "invalid_request_error"}, (*APIError).IsAuth, false},
		{"403+合规码→not auth（合规优先）", APIError{Code: "DataInspectionFailed", HTTP: 403}, (*APIError).IsAuth, false},
		{"403+限流码→not auth", APIError{Code: "Throttling.RateQuota", HTTP: 403}, (*APIError).IsAuth, false},
		{"403+quota→not auth", APIError{Code: "quota_exceeded", HTTP: 403}, (*APIError).IsAuth, false},
		{"elevenlabs unauthenticated→auth", APIError{Code: "unauthenticated"}, (*APIError).IsAuth, true},
		// IsRateLimited
		{"429→rate limited", APIError{HTTP: 429}, (*APIError).IsRateLimited, true},
		{"openai rate limit→rl", APIError{Code: "rate_limit_exceeded"}, (*APIError).IsRateLimited, true},
		{"中划线变体→rl", APIError{Code: "rate-limit-exceeded"}, (*APIError).IsRateLimited, true},
		{"驼峰变体→rl", APIError{Code: "RateLimitExceeded"}, (*APIError).IsRateLimited, true},
		{"dashscope throttling→rl", APIError{Code: "Throttling.RateQuota"}, (*APIError).IsRateLimited, true},
		{"小写 throttling→rl", APIError{Code: "throttling.requestthrottled"}, (*APIError).IsRateLimited, true},
		{"裸 Throttling→rl", APIError{Code: "Throttling"}, (*APIError).IsRateLimited, true},
		{"AllocationQuota→not rl（额度耗尽）", APIError{Code: "Throttling.AllocationQuota"}, (*APIError).IsRateLimited, false},
		{"insufficient_quota→not rl", APIError{Code: "insufficient_quota"}, (*APIError).IsRateLimited, false},
		// IsQuota
		{"insufficient_quota→quota", APIError{Code: "insufficient_quota"}, (*APIError).IsQuota, true},
		{"驼峰 quota→quota", APIError{Code: "InsufficientQuota"}, (*APIError).IsQuota, true},
		{"AllocationQuota→quota", APIError{Code: "Throttling.AllocationQuota"}, (*APIError).IsQuota, true},
		{"arrearage→quota", APIError{Code: "Arrearage"}, (*APIError).IsQuota, true},
		{"quota_exceeded→quota", APIError{Code: "quota_exceeded"}, (*APIError).IsQuota, true},
		// IsContextOverflow
		{"context overflow", APIError{Code: "context_length_exceeded"}, (*APIError).IsContextOverflow, true},
		{"驼峰 overflow", APIError{Code: "MaximumContextLength"}, (*APIError).IsContextOverflow, true},
		// IsContentFiltered
		{"content filter code", APIError{Code: "content_filter"}, (*APIError).IsContentFiltered, true},
		{"data inspection", APIError{Code: "DataInspectionFailed"}, (*APIError).IsContentFiltered, true},
		{"type content filter", APIError{Type: "content_filter"}, (*APIError).IsContentFiltered, true},
		// 兜底
		{"plain body no code", APIError{HTTP: 500, Body: "internal"}, (*APIError).IsRateLimited, false},
		{"empty code safe", APIError{}, (*APIError).IsAuth, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.check(&c.err); got != c.want {
				t.Fatalf("got %v, want %v（err=%v）", got, c.want, &c.err)
			}
		})
	}
}

func TestErrorMessage(t *testing.T) {
	e := &APIError{Vendor: "qianwen", Code: "Throttling.RateQuota", HTTP: 429, RequestID: "r-1", Message: "太快"}
	if s := e.Error(); s != "qianwen: HTTP 429: code=Throttling.RateQuota: request_id=r-1: 太快" {
		t.Fatalf("Error() = %q", s)
	}
	b := &APIError{Vendor: "openai", HTTP: 500, Body: "boom"}
	if s := b.Error(); s != "openai: HTTP 500: boom" {
		t.Fatalf("Error() = %q", s)
	}
	// 流中错误事件没有 HTTP：不输出 "HTTP 0"。
	s := &APIError{Vendor: "deepseek", Code: "insufficient_quota", Message: "没钱"}
	if s.Error() != "deepseek: code=insufficient_quota: 没钱" {
		t.Fatalf("Error() = %q", s.Error())
	}
}

func TestEnvelopeParsers(t *testing.T) {
	// OpenAI：字符串码
	e := FromOpenAI("openai", 401, []byte(`{"error":{"message":"Incorrect API key","type":"invalid_request_error","code":"invalid_api_key"}}`))
	if e.Code != "invalid_api_key" || e.HTTP != 401 || !e.IsAuth() {
		t.Fatalf("FromOpenAI = %+v", e)
	}
	// OpenAI：整型码（部分网关形态）
	e = FromOpenAI("deepseek", 429, []byte(`{"error":{"message":"too fast","code":429}}`))
	if e.Code != "429" || !e.IsRateLimited() {
		t.Fatalf("FromOpenAI 整型码 = %+v", e)
	}
	// OpenAI：解析不出 → 原始体
	e = FromOpenAI("openai", 502, []byte(`<html>bad gateway</html>`))
	if e.Code != "" || e.Body == "" {
		t.Fatalf("无信封应留 Body = %+v", e)
	}
	// DashScope：顶层信封带 request_id
	e = FromDashScope("qianwen", 200, []byte(`{"code":"InvalidApiKey","message":"bad","request_id":"req-9"}`))
	if e.Code != "InvalidApiKey" || e.RequestID != "req-9" || !e.IsAuth() {
		t.Fatalf("FromDashScope = %+v", e)
	}
	// DashScope 信封解析不出 → 回试 OpenAI → 都不行留原始体
	e = FromDashScope("qianwen", 502, []byte(`<html>502</html>`))
	if e.Code != "" || e.Body == "" {
		t.Fatalf("FromDashScope fallback = %+v", e)
	}
	// ElevenLabs：detail 信封
	e = FromElevenLabs("elevenlabs", 401, []byte(`{"detail":{"status":"quota_exceeded","message":"limit"}}`))
	if e.Code != "quota_exceeded" || !e.IsQuota() {
		t.Fatalf("FromElevenLabs = %+v", e)
	}
	// 体超限截断
	long := make([]byte, bodyLimit+100)
	for i := range long {
		long[i] = 'x'
	}
	if got := truncateBody(long); len(got) != bodyLimit+len("…(truncated)") {
		t.Fatalf("truncateBody len = %d", len(got))
	}
}
