package httpx

import (
	"context"
	"net/http"
	"time"
)

type Pinger interface {
	Ping(ctx context.Context) error
}

// Health answers /healthz, tech.md section 19.1. It stays outside the API
// error format on purpose: load balancers only read the status code.
func Health(db Pinger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := db.Ping(ctx); err != nil {
			JSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
			return
		}
		JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
}
