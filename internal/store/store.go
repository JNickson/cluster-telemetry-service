package store

import (
	"sync"

	"github.com/JNickson/cluster-telemetry-service/internal/nodes"
	"github.com/JNickson/cluster-telemetry-service/internal/pods"
)

type Store struct {
	mu    sync.RWMutex
	nodes []nodes.Node
	pods  []pods.Pod
}

func New() *Store {
	return &Store{
		nodes: make([]nodes.Node, 0),
		pods:  make([]pods.Pod, 0),
	}
}

func (s *Store) ReplaceNodes(nodes []nodes.Node) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nodes = nodes
}

func (s *Store) ListNodes() []nodes.Node {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]nodes.Node, len(s.nodes))
	copy(out, s.nodes)
	return out
}

func (s *Store) ListPods() []pods.Pod {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]pods.Pod, len(s.pods))
	copy(out, s.pods)
	return out
}

func (s *Store) ReplacePods(pods []pods.Pod) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pods = pods
}
