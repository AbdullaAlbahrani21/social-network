package notifications

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"

	"social-network/backend/pkg/middleware"
)

type Type string

const (
	TypeFollowRequest     Type = "follow_request"
	TypeFollowAccepted    Type = "follow_accepted"
	TypeGroupInvite       Type = "group_invite"
	TypeGroupJoinRequest  Type = "group_join_request"
	TypeGroupEventCreated Type = "group_event_created"
	TypePostLiked         Type = "post_liked"
	TypeCommentCreated    Type = "comment_created"
	TypeMessageReceived   Type = "message_received"

	TypeInvitationDeclined Type = "group_invite_declined"
)

type Notification struct {
	ID         int64  `json:"id"`
	UserID     int64  `json:"user_id"`
	ActorID    int64  `json:"actor_id"`
	Type       string `json:"type"`
	TargetType string `json:"target_type"`
	TargetID   int64  `json:"target_id"`
	Message    string `json:"message"`
	CreatedAt  string `json:"created_at"`
	IsRead     bool   `json:"is_read"`
}

func RegisterRoutes(mux *http.ServeMux, db *sql.DB) {
	mux.HandleFunc("GET /api/notifications", middleware.RequireAuth(db, ListNotificationsHandler(db)))
	mux.HandleFunc("POST /api/notifications/{id}/read", middleware.RequireAuth(db, MarkNotificationReadHandler(db)))
	// A literal segment always beats a wildcard in net/http's patterns, so "/read" and "{id}/read" can never be ambiguous.
	mux.HandleFunc("POST /api/notifications/read", middleware.RequireAuth(db, MarkAllNotificationsReadHandler(db)))
}

func messageFor(notifType Type, actorID, targetID int64) string {
	switch notifType {
	case TypeFollowRequest:
		return fmt.Sprintf("User %d requested to follow you.", actorID)
	case TypeFollowAccepted:
		return fmt.Sprintf("User %d accepted your follow request.", actorID)
	case TypeGroupInvite:
		return fmt.Sprintf("You were invited to group %d.", targetID)
	case TypeGroupJoinRequest:
		return fmt.Sprintf("User %d requested to join your group.", actorID)
	case TypeInvitationDeclined:
		return fmt.Sprintf("User %d declined your group invitation.", actorID)
	case TypeGroupEventCreated:
		return fmt.Sprintf("A new event was posted in group %d.", targetID)
	case TypePostLiked:
		return fmt.Sprintf("User %d liked your post.", actorID)
	case TypeCommentCreated:
		return fmt.Sprintf("User %d commented on your post.", actorID)
	case TypeMessageReceived:
		return fmt.Sprintf("User %d sent you a message.", actorID)
	default:
		return "New notification"
	}
}

type querier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func ActorName(ctx context.Context, q querier, userID int64) (string, error) {
	var name string

	err := q.QueryRowContext(
		ctx,
		`SELECT first_name || ' ' || last_name FROM users WHERE id = ?`,
		userID,
	).Scan(&name)

	return name, err
}

func Create(db *sql.DB, userID int64, notifType Type, actorID int64, targetType string, targetID int64, hubPush func(int64, interface{}), message string) error {
	messageText := message
	if messageText == "" {
		messageText = messageFor(notifType, actorID, targetID)
	}

	// RETURNING gives the id and the DEFAULT CURRENT_TIMESTAMP the database stored, so the pushed payload sorts correctly against the fetched list.
	var notifID int64
	var createdAt string
	err := db.QueryRow(`
		INSERT INTO notifications (user_id, actor_id, type, target_type, target_id, message, is_read)
		VALUES (?, ?, ?, ?, ?, ?, 0)
		RETURNING id, created_at`,
		userID, actorID, string(notifType), targetType, targetID, messageText,
	).Scan(&notifID, &createdAt)
	if err != nil {
		return err
	}

	if err := pruneToCap(db, userID); err != nil {
		log.Printf("notifications: failed to prune user %d's notifications to the cap: %v", userID, err)
	}

	push(hubPush, Notification{
		ID:         notifID,
		UserID:     userID,
		ActorID:    actorID,
		Type:       string(notifType),
		TargetType: targetType,
		TargetID:   targetID,
		Message:    messageText,
		CreatedAt:  createdAt,
		IsRead:     false,
	}, true)

	return nil
}

func CreateOrRefresh(db *sql.DB, userID int64, notifType Type, actorID int64, targetType string, targetID int64, hubPush func(int64, interface{}), message string) error {
	messageText := message
	if messageText == "" {
		messageText = messageFor(notifType, actorID, targetID)
	}

	// ORDER BY ... LIMIT 1 sits in a subquery: SQLite accepts them directly on an UPDATE only in builds compiled with SQLITE_ENABLE_UPDATE_DELETE_LIMIT.
	var (
		notifID   int64
		createdAt string
	)
	err := db.QueryRow(`
		UPDATE notifications
		SET message = ?, target_id = ?, created_at = CURRENT_TIMESTAMP
		WHERE id = (
			SELECT id FROM notifications
			WHERE user_id = ? AND actor_id = ? AND type = ? AND is_read = 0
			ORDER BY created_at DESC, id DESC
			LIMIT 1
		)
		RETURNING id, created_at`,
		messageText, targetID, userID, actorID, string(notifType),
	).Scan(&notifID, &createdAt)

	if errors.Is(err, sql.ErrNoRows) {
		return Create(db, userID, notifType, actorID, targetType, targetID, hubPush, messageText)
	}
	if err != nil {
		return err
	}

	push(hubPush, Notification{
		ID:         notifID,
		UserID:     userID,
		ActorID:    actorID,
		Type:       string(notifType),
		TargetType: targetType,
		TargetID:   targetID,
		Message:    messageText,
		CreatedAt:  createdAt,
		IsRead:     false,
	}, false)

	return nil
}

func push(hubPush func(int64, interface{}), n Notification, isNew bool) {
	if hubPush == nil {
		return
	}

	hubPush(n.UserID, map[string]interface{}{
		"type":         "notification",
		"notification": n,
		"is_new":       isNew,
	})
}

const MaxStoredPerUser = 50

func pruneToCap(db *sql.DB, userID int64) error {
	// MAX(0, ...) matters: a negative LIMIT means "no limit" in SQLite, so a user under the cap would have every read notification deleted.
	_, err := db.Exec(`
		DELETE FROM notifications
		WHERE id IN (
			SELECT id FROM notifications
			WHERE user_id = ? AND is_read = 1
			ORDER BY created_at ASC, id ASC
			LIMIT MAX(0, (SELECT COUNT(*) FROM notifications WHERE user_id = ?) - ?)
		)`,
		userID, userID, MaxStoredPerUser,
	)
	return err
}

type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func DeleteForTarget(ctx context.Context, q execer, userID int64, notifType Type, targetType string, targetID int64) error {
	_, err := q.ExecContext(ctx, `
		DELETE FROM notifications
		WHERE user_id = ?
		  AND type = ?
		  AND target_type = ?
		  AND target_id = ?`,
		userID, string(notifType), targetType, targetID,
	)
	return err
}

// The actor column is needed wherever one target carries the same notification type from several people -- every liker of one post.
func DeleteForTargetFromActor(ctx context.Context, q execer, userID int64, notifType Type, targetType string, targetID, actorID int64) error {
	_, err := q.ExecContext(ctx, `
		DELETE FROM notifications
		WHERE user_id = ?
		  AND type = ?
		  AND target_type = ?
		  AND target_id = ?
		  AND actor_id = ?`,
		userID, string(notifType), targetType, targetID, actorID,
	)
	return err
}
