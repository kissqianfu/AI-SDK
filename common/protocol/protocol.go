// Package protocol 是模型协议名的唯一词表；只管名字与默认协议，不判断谁支持什么接口。
package protocol

import "strings"

const (
	// DeepSeek 是 DeepSeek 官方（对话/视觉/联网搜索）。
	DeepSeek = "deepseek"
	// OpenAI 是 OpenAI 兼容协议（对话、图像生成、语音识别、向量化）。
	OpenAI = "openai"
	// Qianwen 是通义千问（对话与 ASR 都走 OpenAI 兼容 + input_audio）。
	Qianwen = "qianwen"
	// ElevenLabs 是 ElevenLabs TTS（mp3 直传）。
	ElevenLabs = "elevenlabs"
	// Breeze 是 Breeze TTS（ElevenLabs 同形网关，mp3 直传）。
	Breeze = "breeze"
)

// Normalize 统一写法：去空格、转小写。空串原样返回（表示「没配」）。
func Normalize(p string) string { return strings.ToLower(strings.TrimSpace(p)) }

// 各能力的默认协议由 provider 的能力注册表自持；谁支持哪个接口由各家在 register.go 登记，
// 都不在这里判断，避免第二份真相。
