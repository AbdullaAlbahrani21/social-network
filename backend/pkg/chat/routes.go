package chat

import (
	"database/sql"
	"net/http"

	"social-network/backend/pkg/middleware"
)

type SentMessageFunc func(senderID int64, targetType string, targetID int64)

func RegisterRoutes(mux *http.ServeMux, db *sql.DB, hubPush func(int64, interface{}), sentMessage SentMessageFunc) {
	mux.HandleFunc("GET /api/conversations", middleware.RequireAuth(db, ListConversationsHandler(db)))
	mux.HandleFunc("GET /api/messages/{userID}", middleware.RequireAuth(db, GetConversationHandler(db)))
	mux.HandleFunc("POST /api/messages", middleware.RequireAuth(db, SendMessageHandler(db, hubPush, sentMessage)))
	mux.HandleFunc("POST /api/messages/read", middleware.RequireAuth(db, MarkReadHandler(db)))

	mux.HandleFunc("POST /api/groups/messages", middleware.RequireAuth(db, HandleSendGroupMessage(db, hubPush, sentMessage)))
	mux.HandleFunc("GET /api/groups/{groupID}/messages", middleware.RequireAuth(db, GetGroupConversationHandler(db)))
}
