package quote

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"strings"
	"time"
)

const maxQuoteImageBytes = 5 << 20

var (
	quoteImageHTTPClient = &http.Client{Timeout: 5 * time.Second}
	quoteImageRetryDelay = 200 * time.Millisecond
)

type MessageInput struct {
	UserID     int64
	Nickname   string
	RawMessage string
	Message    any
}

type ImageResolver interface {
	ResolveImage(ctx context.Context, file string) (string, error)
}

func BuildPayload(ctx context.Context, input MessageInput, resolver ImageResolver) (Payload, error) {
	message := input.Message
	if message != nil {
		message = enrichMessageImages(ctx, message, resolver)
	}
	content := ContentFromMessage(input.RawMessage, message)
	return Payload{{
		UserID:       input.UserID,
		UserNickname: Nickname(input.Nickname),
		Message:      content,
	}}, nil
}

func enrichMessageImages(ctx context.Context, raw any, resolver ImageResolver) any {
	segments, ok := decodeOneBotSegments(raw)
	if !ok {
		return raw
	}
	out := make([]map[string]any, 0, len(segments))
	for _, segment := range segments {
		outSegment := map[string]any{
			"type": segment.Type,
			"data": cloneAnyMap(segment.Data),
		}
		if quoteImageSegmentType(segment.Type) {
			outSegment["data"] = enrichImageData(ctx, segment.Data, resolver)
		}
		out = append(out, outSegment)
	}
	return out
}

func enrichImageData(ctx context.Context, data map[string]any, resolver ImageResolver) map[string]any {
	out := cloneAnyMap(data)
	if url := segmentDataString(data, "url"); url != "" {
		if dataURI := quoteImageHTTPDataURI(ctx, url); dataURI != "" {
			out["url"] = dataURI
			return out
		}
		if isUsableImageSource(url) {
			return out
		}
	}
	file := segmentDataString(data, "file")
	if dataURI := quoteImageHTTPDataURI(ctx, file); dataURI != "" {
		out["url"] = dataURI
		return out
	}
	if file == "" || isUsableImageSource(file) {
		return out
	}
	if resolver == nil {
		return out
	}
	resolved, err := resolver.ResolveImage(ctx, file)
	if err != nil || strings.TrimSpace(resolved) == "" {
		return out
	}
	out["url"] = resolved
	return out
}

func quoteImageHTTPDataURI(ctx context.Context, source string) string {
	if !httpImageSource(source) {
		return ""
	}
	for attempt := 0; attempt < 3; attempt++ {
		dataURI, ok := fetchQuoteImageDataURI(ctx, source)
		if ok {
			return dataURI
		}
		if attempt == 2 || quoteImageRetryDelay <= 0 {
			continue
		}
		timer := time.NewTimer(quoteImageRetryDelay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ""
		case <-timer.C:
		}
	}
	return ""
}

func fetchQuoteImageDataURI(ctx context.Context, source string) (string, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if err != nil {
		return "", false
	}
	resp, err := quoteImageHTTPClient.Do(req)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", false
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxQuoteImageBytes+1))
	if err != nil || len(data) == 0 || len(data) > maxQuoteImageBytes {
		return "", false
	}
	contentType := strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0])
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}
	if !strings.HasPrefix(strings.ToLower(contentType), "image/") {
		return "", false
	}
	return "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(data), true
}

func quoteImageSegmentType(segmentType string) bool {
	switch segmentType {
	case "image", "mface", "marketface", "emoji":
		return true
	default:
		return false
	}
}

func httpImageSource(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
}

func cloneAnyMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
