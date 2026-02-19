#!/usr/bin/env bash

set -euo pipefail

PERF_DIR="${PERF_DIR:-.perf}"
BENCH_TIME="${BENCH_TIME:-10s}"

run_tests() {
  go test ./...
}

run_benchmarks() {
  go test -run '^$' -bench . -benchmem -benchtime="${BENCH_TIME}" ./internal/pods ./internal/runtime
}

run_profiles() {
  mkdir -p "${PERF_DIR}"
  go test -run '^$' -bench BenchmarkPodLogsCollectorStream -benchmem -benchtime="${BENCH_TIME}" -cpuprofile "${PERF_DIR}/cpu.out" -memprofile "${PERF_DIR}/mem.out" ./internal/pods

  echo "CPU profile: ${PERF_DIR}/cpu.out"
  echo "Memory profile: ${PERF_DIR}/mem.out"
  echo "Inspect with: go tool pprof -http=:8080 ${PERF_DIR}/cpu.out"
}

run_all() {
  run_tests
  run_benchmarks
  run_profiles
}

cmd="${1:-all}"
case "${cmd}" in
  test)
    run_tests
    ;;
  bench)
    run_benchmarks
    ;;
  profile)
    run_profiles
    ;;
  all)
    run_all
    ;;
  *)
    echo "Usage: $0 [test|bench|profile|all]"
    exit 1
    ;;
esac
