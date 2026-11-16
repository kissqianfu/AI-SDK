package provider_test

import (
	"context"
	"strings"
	"testing"

	_ "github.com/kissqianfu/AI-SDK/aisdk"
	"github.com/kissqianfu/AI-SDK/common/protocol"
	"github.com/kissqianfu/AI-SDK/common/types"
	"github.com/kissqianfu/AI-SDK/core/provider"
)

// 没实现对话的协议必须在构造时就报错，而不是发出去才 404。
func TestNewChatRejectsNonChatProtocol(t *testing.T) {
	_, err := provider.NewChat(provider.ChatParams{Protocol: protocol.ElevenLabs, APIKey: "k", Model: "m"})
	if err == nil || !strings.Contains(err.Error(), "没有 chat 实现") {
		t.Fatalf("elevenlabs 构造对话应报没有实现，实得 %v", err)
	}
}

// 构造时端点由 SDK 内部按 protocol 决定（三家都能直接构造成功，不发请求）。
func TestNewChatBuiltins(t *testing.T) {
	for _, p := range []string{protocol.DeepSeek, protocol.OpenAI, protocol.Qianwen} {
		m, err := provider.NewChat(provider.ChatParams{Protocol: p, APIKey: "k", Model: "m"})
		if err != nil {
			t.Fatalf("%q 构造失败: %v", p, err)
		}
		if m == nil {
			t.Fatalf("%q 返回 nil 模型", p)
		}
	}
}

// 注册表是扩展点：接一家新厂商只需要在自己包里登记，分派逻辑一行不用改。
func TestRegisterExtendsProviders(t *testing.T) {
	const proto = "test-vendor-chat"
	provider.RegisterChat(proto, "https://api.test-vendor.example/v1",
		func(p provider.ChatParams) (types.ChatModel, error) {
			// 注册表应在构造前归一化 protocol（大小写不敏感）。
			if p.Protocol != proto {
				t.Fatalf("构造回调收到的 protocol = %q, want %q", p.Protocol, proto)
			}
			return stubModel{}, nil
		})
	// 大写传入也应命中登记项（归一化）。
	if _, err := provider.NewChat(provider.ChatParams{Protocol: "TEST-VENDOR-CHAT", APIKey: "k", Model: "m"}); err != nil {
		t.Fatalf("NewChat() error = %v", err)
	}
}

// 画图分派：openai 与 qianwen 已注册，deepseek 故意没有——它确实不提供画图。
func TestNewImageDispatch(t *testing.T) {
	for _, proto := range []string{protocol.OpenAI, protocol.Qianwen} {
		fn, err := provider.NewImage(provider.ImageParams{
			Protocol: proto,
			APIKey:   func(context.Context) (string, error) { return "k", nil },
			Model:    "m",
		})
		if err != nil || fn == nil {
			t.Fatalf("NewImage(%q) 应构造成功，实得 fn=%v err=%v", proto, fn, err)
		}
	}
	if _, err := provider.NewImage(provider.ImageParams{
		Protocol: protocol.DeepSeek,
		Model:    "m",
		APIKey:   func(context.Context) (string, error) { return "k", nil },
	}); err == nil || !strings.Contains(err.Error(), "没有 imagegen 实现") {
		t.Fatalf("deepseek 画图应报没有实现，实得 %v", err)
	}
	// 回调型 APIKey 为 nil：构造期报错。
	if _, err := provider.NewImage(provider.ImageParams{Protocol: protocol.OpenAI, Model: "m"}); err == nil ||
		!strings.Contains(err.Error(), "APIKey") {
		t.Fatalf("NewImage 缺 APIKey 回调应报错，实得 %v", err)
	}
}

// 转写分派：openai 与千问各有一套完全不同的请求形状，但调用方只认 (音频, MIME)。
func TestNewASRDispatch(t *testing.T) {
	for _, proto := range []string{protocol.OpenAI, protocol.Qianwen} {
		tr, err := provider.NewASR(provider.ASRParams{
			Protocol: proto,
			Model:    "asr-model",
			APIKey:   func(context.Context) (string, error) { return "k", nil },
		})
		if err != nil || tr == nil {
			t.Fatalf("NewASR(%q) 应构造成功，实得 %v/%v", proto, tr, err)
		}
	}
	// 千问这条路必须有模型名：它没有可用的兜底 ASR 模型。
	if _, err := provider.NewASR(provider.ASRParams{Protocol: protocol.Qianwen}); err == nil {
		t.Fatal("千问转写缺 model 名应报错")
	}
}

