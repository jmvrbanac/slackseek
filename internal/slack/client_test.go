package slack

import (
	"context"
	"testing"
	"time"

	slackgo "github.com/slack-go/slack"
)

// TestCallWithRetry_RateLimited_RetriesOnce verifies that a single 429 response
// causes exactly one retry after the Retry-After duration.
func TestCallWithRetry_RateLimited_RetriesOnce(t *testing.T) {
	c := &Client{api: slackgo.New("fake-token")}
	calls := 0
	start := time.Now()

	err := c.callWithRetry(context.Background(), func() error {
		calls++
		if calls == 1 {
			return &slackgo.RateLimitedError{RetryAfter: 50 * time.Millisecond}
		}
		return nil
	})

	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("expected nil error after retry, got: %v", err)
	}
	if calls != 2 {
		t.Errorf("expected exactly 2 calls (1 retry), got %d", calls)
	}
	if elapsed < 50*time.Millisecond {
		t.Errorf("expected at least 50ms delay for Retry-After, got %v", elapsed)
	}
}

// TestCallWithRetry_ThreeConsecutive500s verifies that three 5xx errors trigger
// exponential backoff and the final error is returned after maxAttempts calls.
func TestCallWithRetry_ThreeConsecutive500s(t *testing.T) {
	c := &Client{api: slackgo.New("fake-token")}
	calls := 0

	err := c.callWithRetry(context.Background(), func() error {
		calls++
		return slackgo.StatusCodeError{Code: 500, Status: "Internal Server Error"}
	})

	if err == nil {
		t.Fatal("expected error after 3 failed attempts, got nil")
	}
	if calls != maxAttempts {
		t.Errorf("expected %d calls (maxAttempts), got %d", maxAttempts, calls)
	}
}

// TestCallWithRetry_SuccessOnFirstAttempt verifies that a successful fn is not retried.
func TestCallWithRetry_SuccessOnFirstAttempt(t *testing.T) {
	c := &Client{api: slackgo.New("fake-token")}
	calls := 0

	err := c.callWithRetry(context.Background(), func() error {
		calls++
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 call (no retry), got %d", calls)
	}
}
