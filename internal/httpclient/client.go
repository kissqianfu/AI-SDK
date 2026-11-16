// Package httpclient 统一构造 HTTP client：超时、连接池与禁用环境代理策略集中一处。
package httpclient

import (
	"net/http"
	"time"
)

// New 构造带超时与连接池的 client。
func New(timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = 30 * time.Second // 兜底：调用方一律显式传值，这里只防手滑传 0
	}
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   10,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
}
