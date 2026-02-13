package runtime

import (
	"context"
	"log/slog"
	"time"
)

func (a *App) startNodeReconciler(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			nodes, err := a.nodesService.BuildSnapshot(ctx)
			if err != nil {
				slog.Error("failed to refresh nodes", "error", err)
				continue
			}
			a.store.ReplaceNodes(nodes)
			slog.Info("nodes snapshot refreshed",
				"count", len(nodes),
				"time", time.Now(),
			)
		}
	}
}

func (a *App) startPodReconciler(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pods, err := a.podsService.BuildSnapshot(ctx)
			if err != nil {
				slog.Error("failed to refresh pods", "error", err)
				continue
			}

			a.store.ReplacePods(pods)

			slog.Info("pods snapshot refreshed",
				"count", len(pods),
				"time", time.Now(),
			)
		}
	}
}
