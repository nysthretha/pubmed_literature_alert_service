package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/publisher"
)

func triggerDigestHandler(pub *publisher.Publisher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		if err := pub.PublishManualTrigger(ctx); err != nil {
			slog.Error("publish manual trigger failed", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		slog.Info("manual digest trigger queued")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "queued"})
	}
}
