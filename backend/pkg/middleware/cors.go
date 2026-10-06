package middleware

import (
	"net/http"

	"social-network/backend/pkg/config"
)

// One variable for both the CORS headers and pkg/websocket's handshake check, so the two can never disagree about who is allowed in.
var AllowedOrigin = config.Env("ALLOWED_ORIGIN", "http://localhost:5173")

func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(
			"Access-Control-Allow-Origin",
			AllowedOrigin,
		)

		w.Header().Set(
			"Access-Control-Allow-Credentials",
			"true",
		)

		w.Header().Set(
			"Access-Control-Allow-Headers",
			"Content-Type",
		)

		w.Header().Set(
			"Access-Control-Allow-Methods",
			"GET, POST, PUT, DELETE, OPTIONS",
		)

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
