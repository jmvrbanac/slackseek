package slack

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/cenkalti/backoff/v4"
	slackgo "github.com/slack-go/slack"
)

// maxAttempts is the total number of attempts (initial + retries) for retryable errors.
const maxAttempts = 3

// Client wraps the slack-go API client with rate-limit retry logic.
type Client struct {
	api *slackgo.Client
}

// cookieTransport wraps an http.RoundTripper and injects the Slack session
// cookie on every request, which is required for xoxc- tokens.
type cookieTransport struct {
	cookie string
	base   http.RoundTripper
}

func (t *cookieTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r2 := r.Clone(r.Context())
	r2.Header.Set("Cookie", "d="+t.cookie)
	return t.base.RoundTrip(r2)
}

// NewClient returns an authenticated Client. When cookie is non-empty it is
// injected as "Cookie: d=<value>" on every request (required for xoxc- tokens
// extracted from the Slack desktop app). Pass a non-nil httpClient to override
// the default HTTP transport (useful in tests).
func NewClient(token, cookie string, httpClient *http.Client) *Client {
	base := http.RoundTripper(http.DefaultTransport)
	if httpClient != nil && httpClient.Transport != nil {
		base = httpClient.Transport
	}

	var transport http.RoundTripper = base
	if cookie != "" {
		transport = &cookieTransport{cookie: cookie, base: base}
	}

	return &Client{api: slackgo.New(token, slackgo.OptionHTTPClient(&http.Client{Transport: transport}))}
}

// callWithRetry executes fn and retries on recoverable Slack API errors:
//   - HTTP 429: honors the Retry-After header duration (max 3 total attempts)
//   - HTTP 5xx: exponential back-off with jitter (max 3 total attempts)
//   - Other errors: returned immediately without retry
func (c *Client) callWithRetry(ctx context.Context, fn func() error) error {
	boff := backoff.NewExponentialBackOff()
	boff.MaxElapsedTime = 0 // disable time-based cap; rely on attempt count only

	var rateLimited *slackgo.RateLimitedError

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		err := fn()
		if err == nil {
			return nil
		}

		isLastAttempt := attempt+1 >= maxAttempts

		// 429: sleep the exact Retry-After duration, then retry.
		if errors.As(err, &rateLimited) {
			if isLastAttempt {
				return fmt.Errorf("rate limited after %d attempts: %w", maxAttempts, err)
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(rateLimited.RetryAfter):
			}
			continue
		}

		// 5xx: exponential back-off, then retry.
		var sce slackgo.StatusCodeError
		if errors.As(err, &sce) && sce.Code >= http.StatusInternalServerError {
			if isLastAttempt {
				return fmt.Errorf("server error after %d attempts: %w", maxAttempts, err)
			}
			wait := boff.NextBackOff()
			if wait == backoff.Stop {
				return fmt.Errorf("server error: back-off exhausted: %w", err)
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
			}
			continue
		}

		// Non-retryable error.
		return err
	}
	return nil
}
