# Cluster Telemetry Service

Lightweight Kubernetes observability service built with client-go informers.

Watches Nodes and Pods, builds 30s snapshots from the informer cache, and exposes them via a HTTP API.

Run locally:

`go run main.go`

or:

`make dev`

API: http://localhost:8001

Endpoints:
  - /healthz
  - /readyz
  - /api/v1/nodes
  - /api/v1/pods
  - /api/v1/pods/logs/stream?namespace=<ns>

Stream query options:
  - format: json (default) or text
  - frequencyMs: emit interval in milliseconds (default 500, min 100, max 10000)
  - fromStart: true/false (default false)
  - tailLines: when fromStart=true, limit initial historical lines

Behavior:
  - stream is always namespace-scoped (all pods in the namespace)

## Testing

### Strategy
Golden file tests are used for the transformation logic (e.g. mapPod) to validate output structure and shape.

Golden tests define only the input and compare the function output against a stored snapshot.

**When modifying transformation logic:**
- Run `go test -v ./...`
- If output changes are intentional, run `go test -v ./... -update` to regenerate expected values.

Do not run -update blindly: it accepts the current behaviour as the new shape.

cmp.Diff is used to display a clear -expected / +actual structural diff on failure and during updates.

## Performance

Run the full suite with make:

- `make lint`
- `make test`
- `make bench`
- `make profile`
- `make perf`

## GitHub Actions

- `CI` workflow runs on PRs and `main`: formatting, `go vet`, `govulncheck`, `golangci-lint`, typecheck, build, Docker build check, and tests.
- `Release` workflow runs on `v*.*.*` tags and pushes Docker images to GHCR.
- Dependabot is enabled for Go modules and GitHub Actions updates.

Or use the helper script:

- `./scripts/perf.sh all`
- `./scripts/perf.sh profile`

Profiles are written to `.perf/cpu.out` and `.perf/mem.out`.
