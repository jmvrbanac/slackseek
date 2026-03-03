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
	api         *slackgo.Client
	onRateLimit func(time.Duration) // called before sleeping on HTTP 429; may be nil
}

// SetRateLimitCallback registers fn to be called before sleeping on a 429
// response. fn receives the Retry-After duration. Pass nil to clear.
func (c *Client) SetRateLimitCallback(fn func(time.Duration)) {
	c.onRateLimit = fn
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

	transport := base
	if cookie != "" {
		transport = &cookieTransport{cookie: cookie, base: base}
	}

	return &Client{api: slackgo.New(token, slackgo.OptionHTTPClient(&http.Client{Transport: transport}))}
}

// ctxSleep waits for duration d or until ctx is cancelled.
func ctxSleep(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}

// retryOnce executes fn once and returns (shouldRetry, err).
// shouldRetry=true means the caller should loop again (delay already applied).
// shouldRetry=false with nil err means fn succeeded.
func (c *Client) retryOnce(ctx context.Context, fn func() error, boff *backoff.ExponentialBackOff, isLastAttempt bool) (bool, error) {
	err := fn()
	if err == nil {
		return false, nil
	}

	var rateLimited *slackgo.RateLimitedError
	if errors.As(err, &rateLimited) {
		if isLastAttempt {
			return false, fmt.Errorf("rate limited after %d attempts: %w", maxAttempts, err)
		}
		if c.onRateLimit != nil {
			c.onRateLimit(rateLimited.RetryAfter)
		}
		if sleepErr := ctxSleep(ctx, rateLimited.RetryAfter); sleepErr != nil {
			return false, sleepErr
		}
		return true, nil
	}

	var sce slackgo.StatusCodeError
	if errors.As(err, &sce) && sce.Code >= http.StatusInternalServerError {
		if isLastAttempt {
			return false, fmt.Errorf("server error after %d attempts: %w", maxAttempts, err)
		}
		wait := boff.NextBackOff()
		if wait == backoff.Stop {
			return false, fmt.Errorf("server error: back-off exhausted: %w", err)
		}
		if sleepErr := ctxSleep(ctx, wait); sleepErr != nil {
			return false, sleepErr
		}
		return true, nil
	}

	return false, err
}

// callWithRetry executes fn and retries on recoverable Slack API errors:
//   - HTTP 429: honors the Retry-After header duration (max 3 total attempts)
//   - HTTP 5xx: exponential back-off with jitter (max 3 total attempts)
//   - Other errors: returned immediately without retry
func (c *Client) callWithRetry(ctx context.Context, fn func() error) error {
	boff := backoff.NewExponentialBackOff()
	boff.MaxElapsedTime = 0

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		retry, err := c.retryOnce(ctx, fn, boff, attempt+1 >= maxAttempts)
		if err != nil {
			return err
		}
		if !retry {
			return nil
		}
	}
	return nil
}
