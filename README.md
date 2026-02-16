# Cluster Telemetry Service

Lightweight Kubernetes observability service built with client-go informers.

Watches Nodes and Pods, builds 30s snapshots from the informer cache, and exposes them via a HTTP API.

Run locally:

`go run main.go`

API: http://localhost:8001

Endpoints
• /healthz
• /readyz
• /api/v1/nodes
• /api/v1/pods
• /api/v1/pods/logs?namespace=<ns>&name=<pod>

## Testing

Strategy
• Golden file tests are used for the transformation logic (e.g. mapPod) to validate output structure and shape.

Golden tests define only the input and compare the function output against a stored snapshot.

When modifying transformation logic:
• Run `go test -v ./...`
• If output changes are intentional, run `go test -v ./... -update` to regenerate expected values.

Do not run -update blindly — it accepts the current behaviour as the new shape.

cmp.Diff is used to display a clear -expected / +actual structural diff on failure and during updates.
