package crawler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func shouldKeepImageBySize(ctx context.Context, imageURL string, minBytes int64) (bool, error) {
	if minBytes <= 0 {
		return true, nil
	}

	client := &http.Client{Timeout: 12 * time.Second}

	// Fast path: use HEAD + Content-Length.
	headReq, err := http.NewRequestWithContext(ctx, http.MethodHead, imageURL, nil)
	if err == nil {
		headResp, headErr := client.Do(headReq)
		if headErr == nil {
			defer headResp.Body.Close()
			if length := parseContentLength(headResp.Header.Get("Content-Length")); length >= 0 {
				return length >= minBytes, nil
			}
		}
	}

	// Fallback: GET first few bytes and trust Content-Length when present.
	getReq, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return false, fmt.Errorf("build image request failed: %w", err)
	}
	getReq.Header.Set("Range", "bytes=0-1023")

	resp, err := client.Do(getReq)
	if err != nil {
		return false, fmt.Errorf("fetch image header failed: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))

	if length := parseContentLength(resp.Header.Get("Content-Length")); length >= 0 {
		return length >= minBytes, nil
	}

	// Unknown size, keep it to avoid false negatives.
	return true, nil
}

func parseContentLength(raw string) int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return -1
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return -1
	}
	return v
}
