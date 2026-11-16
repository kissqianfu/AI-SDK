package text

import "testing"

// TruncateHead 是字节级截断（错误响应体用）：不保证多字节字符完整，只保证长度。
func TestTruncateHead(t *testing.T) {
	cases := []struct {
		in   string
		n    int
		want string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello", 3, "hel..."},
		{"你好", 3, "你..."},                             // 「你」正好 3 字节，整字保留
		{"你好", 4, "你" + string([]byte{0xe5}) + "..."}, // 字节切半是有意的
		{"hello", 0, ""},
		{"", 5, ""},
	}
	for _, c := range cases {
		if got := TruncateHead([]byte(c.in), c.n); got != c.want {
			t.Fatalf("TruncateHead(%q, %d) = %q, want %q", c.in, c.n, got, c.want)
		}
	}
}
