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

// T036: rateLimiter unit tests

func TestRateLimiter_FirstCallIsImmediate(t *testing.T) {
	l := &rateLimiter{interval: 200 * time.Millisecond}
	start := time.Now()
	if err := l.Wait(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Millisecond {
		t.Errorf("first Wait should be immediate, took %v", elapsed)
	}
}

func TestRateLimiter_SecondCallWaitsForInterval(t *testing.T) {
	interval := 100 * time.Millisecond
	l := &rateLimiter{interval: interval}
	_ = l.Wait(context.Background()) // first call — immediate
	start := time.Now()
	if err := l.Wait(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed < interval-5*time.Millisecond {
		t.Errorf("second Wait should block ~%v, blocked only %v", interval, elapsed)
	}
}

func TestRateLimiter_CancelledContextUnblocks(t *testing.T) {
	l := &rateLimiter{interval: 10 * time.Second}
	_ = l.Wait(context.Background()) // consume free first call
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	start := time.Now()
	err := l.Wait(ctx)
	if err == nil {
		t.Fatal("expected context cancellation error, got nil")
	}
	if time.Since(start) > 500*time.Millisecond {
		t.Errorf("Wait should have unblocked quickly after cancel, took %v", time.Since(start))
	}
}

// T041: SetPageFetchedCallback tests

func TestClient_PageFetchedCallbackNilByDefault(t *testing.T) {
	c := &Client{api: slackgo.New("fake-token")}
	if c.pageFetchedFn != nil {
		t.Error("pageFetchedFn should be nil by default")
	}
}

func TestClient_SetPageFetchedCallback_Invoked(t *testing.T) {
	c := &Client{api: slackgo.New("fake-token")}
	var got []int
	c.SetPageFetchedCallback(func(n int) { got = append(got, n) })
	c.pageFetchedFn(10)
	c.pageFetchedFn(20)
	if len(got) != 2 || got[0] != 10 || got[1] != 20 {
		t.Errorf("expected [10 20], got %v", got)
	}
}

func TestClient_SetPageFetchedCallback_ClearWithNil(t *testing.T) {
	c := &Client{api: slackgo.New("fake-token")}
	c.SetPageFetchedCallback(func(_ int) {})
	c.SetPageFetchedCallback(nil)
	if c.pageFetchedFn != nil {
		t.Error("expected pageFetchedFn to be nil after clearing with nil")
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
