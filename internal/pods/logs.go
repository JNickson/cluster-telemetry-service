package pods

import (
	"context"
	"io"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

type LogsService struct {
	client kubernetes.Interface
}

func NewLogsService(client kubernetes.Interface) *LogsService {
	return &LogsService{
		client: client,
	}
}

func (s *LogsService) GetLogs(
	ctx context.Context,
	namespace,
	name string,
	tailLines int64,
) (string, error) {

	req := s.client.CoreV1().
		Pods(namespace).
		GetLogs(name, &v1.PodLogOptions{
			TailLines: &tailLines,
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
