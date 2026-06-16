package knowledge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type WPSClient struct {
	ShareURL  string
	SID       string
	CacheFile string
	HTTP      *http.Client
}

func (c WPSClient) Download(ctx context.Context) ([]byte, error) {
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.ShareURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	if c.SID != "" {
		req.AddCookie(&http.Cookie{Name: "wps_sid", Value: c.SID})
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("wps share request failed: %s", resp.Status)
	}
	var meta struct {
		DownloadURL string `json:"download_url"`
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(body, &meta); err != nil || meta.DownloadURL == "" {
		if err := ensureXLSX(body); err != nil {
			return nil, err
		}
		return body, c.save(body)
	}
	req, err = http.NewRequestWithContext(ctx, http.MethodGet, meta.DownloadURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err = client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("wps download failed: %s", resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if err := ensureXLSX(data); err != nil {
		return nil, err
	}
	return data, c.save(data)
}

func ensureXLSX(data []byte) error {
	if len(data) >= 4 && bytes.Equal(data[:4], []byte{'P', 'K', 0x03, 0x04}) {
		return nil
	}
	preview := string(bytes.TrimSpace(data))
	if len(preview) > 120 {
		preview = preview[:120]
	}
	return fmt.Errorf("wps download is not an xlsx file; share_url must be a WPS 导出文档链接 or direct xlsx URL, and protected documents need a valid wps_sid; normal 365.kdocs.cn/l share pages return HTML, response preview: %q", preview)
}

func (c WPSClient) save(data []byte) error {
	if c.CacheFile == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(c.CacheFile), 0o755); err != nil {
		return err
	}
	return os.WriteFile(c.CacheFile, data, 0o644)
}
