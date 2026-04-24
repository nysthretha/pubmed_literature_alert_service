package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/publisher"
)

// healthzHandler returns 200 when Postgres and RabbitMQ are both reachable,
// 503 otherwise. No auth — designed to be pinged by external monitors.
// Results are computed fresh on every call (no caching) so a cached-OK never
// masks a live failure.
func healthzHandler(db *sql.DB, pub *publisher.Publisher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()

		checks := map[string]string{}
		healthy := true

		if err := db.PingContext(ctx); err != nil {
			checks["postgres"] = "fail"
			healthy = false
		} else {
			checks["postgres"] = "ok"
		}

		if pub == nil || !pub.IsHealthy() {
			checks["rabbitmq"] = "fail"
			healthy = false
		} else {
			checks["rabbitmq"] = "ok"
		}

		status := "ok"
		code := http.StatusOK
		if !healthy {
			status = "degraded"
			code = http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": status,
			"checks": checks,
		})
	}
}
