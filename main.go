package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/JNickson/cluster-telemetry-service/internal/clients"
	"github.com/JNickson/cluster-telemetry-service/internal/handlers"
	"github.com/JNickson/cluster-telemetry-service/internal/nodes"
	"github.com/JNickson/cluster-telemetry-service/internal/pods"
	"github.com/JNickson/cluster-telemetry-service/internal/utils"
	"k8s.io/client-go/rest"
)

func main() {
	setupLogger()

	cfg := mustKubeConfig()
	nodeService, podService := mustServices(cfg)

	mux := setupRouter(nodeService, podService)

	runServer(mux)
}

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

func mustServices(cfg *rest.Config) (nodes.Service, pods.Service) {
	kubeClient, err := clients.NewKubeClient(cfg)
	if err != nil {
		slog.Error("failed to create kube client", "error", err)
		os.Exit(1)
	}

	metricClient, err := clients.NewMetricsClient(cfg)
	if err != nil {
		slog.Error("failed to create metrics client", "error", err)
		os.Exit(1)
	}

	nodeService := nodes.NewNodeService(kubeClient, metricClient)
	podService := pods.NewPodService(kubeClient)

	return nodeService, podService
}

func setupRouter(nodeService nodes.Service, podService pods.Service) http.Handler {
	api := http.NewServeMux()
	api.HandleFunc("/nodes", handlers.NodesHandler(nodeService))
	api.HandleFunc("/pods", handlers.PodsHandler(podService))

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", handlers.HealthHandler())
	mux.HandleFunc("/readyz", handlers.ReadyHandler())
	mux.Handle("/api/v1/", http.StripPrefix("/api/v1", api))

	return mux
}

func runServer(handler http.Handler) {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8001"
	}

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		slog.Info("starting server", "port", port)

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	<-ctx.Done()
	slog.Info("shutting down server")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown failed", "error", err)
	}
}
