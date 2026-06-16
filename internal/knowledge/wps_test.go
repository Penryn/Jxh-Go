package knowledge_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zjutjh/jxh-go/internal/knowledge"
)

func TestWPSDownloadRejectsHTMLLoginPage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<!DOCTYPE html><html>login</html>"))
	}))
	defer server.Close()

	client := knowledge.WPSClient{ShareURL: server.URL, HTTP: server.Client()}
	_, err := client.Download(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "not an xlsx") {
		t.Fatalf("error = %v", err)
	}
}

func TestWPSDownloadExplainsNormalSharePageIsNotExportLink(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<!DOCTYPE html><html><head><script>window.__WPSENV__={}</script></head></html>"))
	}))
	defer server.Close()

	client := knowledge.WPSClient{ShareURL: server.URL, HTTP: server.Client()}
	_, err := client.Download(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "导出文档链接") {
		t.Fatalf("error should mention export document link, got %v", err)
	}
}
