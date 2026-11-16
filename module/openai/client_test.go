package openai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c, err := New(Config{BaseURL: srv.URL + "/v1", APIKey: func(context.Context) (string, error) { return "sk-test", nil }})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return c
}

func TestTranscribe(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/audio/transcriptions" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("multipart: %v", err)
		}
		if r.FormValue("model") != "whisper-1" {
			t.Errorf("model = %q", r.FormValue("model"))
		}
		_, _ = w.Write([]byte(`{"text":"你好世界"}`))
	})
	text, err := c.Transcribe(context.Background(), "whisper-1", []byte("audio"), "a.mp3", "audio/mpeg")
	if err != nil || text != "你好世界" {
		t.Fatalf("Transcribe() = %q/%v", text, err)
	}
}

func TestTranscribePlainText(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("纯文本转写结果"))
	})
	got, err := c.Transcribe(context.Background(), "m", []byte("audio"), "a.mp3", "audio/mpeg")
	if err != nil || got != "纯文本转写结果" {
		t.Fatalf("Transcribe(纯文本) = %q/%v", got, err)
	}
}

func TestGenerateImageB64(t *testing.T) {
	png := base64.StdEncoding.EncodeToString([]byte("png"))
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/images/generations" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"b64_json": png}}})
	})
	data, mime, err := c.GenerateImage(context.Background(), "dall-e-3", "一只猫", "1024x1024")
	if err != nil || string(data) != "png" || mime != "image/png" {
		t.Fatalf("GenerateImage() = %q/%q/%v", data, mime, err)
	}
}

// 空结果与解码失败分开报：合并写曾渲染出 "%!w(<nil>)" 的坏错误串。
func TestGenerateImageEmptyData(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":[]}`))
	})
	_, _, err := c.GenerateImage(context.Background(), "m", "p", "s")
	if err == nil || !strings.Contains(err.Error(), "没有 data") {
		t.Fatalf("空 data 应报独立错误，实得 %v", err)
	}
}

func TestErrors(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusBadRequest)
	})
	if _, err := c.Transcribe(context.Background(), "m", []byte("a"), "a.mp3", "audio/mpeg"); err == nil {
		t.Fatal("Transcribe(400) error = nil")
	}
	if _, _, err := c.GenerateImage(context.Background(), "m", "p", "s"); err == nil {
		t.Fatal("GenerateImage(400) error = nil")
	}
	if _, err := New(Config{BaseURL: ""}); err == nil {
		t.Fatal("empty baseURL 应报错")
	}
}
