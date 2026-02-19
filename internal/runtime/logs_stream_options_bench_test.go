package runtime

import (
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/JNickson/cluster-telemetry-service/internal/pods"
)

func BenchmarkPodLogsStreamOptionsFromQuery(b *testing.B) {
	req := httptest.NewRequest(
		"GET",
		"/api/v1/pods/logs/stream?format=json&frequencyMs=250&fromStart=true&tailLines=200",
		nil,
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := podLogsStreamOptionsFromQuery(req); err != nil {
			b.Fatalf("unexpected parse error: %v", err)
		}
	}
}

func BenchmarkWritePodLogStreamRecordJSON(b *testing.B) {
	record := pods.LogStreamRecord{
		Namespace: "default",
		Pod:       "api-0",
		Message:   "hello benchmark",
		Timestamp: time.Date(2026, time.February, 19, 12, 0, 0, 0, time.UTC),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := writePodLogStreamRecord(io.Discard, record, podLogsStreamFormatJSON); err != nil {
			b.Fatalf("unexpected write error: %v", err)
		}
	}
}
