package ratelimit

import (
	"context"
	"sync"
	"time"

	"github.com/go-redis/redis_rate/v10"
	"github.com/redis/go-redis/v9"
)

type Limit struct {
	Rate   int
	Burst  int
	Period time.Duration
}

type Result struct {
	Allowed    int
	Remaining  int
	RetryAfter time.Duration
	ResetAfter time.Duration
}

type RateLimiter interface {
	Allow(ctx context.Context, key string, limit Limit) (*Result, error)
}

// redisLimiter wraps redis_rate.Limiter for production use with Valkey/Redis.
type redisLimiter struct {
	limiter *redis_rate.Limiter
}

func NewRedisLimiter(rdb *redis.Client) RateLimiter {
	return &redisLimiter{limiter: redis_rate.NewLimiter(rdb)}
}

func (r *redisLimiter) Allow(ctx context.Context, key string, limit Limit) (*Result, error) {
	res, err := r.limiter.Allow(ctx, key, redis_rate.Limit{
		Rate:   limit.Rate,
		Burst:  limit.Burst,
		Period: limit.Period,
	})
	if err != nil {
		return nil, err
	}
	return &Result{
		Allowed:    res.Allowed,
		Remaining:  res.Remaining,
		RetryAfter: res.RetryAfter,
		ResetAfter: res.ResetAfter,
	}, nil
}

// localLimiter implements GCRA in-memory for local testing without Redis.
type localLimiter struct {
	mu      sync.Mutex
	entries map[string]time.Time // key → TAT (Theoretical Arrival Time)
}

func NewLocalRateLimiter() RateLimiter {
	return &localLimiter{entries: make(map[string]time.Time)}
}

func (l *localLimiter) Allow(_ context.Context, key string, limit Limit) (*Result, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	emissionInterval := limit.Period / time.Duration(limit.Rate)
	burstOffset := emissionInterval * time.Duration(limit.Burst)

	tat, exists := l.entries[key]
	if !exists || tat.Before(now) {
		tat = now
	}

	newTAT := tat.Add(emissionInterval)
	allowAt := newTAT.Add(-burstOffset)

	diff := now.Sub(allowAt)
	if diff < 0 {
		retryAfter := allowAt.Sub(now)
		resetAfter := tat.Sub(now)
		if resetAfter < 0 {
			resetAfter = 0
		}
		return &Result{
			Allowed:    0,
			Remaining:  0,
			RetryAfter: retryAfter,
			ResetAfter: resetAfter,
		}, nil
	}

	l.entries[key] = newTAT

	remaining := int(diff / emissionInterval)
	if remaining > limit.Burst {
		remaining = limit.Burst
	}
	resetAfter := newTAT.Sub(now)
	if resetAfter < 0 {
		resetAfter = 0
	}

	l.cleanup(now)

	return &Result{
		Allowed:    1,
		Remaining:  remaining,
		RetryAfter: -1,
		ResetAfter: resetAfter,
	}, nil
}

func (l *localLimiter) cleanup(now time.Time) {
	for key, tat := range l.entries {
		if tat.Before(now) {
			delete(l.entries, key)
		}
	}
}
