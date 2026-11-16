package tts

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// fakeSynth 返回固定音频。
type fakeSynth struct {
	audio []byte
	err   error
}

func (f *fakeSynth) Speech(context.Context, string, string) ([]byte, error) {
	return f.audio, f.err
}

func TestEngineSynthesize(t *testing.T) {
	engine, err := NewEngine(&fakeSynth{audio: []byte("MP3")}, "voice-1", "audio/mpeg", nil)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	audio, err := engine.Synthesize(context.Background(), " 你好 ")
	if err != nil {
		t.Fatalf("Synthesize() error = %v", err)
	}
	if string(audio) != "MP3" {
		t.Fatalf("audio = %q", audio)
	}
}

func TestEngineValidation(t *testing.T) {
	if _, err := NewEngine(nil, "v", "audio/mpeg", nil); err == nil {
		t.Fatal("nil synth 未报错")
	}
	if _, err := NewEngine(&fakeSynth{}, "v", "text/plain", nil); err == nil {
		t.Fatal("非 audio/ mime 未报错")
	}
	if _, err := NewEngine(&fakeSynth{}, "  ", "audio/mpeg", nil); err == nil {
		t.Fatal("空 voice_id 未报错")
	}
	engine, _ := NewEngine(&fakeSynth{audio: []byte("P")}, "v", "audio/mpeg", nil)
	if _, err := engine.Synthesize(context.Background(), ""); err == nil {
		t.Fatal("空文本未报错")
	}
	long := strings.Repeat("字", maxSynthRunes+1)
	if _, err := engine.Synthesize(context.Background(), long); err == nil {
		t.Fatal("超长文本未报错")
	}
}

func TestEngineErrors(t *testing.T) {
	synthErr, _ := NewEngine(&fakeSynth{err: errors.New("boom")}, "v", "audio/mpeg", nil)
	if _, err := synthErr.Synthesize(context.Background(), "hi"); err == nil {
		t.Fatal("合成失败未透传")
	}
	emptyAudio, _ := NewEngine(&fakeSynth{audio: nil}, "v", "audio/mpeg", nil)
	if _, err := emptyAudio.Synthesize(context.Background(), "hi"); err == nil {
		t.Fatal("空音频未报错")
	}
}

func TestMIME(t *testing.T) {
	engine, err := NewEngine(&fakeSynth{audio: []byte("MP3BYTES")}, "voc_x", "audio/mpeg", nil)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	if engine.MIME() != "audio/mpeg" {
		t.Fatalf("MIME() = %q", engine.MIME())
	}
}
