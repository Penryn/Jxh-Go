package knowledge

import (
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
	return data, c.save(data)
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
