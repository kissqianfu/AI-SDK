# AI-SDK

多厂商大模型能力的 Go 接入层：**对话、向量化、语音转写、语音合成、图像生成、网页搜索、网页抓取**七类能力，openai / deepseek / qianwen / breeze / elevenlabs 五家实现。只做协议接入，不管配置与密钥；零第三方依赖。

## 安装

```bash
go get github.com/kissqianfu/AI-SDK
```

## 版本

版本只由 git tag 表达（`vX.Y.Z`），源码内不维护版本文件；使用方按 tag 引用：
`go get github.com/kissqianfu/AI-SDK@v0.8.0`。

## 用法

```go
import (
    "os"

    "github.com/kissqianfu/AI-SDK/core/provider"
    "github.com/kissqianfu/AI-SDK/common/types"
    _ "github.com/kissqianfu/AI-SDK/aisdk" // blank import 装配全部厂商实现
)

model, _ := provider.NewChat(provider.ChatParams{
    Protocol: "openai", // 留空 = 默认协议（对话 deepseek；向量/画图/转写 openai；合成 elevenlabs；搜索/抓取 deepseek）
    APIKey:   os.Getenv("OPENAI_KEY"),
    Model:    "gpt-4o",
})
resp, _ := model.Chat(ctx, types.ChatRequest{Messages: []types.ChatMessage{{Role: "user", Content: "你好"}}})
stream, _ := model.Stream(ctx, req) // 流式
emb, _ := provider.NewEmbedder(provider.EmbedParams{Protocol: "openai", Model: "text-embedding-3-small",
    APIKey: func(ctx context.Context) (string, error) { return os.Getenv("OPENAI_KEY"), nil }})
tts, _ := provider.NewTTS(provider.TTSParams{Protocol: "elevenlabs", Model: "eleven_turbo_v2_5", Voice: "<voice_id>", APIKey: "..."})
audio, _ := tts.Synthesize(ctx, "文本") // mp3 字节；tts.MIME() 给产物类型
asr, _  := provider.NewASR(provider.ASRParams{Protocol: "openai", Model: "whisper-1", /* APIKey 回调 */})
img, _  := provider.NewImage(provider.ImageParams{Protocol: "openai", Model: "dall-e-3", /* APIKey 回调 */})
searcher, _ := provider.NewSearch(provider.SearchParams{APIKey: os.Getenv("DEEPSEEK_KEY")})
results, _ := searcher.Search(ctx, types.SearchRequest{Query: "AI SDK", Count: 5})
fetcher, _ := provider.NewFetch(provider.FetchParams{APIKey: os.Getenv("DEEPSEEK_KEY")})
page, _ := fetcher.Fetch(ctx, types.FetchRequest{URL: "https://example.com/a"}) // Markdown 正文
```

## 错误处理

所有厂商路径的 API 错误统一转成 `common/apierr.APIError`，`errors.As` 拿到后按语义分支：

```go
var ae *apierr.APIError
if errors.As(err, &ae) {
    switch {
    case ae.IsAuth():            // 换 key，不要原样重试
    case ae.IsRateLimited():     // 退避后重试
    case ae.IsQuota():           // 余额/配额耗尽，重试无意义
    case ae.IsContextOverflow(): // 截断历史后重发
    case ae.IsContentFiltered(): // 重发同一输入不会变
    }
}
```
## 扩展

新厂商的新接口：`module/<厂商>` 实现 + `register.go` 登记 + `aisdk/modules.go` 加一行 blank import；
OpenAI 兼容 wire 细节归 `core/compat`，ElevenLabs 同形网关归 `core/ttscompat`。
