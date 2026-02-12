package clients

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	metricsclient "k8s.io/metrics/pkg/client/clientset/versioned"
)

func NewKubeConfig() (*rest.Config, error) {
	// Try in-cluster first
	cfg, err := rest.InClusterConfig()
	if err == nil {
		slog.Info("Using in-cluster Kubernetes config")
		return cfg, nil
	}

	slog.Warn("In-cluster config failed, falling back to kubeconfig",
		"error", err,
		"host", os.Getenv("KUBERNETES_SERVICE_HOST"),
		"port", os.Getenv("KUBERNETES_SERVICE_PORT"),
	)

	// Fallback to local kubeconfig
	home, _ := os.UserHomeDir()
	kubeconfig := filepath.Join(home, ".kube", "config")

	cfg, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	slog.Info("Using local kubeconfig", "path", kubeconfig)
	return cfg, nil
}

func NewKubeClient(cfg *rest.Config) (*kubernetes.Clientset, error) {
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create kube client: %w", err)
	}

	slog.Info("Kubernetes client initialised")
	return client, nil
}

func NewMetricsClient(cfg *rest.Config) (*metricsclient.Clientset, error) {
	client, err := metricsclient.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create metrics client: %w", err)
	}

	slog.Info("Metrics client initialised")
	return client, nil
}
