package quote

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Payload struct {
	MessageID  int64  `json:"message_id"`
	RawMessage string `json:"raw_message"`
	Nickname   string `json:"nickname,omitempty"`
	AvatarURL  string `json:"avatar_url,omitempty"`
}

type Client struct {
	baseURL string
	client  *http.Client
}

func NewClient(baseURL string, client *http.Client) *Client {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), client: client}
}

func (c *Client) Generate(ctx context.Context, payload Payload) (string, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/base64/", bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("quote server returned %s: %s", resp.Status, string(body))
	}
	return string(body), nil
}
