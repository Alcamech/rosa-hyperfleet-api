# ROSA Hyperfleet API

ROSA HCP regional cluster management — platform API, operator, and backing database library.

| Directory | Description |
| --- | --- |
| `platform-api/` | REST gateway (SigV4 auth, Cedar/AVP authz, ZOA) |
| `hyperfleet-operator/` | Kubernetes operator (Cluster, NodePool, Placement CRDs) |
| `hyperfleet-db/` | PostgreSQL-backed controller-runtime library |
| `test/` | E2E tests (API, CLI, monitoring, ZOA) |

## Quick Start

```bash
make build   # all components → bin/
make test    # all unit tests
make lint    # golangci-lint
make help    # full target list
```

## Module Layout

```
hyperfleet-db/go.mod             ← standalone
api/go.mod                       ← standalone (CRD types)
hyperfleet-operator/go.mod       ← requires: hyperfleet-db, api
platform-api/go.mod              ← requires: hyperfleet-db, api
```

## Docs

- [OpenAPI spec](platform-api/openapi/openapi.yaml)
- [ZOA Trusted Actions](docs/api/zoa-endpoints.md)
- [Authorization](docs/authz.md)
- [Konflux / Quay image tags](docs/konflux/quay-image-tags.md)
