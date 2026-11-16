// Package text 是 SDK 私有的文本工具：错误响应体截断等。
// 使用方不得 import 本包（internal 约束）。
package text

// TruncateHead 按字节截断到最多 n 字节，超出时以 "..." 结尾（不保证多字节字符不被切半）。
func TruncateHead(b []byte, n int) string {
	if n <= 0 {
		return ""
	}
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}
