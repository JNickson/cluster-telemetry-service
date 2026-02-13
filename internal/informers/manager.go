package informers

import (
	"context"
	"log/slog"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
)

type Manager struct {
	factory informers.SharedInformerFactory
}

func NewManager(client kubernetes.Interface) *Manager {
	factory := informers.NewSharedInformerFactory(client, 0)

	return &Manager{
		factory: factory,
	}
}

func (m *Manager) Factory() informers.SharedInformerFactory {
	return m.factory
}

func (m *Manager) Start(ctx context.Context) {
	slog.Info("starting informers")

	m.factory.Start(ctx.Done())
	m.factory.WaitForCacheSync(ctx.Done())

	slog.Info("informers synced")
}
