# Rate Limiting — Testing Requirements & Process

## Testing Layers

| Layer | Backend | Files | Validates |
|---|---|---|---|
| **Unit — middleware** | miniredis | `pkg/ratelimit/middleware_test.go` | HTTP middleware behavior with Redis adapter |
| **Unit — GCRA algorithm** | in-memory GCRA | `pkg/ratelimit/local_rate_limiter_test.go` | GCRA math used by CLI test mode |
| **E2E** | real Valkey (ElastiCache) | `test/e2e-api/ratelimit_e2e_test.go` | Full path: API Gateway -> middleware -> Valkey -> response |

## Unit Tests — Middleware (`middleware_test.go`)

Uses [miniredis](https://github.com/alicebob/miniredis) as an in-process Redis mock. Tests the middleware through the `NewRedisLimiter` adapter — the same code path used in production with Valkey/ElastiCache.

### Scenarios covered

| Test | What it validates |
|---|---|
| `SetsRateLimitHeadersOnAllowedRequests` | `X-RateLimit-Limit`, `Remaining`, `Reset` headers present |
| `AllowsRequestsUnderLimit` | Requests within burst all return 200 |
| `DeniesRequestsOverLimit` | Request over burst returns 429 + `Retry-After` |
| `IsolatesRateLimitsByAccount` | Different accounts have independent GCRA buckets |
| `SkipsWhenNoAccountID` | No account ID -> no rate limiting, no headers |
| `SkipsExemptAccounts` | Exempt accounts bypass rate limiting; non-exempt accounts are limited |
| `AppliesRouteOverrides` | Route-specific rate/burst override default; other methods unaffected |
| `DifferentiatesHTTPMethods` | GET and POST have separate limit buckets |
| `FailOpenWhenRedisDown` | Returns 200 when Redis is unreachable (fail-open) |
| `RecoverAfterWindow` | Requests allowed again after the GCRA window resets |
| `DisabledConfig` | `enabled: false` disables all rate limiting |
| `429ResponseFormat` | 429 body: `kind=Error`, `code=429`, reason with method/path |
| `KeyStructure` | Redis key format is `rate:rl:{account}:{method}:{path}` |
| `ConcurrentRequests` | Thread-safe under concurrent access |
| `PrometheusMetrics_*` | `ratelimit_requests_total` counter with `ok`/`over_limit`/`failure_mode_allowed` labels |

### Running

```bash
go test -race -count=1 -v ./pkg/ratelimit/...
```

## Unit Tests — GCRA Algorithm (`local_rate_limiter_test.go`)

Tests the in-memory GCRA implementation directly via the `RateLimiter.Allow()` interface — no HTTP middleware wrapping. This is the same algorithm used by `RATE_LIMIT_TEST_MODE=true` for CLI and manual testing.

### Scenarios covered

| Test | What it validates |
|---|---|
| `AllowsWithinBurst` | Requests within burst all return `Allowed=1` |
| `DeniesOverBurst` | Request over burst returns `Allowed=0` with positive `RetryAfter` |
| `RemainingDecreases` | `Remaining` count decreases with each allowed request |
| `IsolatesKeys` | Different keys have independent GCRA state |
| `ResetAfterIsPositive` | `ResetAfter` is positive on allowed requests |
| `DeniedResultHasZeroRemaining` | Denied requests have `Remaining=0` |
| `ConcurrentAccess` | Thread-safe under goroutine contention |
| `NeverReturnsError` | In-memory limiter never returns an error (no fail-open needed) |

### Running

```bash
go test -race -count=1 -v -run TestLocalLimiter ./pkg/ratelimit/...
```

## CLI / Manual Testing

Start the API locally with rate limiting in test mode — no Redis or Valkey required:

```bash
RATE_LIMIT_TEST_MODE=true go run ./cmd/rosa-regional-platform-api serve
```

This uses the in-memory GCRA with low defaults:
- **rate**: 3 requests per second
- **burst**: 6 (auto-derived: `rate * 2`)
- **window**: 1 second

Test with curl:

```bash
# Should succeed (within burst of 6)
seq 6 | xargs -I {} -P 0 curl -s -o /dev/null -w "%{http_code}\n" http://localhost:8000/api/v0/clusters -H "X-Amz-Account-Id: 123456789012"

# 7th request should return 429
seq 7 | xargs -I {} -P 0 curl -s -o /dev/null -w "%{http_code}\n" http://localhost:8000/api/v0/clusters -H "X-Amz-Account-Id: 123456789012"
```

## E2E Tests (`ratelimit_e2e_test.go`)

Run against a deployed API backed by real Valkey in ElastiCache. The test auto-discovers the server's rate limit from the `X-RateLimit-Limit` response header, so no rate configuration needs to be passed to the test runner.

When `RATE_LIMIT_TEST_MODE=true` is set on the **server** (not the test runner), the API uses low defaults (rate=3, burst=6, window=1s) for faster, more predictable E2E testing. The test runner can also set `RATE_LIMIT_TEST_MODE=true` to assume rate=3 without auto-discovery.

Tests use concurrent goroutines with a start-gate channel pattern to fire requests simultaneously, overwhelming the GCRA burst before token replenishment (sequential requests over the network are too slow at ~300ms each).

### Environment variables

| Variable | Required | Default | Description |
|---|---|---|---|
| `E2E_BASE_URL` | Yes | — | Deployed API URL |
| `E2E_ACCOUNT_ID` | No | AWS STS caller identity | Account ID for rate-limited requests |
| `RATE_LIMIT_TEST_MODE` | No | — | If `true`, assumes rate=3; otherwise auto-discovers from `X-RateLimit-Limit` header |
| `E2E_EXEMPT_ACCOUNT_ID` | No | — | Exempt account to test; test skipped if unset |

### Scenarios covered

| Test | What it validates |
|---|---|
| Headers on allowed requests | `X-RateLimit-Limit`, `Remaining`, `Reset` present with valid values |
| 429 when over limit | Concurrent requests exceed burst, at least one gets 429 |
| 429 response body + Retry-After | `kind=Error`, `code=429`, `reason` contains "Too Many Requests" |
| Exempt accounts bypass rate limiting | Exempt account never gets 429 (skipped if `E2E_EXEMPT_ACCOUNT_ID` unset) |
| Per-account isolation | Exhausting one account's limit doesn't affect a different account |
| Rate limit resets after window | After 429, waiting for reset window allows requests again |

### Running

```bash
# All E2E tests
make test-e2e-api

# Rate limit tests only
make test-e2e-api E2E_LABEL_FILTER=ratelimit

# With test mode (assumes rate=3, skips header auto-discovery)
RATE_LIMIT_TEST_MODE=true make test-e2e-api E2E_LABEL_FILTER=ratelimit
```

## Test Mode Configuration (`RATE_LIMIT_TEST_MODE`)

`RATE_LIMIT_TEST_MODE=true` sets low rate limit defaults for testability. The backend depends on whether `REDIS_ENDPOINT` is also set:

| `RATE_LIMIT_TEST_MODE` | `REDIS_ENDPOINT` | Backend | Use case |
|---|---|---|---|
| `true` | unset | In-memory GCRA | CLI / local development |
| `true` | set | Real Valkey | E2E testing in deployed environments |
| `false` | set | Real Valkey | Production |

Test mode defaults: `rate=3`, `burst=6`, `window=1s`.

## Scenario Coverage Matrix

| Scenario | Unit (middleware) | Unit (GCRA) | E2E |
|---|---|---|---|
| Requests under limit allowed | x | x | x |
| Requests over limit denied (429) | x | x | x |
| Rate limit headers present | x | | x |
| 429 response body format | x | | x |
| Retry-After header | x | x | x |
| No account ID skips rate limiting | x | | |
| Exempt accounts bypass | x | | x |
| Per-account isolation | x | x | x |
| Rate limit resets after window | x | | x |
| Fail-open on backend error | x | | |
| Concurrent request safety | x | x | x |
| Route-specific overrides | x | | |
| HTTP method differentiation | x | | |
| Prometheus metrics | x | | |
| Redis key structure | x | | |
| Disabled config | x | | |
| Remaining decreases | | x | |
| Never returns error | | x | |
