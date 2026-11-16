// Package compat 的公共层校验测试；行为由各厂商包的测试覆盖。
package compat

import (
	"bufio"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/kissqianfu/AI-SDK/common/apierr"
)

// 模型由使用方指定：公共层兜底校验，防空模型穿透到请求期。
func TestNewRejectsMissingParams(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
		want string
	}{
		{"缺 key", Config{BaseURL: "https://x", Model: "m"}, "APIKey 不能为空"},
		{"缺端点", Config{APIKey: "k", Model: "m"}, "BaseURL 不能为空"},
		{"缺模型", Config{APIKey: "k", BaseURL: "https://x"}, "Model 不能为空"},
	}
	for _, c := range cases {
		if _, err := New(c.cfg); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Fatalf("%s: New() error = %v, want 含 %q", c.name, err, c.want)
		}
	}
}

// 非 200 的 OpenAI 错误信封要转成 *apierr.APIError。
func TestAPIErrFromEnvelope(t *testing.T) {
	body := []byte(`{"error":{"message":"Incorrect API key","type":"invalid_request_error","code":"invalid_api_key"}}`)
	var ae *apierr.APIError
	if !errors.As(apierr.FromOpenAI("openai", 401, body), &ae) {
		t.Fatal("应返回 *apierr.APIError")
	}
	if ae.Code != "invalid_api_key" || !ae.IsAuth() {
		t.Fatalf("Code=%q IsAuth()=false", ae.Code)
	}
	// 解析不出信封：保留原始体，码为空。
	plain := apierr.FromOpenAI("openai", 502, []byte(`<html>bad gateway</html>`))
	if plain.Code != "" || plain.Body == "" {
		t.Fatalf("无信封应留 Body，实得 %+v", plain)
	}
	// 码为整型的网关也要能取出来。
	num := []byte(`{"error":{"message":"too fast","code":429}}`)
	if !errors.As(apierr.FromOpenAI("deepseek", 429, num), &ae) || ae.Code != "429" || !ae.IsRateLimited() {
		t.Fatalf("整型码解析失败: %+v", ae)
	}
}

// 流中 error 事件也要转成同一形态。
func TestStreamErrorEvent(t *testing.T) {
	body := "data: {\"error\":{\"message\":\"quota exceeded\",\"code\":\"insufficient_quota\"}}\n\ndata: [DONE]\n\n"
	s := &sseStream{body: io.NopCloser(strings.NewReader(body)), dialect: BaseDialect{}}
	s.scanner = bufio.NewScanner(s.body)
	s.scanner.Buffer(make([]byte, 0, 1024), 1024*1024)
	_, err := s.Recv()
	var ae *apierr.APIError
	if !errors.As(err, &ae) || ae.Code != "insufficient_quota" || !ae.IsQuota() {
		t.Fatalf("流中错误应转 APIError: %v", err)
	}
	if _, err := s.Recv(); !errors.Is(err, io.EOF) {
		t.Fatalf("出错后 Recv 应立即 EOF: %v", err)
	}
}
