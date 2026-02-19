package runtime

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/JNickson/cluster-telemetry-service/internal/pods"
	"github.com/stretchr/testify/require"
)

func TestPodLogsStreamOptionsFromQuery(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		want        podLogsStreamOptions
		wantErr     bool
		errContains string
	}{
		{
			name: "defaults",
			url:  "/api/v1/pods/logs/stream",
			want: podLogsStreamOptions{
				Format:    podLogsStreamFormatJSON,
				Frequency: defaultPodLogsStreamFrequency,
				FromStart: false,
			},
		},
		{
			name: "parses explicit values",
			url:  "/api/v1/pods/logs/stream?format=text&frequencyMs=250&fromStart=true&tailLines=40",
			want: podLogsStreamOptions{
				Format:    podLogsStreamFormatText,
				Frequency: 250 * time.Millisecond,
				FromStart: true,
				TailLines: ptrInt64(40),
			},
		},
		{
			name:        "invalid format",
			url:         "/api/v1/pods/logs/stream?format=xml",
			wantErr:     true,
			errContains: "invalid format",
		},
		{
			name:        "invalid frequency",
			url:         "/api/v1/pods/logs/stream?frequencyMs=abc",
			wantErr:     true,
			errContains: "invalid frequencyMs",
		},
		{
			name:        "frequency too low",
			url:         "/api/v1/pods/logs/stream?frequencyMs=50",
			wantErr:     true,
			errContains: "frequencyMs must be between",
		},
		{
			name:        "invalid fromStart",
			url:         "/api/v1/pods/logs/stream?fromStart=yep",
			wantErr:     true,
			errContains: "invalid fromStart",
		},
		{
			name:        "invalid tailLines",
			url:         "/api/v1/pods/logs/stream?tailLines=0",
			wantErr:     true,
			errContains: "tailLines must be >= 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.url, nil)

			got, err := podLogsStreamOptionsFromQuery(req)

			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errContains)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestWritePodLogStreamRecord(t *testing.T) {
	record := pods.LogStreamRecord{
		Namespace: "default",
		Pod:       "api-0",
		Message:   "hello",
		Timestamp: time.Date(2026, time.February, 19, 12, 0, 0, 0, time.UTC),
	}

	tests := []struct {
		name    string
		format  podLogsStreamFormat
		want    string
		wantErr bool
	}{
		{
			name:   "writes json ndjson line",
			format: podLogsStreamFormatJSON,
			want:   "{\"namespace\":\"default\",\"pod\":\"api-0\",\"message\":\"hello\",\"timestamp\":\"2026-02-19T12:00:00Z\"}\n",
		},
		{
			name:   "writes text line",
			format: podLogsStreamFormatText,
			want:   "hello\n",
		},
		{
			name:    "rejects unknown format",
			format:  podLogsStreamFormat("yaml"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out stringWriter

			err := writePodLogStreamRecord(&out, record, tt.format)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.want, out.String())
		})
	}
}

type stringWriter struct {
	b []byte
}

func (w *stringWriter) Write(p []byte) (int, error) {
	w.b = append(w.b, p...)
	return len(p), nil
}

func (w *stringWriter) String() string {
	return string(w.b)
}

func ptrInt64(v int64) *int64 {
	return &v
}
