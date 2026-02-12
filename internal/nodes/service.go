package nodes

import (
	"context"
	"fmt"

	"github.com/JNickson/cluster-telemetry-service/internal/utils"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	metricsclient "k8s.io/metrics/pkg/client/clientset/versioned"
)

type Service interface {
	FetchNodes(ctx context.Context) ([]Node, error)
}
type NodeService struct {
	kubeClient    *kubernetes.Clientset
	metricsClient *metricsclient.Clientset
}

func NewNodeService(
	kubeClient *kubernetes.Clientset,
	metricsClient *metricsclient.Clientset,
) *NodeService {
	return &NodeService{
		kubeClient:    kubeClient,
		metricsClient: metricsClient,
	}
}

func (s *NodeService) FetchNodes(ctx context.Context) ([]Node, error) {
	nodesList, err := s.kubeClient.CoreV1().
		Nodes().
		List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	metricsList, err := s.metricsClient.MetricsV1beta1().
		NodeMetricses().
		List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get node metrics: %w", err)
	}

	metricsByNode := map[string]v1.ResourceList{}
	for _, m := range metricsList.Items {
		metricsByNode[m.Name] = m.Usage
	}

	nodes := make([]Node, 0, len(nodesList.Items))
	for _, n := range nodesList.Items {

		workloads, err := s.workloadsForNode(ctx, n.Name)
		if err != nil {
			return nil, err
		}

		nodes = append(nodes, mapNode(
			n,
			metricsByNode[n.Name],
			workloads,
		))
	}

	return nodes, nil
}

func mapNode(
	n v1.Node,
	usage v1.ResourceList,
	workloads NodeWorkloads,
) Node {
	ready := false
	conditions := make([]Condition, 0)

	for _, c := range n.Status.Conditions {
		if c.Type == v1.NodeReady && c.Status == v1.ConditionTrue {
			ready = true
		}
		conditions = append(conditions, Condition{
			Type:    string(c.Type),
			Status:  string(c.Status),
			Reason:  c.Reason,
			Message: c.Message,
		})
	}

	taints := []string{}
	for _, t := range n.Spec.Taints {
		taints = append(taints, fmt.Sprintf("%s=%s:%s", t.Key, t.Value, t.Effect))
	}

	// --- REAL METRICS ---
	cpuUsed := usage[v1.ResourceCPU]
	memUsed := usage[v1.ResourceMemory]

	cpuAlloc := n.Status.Allocatable[v1.ResourceCPU]
	memAlloc := n.Status.Allocatable[v1.ResourceMemory]

	return Node{
		Name:  n.Name,
		Ready: ready,
		Age:   utils.AgeSince(n.CreationTimestamp.Time),

		Labels: n.Labels,
		Taints: taints,

		CPU: Usage{
			Used:  fmt.Sprintf("%dm", cpuUsed.MilliValue()),
			Total: fmt.Sprintf("%dm", cpuAlloc.MilliValue()),
		},
		Memory: Usage{
			Used:  fmt.Sprintf("%dMi", memUsed.Value()/(1024*1024)),
			Total: fmt.Sprintf("%dMi", memAlloc.Value()/(1024*1024)),
		},

		Conditions: conditions,
		Workloads:  workloads,
	}
}

func (s *NodeService) workloadsForNode(
	ctx context.Context,
	nodeName string,
) (NodeWorkloads, error) {

	pods, err := s.kubeClient.CoreV1().
		Pods("").
		List(ctx, metav1.ListOptions{
			FieldSelector: "spec.nodeName=" + nodeName,
		})
	if err != nil {
		return NodeWorkloads{}, err
	}

	deployments := map[string]Workload{}
	statefulSets := map[string]Workload{}
	system := map[string]struct{}{}

	for _, pod := range pods.Items {

		// kube-system pods → System
		if pod.Namespace == "kube-system" {
			system[pod.Name] = struct{}{}
			continue
		}

		for _, owner := range pod.OwnerReferences {

			switch owner.Kind {

			case "ReplicaSet":
				deployName := owner.Name
				if i := utils.LastIndex(owner.Name, "-"); i > 0 {
					deployName = owner.Name[:i]
				}

				key := pod.Namespace + "/" + deployName

				w := deployments[key]
				w.Namespace = pod.Namespace
				w.Name = deployName
				w.Pods++
				deployments[key] = w

			case "StatefulSet":
				key := pod.Namespace + "/" + owner.Name

				w := statefulSets[key]
				w.Namespace = pod.Namespace
				w.Name = owner.Name
				w.Pods++
				statefulSets[key] = w
			}
		}
	}

	return NodeWorkloads{
		Deployments:  utils.MapValuesToSlice(deployments),
		StatefulSets: utils.MapValuesToSlice(statefulSets),
		System:       utils.MapKeysToSlice(system),
	}, nil
}
