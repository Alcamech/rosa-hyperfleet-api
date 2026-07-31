# api-codegen

![Coverage](https://img.shields.io/badge/coverage-31.3%25-yellow)

Build-time code generators for the ROSA Hyperfleet API. These tools scan Go types with markers and generate passthrough types, OpenAPI schemas, CRD variants, conversion functions, and field validation metadata.

## Design

This tooling follows the same patterns as the GCP HCP **gecko/orlop** codegen:

- **JSON roundtrip conversions** — `conversion-gen` uses `json.Marshal` / `json.Unmarshal` to project CRD types into REST types (and back), rather than hand-writing per-field assignment code. Hidden fields drop automatically because REST types omit them. Service-set fields are enriched via a JSON overlay on unproject.
- **Controller-tools for OpenAPI** — `openapi-gen` delegates to `sigs.k8s.io/controller-tools` (`crd.Parser` + `loader.LoadRoots()`) to produce OpenAPI v3 schemas from CRD types, replacing hand-built Swagger 2.0 output.
- **`text/template` for code generation** — all generated Go source is produced through `text/template`, not `fmt.Sprintf` / `strings.Builder`. This makes the output shape readable and auditable in the template itself.
- **Dynamic type discovery** — resource types, imports, and field metadata are derived from the Go AST and the field registry at generation time. No hard-coded type lists that silently break when CRD types are added or removed.

## Generators

| Command | Purpose |
|---------|---------|
| `passthrough-gen` | Generate passthrough struct types from HyperShift API types |
| `marker-scanner` | Extract `+hyperfleet:` marker metadata from Go types |
| `openapi-gen` | Generate OpenAPI v3 schemas via controller-tools CRD extraction |
| `conversion-gen` | Generate JSON-roundtrip conversion functions (CRD ↔ REST) |
| `crd-variants` | Produce CRD variants filtered by feature gates |
| `featuregate-info` | Emit feature gate metadata for CRD fields |
| `verify-configuration` | Validate marker consistency across types |

## Package layout

```
cmd/
  conversion-gen/       CLI entry point
  openapi-gen/          CLI entry point
  passthrough-gen/      CLI entry point
  marker-scanner/       CLI entry point
  crd-variants/         CLI entry point
  featuregate-info/     CLI entry point
  verify-configuration/ CLI entry point
pkg/
  conversion/           JSON-roundtrip Project/Unproject generator (text/template)
  openapi/              Controller-tools-based OpenAPI v3 schema generator
  passthrough/          HyperShift passthrough type generator
  markers/              Marker definitions (WriteMode, FieldMeta) and scanner
  registry/             Generated field metadata registry (consumed by all generators)
  featuregate/          CRD variant generator and feature gate logic
  validation/           Field-level validation against registry metadata
```

## Usage

```bash
make build-api-codegen     # Build all 7 generator binaries
make test-api-codegen      # Run tests
make coverage-api-codegen  # Generate coverage report
```
