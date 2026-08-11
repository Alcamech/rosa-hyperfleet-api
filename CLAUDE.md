# CLAUDE.md

## Project Overview

ROSA Hyperfleet API — ROSA HCP regional cluster management.

Three components:

- **platform-api/** — Stateless REST gateway (SigV4 auth, Cedar/AVP authz, ZOA)
- **hyperfleet-operator/** — Kubernetes operator (Cluster, NodePool, Placement, ManagementCluster, Manifest CRDs)
- **hyperfleet-db/** — PostgreSQL-backed controller-runtime library

## Build & Test

```bash
make build              # All components
make test               # All unit tests
make lint               # golangci-lint v2 across all modules
make verify             # go.mod tidiness
make deps               # Download and tidy all modules

make build-api          # Platform API
make build-operator     # Fleet operator (manager + compactor)
make build-hyperfleet-db      # FleetDB library

make test-api           # API unit tests
make test-operator      # Operator unit tests
make test-hyperfleet-db       # FleetDB unit tests
make test-operator-int  # Operator integration tests (Postgres + DynamoDB)

make manifests          # Generate CRDs (controller-gen)
make generate           # Generate deepcopy

make codegen            # Full codegen pipeline (openapi, passthrough, conversion)
make generate-clientset # Regenerate typed clientset from CRD types
make generate-openapi   # Regenerate OpenAPI spec from CRD types

make verify-codegen     # Verify codegen output is up to date
make verify-clientset   # Verify clientset matches committed files
make verify-openapi     # Verify OpenAPI spec is up to date

make test-unit          # All unit tests (api, operator, codegen, clientset)
make test-integration   # Integration tests (fleetdb, operator)
make test-api-codegen   # Codegen tool tests
make test-clientset     # Clientset tests
```

## Module Layout

```
hyperfleet-db/go.mod                    ← standalone
api/go.mod                              ← standalone (CRD types, v1alpha1)
clientset/go.mod                        ← generated typed K8s client for HyperFleet CRDs
hyperfleet-operator/go.mod              ← requires: fleetdb, api
platform-api/go.mod                     ← requires: fleetdb, api
hack/api-codegen/go.mod                 ← codegen tools (openapi-gen, crd-variants, conversion-gen)
hack/clientset/cmd/bridge-gen/go.mod      ← wire generation for clientset
hack/tools/go.mod                       ← dev tooling dependencies
```

Cross-module refs use permanent `replace` directives to sibling dirs.

## Key Conventions

- Multi-module monorepo: separate go.mod per component
- Ginkgo/Gomega for testing
- OpenAPI-first API design
- CRD types in standalone `api/` module, imported by hyperfleet-operator and platform-api
- golangci-lint v2 with custom logcheck plugin
