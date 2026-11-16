// fetch.go：千问系公共 HTTP 助手——postJSON（JSON POST 骨架）与 fetchArtifact（OSS 产物下载）。

package qianwen

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// postJSON 发 JSON POST 并返回响应体与状态码（非 200 也返回体，由调用方按信封判定）。
func postJSON(ctx context.Context, client *http.Client, endpoint, bearer string, body any, max int64, what string) ([]byte, int, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, 0, fmt.Errorf("qianwen: 编码%s请求: %w", what, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return nil, 0, fmt.Errorf("qianwen: 构造%s请求: %w", what, err)
	}
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("qianwen: %s请求: %w", what, err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, max))
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("qianwen: 读%s响应: %w", what, err)
	}
	return data, resp.StatusCode, nil
}

// fetchArtifact 下载产物地址，返回内容与 Content-Type（原样，兜底由调用方定）。
func fetchArtifact(ctx context.Context, client *http.Client, url string, max int64, what string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", fmt.Errorf("qianwen: 构造%s下载: %w", what, err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("qianwen: 下载%s: %w", what, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("qianwen: 下载%s HTTP %d", what, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, max))
	if err != nil {
		return nil, "", fmt.Errorf("qianwen: 读%s: %w", what, err)
	}
	if len(data) == 0 {
		return nil, "", fmt.Errorf("qianwen: %s内容为空", what)
	}
	return data, strings.TrimSpace(resp.Header.Get("Content-Type")), nil
}
