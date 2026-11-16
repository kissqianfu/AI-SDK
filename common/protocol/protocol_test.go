package protocol

import "testing"

// 大小写与空格不敏感——写 DeepSeek 不该被拒。
func TestNormalizeCaseInsensitive(t *testing.T) {
	if Normalize("  DeepSeek ") != DeepSeek {
		t.Fatalf("Normalize = %q", Normalize("  DeepSeek "))
	}
}
