package pods

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPodServiceStreamNamespaceLogs(t *testing.T) {
	tests := []struct {
		name        string
		streamErr   error
		wantErr     bool
		wantErrText string
	}{
		{
			name: "streams via collector",
		},
		{
			name:        "returns collector error",
			streamErr:   errors.New("collector boom"),
			wantErr:     true,
			wantErrText: "collector boom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakePodLogsStreamer{streamErr: tt.streamErr}
			svc := &PodService{logsCollector: fake}

			err := svc.StreamNamespaceLogs(context.Background(), "default", LogStreamOptions{}, func(LogStreamRecord) error {
				return nil
			})

			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.wantErrText)
				return
			}

			require.NoError(t, err)
			require.Equal(t, "default", fake.lastNamespace)
		})
	}
}

type fakePodLogsStreamer struct {
	lastNamespace string
	streamErr     error
}

func (f *fakePodLogsStreamer) StreamNamespace(
	_ context.Context,
	namespace string,
	_ LogStreamOptions,
	onRecord func(LogStreamRecord) error,
) error {
	f.lastNamespace = namespace

	if onRecord != nil {
		_ = onRecord(LogStreamRecord{
			Namespace: namespace,
			Pod:       "api-0",
			Message:   "line",
			Timestamp: time.Now().UTC(),
		})
	}

	return f.streamErr
}
