// Package slack is a thin Slack Web API client covering only the methods
// slapex needs (doc/design/slack-api-usage.md). It implements the rate limit
// policy from decision log 0025: honour 429 + Retry-After, exponential
// backoff for transient failures, at most 5 retries per request, and
// self-pacing of roughly 1 request/sec per method.
package slack

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const apiBase = "https://slack.com/api/"

const (
	maxRetries = 5
	maxBackoff = 60 * time.Second
	methodPace = 1 * time.Second
	pageLimit  = 200
)

// APIError is a Slack-level failure (HTTP 200 with ok: false).
type APIError struct {
	Method string
	Code   string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("slack api %s failed: %s", e.Method, e.Code)
}

// ErrTooLarge is returned by Download when the body exceeds the given limit.
var ErrTooLarge = errors.New("download exceeds size limit")

type Client struct {
	token      string
	httpClient *http.Client
	lastCall   map[string]time.Time
	// Logf reports progress such as rate limit waits. Defaults to a no-op.
	Logf func(format string, args ...any)
}

func New(token string) *Client {
	return &Client{
		token:      token,
		httpClient: &http.Client{Timeout: 120 * time.Second},
		lastCall:   map[string]time.Time{},
		Logf:       func(string, ...any) {},
	}
}

type apiEnvelope struct {
	OK               bool   `json:"ok"`
	Error            string `json:"error"`
	ResponseMetadata struct {
		NextCursor string `json:"next_cursor"`
	} `json:"response_metadata"`
}

func (c *Client) pace(key string) {
	if last, ok := c.lastCall[key]; ok {
		if wait := methodPace - time.Since(last); wait > 0 {
			time.Sleep(wait)
		}
	}
	c.lastCall[key] = time.Now()
}

// call POSTs a form-encoded Web API request and decodes the body into out.
func (c *Client) call(ctx context.Context, method string, params url.Values, out any) (string, error) {
	c.pace(method)
	body, err := c.withRetry(ctx, "api "+method, func() (*http.Response, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBase+method,
			strings.NewReader(params.Encode()))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Authorization", "Bearer "+c.token)
		return c.httpClient.Do(req)
	})
	if err != nil {
		return "", fmt.Errorf("slack api %s: %w", method, err)
	}
	var env apiEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return "", fmt.Errorf("slack api %s: decode response: %w", method, err)
	}
	if !env.OK {
		return "", &APIError{Method: method, Code: env.Error}
	}
	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			return "", fmt.Errorf("slack api %s: decode payload: %w", method, err)
		}
	}
	return env.ResponseMetadata.NextCursor, nil
}

// withRetry runs doReq honouring 429 + Retry-After and retrying transient
// failures (5xx, network errors) with exponential backoff and jitter.
func (c *Client) withRetry(ctx context.Context, what string, doReq func() (*http.Response, error)) ([]byte, error) {
	var lastErr error
	for attempt := 0; ; attempt++ {
		if attempt > maxRetries {
			return nil, fmt.Errorf("giving up after %d retries: %w", maxRetries, lastErr)
		}
		if attempt > 0 {
			backoff := min(time.Duration(1<<(attempt-1))*time.Second, maxBackoff)
			backoff += time.Duration(rand.Int63n(int64(time.Second)))
			c.Logf("retrying %s in %s (%s)", what, backoff.Round(time.Second), lastErr)
			if err := sleepCtx(ctx, backoff); err != nil {
				return nil, err
			}
		}
		resp, err := doReq()
		if err != nil {
			lastErr = err
			continue
		}
		switch {
		case resp.StatusCode == http.StatusTooManyRequests:
			wait := retryAfter(resp)
			resp.Body.Close()
			lastErr = fmt.Errorf("rate limited (429)")
			c.Logf("rate limited on %s, waiting %s as instructed by Slack", what, wait.Round(time.Second))
			if err := sleepCtx(ctx, wait); err != nil {
				return nil, err
			}
			continue
		case resp.StatusCode >= 500:
			resp.Body.Close()
			lastErr = fmt.Errorf("server error: HTTP %d", resp.StatusCode)
			continue
		case resp.StatusCode != http.StatusOK:
			defer resp.Body.Close()
			return nil, fmt.Errorf("unexpected HTTP %d", resp.StatusCode)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}
		return body, nil
	}
}

func retryAfter(resp *http.Response) time.Duration {
	if v := resp.Header.Get("Retry-After"); v != "" {
		if secs, err := strconv.Atoi(v); err == nil && secs > 0 {
			return time.Duration(secs)*time.Second + time.Duration(rand.Int63n(int64(time.Second)))
		}
	}
	return 5 * time.Second
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// Download fetches an asset URL (files.slack.com etc.) with the bot token.
// When limit > 0 and the body exceeds it, ErrTooLarge is returned.
func (c *Client) Download(ctx context.Context, srcURL string, limit int64, w io.Writer) (written int64, contentType string, err error) {
	c.pace("download")
	body, ct, err := c.downloadRetry(ctx, srcURL)
	if err != nil {
		return 0, "", err
	}
	defer body.Close()
	reader := io.Reader(body)
	if limit > 0 {
		reader = io.LimitReader(body, limit+1)
	}
	written, err = io.Copy(w, reader)
	if err != nil {
		return written, ct, err
	}
	if limit > 0 && written > limit {
		return written, ct, ErrTooLarge
	}
	return written, ct, nil
}

func (c *Client) downloadRetry(ctx context.Context, srcURL string) (io.ReadCloser, string, error) {
	var lastErr error
	for attempt := 0; ; attempt++ {
		if attempt > maxRetries {
			return nil, "", fmt.Errorf("giving up after %d retries: %w", maxRetries, lastErr)
		}
		if attempt > 0 {
			backoff := min(time.Duration(1<<(attempt-1))*time.Second, maxBackoff)
			if err := sleepCtx(ctx, backoff); err != nil {
				return nil, "", err
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, srcURL, nil)
		if err != nil {
			return nil, "", err
		}
		req.Header.Set("Authorization", "Bearer "+c.token)
		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		switch {
		case resp.StatusCode == http.StatusTooManyRequests:
			wait := retryAfter(resp)
			resp.Body.Close()
			lastErr = fmt.Errorf("rate limited (429)")
			c.Logf("rate limited on download, waiting %s", wait.Round(time.Second))
			if err := sleepCtx(ctx, wait); err != nil {
				return nil, "", err
			}
			continue
		case resp.StatusCode >= 500:
			resp.Body.Close()
			lastErr = fmt.Errorf("server error: HTTP %d", resp.StatusCode)
			continue
		case resp.StatusCode != http.StatusOK:
			resp.Body.Close()
			return nil, "", fmt.Errorf("unexpected HTTP %d", resp.StatusCode)
		}
		return resp.Body, resp.Header.Get("Content-Type"), nil
	}
}
