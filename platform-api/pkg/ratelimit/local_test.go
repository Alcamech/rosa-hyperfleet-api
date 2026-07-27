package ratelimit

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestLocalLimiter_AllowsWithinBurst(t *testing.T) {
	lim := NewLocalRateLimiter()
	limit := Limit{Rate: 5, Burst: 5, Period: time.Second}

	for i := 0; i < 5; i++ {
		res, err := lim.Allow(context.Background(), "key1", limit)
		if err != nil {
			t.Fatalf("request %d: unexpected error: %v", i, err)
		}
		if res.Allowed != 1 {
			t.Fatalf("request %d: expected Allowed=1, got %d", i, res.Allowed)
		}
	}
}

func TestLocalLimiter_DeniesOverBurst(t *testing.T) {
	lim := NewLocalRateLimiter()
	limit := Limit{Rate: 3, Burst: 3, Period: time.Second}

	for i := 0; i < 3; i++ {
		res, err := lim.Allow(context.Background(), "key1", limit)
		if err != nil {
			t.Fatalf("request %d: unexpected error: %v", i, err)
		}
		if res.Allowed != 1 {
			t.Fatalf("request %d: expected Allowed=1, got %d", i, res.Allowed)
		}
	}

	res, err := lim.Allow(context.Background(), "key1", limit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Allowed != 0 {
		t.Fatalf("expected Allowed=0 after burst, got %d", res.Allowed)
	}
	if res.RetryAfter <= 0 {
		t.Errorf("expected positive RetryAfter, got %v", res.RetryAfter)
	}
}

func TestLocalLimiter_RemainingDecreases(t *testing.T) {
	lim := NewLocalRateLimiter()
	limit := Limit{Rate: 5, Burst: 5, Period: time.Second}

	var prev = -1
	for i := 0; i < 5; i++ {
		res, err := lim.Allow(context.Background(), "key1", limit)
		if err != nil {
			t.Fatalf("request %d: unexpected error: %v", i, err)
		}
		if prev >= 0 && res.Remaining >= prev {
			t.Errorf("request %d: remaining should decrease, prev=%d cur=%d", i, prev, res.Remaining)
		}
		prev = res.Remaining
	}
}

func TestLocalLimiter_IsolatesKeys(t *testing.T) {
	lim := NewLocalRateLimiter()
	limit := Limit{Rate: 2, Burst: 2, Period: time.Second}

	for i := 0; i < 2; i++ {
		if _, err := lim.Allow(context.Background(), "key-a", limit); err != nil {
			t.Fatalf("key-a request %d: unexpected error: %v", i, err)
		}
	}

	res, err := lim.Allow(context.Background(), "key-a", limit)
	if err != nil {
		t.Fatalf("key-a exhaustion check: unexpected error: %v", err)
	}
	if res.Allowed != 0 {
		t.Fatal("key-a should be exhausted")
	}

	res, err = lim.Allow(context.Background(), "key-b", limit)
	if err != nil {
		t.Fatalf("key-b: unexpected error: %v", err)
	}
	if res.Allowed != 1 {
		t.Fatal("key-b should have its own bucket")
	}
}

func TestLocalLimiter_ResetAfterIsPositive(t *testing.T) {
	lim := NewLocalRateLimiter()
	limit := Limit{Rate: 5, Burst: 5, Period: time.Second}

	res, err := lim.Allow(context.Background(), "key1", limit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ResetAfter <= 0 {
		t.Errorf("expected positive ResetAfter on allowed request, got %v", res.ResetAfter)
	}
}

func TestLocalLimiter_DeniedResultHasZeroRemaining(t *testing.T) {
	lim := NewLocalRateLimiter()
	limit := Limit{Rate: 1, Burst: 1, Period: time.Second}

	if _, err := lim.Allow(context.Background(), "key1", limit); err != nil {
		t.Fatalf("setup: unexpected error: %v", err)
	}

	res, err := lim.Allow(context.Background(), "key1", limit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Allowed != 0 {
		t.Fatal("expected denial")
	}
	if res.Remaining != 0 {
		t.Errorf("expected Remaining=0 on denial, got %d", res.Remaining)
	}
}

func TestLocalLimiter_ConcurrentAccess(t *testing.T) {
	lim := NewLocalRateLimiter()
	limit := Limit{Rate: 10, Burst: 10, Period: time.Second}

	var allowed atomic.Int32
	var denied atomic.Int32
	total := 20

	var wg sync.WaitGroup
	wg.Add(total)
	for i := 0; i < total; i++ {
		go func() {
			defer wg.Done()
			res, err := lim.Allow(context.Background(), "key1", limit)
			if err != nil {
				return
			}
			if res.Allowed == 1 {
				allowed.Add(1)
			} else {
				denied.Add(1)
			}
		}()
	}
	wg.Wait()

	a := int(allowed.Load())
	d := int(denied.Load())
	if a+d != total {
		t.Errorf("allowed(%d) + denied(%d) != total(%d)", a, d, total)
	}
	if a > 10 {
		t.Errorf("expected at most 10 allowed (burst), got %d", a)
	}
	if d == 0 {
		t.Error("expected at least some denials")
	}
}

func TestLocalLimiter_NeverReturnsError(t *testing.T) {
	lim := NewLocalRateLimiter()
	limit := Limit{Rate: 1, Burst: 1, Period: time.Second}

	for i := 0; i < 10; i++ {
		_, err := lim.Allow(context.Background(), "key1", limit)
		if err != nil {
			t.Fatalf("request %d: local limiter should never error, got: %v", i, err)
		}
	}
}
