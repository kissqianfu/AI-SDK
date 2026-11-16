// params.go：全部能力 Params 的公共底座（endpointBase）与构造期校验 helper。

package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// endpointBase 是各能力 Params 的公共底座：Protocol 字段 + 归属查询/回填。
type endpointBase struct {
	Protocol string // 留空 = 该能力默认协议
}

func (p *endpointBase) protocolName() string { return p.Protocol }
func (p *endpointBase) setEndpoint(proto string) {
	p.Protocol = proto
}

// requireModel 校验 Model 必填（capability 用于错误文案）。
func requireModel(capability, model string) error {
	if strings.TrimSpace(model) != "" {
		return nil
	}
	return fmt.Errorf("provider: %s缺少 Model（模型由使用方指定，SDK 不代选）", capability)
}

// requireVoice 校验 Voice 必填（音色由使用方指定，SDK 不代选）。
func requireVoice(voice string) error {
	if strings.TrimSpace(voice) != "" {
		return nil
	}
	return errors.New("provider: 缺少 Voice（音色由使用方指定）")
}

// requireKeyFn 校验回调型 APIKey 必填（缺了当场报错，不带空密钥换运行期 401）。
func requireKeyFn(fn func(context.Context) (string, error)) error {
	if fn == nil {
		return errors.New("provider: 缺少 APIKey 回调（密钥由使用方注入）")
	}
	return nil
}

// errMissingKey 是字符串型密钥缺失的构造期错误。
func errMissingKey(what string) error {
	return fmt.Errorf("provider: %s缺少 APIKey（密钥由使用方注入）", what)
}
