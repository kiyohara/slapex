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
	baseURL    string
	httpClient *http.Client
	lastCall   map[string]time.Time
	// sleep performs pacing and retry waits. Tests replace it with a fake
	// that records the requested durations without sleeping.
	sleep func(context.Context, time.Duration) error
	// Logf reports progress such as rate limit waits. Defaults to a no-op.
	Logf func(format string, args ...any)
}

// Option customizes a Client.
type Option func(*Client)

// WithBaseURL points the client at an alternate Slack Web API base URL.
func WithBaseURL(baseURL string) Option {
	return func(c *Client) {
		if !strings.HasSuffix(baseURL, "/") {
			baseURL += "/"
		}
		c.baseURL = baseURL
	}
}

// WithSleeper replaces request pacing and retry sleeps.
func WithSleeper(sleep func(context.Context, time.Duration) error) Option {
	return func(c *Client) {
		c.sleep = sleep
	}
}

// New creates a Slack Web API client for token.
func New(token string, opts ...Option) *Client {
	c := &Client{
		token:      token,
		baseURL:    apiBase,
		httpClient: &http.Client{Timeout: 120 * time.Second},
		lastCall:   map[string]time.Time{},
		sleep:      sleepCtx,
		Logf:       func(string, ...any) {},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

type apiEnvelope struct {
	OK               bool   `json:"ok"`
	Error            string `json:"error"`
	ResponseMetadata struct {
		NextCursor string `json:"next_cursor"`
	} `json:"response_metadata"`
}

func (c *Client) pace(ctx context.Context, key string) error {
	if last, ok := c.lastCall[key]; ok {
		if wait := methodPace - time.Since(last); wait > 0 {
			if err := c.sleep(ctx, wait); err != nil {
				return err
			}
		}
	}
	c.lastCall[key] = time.Now()
	return nil
}

// call POSTs a form-encoded Web API request and decodes the body into out.
func (c *Client) call(ctx context.Context, method string, params url.Values, out any) (string, error) {
	if err := c.pace(ctx, method); err != nil {
		return "", fmt.Errorf("slack api %s: %w", method, err)
	}
	body, err := c.withRetry(ctx, "api "+method, func() (*http.Response, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+method,
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
	// skipBackoff is set when a 429 already waited out Retry-After; the wait
	// happens at detection so it is honoured even when retries are exhausted.
	skipBackoff := false
	for attempt := 0; ; attempt++ {
		if attempt > maxRetries {
			return nil, fmt.Errorf("giving up after %d retries: %w", maxRetries, lastErr)
		}
		if attempt > 0 && !skipBackoff {
			wait := backoffWait(attempt)
			c.Logf("retrying %s in %s (%s)", what, wait.Round(time.Second), lastErr)
			if err := c.sleep(ctx, wait); err != nil {
				return nil, err
			}
		}
		skipBackoff = false
		resp, err := doReq()
		if err != nil {
			lastErr = err
			continue
		}
		switch {
		case resp.StatusCode == http.StatusTooManyRequests:
			resp.Body.Close()
			lastErr = fmt.Errorf("rate limited (429)")
			if wait, ok := retryAfter(resp); ok {
				c.Logf("rate limited on %s, waiting %s as instructed by Slack", what, wait.Round(time.Second))
				if err := c.sleep(ctx, wait); err != nil {
					return nil, err
				}
				skipBackoff = true
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

// backoffWait is the exponential backoff before the given retry attempt
// (1-based): 1s, 2s, 4s, ... capped at maxBackoff, plus up to 1s of jitter.
func backoffWait(attempt int) time.Duration {
	wait := min(time.Duration(1<<(attempt-1))*time.Second, maxBackoff)
	return wait + time.Duration(rand.Int63n(int64(time.Second)))
}

// retryAfter parses the Retry-After header, adding up to 1s of jitter. It
// reports false when the header is missing or unusable; the caller then
// falls back to exponential backoff.
func retryAfter(resp *http.Response) (time.Duration, bool) {
	if v := resp.Header.Get("Retry-After"); v != "" {
		if secs, err := strconv.Atoi(v); err == nil && secs > 0 {
			return time.Duration(secs)*time.Second + time.Duration(rand.Int63n(int64(time.Second))), true
		}
	}
	return 0, false
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
	if err := c.pace(ctx, "download"); err != nil {
		return 0, "", err
	}
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
	// skipBackoff: see withRetry.
	skipBackoff := false
	for attempt := 0; ; attempt++ {
		if attempt > maxRetries {
			return nil, "", fmt.Errorf("giving up after %d retries: %w", maxRetries, lastErr)
		}
		if attempt > 0 && !skipBackoff {
			wait := backoffWait(attempt)
			c.Logf("retrying download in %s (%s)", wait.Round(time.Second), lastErr)
			if err := c.sleep(ctx, wait); err != nil {
				return nil, "", err
			}
		}
		skipBackoff = false
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
			resp.Body.Close()
			lastErr = fmt.Errorf("rate limited (429)")
			if wait, ok := retryAfter(resp); ok {
				c.Logf("rate limited on download, waiting %s as instructed by Slack", wait.Round(time.Second))
				if err := c.sleep(ctx, wait); err != nil {
					return nil, "", err
				}
				skipBackoff = true
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
