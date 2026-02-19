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
	store        *store.Store
	manager      *informers.Manager
	nodesService nodes.Service
	podsService  pods.Service
	server       *http.Server
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
	podsService := pods.NewPodService(podLister, kubeClient)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8001"
	}

	app := &App{store: st, manager: manager, nodesService: nodeService, podsService: podsService}

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           app.setupRouter(),
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      0,
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

	api.HandleFunc("/pods/logs/stream", func(w http.ResponseWriter, r *http.Request) {
		namespace := r.URL.Query().Get("namespace")

		if namespace == "" {
			http.Error(w, "namespace required", http.StatusBadRequest)
			return
		}

		if r.URL.Query().Get("name") != "" {
			http.Error(w, "name is not supported; stream is namespace-scoped", http.StatusBadRequest)
			return
		}

		opts, err := podLogsStreamOptionsFromQuery(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		if opts.Format == podLogsStreamFormatJSON {
			w.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
		} else {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		}
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()

		lastSentAt := time.Time{}

		streamOpts := pods.LogStreamOptions{
			FromStart: opts.FromStart,
			TailLines: opts.TailLines,
		}

		handleRecord := func(record pods.LogStreamRecord) error {
			if !lastSentAt.IsZero() {
				wait := opts.Frequency - time.Since(lastSentAt)
				if wait > 0 {
					timer := time.NewTimer(wait)
					defer timer.Stop()

					select {
					case <-r.Context().Done():
						return r.Context().Err()
					case <-timer.C:
					}
				}
			}

			if err := writePodLogStreamRecord(w, record, opts.Format); err != nil {
				return err
			}

			lastSentAt = time.Now()
			flusher.Flush()
			return nil
		}

		err = a.podsService.StreamNamespaceLogs(r.Context(), namespace, streamOpts, handleRecord)

		if err != nil && r.Context().Err() == nil {
			slog.Warn("pod logs stream ended with error", "namespace", namespace, "error", err)
		}
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", handlers.HealthHandler())
	mux.HandleFunc("/readyz", handlers.ReadyHandler())
	mux.Handle("/api/v1/", http.StripPrefix("/api/v1", api))

	return mux
}
