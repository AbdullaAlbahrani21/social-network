package main

import (
	"log"
	"net/http"

	"social-network/backend/pkg/auth"
	"social-network/backend/pkg/chat"
	"social-network/backend/pkg/comments"
	"social-network/backend/pkg/config"
	"social-network/backend/pkg/db/sqlite"
	"social-network/backend/pkg/events"
	"social-network/backend/pkg/followers"
	"social-network/backend/pkg/groups"
	"social-network/backend/pkg/media"
	"social-network/backend/pkg/middleware"
	"social-network/backend/pkg/notifications"
	"social-network/backend/pkg/posts"
	"social-network/backend/pkg/profile"
	"social-network/backend/pkg/websocket"
)

func main() {
	address := ":" + config.Env("PORT", "8080")
	dbPath := config.Env("DB_PATH", "./social_network.db")

	db, err := sqlite.Connect(dbPath)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	hub := websocket.NewHub(db)
	mux := http.NewServeMux()

	media.RegisterRoutes(mux, db, media.Dir)

	auth.RegisterRoutes(mux, db, hub.CloseSession)
	profile.RegisterRoutes(mux, db)
	followers.RegisterRoutes(mux, db, hub.Push)
	posts.RegisterRoutes(mux, db, hub.Push)
	comments.RegisterRoutes(mux, db, hub.Push)
	groups.RegisterRoutes(mux, db, hub.Push)
	events.RegisterRoutes(mux, db, hub.Push)

	typing := chat.NewTypingRelay(db, hub.Push)
	hub.HandleInbound(chat.TypingFrameType, typing.HandleFrame)

	chat.RegisterRoutes(mux, db, hub.Push, typing.SentMessage)

	notifications.RegisterRoutes(mux, db)
	websocket.RegisterRoutes(mux, hub)

	log.Printf(
		"listening on %s (db %s, uploads %s, origin %s)",
		address,
		dbPath,
		media.Dir,
		middleware.AllowedOrigin,
	)
	if err := http.ListenAndServe(
		address,
		middleware.CORS(mux),
	); err != nil {
		log.Fatal(err)
	}
}
