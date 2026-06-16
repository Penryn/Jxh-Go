package vector_test

import (
	"context"
	"strings"
	"testing"

	"github.com/zjutjh/jxh-go/internal/vector"
)

func TestNewEinoEmbedderRejectsUnknownProvider(t *testing.T) {
	_, err := vector.NewEinoEmbedder(context.Background(), vector.EinoEmbedderConfig{
		Provider: "unknown",
		BaseURL:  "https://example.com/v1",
		APIKey:   "key",
		Model:    "model",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "unsupported eino embedding provider") {
		t.Fatalf("error = %v", err)
	}
}

func TestNewEinoEmbedderSupportsArkProvider(t *testing.T) {
	embedder, err := vector.NewEinoEmbedder(context.Background(), vector.EinoEmbedderConfig{
		Provider: "ark",
		APIKey:   "key",
		Model:    "ep-20260617000000-demo",
	})
	if err != nil {
		t.Fatalf("NewEinoEmbedder returned error: %v", err)
	}
	if embedder == nil {
		t.Fatal("embedder is nil")
	}
}
