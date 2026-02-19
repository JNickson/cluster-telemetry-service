.PHONY: dev build fmt fmt-check vulncheck lint test ci bench profile perf clean-perf

PERF_DIR ?= .perf
BENCH_TIME ?= 10s
BENCH_PKG ?= ./internal/pods ./internal/runtime

dev:
	go run ./main.go

build:
	go build ./...

fmt:
	gofmt -w .

fmt-check:
	@test -z "$$(gofmt -l .)" || (echo "gofmt needed:" && gofmt -l . && exit 1)

vulncheck:
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

lint:
	go run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8 run -v ./...

test:
	go test -v ./...

ci: fmt-check vulncheck lint test build

bench:
	go test -run '^$$' -bench . -benchmem -benchtime=$(BENCH_TIME) $(BENCH_PKG)

profile:
	mkdir -p $(PERF_DIR)
	go test -run '^$$' -bench BenchmarkPodLogsCollectorStream -benchmem -benchtime=$(BENCH_TIME) -cpuprofile $(PERF_DIR)/cpu.out -memprofile $(PERF_DIR)/mem.out ./internal/pods

perf: test bench profile

clean-perf:
	rm -rf $(PERF_DIR)
