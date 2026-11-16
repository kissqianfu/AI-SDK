// Package apierr 定义厂商错误的统一载体 *APIError、语义判定与各协议信封解析，判定走分词前缀匹配。
package apierr

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"unicode"
)

// bodyLimit 是解析不出错误码时保留的原始响应体上限。
const bodyLimit = 512

// APIError 是厂商返回的错误（HTTP 非 2xx，或 HTTP 200 但业务信封带错误码）。
type APIError struct {
	Vendor    string // 厂商名（"openai" / "deepseek" / "qianwen" / "breeze" / "elevenlabs"）
	Code      string // 厂商错误码；空表示响应体没有结构化错误码（只有原始 Body）
	Type      string // OpenAI 系信封的 error.type（authentication_error / rate_limit_error / ...）
	Message   string
	HTTP      int    // HTTP 状态码（业务信封失败时可能是 200；流中错误事件没有 HTTP，为 0）
	RequestID string // DashScope request_id 等，便于提工单
	Body      string // 解析不出错误码时的截断原始响应体（≤512 字节）
}

// Error 实现 error。
func (e *APIError) Error() string {
	parts := []string{e.Vendor}
	if e.HTTP != 0 {
		parts = append(parts, fmt.Sprintf("HTTP %d", e.HTTP))
	}
	if e.Code != "" {
		parts = append(parts, "code="+e.Code)
	}
	if e.RequestID != "" {
		parts = append(parts, "request_id="+e.RequestID)
	}
	head := strings.Join(parts, ": ")
	if e.Message != "" {
		return head + ": " + e.Message
	}
	if e.Body != "" {
		return head + ": " + e.Body
	}
	return head
}

