package pods

import (
	"bufio"
	"context"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

type DBWriter interface {
	Write(record LogRecord) error
}

type LogRecord struct {
	Namespace string
	Pod       string
	Message   string
	Timestamp time.Time
}

type PodLogsCollector struct {
	podLister  corev1listers.PodLister
	kubeClient kubernetes.Interface
	db         DBWriter

	mu      sync.Mutex
	streams map[string]context.CancelFunc
}

func NewPodLogsCollector(
	podLister corev1listers.PodLister,
	client kubernetes.Interface,
	db DBWriter,
) *PodLogsCollector {
	return &PodLogsCollector{
		podLister:  podLister,
		kubeClient: client,
		db:         db,
		streams:    make(map[string]context.CancelFunc),
	}
}

func (c *PodLogsCollector) Start(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			c.stopAll()
			return
		case <-ticker.C:
			c.reconcile()
		}
	}
}

func (c *PodLogsCollector) reconcile() {
	pods, err := c.podLister.List(labels.Everything())
	if err != nil {
		return
	}

	active := make(map[string]struct{})

	for _, pod := range pods {

		if pod.Spec.NodeName == "" {
			continue
		}

		key := pod.Namespace + "/" + pod.Name
		active[key] = struct{}{}

		c.mu.Lock()
		_, exists := c.streams[key]
		c.mu.Unlock()

		if !exists {
			c.startStream(pod.Namespace, pod.Name)
		}
	}

	c.cleanupStale(active)
}

func (c *PodLogsCollector) startStream(namespace, name string) {
	ctx, cancel := context.WithCancel(context.Background())

	key := namespace + "/" + name

	c.mu.Lock()
	c.streams[key] = cancel
	c.mu.Unlock()

	go c.streamPodLogs(ctx, namespace, name)
}

func (c *PodLogsCollector) cleanupStale(active map[string]struct{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for key, cancel := range c.streams {
		if _, exists := active[key]; !exists {
			cancel()
			delete(c.streams, key)
		}
	}
}

func (c *PodLogsCollector) stopAll() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for key, cancel := range c.streams {
		cancel()
		delete(c.streams, key)
	}
}

func (c *PodLogsCollector) streamPodLogs(
	ctx context.Context,
	namespace,
	name string,
) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		req := c.kubeClient.CoreV1().
			Pods(namespace).
			GetLogs(name, &v1.PodLogOptions{
				Follow: true,
			})

		stream, err := req.Stream(ctx)
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}

		scanner := bufio.NewScanner(stream)

		for scanner.Scan() {
			line := scanner.Text()

			if c.db != nil {
				_ = c.db.Write(LogRecord{
					Namespace: namespace,
					Pod:       name,
					Message:   line,
					Timestamp: time.Now(),
				})
			}
		}

		stream.Close()

		time.Sleep(2 * time.Second)
	}
}
