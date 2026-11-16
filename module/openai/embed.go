package openai

import (
	"context"
	"encoding/json"
	"fmt"
)

// DefaultEmbedBaseURL 是向量化端点前缀（与对话端点值相同但语义分开，便于单独换域名）。
const DefaultEmbedBaseURL = "https://api.openai.com/v1"

// embedder 是 types.Embedder 的 OpenAI 兼容实现（/embeddings，批量）。
type embedder struct {
	client *Client
	model  string
}

// Embed 实现 types.Embedder（批量）。
func (e *embedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	raw, err := e.client.do(ctx, "/embeddings", map[string]any{"model": e.model, "input": texts})
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
			Index     int       `json:"index"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("openai: decode embeddings: %w", err)
	}
	out := make([][]float32, len(texts))
	for _, item := range resp.Data {
		if item.Index >= 0 && item.Index < len(out) {
			out[item.Index] = item.Embedding
		}
	}
	for i, vec := range out {
		if len(vec) == 0 {
			return nil, fmt.Errorf("openai: embeddings[%d] 为空", i)
		}
	}
	return out, nil
}
