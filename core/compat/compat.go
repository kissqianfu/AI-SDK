// Package compat 是 OpenAI 兼容 Chat Completions 的共享实现：公共流程只有一份，
// 厂商差异（思考参数、缓存 token 字段、端点）抽成 Dialect 由各家声明。
package compat

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/kissqianfu/AI-SDK/common/apierr"
	"github.com/kissqianfu/AI-SDK/common/types"
	"github.com/kissqianfu/AI-SDK/internal/httpclient"
)

// maxBody 限制响应体读取。
const maxBody = 4 << 20

// wireError 是流中 error 事件的增量结构（非 200 响应体的信封解析在 apierr.FromOpenAI）。
type wireError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    any    `json:"code"`
}

// Dialect 描述一家厂商与标准 OpenAI 的差异；新增厂商 = 实现 + 登记一行。
type Dialect interface {
	// Name 用于错误信息与日志前缀。
	Name() string
	// EncodeThinking 把「要不要思考」写进请求体（是否需显式关闭由方言决定）。
	EncodeThinking(p *Payload, on bool)
	// CachedTokens 取「命中缓存的输入 token」，各家字段名不同。
	CachedTokens(u Usage) int
}

// BaseDialect 是纯 OpenAI 兼容行为，导出给厂商方言嵌入，只覆写差异方法。
type BaseDialect struct{}

func (BaseDialect) Name() string { return "openai" }

func (BaseDialect) EncodeThinking(*Payload, bool) {}

func (BaseDialect) CachedTokens(u Usage) int {
	if u.PromptTokensDetails != nil {
		return u.PromptTokensDetails.CachedTokens
	}
	return u.PromptCacheHitTokens
}

// Config 是适配器参数（key 由使用方注入）。
type Config struct {
	// Name 是错误信息里的厂商前缀；留空用 "compat"。
	Name    string
	APIKey  string
	BaseURL string
	Model   string
	HTTP    *http.Client
	Logger  *slog.Logger
	// Dialect 是厂商差异；nil 表示纯 OpenAI 行为。
	Dialect Dialect
}

// Model 实现 types.ChatModel（Chat + Stream）。
type Model struct {
	apiKey  string
	baseURL string
	model   string
	http    *http.Client
	log     *slog.Logger
	dialect Dialect
}

var _ types.ChatModel = (*Model)(nil)

// New 构造适配器：三段校验（APIKey/BaseURL/Model）在这里一次做完，各家的 New 负责补默认端点。
func New(cfg Config) (*Model, error) {
	name := cfg.Name
	if name == "" {
		name = "compat"
	}
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("%s: APIKey 不能为空", name)
	}
	if strings.TrimSpace(cfg.BaseURL) == "" {
		return nil, fmt.Errorf("%s: BaseURL 不能为空（各家的 New 应先补默认端点）", name)
	}
	if strings.TrimSpace(cfg.Model) == "" {
		return nil, fmt.Errorf("%s: Model 不能为空（模型由使用方指定，各家的 New 应先校验）", name)
	}
	if cfg.HTTP == nil {
		cfg.HTTP = httpclient.New(60 * time.Second)
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	d := cfg.Dialect
	if d == nil {
		d = BaseDialect{}
	}
	return &Model{
		apiKey:  cfg.APIKey,
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		model:   cfg.Model,
		http:    cfg.HTTP,
		log:     cfg.Logger,
		dialect: d,
	}, nil
}

// Payload 是请求体（OpenAI 兼容线格式）；三个思考字段并存是有意的，由 Dialect 决定填哪个，只序列化非空项。
type Payload struct {
	Model          string          `json:"model"`
	Messages       []wireMessage   `json:"messages"`
	Temperature    *float64        `json:"temperature,omitempty"`
	MaxTokens      int             `json:"max_tokens,omitempty"`
	Stop           []string        `json:"stop,omitempty"`
	Tools          []wireTool      `json:"tools,omitempty"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
	Stream         bool            `json:"stream"`
	StreamOptions  *streamOptions  `json:"stream_options,omitempty"`

	// Thinking 是 DeepSeek 的写法：{"type":"enabled"}。
	Thinking *ThinkingParam `json:"thinking,omitempty"`
	// EnableThinking 是千问的写法：顶层布尔（qwen3 系默认 true，必须显式关）。
	EnableThinking *bool `json:"enable_thinking,omitempty"`
	// ReasoningEffort 是 OpenAI o 系列/gpt-5 的写法：low/medium/high。
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
}

// ThinkingParam 是思考模式开关（DeepSeek 形状：参数名 thinking，适配器负责映射）。
type ThinkingParam struct {
	Type string `json:"type"`
}

// wireMessage 是线上消息格式（tool 调用往返需要 tool_calls / tool_call_id）。
type wireMessage struct {
	Role       string         `json:"role"`
	Content    any            `json:"content"`
	Name       string         `json:"name,omitempty"`
	ToolCalls  []wireToolCall `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
}

