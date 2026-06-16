package ai_test

import (
	"context"
	"strings"
	"testing"

	"github.com/zjutjh/jxh-go/internal/ai"
)

func TestNewEinoChatRejectsUnknownProvider(t *testing.T) {
	_, err := ai.NewEinoChat(context.Background(), ai.EinoChatConfig{
		Provider: "unknown",
		BaseURL:  "https://example.com/v1",
		APIKey:   "key",
		Model:    "model",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "unsupported eino chat provider") {
		t.Fatalf("error = %v", err)
	}
}

func TestNewEinoChatSupportsArkProvider(t *testing.T) {
	chat, err := ai.NewEinoChat(context.Background(), ai.EinoChatConfig{
		Provider: "ark",
		APIKey:   "key",
		Model:    "ep-20260617000000-demo",
	})
	if err != nil {
		t.Fatalf("NewEinoChat returned error: %v", err)
	}
	if chat == nil {
		t.Fatal("chat is nil")
	}
}
