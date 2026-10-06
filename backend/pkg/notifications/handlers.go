package notifications

import (
	"database/sql"
	"net/http"
	"strconv"

	"social-network/backend/pkg/middleware"
	"social-network/backend/pkg/paging"
	"social-network/backend/pkg/response"
)

func ListNotificationsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserID(r)
		if !ok {
			response.Error(w, http.StatusUnauthorized, "not logged in")
			return
		}

		limit, offset := paging.Parse(r)

		// id breaks ties: CURRENT_TIMESTAMP has one-second resolution, so without a total order same-second notifications repeat or vanish across a page boundary.
		rows, err := db.Query(`
			SELECT id, user_id, actor_id, type, target_type, target_id, message, created_at, is_read
			FROM notifications
			WHERE user_id = ?
			ORDER BY created_at DESC, id DESC
			LIMIT ? OFFSET ?`, userID, limit, offset)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to load notifications")
			return
		}
		defer rows.Close()

		out := []Notification{}
		for rows.Next() {
			var n Notification
			var actorID sql.NullInt64
			var message sql.NullString

			if err := rows.Scan(&n.ID, &n.UserID, &actorID, &n.Type, &n.TargetType, &n.TargetID, &message, &n.CreatedAt, &n.IsRead); err != nil {
				response.Error(w, http.StatusInternalServerError, "failed to load notifications")
				return
			}

			if actorID.Valid {
				n.ActorID = actorID.Int64
			}
			if message.Valid {
				n.Message = message.String
			}
			out = append(out, n)
		}
		// rows.Next() also returns false on error, so a mid-iteration failure would otherwise be served as a successful short page.
		if err := rows.Err(); err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to load notifications")
			return
		}

		response.JSON(w, http.StatusOK, out)
	}
}

func MarkNotificationReadHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserID(r)
		if !ok {
			response.Error(w, http.StatusUnauthorized, "not logged in")
			return
		}

		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || id < 1 {
			response.Error(w, http.StatusBadRequest, "invalid notification id")
			return
		}

		_, err = db.Exec("UPDATE notifications SET is_read = 1 WHERE id = ? AND user_id = ?", id, userID)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to mark notification read")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func MarkAllNotificationsReadHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserID(r)
		if !ok {
			response.Error(w, http.StatusUnauthorized, "not logged in")
			return
		}

		// is_read = 0 in the WHERE, not just the SET: otherwise RowsAffected counts rows that were already read.
		res, err := db.Exec(
			"UPDATE notifications SET is_read = 1 WHERE user_id = ? AND is_read = 0",
			userID,
		)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to mark notifications read")
			return
		}

		updated, err := res.RowsAffected()
		if err != nil {
			updated = 0
		}

		response.JSON(w, http.StatusOK, map[string]int64{"updated": updated})
	}
}
