package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/JNickson/cluster-telemetry-service/internal/pods"
)

func PodsHandler(svc pods.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pods, err := svc.FetchPods(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(pods)
	}
}
