// Package api is a thin Discord REST API HTTP client.
package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Client is a thin Discord REST API HTTP client.
type Client struct {
	Token      string
	BaseURL    string // overridable for tests; default https://discord.com/api/v10
	HTTP       *http.Client
	MaxRetries int // rate-limit / index-warm-up retries
	// MaxRetryWait caps a single server-requested wait; a longer retry_after
	// fails the call instead of blocking the caller for an unbounded time.
	MaxRetryWait time.Duration
}

// New returns a Client for the given bot token.
func New(token string) *Client {
	return &Client{
		Token:        token,
		BaseURL:      "https://discord.com/api/v10",
		HTTP:         &http.Client{Timeout: 30 * time.Second},
		MaxRetries:   3,
		MaxRetryWait: time.Minute,
	}
}

// Do invokes a Discord REST endpoint. method is an HTTP verb; path starts with
// "/" and may carry a query string. A non-nil body is marshaled as JSON.
// Returns the raw response bytes (empty for 204 No Content).
func (c *Client) Do(method, path string, body any) ([]byte, error) {
	return c.DoSecretPath(method, path, path, body)
}

// DoSecretPath is Do for a path that embeds a credential (a webhook token).
// shown stands in for path in every error, including transport errors that
// would otherwise quote the full request URL.
func (c *Client) DoSecretPath(method, path, shown string, body any) ([]byte, error) {
	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("%s %s: encoding body: %w", method, shown, err)
		}
	}
	return c.roundTrip(method, path, shown, "application/json", func() (io.Reader, error) {
		if payload == nil {
			return nil, nil
		}
		return bytes.NewReader(payload), nil
	})
}

// roundTrip performs the request with rate-limit and search-index retries.
// makeBody is called per attempt so retries resend a fresh reader. shown is the
// path reported in errors (equal to path unless it carries a secret). An empty
// Token sends no Authorization header (webhook execution is token-in-path).
func (c *Client) roundTrip(method, path, shown, contentType string, makeBody func() (io.Reader, error)) ([]byte, error) {
	if err := checkPath(path); err != nil {
		return nil, fmt.Errorf("%s %s: %w", method, shown, err)
	}
	endpoint := c.BaseURL + path

	for attempt := 0; ; attempt++ {
		bodyReader, err := makeBody()
		if err != nil {
			return nil, err
		}
		req, err := http.NewRequest(method, endpoint, bodyReader)
		if err != nil {
			return nil, err
		}
		if bodyReader != nil {
			req.Header.Set("Content-Type", contentType)
		}
		if c.Token != "" {
			req.Header.Set("Authorization", "Bot "+c.Token)
		}

		resp, err := c.HTTP.Do(req)
		if err != nil {
			// A transport error quotes the request URL; keep the secret out.
			var urlErr *url.Error
			if errors.As(err, &urlErr) {
				urlErr.URL = c.BaseURL + shown
			}
			return nil, err
		}
		raw, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("%s %s: reading response body: %w", method, shown, err)
		}

		if wait, retry := retryDelay(resp, raw); retry {
			if attempt < c.MaxRetries && wait <= c.MaxRetryWait {
				time.Sleep(wait)
				continue
			}
			return nil, newAPIError(method, shown, resp.StatusCode, raw)
		}

		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			return nil, newAPIError(method, shown, resp.StatusCode, raw)
		}
		return raw, nil
	}
}

// checkPath refuses a request path that would not reach the endpoint it names:
// dot segments ("..", also percent-encoded) are collapsed server-side and so
// retarget the call, and control characters or a fragment corrupt the request
// line. It is the last line of defense behind per-flag ID validation.
func checkPath(path string) error {
	for _, r := range path {
		if r <= ' ' || r == 0x7f || r == '#' || r == '\\' {
			return errors.New("invalid character in request path")
		}
	}
	route, _, _ := strings.Cut(path, "?")
	for _, seg := range strings.Split(route, "/") {
		if dec, err := url.PathUnescape(seg); err != nil || dec == "." || dec == ".." {
			return errors.New("request path must not contain dot segments")
		}
	}
	return nil
}

// retryDelay reports whether the response is retryable (rate limit, or a
// message-search index that is still warming up) and for how long to wait.
func retryDelay(resp *http.Response, raw []byte) (time.Duration, bool) {
	warming := false
	if resp.StatusCode == http.StatusAccepted {
		var probe struct {
			Code int `json:"code"`
		}
		if json.Unmarshal(raw, &probe) == nil && probe.Code == codeIndexNotReady {
			warming = true
		}
	}
	if resp.StatusCode != http.StatusTooManyRequests && !warming {
		return 0, false
	}

	var body struct {
		RetryAfter float64 `json:"retry_after"`
	}
	if json.Unmarshal(raw, &body) == nil && body.RetryAfter > 0 {
		return time.Duration(body.RetryAfter * float64(time.Second)), true
	}
	if h := resp.Header.Get("Retry-After"); h != "" {
		if secs, err := strconv.ParseFloat(h, 64); err == nil && secs > 0 {
			return time.Duration(secs * float64(time.Second)), true
		}
	}
	return time.Second, true
}
