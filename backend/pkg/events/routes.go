package events

import (
	"database/sql"
	"net/http"

	"social-network/backend/pkg/middleware"
)

func RegisterRoutes(mux *http.ServeMux, db *sql.DB, hubPush func(int64, interface{})) {
	mux.HandleFunc("POST /api/groups/{id}/events", middleware.RequireAuth(db, createEventHandler(db, hubPush)))
	mux.HandleFunc("GET /api/groups/{id}/events", middleware.RequireAuth(db, listEventsHandler(db)))
	mux.HandleFunc("POST /api/events/{id}/rsvp", middleware.RequireAuth(db, rsvpHandler(db)))
}
