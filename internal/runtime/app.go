package runtime

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/JNickson/cluster-telemetry-service/internal/clients"
	"github.com/JNickson/cluster-telemetry-service/internal/handlers"
	"github.com/JNickson/cluster-telemetry-service/internal/informers"
	"github.com/JNickson/cluster-telemetry-service/internal/nodes"
	"github.com/JNickson/cluster-telemetry-service/internal/pods"
	"github.com/JNickson/cluster-telemetry-service/internal/store"
	"github.com/JNickson/cluster-telemetry-service/internal/utils"
	"k8s.io/client-go/rest"
)

type App struct {
	store            *store.Store
	manager          *informers.Manager
	nodesService     nodes.Service
	podsService      pods.Service
	podLogsService   *pods.LogsService
	podLogsCollector *pods.PodLogsCollector
	server           *http.Server
}

func New(cfg *rest.Config) (*App, error) {
	kubeClient, err := clients.NewKubeClient(cfg)
	if err != nil {
		return nil, err
	}

	metricsClient, err := clients.NewMetricsClient(cfg)
	if err != nil {
		return nil, err
	}

	st := store.New()
	manager := informers.NewManager(kubeClient)

	factory := manager.Factory()
	nodeLister := factory.Core().V1().Nodes().Lister()
	podLister := factory.Core().V1().Pods().Lister()

	nodeService := nodes.NewNodeService(nodeLister, podLister, metricsClient)
	podsService := pods.NewPodService(podLister)
	podLogsService := pods.NewLogsService(kubeClient)

	// TODO: Implement real db writer
	var dbWriter pods.DBWriter = nil
	var podLogsCollector *pods.PodLogsCollector

	if dbWriter != nil {
		podLogsCollector = pods.NewPodLogsCollector(
			podLister,
			kubeClient,
			dbWriter,
		)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8001"
	}

	app := &App{
		store:            st,
		manager:          manager,
		nodesService:     nodeService,
		podsService:      podsService,
		podLogsService:   podLogsService,
		podLogsCollector: podLogsCollector,
	}

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           app.setupRouter(),
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
	}

	app.server = server

	return app, nil
}

func (a *App) Start(ctx context.Context) {

	go a.manager.Start(ctx)

	time.Sleep(1 * time.Second)

	go a.startNodeReconciler(ctx)
	go a.startPodReconciler(ctx)

	if a.podLogsCollector != nil {
		go a.podLogsCollector.Start(ctx)
	}

	go func() {
		slog.Info("starting server", "addr", a.server.Addr)
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "error", err)
		}
	}()

	<-ctx.Done()

	slog.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = a.server.Shutdown(shutdownCtx)
}

func (a *App) setupRouter() http.Handler {
	api := http.NewServeMux()

	api.HandleFunc("/nodes", func(w http.ResponseWriter, r *http.Request) {
		utils.WriteJSON(w, http.StatusOK, a.store.ListNodes())
	})

	api.HandleFunc("/pods", func(w http.ResponseWriter, r *http.Request) {
		utils.WriteJSON(w, http.StatusOK, a.store.ListPods())
	})

	api.HandleFunc("/pods/logs", func(w http.ResponseWriter, r *http.Request) {
		namespace := r.URL.Query().Get("namespace")
		name := r.URL.Query().Get("name")

		if namespace == "" || name == "" {
			http.Error(w, "namespace and name required", http.StatusBadRequest)
			return
		}

		logs, err := a.podLogsService.GetLogs(r.Context(), namespace, name, 300)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(logs))
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", handlers.HealthHandler())
	mux.HandleFunc("/readyz", handlers.ReadyHandler())
	mux.Handle("/api/v1/", http.StripPrefix("/api/v1", api))

	return mux
}
