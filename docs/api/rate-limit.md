# Rate Limiting

Per-account rate limiting for the ROSA Regional Platform API using the GCRA (Generic Cell Rate Algorithm).

## How It Works

Every request is keyed by `rate:rl:{account_id}:{method}:{path}` (the `rate:` prefix is added by the `redis_rate` library). The middleware extracts the AWS account ID from the `X-Amz-Account-Id` header, looks up the matching route limit (or falls back to the default), and calls the rate limiter. Requests without an account ID are not rate limited.

The algorithm is GCRA — a leaky-bucket variant that smoothly distributes requests over time rather than allowing the full budget to be consumed in a burst at the start of a window.

### Fail-open

If the backing store (Valkey/Redis) is unreachable or returns an error, the request is **allowed through** and a `failure_mode_allowed` metric is emitted. The fail-open timeout is controlled by `redisTimeout` (default 20ms).

### Exempt accounts

Accounts listed in `exemptAccounts` bypass rate limiting entirely.

## Configuration

Rate limits are configured via a YAML file mounted as a ConfigMap at `/etc/ratelimit/limits.yaml` (override with `RATE_LIMIT_CONFIG_FILE`).

```yaml
enabled: true

redisTimeout: 20 # ms before fail-open on backend error

exemptAccounts:
  - "111111111111"
  - "222222222222"

default:
  rate: 100 # requests per window
  burst: 200 # max burst (spike allowance)
  window: 1 # window duration in seconds

routes:
  - path: "/api/v0/clusters"
    method: POST
    rate: 5
    burst: 10
    window: 1
  - path: "/api/v0/clusters/*"
    method: GET
    rate: 50
    burst: 100
    window: 1
```

### Defaults (when omitted)

| Field          | Default    |
| -------------- | ---------- |
| `rate`         | 100        |
| `burst`        | `rate * 2` |
| `window`       | 1 (second) |
| `redisTimeout` | 20 (ms)    |

Route-level `burst` defaults to `rate * 2` and `window` inherits from `default.window` if not set.

### Environment variables

| Variable                    | Description                                                         |
| --------------------------- | ------------------------------------------------------------------- |
| `RATE_LIMIT_ENABLED`        | Set to `true` to enable rate limiting                               |
| `RATE_LIMIT_TEST_MODE`      | Set to `true` for test mode (rate=3, burst=6, window=1s, in-memory) |
| `RATE_LIMIT_CONFIG_FILE`    | Path to limits YAML (default `/etc/ratelimit/limits.yaml`)          |
| `RATE_LIMIT_IN_MEMORY`      | Set to `true` to use in-memory GCRA instead of Redis                |
| `REDIS_ENDPOINT`            | Valkey/Redis address (required when not in-memory)                  |
| `RATE_LIMIT_DEFAULT_RATE`   | Override default rate                                               |
| `RATE_LIMIT_DEFAULT_BURST`  | Override default burst                                              |
| `RATE_LIMIT_DEFAULT_WINDOW` | Override default window                                             |

When `RATE_LIMIT_CONFIG_FILE` is set, the YAML file is loaded and `RATE_LIMIT_DEFAULT_*` env vars are ignored. When no config file is set, `NewDefaultConfig()` is used with `RATE_LIMIT_DEFAULT_*` overrides applied.

## Test Mode

Run rate limiting locally without Redis/Valkey:

```bash
cd platform-api
RATE_LIMIT_TEST_MODE=true go run ./cmd/... serve
```

Test mode uses an in-memory GCRA implementation. It explicitly sets `rate=3`, `burst=6`, `window=1s` — all three values are hardcoded, not derived from production defaults.

## Response Headers

All rate-limited requests include:

| Header                  | Description                              |
| ----------------------- | ---------------------------------------- |
| `X-RateLimit-Limit`     | Configured rate for the matched route    |
| `X-RateLimit-Remaining` | Remaining requests in the current window |
| `X-RateLimit-Reset`     | Seconds until the limit resets           |

Denied requests (429) additionally include:

| Header        | Description                                    |
| ------------- | ---------------------------------------------- |
| `Retry-After` | Seconds until the next request will be allowed |

### 429 Response Body

```json
{
  "kind": "Error",
  "code": "429",
  "reason": "429 Too Many Requests — POST /api/v0/clusters, 5 req/1s, retry after 1s"
}
```

## Observability

### Prometheus metric

```
ratelimit_requests_total{method, path, result}
```

| `result` label         | Meaning                                    |
| ---------------------- | ------------------------------------------ |
| `ok`                   | Request allowed                            |
| `over_limit`           | Request denied (429)                       |
| `failure_mode_allowed` | Backend error, request allowed (fail-open) |

The `path` label is the matched route pattern (e.g. `/api/v0/clusters`) or `"default"` for unmatched routes.

### Denial logging

Every denied request logs at WARN level:

```json
{
  "msg": "rate limit exceeded",
  "account_id": "123456789012",
  "method": "POST",
  "path": "/api/v0/clusters",
  "rate": 5,
  "retry_after": 1
}
```

## Architecture

```
platform-api/pkg/ratelimit/
  config.go                 # Config struct, YAML loader, defaults
  middleware.go             # HTTP middleware, metrics, response writer
  local.go                  # RateLimiter interface, Redis adapter, in-memory GCRA
  match.go                  # Wildcard path matching
```

The `RateLimiter` interface abstracts the backend:

- **`NewRedisLimiter(rdb)`** — wraps `go-redis/redis_rate/v10` for production (Valkey/ElastiCache)
- **`NewLocalRateLimiter()`** — in-memory GCRA for test mode

## Running Tests

```bash
cd platform-api
go test -race -count=1 ./pkg/ratelimit/...
```

Tests use `miniredis` for the Redis backend and cover: allow/deny behavior, per-account isolation, route overrides, method differentiation, fail-open, concurrent requests, Prometheus metrics, and the in-memory limiter.
