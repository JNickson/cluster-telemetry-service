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
