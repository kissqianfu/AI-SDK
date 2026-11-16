// Package provider 按（能力, 协议）分派到厂商构造器；厂商在各自 register.go 的 init 中登记。
package provider

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/kissqianfu/AI-SDK/common/protocol"
)

// capability 是模型的接口类别（对话/画图/转写/合成/向量化/搜索/抓取），纯内部细节。
type capability string

const (
	capChat  capability = "chat"
	capImage capability = "imagegen"
	capASR   capability = "asr"
	capTTS   capability = "tts"
	capEmbed capability = "embed"
	// capSearch / capFetch 在 search.go / fetch.go（各自与注册表放一起）。
)

// endpointed 是各接口参数结构体的公共部分；注册表用它统一做 protocol 缺省补齐。
type endpointed interface {
	protocolName() string
	setEndpoint(proto string)
}

// recipe 是一家的一个接口登记项。
type recipe[P endpointed, T any] struct {
	// defaultBaseURL 是该协议的默认端点，随登记一起交出。
	defaultBaseURL string
	build          func(P) (T, error)
}

// registry 是「能力 → 协议 → 登记项」的查表。
type registry[P endpointed, T any] struct {
	cap      capability
	fallback string // 构造未指定 protocol 时用哪个协议
	mu       sync.RWMutex
	items    map[string]recipe[P, T]
}

func newRegistry[P endpointed, T any](cap capability, fallback string) *registry[P, T] {
	return &registry[P, T]{
		cap:      cap,
		fallback: fallback,
		items:    make(map[string]recipe[P, T], 4),
	}
}

// register 登记一条（协议名大小写不敏感，后登记的覆盖先前的）。
func (r *registry[P, T]) register(proto, defaultBaseURL string, build func(P) (T, error)) {
	key := protocol.Normalize(proto)
	if key == "" {
		return
	}
	r.mu.Lock()
	r.items[key] = recipe[P, T]{defaultBaseURL: defaultBaseURL, build: build}
	r.mu.Unlock()
}

// names 返回已注册协议（排好序，用于报错）。
func (r *registry[P, T]) names() []string {
	r.mu.RLock()
	out := make([]string, 0, len(r.items))
	for name := range r.items {
		out = append(out, name)
	}
	r.mu.RUnlock()
	sort.Strings(out)
	return out
}

func (r *registry[P, T]) lookup(proto string) (recipe[P, T], bool) {
	r.mu.RLock()
	rec, ok := r.items[protocol.Normalize(proto)]
	r.mu.RUnlock()
	return rec, ok
}

// build 补齐 protocol 后交给该家实现构造，端点用登记的默认端点。
func (r *registry[P, T]) build(p P) (T, error) {
	var zero T
	proto := protocol.Normalize(p.protocolName())
	if proto == "" {
		proto = r.fallback
	}
	rec, ok := r.lookup(proto)
	if !ok {
		if names := r.names(); len(names) > 0 {
			return zero, fmt.Errorf("provider: 协议 %q 没有 %s 实现（已注册：%s）",
				proto, r.cap, strings.Join(names, "/"))
		}
		return zero, fmt.Errorf("provider: 协议 %q 没有 %s 实现（一家都没注册）", proto, r.cap)
	}
	if rec.defaultBaseURL == "" {
		return zero, fmt.Errorf("provider: 协议 %q 的 %s 未登记端点", proto, r.cap)
	}
	p.setEndpoint(proto)
	if rec.build == nil {
		return zero, fmt.Errorf("provider: 协议 %q 的 %s 登记项缺少构造函数", proto, r.cap)
	}
	return rec.build(p)
}
