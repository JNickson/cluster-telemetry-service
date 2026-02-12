package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/JNickson/cluster-telemetry-service/internal/nodes"
)

func NodesHandler(svc nodes.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		nodes, err := svc.FetchNodes(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(nodes)
	}
}
