package runtime

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/JNickson/cluster-telemetry-service/internal/pods"
)

type podLogsStreamFormat string

const (
	podLogsStreamFormatText podLogsStreamFormat = "text"
	podLogsStreamFormatJSON podLogsStreamFormat = "json"
)

type podLogsStreamOptions struct {
	Format    podLogsStreamFormat
	Frequency time.Duration
	FromStart bool
	TailLines *int64
}

const (
	defaultPodLogsStreamFrequency = 500 * time.Millisecond
	minPodLogsStreamFrequency     = 100 * time.Millisecond
	maxPodLogsStreamFrequency     = 10 * time.Second
)

func podLogsStreamOptionsFromQuery(r *http.Request) (podLogsStreamOptions, error) {
	q := r.URL.Query()

	format := podLogsStreamFormatJSON
	if raw := q.Get("format"); raw != "" {
		switch podLogsStreamFormat(raw) {
		case podLogsStreamFormatText, podLogsStreamFormatJSON:
			format = podLogsStreamFormat(raw)
		default:
			return podLogsStreamOptions{}, fmt.Errorf("invalid format: %s", raw)
		}
	}

	frequency := defaultPodLogsStreamFrequency
	if raw := q.Get("frequencyMs"); raw != "" {
		ms, err := strconv.Atoi(raw)
		if err != nil {
			return podLogsStreamOptions{}, fmt.Errorf("invalid frequencyMs: %w", err)
		}

		frequency = time.Duration(ms) * time.Millisecond
		if frequency < minPodLogsStreamFrequency || frequency > maxPodLogsStreamFrequency {
			return podLogsStreamOptions{}, fmt.Errorf(
				"frequencyMs must be between %d and %d",
				minPodLogsStreamFrequency/time.Millisecond,
				maxPodLogsStreamFrequency/time.Millisecond,
			)
		}
	}

	fromStart := false
	if raw := q.Get("fromStart"); raw != "" {
		v, err := strconv.ParseBool(raw)
		if err != nil {
			return podLogsStreamOptions{}, fmt.Errorf("invalid fromStart: %w", err)
		}
		fromStart = v
	}

	var tailLines *int64
	if raw := q.Get("tailLines"); raw != "" {
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return podLogsStreamOptions{}, fmt.Errorf("invalid tailLines: %w", err)
		}
		if v < 1 {
			return podLogsStreamOptions{}, fmt.Errorf("tailLines must be >= 1")
		}
		tailLines = &v
	}

	return podLogsStreamOptions{
		Format:    format,
		Frequency: frequency,
		FromStart: fromStart,
		TailLines: tailLines,
	}, nil
}

func writePodLogStreamRecord(w io.Writer, record pods.LogStreamRecord, format podLogsStreamFormat) error {
	switch format {
	case podLogsStreamFormatJSON:
		b, err := json.Marshal(record)
		if err != nil {
			return err
		}

		if _, err := w.Write(append(b, '\n')); err != nil {
			return err
		}

		return nil
	case podLogsStreamFormatText:
		_, err := io.WriteString(w, record.Message+"\n")
		return err
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}
