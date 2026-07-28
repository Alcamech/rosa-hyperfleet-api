# Hyperfleet SDK Examples

Runnable Go programs demonstrating common Hyperfleet API operations.

## Building

Compile all examples into `clientset/examples/bin/`:

```bash
cd clientset
for dir in examples/*/; do
  name=$(basename "$dir")
  [ "$name" = "util" ] && continue
  go build -o "examples/bin/$name" "./$dir"
done
```

Or build a single example:

```bash
cd clientset
go build -o examples/bin/list-clusters ./examples/list-clusters
```

## Running

All examples read configuration from environment variables. The minimum required
variables are:

| Variable | Description |
|---|---|
| `HYPERFLEET_HOST` | Platform API base URL (e.g. `https://{id}.execute-api.{region}.amazonaws.com/prod`) |

AWS credentials are loaded from the default credential chain (`AWS_PROFILE`,
instance metadata, etc.).
