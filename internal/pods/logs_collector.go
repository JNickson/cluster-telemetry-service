package pods

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

type LogStreamOptions struct {
	FromStart bool
	TailLines *int64
}

type logOpenOptions struct {
	SinceTime *time.Time
	TailLines *int64
}

type PodLogsCollector struct {
	kubeClient        kubernetes.Interface
	podLister         corev1listers.PodLister
	retryDelay        time.Duration
	reconcileInterval time.Duration
	openStream        func(context.Context, string, string, logOpenOptions) (io.ReadCloser, error)
	now               func() time.Time

	mu        sync.Mutex
	lastByPod map[string]time.Time
}

func NewPodLogsCollector(client kubernetes.Interface) *PodLogsCollector {
	c := &PodLogsCollector{
		kubeClient:        client,
		retryDelay:        2 * time.Second,
		reconcileInterval: 5 * time.Second,
		now:               time.Now,
		lastByPod:         make(map[string]time.Time),
	}

	c.openStream = c.defaultOpenStream

	return c
}

func (c *PodLogsCollector) WithPodLister(podLister corev1listers.PodLister) *PodLogsCollector {
	c.podLister = podLister
	return c
}

func (c *PodLogsCollector) defaultOpenStream(
	ctx context.Context,
	namespace,
	name string,
	openOpts logOpenOptions,
) (io.ReadCloser, error) {
	logOpts := &v1.PodLogOptions{
		Follow:     true,
		Timestamps: true,
	}

	if openOpts.SinceTime != nil {
		t := metav1.NewTime(openOpts.SinceTime.UTC())
		logOpts.SinceTime = &t
	} else if openOpts.TailLines != nil {
		logOpts.TailLines = openOpts.TailLines
	}

	req := c.kubeClient.CoreV1().
		Pods(namespace).
		GetLogs(name, logOpts)

	return req.Stream(ctx)
}

func (c *PodLogsCollector) Stream(
	ctx context.Context,
	namespace,
	name string,
	options LogStreamOptions,
	onRecord func(LogStreamRecord) error,
) error {
	connected := false
	key := namespace + "/" + name

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		openOpts := logOpenOptions{}
		if sinceTime, ok := c.lastCursor(key); ok {
			openOpts.SinceTime = &sinceTime
		} else if options.FromStart {
			openOpts.TailLines = options.TailLines
		} else {
			now := c.now().UTC()
			openOpts.SinceTime = &now
		}

		stream, err := c.openStream(ctx, namespace, name, openOpts)
		if err != nil {
			if !connected {
				return fmt.Errorf("open pod log stream: %w", err)
			}

			slog.Warn("pod log stream disconnected", "namespace", namespace, "pod", name, "error", err)

			if !sleepWithContext(ctx, c.retryDelay) {
				return nil
			}
			continue
		}

		connected = true

		scanner := bufio.NewScanner(stream)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

		for scanner.Scan() {
			lineTS, message, hasTS := parseTimestampedLine(scanner.Text())
			if !hasTS {
				lineTS = c.now().UTC()
				message = scanner.Text()
			}
			c.updateCursor(key, lineTS)

			if onRecord != nil {
				record := LogStreamRecord{
					Namespace: namespace,
					Pod:       name,
					Message:   message,
					Timestamp: lineTS,
				}

				if err := onRecord(record); err != nil {
					_ = stream.Close()
					return err
				}
			}
		}

		_ = stream.Close()

		if err := scanner.Err(); err != nil && ctx.Err() == nil {
			slog.Warn("pod log scanner failed", "namespace", namespace, "pod", name, "error", err)
		}

		if !sleepWithContext(ctx, c.retryDelay) {
			return nil
		}
	}
}

func (c *PodLogsCollector) StreamNamespace(
	ctx context.Context,
	namespace string,
	options LogStreamOptions,
	onRecord func(LogStreamRecord) error,
) error {
	if c.podLister == nil {
		return errors.New("pod lister is required for namespace stream")
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	records := make(chan LogStreamRecord, 256)

	active := make(map[string]context.CancelFunc)
	stopAll := func() {
		for key, stop := range active {
			stop()
			delete(active, key)
		}
	}

	startPod := func(ns, name string) {
		key := ns + "/" + name
		if _, exists := active[key]; exists {
			return
		}

		podCtx, podCancel := context.WithCancel(ctx)
		active[key] = podCancel

		go func() {
			err := c.Stream(podCtx, ns, name, options, func(record LogStreamRecord) error {
				select {
				case <-podCtx.Done():
					return podCtx.Err()
				case records <- record:
					return nil
				}
			})

			if err != nil && podCtx.Err() == nil {
				slog.Warn("pod namespace stream ended", "namespace", ns, "pod", name, "error", err)
			}
		}()
	}

	reconcile := func() {
		pods, err := c.podLister.Pods(namespace).List(labels.Everything())
		if err != nil {
			slog.Warn("failed to list pods for namespace stream", "namespace", namespace, "error", err)
			return
		}

		seen := make(map[string]struct{}, len(pods))
		for _, pod := range pods {
			if pod.Spec.NodeName == "" {
				continue
			}

			key := pod.Namespace + "/" + pod.Name
			seen[key] = struct{}{}
			startPod(pod.Namespace, pod.Name)
		}

		for key, stop := range active {
			if _, ok := seen[key]; !ok {
				stop()
				delete(active, key)
			}
		}
	}

	reconcile()
	ticker := time.NewTicker(c.reconcileInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			stopAll()
			return nil
		case <-ticker.C:
			reconcile()
		case record := <-records:
			if onRecord != nil {
				if err := onRecord(record); err != nil {
					stopAll()
					return err
				}
			}
		}
	}
}

func parseTimestampedLine(line string) (time.Time, string, bool) {
	space := strings.IndexByte(line, ' ')
	if space <= 0 {
		return time.Time{}, line, false
	}

	ts, err := time.Parse(time.RFC3339Nano, line[:space])
	if err != nil {
		return time.Time{}, line, false
	}

	return ts.UTC(), line[space+1:], true
}

func (c *PodLogsCollector) updateCursor(key string, t time.Time) {
	if t.IsZero() {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if prev, ok := c.lastByPod[key]; ok && !t.After(prev) {
		return
	}

	c.lastByPod[key] = t
}

func (c *PodLogsCollector) lastCursor(key string) (time.Time, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	last, ok := c.lastByPod[key]
	if !ok {
		return time.Time{}, false
	}

	return last.Add(time.Nanosecond), true
}

func sleepWithContext(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
