package pods

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
)

var errBenchmarkStop = errors.New("benchmark stop")

func BenchmarkPodLogsCollectorStream100Lines(b *testing.B) {
	runPodLogsCollectorStreamBenchmark(b, 100)
}

func BenchmarkPodLogsCollectorStream1000Lines(b *testing.B) {
	runPodLogsCollectorStreamBenchmark(b, 1000)
}

func runPodLogsCollectorStreamBenchmark(b *testing.B, lines int) {
	b.Helper()

	payload := buildBenchmarkLogPayload(lines)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		collector := NewPodLogsCollector(nil)
		collector.openStream = func(context.Context, string, string, logOpenOptions) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(payload)), nil
		}

		count := 0
		err := collector.Stream(
			context.Background(),
			"default",
			"api-0",
			LogStreamOptions{FromStart: true},
			func(LogStreamRecord) error {
				count++
				if count >= lines {
					return errBenchmarkStop
				}
				return nil
			},
		)

		if !errors.Is(err, errBenchmarkStop) {
			b.Fatalf("unexpected stream error: %v", err)
		}
	}
}

func buildBenchmarkLogPayload(lines int) string {
	if lines <= 0 {
		return ""
	}

	var sb strings.Builder
	for i := 0; i < lines; i++ {
		sb.WriteString("2026-02-19T12:00:00Z benchmark-line-")
		sb.WriteString(fmt.Sprintf("%d", i))
		sb.WriteByte('\n')
	}

	return sb.String()
}