// wireContent 是多模态消息的一段（text 或 image_url）。
type wireContent struct {
	Type     string        `json:"type"`
	Text     string        `json:"text,omitempty"`
	ImageURL *wireImageURL `json:"image_url,omitempty"`
}

// wireImageURL 是图片引用（data URL 或公网 https）。
type wireImageURL struct {
	URL string `json:"url"`
}

// wireTool 是请求里的工具声明。
type wireTool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		Parameters  json.RawMessage `json:"parameters"`
	} `json:"function"`
}

// wireToolCall 是模型返回/回传的工具调用。
type wireToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// responseFormat 是 JSON 模式。
type responseFormat struct {
	Type string `json:"type"`
}

// streamOptions 请求最终 chunk 带用量（官方 stream_options.include_usage）。
type streamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

// Usage 是用量；缓存命中 token 兼容 DeepSeek 与标准两种字段，由 Dialect 决定取哪个。
type Usage struct {
	PromptTokens         int                  `json:"prompt_tokens"`
	CompletionTokens     int                  `json:"completion_tokens"`
	TotalTokens          int                  `json:"total_tokens"`
	PromptCacheHitTokens int                  `json:"prompt_cache_hit_tokens"`
	PromptTokensDetails  *PromptTokensDetails `json:"prompt_tokens_details"`
}

// PromptTokensDetails 是标准位的缓存明细。
type PromptTokensDetails struct {
	CachedTokens int `json:"cached_tokens"`
}

// toPort 转成 port 的用法（缓存位按方言取）。
func (m *Model) toPort(u Usage) types.TokenUsage {
	return types.TokenUsage{
		PromptTokens:     u.PromptTokens,
		CompletionTokens: u.CompletionTokens,
		TotalTokens:      u.TotalTokens,
		CachedTokens:     m.dialect.CachedTokens(u),
	}
}

