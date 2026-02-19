package pods

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

func TestPodLogsCollectorStream(t *testing.T) {
	tests := []struct {
		name            string
		openStream      func(context.Context, string, string, logOpenOptions) (io.ReadCloser, error)
		wantErrContains string
		wantMessages    []string
	}{
		{
			name: "streams structured records",
			openStream: func(context.Context, string, string, logOpenOptions) (io.ReadCloser, error) {
				return io.NopCloser(strings.NewReader("2026-02-19T12:00:00Z line1\n2026-02-19T12:00:01Z line2\n")), nil
			},
			wantMessages: []string{"line1", "line2"},
		},
		{
			name: "returns first connection error",
			openStream: func(context.Context, string, string, logOpenOptions) (io.ReadCloser, error) {
				return nil, errors.New("boom")
			},
			wantErrContains: "open pod log stream",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collector := NewPodLogsCollector(nil)
			collector.retryDelay = time.Millisecond
			collector.openStream = tt.openStream
			collector.now = func() time.Time {
				return time.Date(2026, time.February, 19, 12, 0, 0, 0, time.UTC)
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			var got []LogStreamRecord

			err := collector.Stream(ctx, "default", "api-0", LogStreamOptions{}, func(record LogStreamRecord) error {
				got = append(got, record)
				if len(got) >= len(tt.wantMessages) && len(tt.wantMessages) > 0 {
					return errors.New("stop")
				}
				return nil
			})

			if tt.wantErrContains != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.wantErrContains)
				return
			}

			require.Error(t, err)
			require.Equal(t, "stop", err.Error())
			require.Len(t, got, len(tt.wantMessages))
			for i, msg := range tt.wantMessages {
				require.Equal(t, "default", got[i].Namespace)
				require.Equal(t, "api-0", got[i].Pod)
				require.Equal(t, msg, got[i].Message)
				require.Equal(t, time.Date(2026, time.February, 19, 12, 0, i, 0, time.UTC), got[i].Timestamp)
			}
		})
	}
}

func TestPodLogsCollectorStreamReconnects(t *testing.T) {
	collector := NewPodLogsCollector(nil)
	collector.retryDelay = time.Millisecond

	var calls atomic.Int32
	collector.openStream = func(context.Context, string, string, logOpenOptions) (io.ReadCloser, error) {
		n := calls.Add(1)
		if n == 1 {
			return io.NopCloser(strings.NewReader("2026-02-19T12:00:00Z first\n")), nil
		}
		return io.NopCloser(strings.NewReader("2026-02-19T12:00:01Z second\n")), nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var got []string
	err := collector.Stream(ctx, "default", "api-0", LogStreamOptions{}, func(record LogStreamRecord) error {
		got = append(got, record.Message)
		if len(got) == 2 {
			return errors.New("done")
		}
		return nil
	})

	require.Error(t, err)
	require.Equal(t, "done", err.Error())
	require.Equal(t, []string{"first", "second"}, got)
	require.GreaterOrEqual(t, calls.Load(), int32(2))
}

func TestPodLogsCollectorStreamUsesCursorForNextSession(t *testing.T) {
	collector := NewPodLogsCollector(nil)
	collector.retryDelay = time.Millisecond

	var lastOpen logOpenOptions
	collector.openStream = func(_ context.Context, _ string, _ string, opts logOpenOptions) (io.ReadCloser, error) {
		lastOpen = opts
		return io.NopCloser(strings.NewReader("2026-02-19T12:00:00Z one\n")), nil
	}

	err := collector.Stream(context.Background(), "default", "api-0", LogStreamOptions{FromStart: true}, func(record LogStreamRecord) error {
		return errors.New("done")
	})
	require.Error(t, err)
	require.Equal(t, "done", err.Error())
	require.Nil(t, lastOpen.SinceTime)

	err = collector.Stream(context.Background(), "default", "api-0", LogStreamOptions{}, func(record LogStreamRecord) error {
		return errors.New("done-2")
	})
	require.Error(t, err)
	require.Equal(t, "done-2", err.Error())
	require.NotNil(t, lastOpen.SinceTime)
}

func TestPodLogsCollectorStreamNamespace(t *testing.T) {
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	require.NoError(t, indexer.Add(&v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "api-0", Namespace: "default"}, Spec: v1.PodSpec{NodeName: "node-a"}}))
	require.NoError(t, indexer.Add(&v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "api-1", Namespace: "default"}, Spec: v1.PodSpec{NodeName: "node-b"}}))

	collector := NewPodLogsCollector(nil).WithPodLister(newPodLister(indexer))
	collector.retryDelay = time.Millisecond
	collector.reconcileInterval = 10 * time.Millisecond

	collector.openStream = func(_ context.Context, namespace, name string, _ logOpenOptions) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("2026-02-19T12:00:00Z hi-from-" + name + "\n")), nil
	}

	seen := map[string]bool{}
	err := collector.StreamNamespace(context.Background(), "default", LogStreamOptions{FromStart: true}, func(record LogStreamRecord) error {
		seen[record.Pod] = true
		if len(seen) == 2 {
			return errors.New("done")
		}
		return nil
	})

	require.Error(t, err)
	require.Equal(t, "done", err.Error())
	require.True(t, seen["api-0"])
	require.True(t, seen["api-1"])
}

func newPodLister(indexer cache.Indexer) *podListerAdapter {
	return &podListerAdapter{indexer: indexer}
}

type podListerAdapter struct {
	indexer cache.Indexer
}

func (p *podListerAdapter) List(selector labels.Selector) (ret []*v1.Pod, err error) {
	for _, item := range p.indexer.List() {
		pod := item.(*v1.Pod)
		if selector.Matches(labels.Set(pod.Labels)) {
			ret = append(ret, pod)
		}
	}
	return ret, nil
}

func (p *podListerAdapter) Pods(namespace string) corev1listers.PodNamespaceLister {
	return &podNamespaceListerAdapter{indexer: p.indexer, namespace: namespace}
}

type podNamespaceListerAdapter struct {
	indexer   cache.Indexer
	namespace string
}

func (p *podNamespaceListerAdapter) List(selector labels.Selector) (ret []*v1.Pod, err error) {
	for _, item := range p.indexer.List() {
		pod := item.(*v1.Pod)
		if pod.Namespace != p.namespace {
			continue
		}
		if selector.Matches(labels.Set(pod.Labels)) {
			ret = append(ret, pod)
		}
	}
	return ret, nil
}

func (p *podNamespaceListerAdapter) Get(name string) (*v1.Pod, error) {
	obj, exists, err := p.indexer.GetByKey(p.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.New("not found")
	}
	return obj.(*v1.Pod), nil
}
