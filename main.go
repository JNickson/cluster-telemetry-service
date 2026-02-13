package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/JNickson/cluster-telemetry-service/internal/clients"
	"github.com/JNickson/cluster-telemetry-service/internal/runtime"
	"github.com/JNickson/cluster-telemetry-service/internal/utils"
	"k8s.io/client-go/rest"
)

func setupLogger() {
	logger := utils.NewLogger()
	slog.SetDefault(logger)
}

func mustKubeConfig() *rest.Config {
	cfg, err := clients.NewKubeConfig()
	if err != nil {
		slog.Error("failed to create kube config", "error", err)
		os.Exit(1)
	}
	return cfg
}

func main() {
	setupLogger()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg := mustKubeConfig()

	app, err := runtime.New(cfg)
	if err != nil {
		slog.Error("failed to create app", "error", err)
		os.Exit(1)
	}

	app.Start(ctx)
}