// chatResponse 是响应体（只取需要的字段）。
type chatResponse struct {
	Choices []struct {
		Message struct {
			Content          string         `json:"content"`
			ReasoningContent string         `json:"reasoning_content"`
			ToolCalls        []wireToolCall `json:"tool_calls"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage Usage `json:"usage"`
}

// Chat 实现非流式对话。
func (m *Model) Chat(ctx context.Context, req types.ChatRequest) (*types.ChatResponse, error) {
	payload := m.buildPayload(req, false)
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("%s: encode request: %w", m.dialect.Name(), err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, m.chatURL(), bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("%s: build request: %w", m.dialect.Name(), err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+m.apiKey)

	resp, err := m.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%s: request: %w", m.dialect.Name(), err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, fmt.Errorf("%s: read response: %w", m.dialect.Name(), err)
	}
	if len(body) >= maxBody {
		// 顶到上限多半是被截断，直接报超限，不喂半截 JSON 给解码器。
		return nil, fmt.Errorf("%s: 响应体超过 %dMB 上限", m.dialect.Name(), maxBody>>20)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, apierr.FromOpenAI(m.dialect.Name(), resp.StatusCode, body)
	}

	var parsed chatResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("%s: decode response: %w", m.dialect.Name(), err)
	}
	if len(parsed.Choices) == 0 {
		return nil, fmt.Errorf("%s: 响应没有 choices", m.dialect.Name())
	}
	choice := parsed.Choices[0]
	return &types.ChatResponse{
		Content:      choice.Message.Content,
		Reasoning:    choice.Message.ReasoningContent,
		ToolCalls:    fromWireToolCalls(choice.Message.ToolCalls),
		FinishReason: choice.FinishReason,
		Usage:        m.toPort(parsed.Usage),
	}, nil
}

// Stream 实现流式对话（SSE）：content 逐段产出，思考段丢弃，工具调用增量按 index 累积。
func (m *Model) Stream(ctx context.Context, req types.ChatRequest) (types.ChatStream, error) {
	payload := m.buildPayload(req, true)
	payload.StreamOptions = &streamOptions{IncludeUsage: true}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("%s: encode stream request: %w", m.dialect.Name(), err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, m.chatURL(), bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("%s: build stream request: %w", m.dialect.Name(), err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+m.apiKey)
	httpReq.Header.Set("Accept", "text/event-stream")

	// 流式时长不可预估，整体超时交给 ctx：复制 client 清零 Timeout。
	client := *m.http
	client.Timeout = 0
	// 首包兜底：补 ResponseHeaderTimeout（只对 *http.Transport 或 nil 生效；
	// 自定义 RoundTripper 无法安全包装，请自设 ctx 超时）。
	if tr, ok := m.http.Transport.(*http.Transport); ok && tr != nil && tr.ResponseHeaderTimeout == 0 {
		trCopy := tr.Clone()
		trCopy.ResponseHeaderTimeout = 30 * time.Second
		client.Transport = trCopy
	} else if m.http.Transport == nil {
		trCopy := http.DefaultTransport.(*http.Transport).Clone()
		trCopy.ResponseHeaderTimeout = 30 * time.Second
		client.Transport = trCopy
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%s: stream request: %w", m.dialect.Name(), err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxBody))
		_ = resp.Body.Close()
		return nil, apierr.FromOpenAI(m.dialect.Name(), resp.StatusCode, body)
	}
	s := &sseStream{body: resp.Body, dialect: m.dialect}
	s.scanner = bufio.NewScanner(resp.Body)
	s.scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	return s, nil
}

// chatURL 拼对话端点。base_url 各家用带 /v1 的完整前缀，这里只补路径。
func (m *Model) chatURL() string { return m.baseURL + "/chat/completions" }

// buildPayload 组装请求体：公共字段在这里，厂商差异交给 Dialect。
func (m *Model) buildPayload(req types.ChatRequest, stream bool) *Payload {
	p := &Payload{
		Model:     m.model,
		Messages:  toWire(req.Messages),
		Tools:     toWireTools(req.Tools),
		MaxTokens: req.MaxTokens,
		Stop:      req.Stop,
		Stream:    stream,
	}
	if req.Temperature != nil {
		p.Temperature = req.Temperature
	}
	if req.JSONMode {
		p.ResponseFormat = &responseFormat{Type: "json_object"}
	}
	// 思考开关永远显式告知；关思考时由方言决定沉默还是明确关掉。
	m.dialect.EncodeThinking(p, req.Thinking)
	return p
}

// streamChunk 是 SSE data: 行的增量结构（只取需要的字段）。
type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content          string           `json:"content"`
			ReasoningContent string           `json:"reasoning_content"`
			ToolCalls        []streamToolCall `json:"tool_calls"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *Usage     `json:"usage"`
	Error *wireError `json:"error"` // 个别网关把错误也作为 SSE 事件下发（HTTP 已 200）
}

// streamToolCall 是工具调用的增量片段（按 index 累积）。
type streamToolCall struct {
	Index    int    `json:"index"`
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// sseStream 实现 types.ChatStream。
type sseStream struct {
	scanner *bufio.Scanner
	body    io.ReadCloser
	dialect Dialect

	toolAcc []wireToolCall // 工具调用增量累积（index 序）
	usage   *types.TokenUsage

	// finish 缓存：finish_reason chunk 之后通常还有 usage chunk，缓存到流结束统一给终局 delta。
	finishReason  string
	finishTools   []types.ToolCall
	finishContent string
	done          bool
}

var _ types.ChatStream = (*sseStream)(nil)

// finalDelta 组装终局增量（finish_reason + 工具调用 + 最后一段正文 + 用量）。
func (s *sseStream) finalDelta() types.Delta {
	d := types.Delta{FinishReason: s.finishReason, Usage: s.usage}
	d.ToolCalls = s.finishTools
	if s.finishContent != "" {
		d.Content = s.finishContent
	}
	return d
}

// Recv 产出下一段增量；流结束返回 io.EOF。
func (s *sseStream) Recv() (types.Delta, error) {
	if s.done {
		return types.Delta{}, io.EOF
	}
	for s.scanner.Scan() {
		line := strings.TrimSpace(s.scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") || strings.HasPrefix(line, "event:") {
			continue
		}
		data, ok := strings.CutPrefix(line, "data:")
		if !ok {
			continue
		}
		data = strings.TrimSpace(data)
		if data == "[DONE]" {
			s.done = true
			if s.finishReason == "" {
				// 网关不发 finish_reason 就直接 [DONE]：按正常结束，但工具调用不能丢。
				s.finishReason = "stop"
				s.finishTools = fromWireToolCalls(s.toolAcc)
			}
			return s.finalDelta(), nil
		}
		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			s.done = true // 流已不可信：防止调用方忽略错误继续 Recv 产生错乱
			return types.Delta{}, fmt.Errorf("%s: decode stream chunk: %w", s.dialect.Name(), err)
		}
		if chunk.Error != nil {
			// HTTP 200 流里的错误事件转成 APIError，status=0 避免输出 "HTTP 0"。
			s.done = true
			return types.Delta{}, apierr.FromOpenAI(s.dialect.Name(), 0, []byte(data))
		}
		if chunk.Usage != nil {
			u := types.TokenUsage{
				PromptTokens:     chunk.Usage.PromptTokens,
				CompletionTokens: chunk.Usage.CompletionTokens,
				TotalTokens:      chunk.Usage.TotalTokens,
				CachedTokens:     s.dialect.CachedTokens(*chunk.Usage),
			}
			s.usage = &u
		}
		if len(chunk.Choices) == 0 {
			continue // include_usage 的最终 chunk 只有 usage
		}
		choice := chunk.Choices[0]
		// 工具调用增量：按 index 累积，finish 时一次性产出。
		for _, tc := range choice.Delta.ToolCalls {
			for len(s.toolAcc) <= tc.Index {
				s.toolAcc = append(s.toolAcc, wireToolCall{})
			}
			acc := &s.toolAcc[tc.Index]
			if tc.ID != "" {
				acc.ID = tc.ID
			}
			if tc.Type != "" {
				acc.Type = tc.Type
			}
			acc.Function.Name += tc.Function.Name
			acc.Function.Arguments += tc.Function.Arguments
		}
		switch choice.FinishReason {
		case "tool_calls":
			s.finishReason = "tool_calls"
			s.finishTools = fromWireToolCalls(s.toolAcc)
			continue // 继续扫，usage chunk 在后面
		case "stop", "length":
			s.finishReason = choice.FinishReason
			s.finishContent = choice.Delta.Content
			continue
		default:
			// 未知 finish_reason 原样透传，不吞不猜：若当常规增量处理，
			// [DONE] 时会被伪报成 "stop"。
			if choice.FinishReason != "" {
				s.finishReason = choice.FinishReason
				s.finishContent = choice.Delta.Content
				continue
			}
		}
		// 常规增量：只产出正文，思考段一律丢弃。
		if choice.Delta.Content != "" {
			return types.Delta{Content: choice.Delta.Content}, nil
		}
	}
	// scanner 耗尽：正常 EOF 或读错误。没有 [DONE] 也把缓存的终局 delta 给出来。
	s.done = true
	if err := s.scanner.Err(); err != nil {
		return types.Delta{}, fmt.Errorf("%s: stream read: %w", s.dialect.Name(), err)
	}
	if s.finishReason != "" {
		return s.finalDelta(), nil
	}
	return types.Delta{}, io.EOF
}

// Close 关闭底层 HTTP 流。
func (s *sseStream) Close() error {
	return s.body.Close()
}

// toWire 转换消息。
func toWire(messages []types.ChatMessage) []wireMessage {
	out := make([]wireMessage, 0, len(messages))
	for _, msg := range messages {
		out = append(out, wireMessage{
			Role:       string(msg.Role),
			Content:    buildWireContent(msg),
			Name:       msg.Name,
			ToolCalls:  toWireToolCalls(msg.ToolCalls),
			ToolCallID: msg.ToolCallID,
		})
	}
	return out
}

// toWireTools 转换工具声明。
func toWireTools(tools []types.ToolSpec) []wireTool {
	out := make([]wireTool, 0, len(tools))
	for _, spec := range tools {
		var wire wireTool
		wire.Type = "function"
		wire.Function.Name = spec.Name
		wire.Function.Description = spec.Description
		wire.Function.Parameters = spec.Schema
		out = append(out, wire)
	}
	return out
}

// toWireToolCalls 转换模型要回传的工具调用。
func toWireToolCalls(calls []types.ToolCall) []wireToolCall {
	out := make([]wireToolCall, 0, len(calls))
	for _, call := range calls {
		var wire wireToolCall
		wire.ID = call.ID
		wire.Type = "function"
		wire.Function.Name = call.Name
		wire.Function.Arguments = string(call.Arguments)
		out = append(out, wire)
	}
	return out
}

// fromWireToolCalls 转换模型返回的工具调用。
func fromWireToolCalls(calls []wireToolCall) []types.ToolCall {
	out := make([]types.ToolCall, 0, len(calls))
	for _, call := range calls {
		out = append(out, types.ToolCall{
			ID:        call.ID,
			Name:      call.Function.Name,
			Arguments: json.RawMessage(call.Function.Arguments),
		})
	}
	return out
}

// buildWireContent 构造消息内容：纯文本直接用 string；有图片时切成多模态数组。
func buildWireContent(msg types.ChatMessage) any {
	if len(msg.Images) == 0 {
		return msg.Content
	}
	parts := []wireContent{{Type: "text", Text: msg.Content}}
	for _, img := range msg.Images {
		if img.URL == "" {
			continue
		}
		parts = append(parts, wireContent{Type: "image_url", ImageURL: &wireImageURL{URL: img.URL}})
	}
	return parts
}
