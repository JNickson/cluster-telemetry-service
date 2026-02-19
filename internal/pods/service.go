package pods

import (
	"context"
	"fmt"

	"github.com/JNickson/cluster-telemetry-service/internal/utils"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

type Service interface {
	BuildSnapshot(ctx context.Context) ([]Pod, error)
	StreamNamespaceLogs(
		ctx context.Context,
		namespace string,
		options LogStreamOptions,
		onRecord func(LogStreamRecord) error,
	) error
}

type podLogsStreamer interface {
	StreamNamespace(
		ctx context.Context,
		namespace string,
		options LogStreamOptions,
		onRecord func(LogStreamRecord) error,
	) error
}

type PodService struct {
	podLister     corev1listers.PodLister
	logsCollector podLogsStreamer
}

func NewPodService(
	podLister corev1listers.PodLister,
	kubeClient kubernetes.Interface,
) *PodService {
	return &PodService{
		podLister:     podLister,
		logsCollector: NewPodLogsCollector(kubeClient).WithPodLister(podLister),
	}
}

func (s *PodService) BuildSnapshot(ctx context.Context) ([]Pod, error) {

	list, err := s.podLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}

	out := make([]Pod, 0, len(list))

	for _, p := range list {

		if p.Spec.NodeName == "" {
			continue
		}

		if p.Status.Phase == v1.PodSucceeded {
			continue
		}

		out = append(out, mapPod(*p))
	}

	return out, nil
}

func (s *PodService) StreamNamespaceLogs(
	ctx context.Context,
	namespace string,
	options LogStreamOptions,
	onRecord func(LogStreamRecord) error,
) error {
	return s.logsCollector.StreamNamespace(ctx, namespace, options, onRecord)
}

func mapPod(p v1.Pod) Pod {
	var (
		ready      = true
		restarts   int32
		containers []Container
	)

	if len(p.Status.ContainerStatuses) == 0 {
		ready = false
	}

	// Build lookup from spec containers
	specMap := make(map[string]v1.Container)
	for _, c := range p.Spec.Containers {
		specMap[c.Name] = c
	}

	for _, cs := range p.Status.ContainerStatuses {

		specContainer := specMap[cs.Name]

		var cpuReq, memReq string

		if req := specContainer.Resources.Requests; req != nil {
			if cpu, ok := req[v1.ResourceCPU]; ok {
				cpuReq = fmt.Sprintf("%dm", cpu.MilliValue())
			}
			if mem, ok := req[v1.ResourceMemory]; ok {
				memReq = fmt.Sprintf("%dMi", mem.Value()/(1024*1024))
			}
		}

		containers = append(containers, Container{
			Name:          cs.Name,
			Ready:         cs.Ready,
			CPURequest:    cpuReq,
			MemoryRequest: memReq,
		})

		restarts += cs.RestartCount

		if !cs.Ready {
			ready = false
		}
	}

	return Pod{
		Name:       p.Name,
		Namespace:  p.Namespace,
		Node:       p.Spec.NodeName,
		Phase:      string(p.Status.Phase),
		Ready:      ready,
		Restarts:   restarts,
		Age:        utils.AgeSince(p.CreationTimestamp.Time),
		Containers: containers,
	}
}
