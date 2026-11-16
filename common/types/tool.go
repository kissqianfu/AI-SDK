package types

import "encoding/json"

// ToolSpec 是一次 function calling 工具的自描述，由调用方按协议要求组装。
type ToolSpec struct {
	Name        string
	Description string
	Schema      json.RawMessage // JSON Schema
}
