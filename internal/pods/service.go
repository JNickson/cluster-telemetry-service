package pods

import (
	"context"

	"github.com/JNickson/cluster-telemetry-service/internal/utils"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

type Service interface {
	BuildSnapshot(ctx context.Context) ([]Pod, error)
}

type PodService struct {
	podLister corev1listers.PodLister
}

func NewPodService(podLister corev1listers.PodLister) *PodService {
	return &PodService{
		podLister: podLister,
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

func mapPod(p v1.Pod) Pod {
	var (
		ready      = true
		restarts   int32
		containers []Container
	)

	if len(p.Status.ContainerStatuses) == 0 {
		ready = false
	}

	for _, cs := range p.Status.ContainerStatuses {
		containers = append(containers, Container{
			Name:  cs.Name,
			Ready: cs.Ready,
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
