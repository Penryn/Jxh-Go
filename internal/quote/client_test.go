package quote_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zjutjh/jxh-go/internal/quote"
)

func TestClientPostsQuotePayload(t *testing.T) {
	var got map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/base64/" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte("base64-image"))
	}))
	defer server.Close()

	client := quote.NewClient(server.URL, server.Client())
	out, err := client.Generate(context.Background(), quote.Payload{MessageID: 123, RawMessage: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if out != "base64-image" || got["message_id"].(float64) != 123 {
		t.Fatalf("out=%q got=%#v", out, got)
	}
}