// FromOpenAI 解析 OpenAI 兼容信封（对话/向量/转写链路共用，SSE error 事件传 status=0）。
func FromOpenAI(vendor string, status int, body []byte) *APIError {
	e := &APIError{Vendor: vendor, HTTP: status}
	var envelope struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    any    `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err == nil &&
		(envelope.Error.Message != "" || envelope.Error.Type != "" || envelope.Error.Code != nil) {
		e.Code = codeAny(envelope.Error.Code)
		e.Type = envelope.Error.Type
		e.Message = envelope.Error.Message
		return e
	}
	e.Body = truncateBody(body)
	return e
}

// FromDashScope 解析 DashScope 信封，业务失败可能 HTTP 200；兼容 OpenAI 信封回试一次。
func FromDashScope(vendor string, status int, body []byte) *APIError {
	e := &APIError{Vendor: vendor, HTTP: status}
	var dash struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(body, &dash); err == nil && dash.Code != "" {
		e.Code, e.Message, e.RequestID = dash.Code, dash.Message, dash.RequestID
		return e
	}
	if oe := FromOpenAI(vendor, status, body); oe.Code != "" || oe.Message != "" {
		return oe
	}
	e.Body = truncateBody(body)
	return e
}

// FromElevenLabs 解析 ElevenLabs 信封（detail 也可能是纯文本）。
func FromElevenLabs(vendor string, status int, body []byte) *APIError {
	e := &APIError{Vendor: vendor, HTTP: status}
	var envelope struct {
		Detail struct {
			Status  string `json:"status"`
			Message string `json:"message"`
		} `json:"detail"`
	}
	if err := json.Unmarshal(body, &envelope); err == nil && envelope.Detail.Status != "" {
		e.Code, e.Message = envelope.Detail.Status, envelope.Detail.Message
		return e
	}
	e.Body = truncateBody(body)
	return e
}

// codeAny 把信封里 any 形态的 code 转成字符串码（字符串直接用，整型码转十进制）。
func codeAny(c any) string {
	switch v := c.(type) {
	case string:
		return v
	case float64: // JSON 数字解码默认形态
		return fmt.Sprintf("%d", int64(v))
	}
	return ""
}

// truncateBody 截断错误体到 bodyLimit（本包保持零内部依赖，不引 internal/text）。
func truncateBody(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) <= bodyLimit {
		return s
	}
	return s[:bodyLimit] + "…(truncated)"
}

// ---- 语义判定：分词序列前缀匹配 ----

// codeTokens 把错误码归一成小写词序列：`.`/`-`/`_`/空格与驼峰边界都拆词。
func codeTokens(code string) []string {
	rs := []rune(strings.TrimSpace(code))
	var out []string
	var cur strings.Builder
	flush := func() {
		if cur.Len() > 0 {
			out = append(out, cur.String())
			cur.Reset()
		}
	}
	for i, r := range rs {
		switch {
		case r == '.' || r == '-' || r == '_' || r == ' ':
			flush()
		case unicode.IsUpper(r):
			prev := rune(0)
			if i > 0 {
				prev = rs[i-1]
			}
			// 驼峰边界：小写/数字后接大写，或连续大写后接小写。
			if unicode.IsLower(prev) || unicode.IsDigit(prev) ||
				(unicode.IsUpper(prev) && i+1 < len(rs) && unicode.IsLower(rs[i+1])) {
				flush()
			}
			cur.WriteRune(unicode.ToLower(r))
		default:
			cur.WriteRune(unicode.ToLower(r))
		}
	}
	flush()
	return out
}

// 判定词表：token 序列的前缀匹配（保守——只认语义明确的前缀，不做模糊包含）。
var (
	authPatterns = [][]string{
		{"invalid", "api", "key"}, {"invalid", "apikey"}, {"incorrect", "api", "key"},
		{"invalid", "token"}, {"unauthorized"}, {"unauthenticated"}, {"forbidden"},
		{"access", "denied"}, {"permission", "denied"},
	}
	ratePatterns = [][]string{
		{"rate", "limit"}, {"request", "limit"}, {"request", "throttled"},
		{"concurrency"}, {"too", "many", "requests"},
	}
	quotaPatterns = [][]string{
		{"insufficient", "quota"}, {"quota", "exceeded"}, {"arrearage"},
		{"throttling", "allocation"}, {"throttling", "resource"}, // DashScope 额度耗尽
	}
	overflowPatterns = [][]string{
		{"context", "length"}, {"maximum", "context", "length"},
	}
	filterPatterns = [][]string{
		{"content", "filter"}, {"content", "policy", "violation"}, {"data", "inspection", "failed"},
	}
)

// matchPrefix 报告 tokens 是否以任一 pattern 开头。
func matchPrefix(tokens []string, patterns [][]string) bool {
	for _, p := range patterns {
		if len(tokens) < len(p) {
			continue
		}
		ok := true
		for i, w := range p {
			if tokens[i] != w {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

// IsAuth 表示凭证/权限类错误，重试无意义，应换 key 或停用通道；具体语义优先于 HTTP 403。
func (e *APIError) IsAuth() bool {
	t := codeTokens(e.Code)
	if matchPrefix(t, filterPatterns) || matchPrefix(t, quotaPatterns) ||
		matchPrefix(t, ratePatterns) || t != nil && t[0] == "throttling" {
		return false
	}
	if matchPrefix(t, authPatterns) {
		return true
	}
	if e.Type == "authentication_error" {
		return true
	}
	return e.HTTP == 401 || e.HTTP == 403
}

// IsRateLimited 表示触发频控，退避后重试有意义；DashScope Throttling.* 按子码分语义。
func (e *APIError) IsRateLimited() bool {
	if e.HTTP == 429 || e.Type == "rate_limit_error" {
		return true
	}
	t := codeTokens(e.Code)
	if len(t) == 0 {
		return false
	}
	if t[0] == "throttling" {
		for _, w := range t[1:] {
			switch w {
			case "allocation", "resource", "quota":
				return false
			case "rate", "request", "concurrency", "throttled":
				return true
			}
		}
		return true // 裸 Throttling/未知子码默认按频控
	}
	return matchPrefix(t, ratePatterns)
}

// IsQuota 表示配额/欠费类错误，重试无意义；HTTP 402 直接命中。
func (e *APIError) IsQuota() bool {
	if e.HTTP == http.StatusPaymentRequired {
		return true
	}
	return matchPrefix(codeTokens(e.Code), quotaPatterns)
}

// IsContextOverflow 表示输入超出模型上下文窗口，应截断/压缩后重发。
func (e *APIError) IsContextOverflow() bool {
	return matchPrefix(codeTokens(e.Code), overflowPatterns)
}

// IsContentFiltered 表示内容被厂商合规拦截，重试同一输入不会变。
func (e *APIError) IsContentFiltered() bool {
	if e.Type == "content_filter" {
		return true
	}
	return matchPrefix(codeTokens(e.Code), filterPatterns)
}
