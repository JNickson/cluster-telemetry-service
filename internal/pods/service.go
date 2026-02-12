package pods

import (
	"context"
	"fmt"
	"io"

	"github.com/JNickson/cluster-telemetry-service/internal/utils"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type Service interface {
	FetchPods(ctx context.Context) ([]Pod, error)
}

type PodService struct {
	kubeClient *kubernetes.Clientset
}

func NewPodService(kubeClient *kubernetes.Clientset) *PodService {
	return &PodService{kubeClient: kubeClient}
}

func (s *PodService) FetchPods(ctx context.Context) ([]Pod, error) {
	list, err := s.kubeClient.CoreV1().
		Pods("").
		List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	out := make([]Pod, 0, len(list.Items))

	for _, p := range list.Items {

		// Ignore pods not yet scheduled
		if p.Spec.NodeName == "" {
			continue
		}
		if p.Status.Phase == v1.PodSucceeded {
			continue
		}

		logs, err := s.fetchLogs(ctx, p)
		if err != nil {
			logs = fmt.Sprintf("failed to fetch logs: %v", err)
		}

		out = append(out, mapPod(p, logs))
	}

	return out, nil
}

func mapPod(p v1.Pod, logs string) Pod {
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
		Logs:       logs,
	}
}

func (s *PodService) fetchLogs(
	ctx context.Context,
	p v1.Pod,
) (string, error) {

	// Skip logs for completed pods (helm jobs etc)
	if p.Status.Phase == v1.PodSucceeded {
		return "pod completed successfully", nil
	}

	req := s.kubeClient.CoreV1().
		Pods(p.Namespace).
		GetLogs(p.Name, &v1.PodLogOptions{
			TailLines: utils.Int64Ptr(300), // last 300 lines
		})

	stream, err := req.Stream(ctx)
	if err != nil {
		return "", err
	}
	defer stream.Close()

	data, err := io.ReadAll(stream)
	if err != nil {
		return "", err
	}

	return string(data), nil
}
