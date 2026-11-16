// 厂商实现的装配点：blank import 本包一次，各家在 register.go 的 init 里向 provider 登记。

package aisdk

import (
	_ "github.com/kissqianfu/AI-SDK/module/breeze"
	_ "github.com/kissqianfu/AI-SDK/module/deepseek"
	_ "github.com/kissqianfu/AI-SDK/module/elevenlabs"
	_ "github.com/kissqianfu/AI-SDK/module/openai"
	_ "github.com/kissqianfu/AI-SDK/module/qianwen"
)