// 合成分派：调用方靠 MIME 判断，不用知道是谁合成的。
func TestNewTTSDispatch(t *testing.T) {
	cases := map[string]string{
		protocol.ElevenLabs: "audio/mpeg",
		protocol.Breeze:     "audio/mpeg",
	}
	for proto, wantMIME := range cases {
		synth, err := provider.NewTTS(provider.TTSParams{
			Protocol: proto,
			Model:    "voice-model",
			Voice:    "voice-1",
			APIKey:   "k",
		})
		if err != nil || synth == nil {
			t.Fatalf("NewTTS(%q) 应构造成功，实得 %v/%v", proto, synth, err)
		}
		if got := synth.MIME(); got != wantMIME {
			t.Fatalf("NewTTS(%q).MIME() = %q, want %q", proto, got, wantMIME)
		}
	}
	if _, err := provider.NewTTS(provider.TTSParams{Protocol: protocol.DeepSeek, Model: "m", Voice: "v"}); err == nil ||
		!strings.Contains(err.Error(), "没有 tts 实现") {
		t.Fatalf("deepseek 合成应报没有实现，实得 %v", err)
	}
}

// 搜索/抓取分派：只有 deepseek 实现（官方自研引擎），空 protocol 落默认协议。
func TestNewSearchFetchDispatch(t *testing.T) {
	s, err := provider.NewSearch(provider.SearchParams{APIKey: "k"})
	if err != nil || s == nil {
		t.Fatalf("NewSearch(默认协议) 应构造成功，实得 %v/%v", s, err)
	}
	f, err := provider.NewFetch(provider.FetchParams{Protocol: protocol.DeepSeek, APIKey: "k"})
	if err != nil || f == nil {
		t.Fatalf("NewFetch(deepseek) 应构造成功，实得 %v/%v", f, err)
	}
	// 搜索/抓取没有模型概念；反向：其他协议没有实现。
	if _, err := provider.NewSearch(provider.SearchParams{Protocol: protocol.OpenAI, APIKey: "k"}); err == nil ||
		!strings.Contains(err.Error(), "没有 search 实现") {
		t.Fatalf("openai 搜索应报没有实现，实得 %v", err)
	}
	if _, err := provider.NewFetch(provider.FetchParams{Protocol: protocol.Qianwen, APIKey: "k"}); err == nil ||
		!strings.Contains(err.Error(), "没有 fetch 实现") {
		t.Fatalf("qianwen 抓取应报没有实现，实得 %v", err)
	}
	// 密钥缺失构造期拒绝：不带空密钥上线换运行期 401。
	if _, err := provider.NewSearch(provider.SearchParams{}); err == nil || !strings.Contains(err.Error(), "缺少 APIKey") {
		t.Fatalf("NewSearch 缺 APIKey 应报错，实得 %v", err)
	}
	if _, err := provider.NewFetch(provider.FetchParams{}); err == nil || !strings.Contains(err.Error(), "缺少 APIKey") {
		t.Fatalf("NewFetch 缺 APIKey 应报错，实得 %v", err)
	}
}

// stubModel 只用来证明注册表能返回模型，行为无关紧要。
type stubModel struct{}

func (stubModel) Chat(context.Context, types.ChatRequest) (*types.ChatResponse, error) {
	return nil, nil
}

func (stubModel) Stream(context.Context, types.ChatRequest) (types.ChatStream, error) {
	return nil, nil
}

// 模型由使用方指定：缺 Model 构造期就拒绝。
func TestNewRejectsEmptyModel(t *testing.T) {
	if _, err := provider.NewChat(provider.ChatParams{Protocol: protocol.OpenAI, APIKey: "k"}); err == nil || !strings.Contains(err.Error(), "缺少 Model") {
		t.Fatalf("NewChat 缺 Model 应报错，实得 %v", err)
	}
	if _, err := provider.NewTTS(provider.TTSParams{Protocol: protocol.ElevenLabs, APIKey: "k", Model: "m"}); err == nil || !strings.Contains(err.Error(), "缺少 Voice") {
		t.Fatalf("NewTTS 缺 Voice 应报错，实得 %v", err)
	}
}
