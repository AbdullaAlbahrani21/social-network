package chat

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"social-network/backend/pkg/groups"
	"social-network/backend/pkg/middleware"
	"social-network/backend/pkg/paging"
	"social-network/backend/pkg/response"
)

const maxContentLength = 2000

func idFromPath(w http.ResponseWriter, r *http.Request, name, label string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil || id < 1 {
		response.Error(w, http.StatusBadRequest, "invalid "+label+" id")
		return 0, false
	}
	return id, true
}

func SendMessageHandler(db *sql.DB, hubPush func(int64, interface{}), sentMessage SentMessageFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserID(r)
		if !ok {
			response.Error(w, http.StatusUnauthorized, "not logged in")
			return
		}

		var req struct {
			ToID    int64  `json:"to_id"`
			Content string `json:"content"`
			TempID  string `json:"temp_id,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			response.Error(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if req.ToID == userID {
			response.Error(w, http.StatusBadRequest, "you cannot message yourself")
			return
		}

		content := strings.TrimSpace(req.Content)
		if content == "" {
			response.Error(w, http.StatusBadRequest, "content is required")
			return
		}
		if len([]rune(content)) > maxContentLength {
			response.Error(w, http.StatusBadRequest, "content is too long")
			return
		}

		allowed, err := CheckCanMessage(db, userID, req.ToID)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to check messaging permission")
			return
		}
		if !allowed {
			response.Error(w, http.StatusForbidden, "you are not allowed to message this user")
			return
		}

		out, err := PersistOneToOne(db, userID, req.ToID, content)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to send message")
			return
		}
		out.TempID = req.TempID

		// Clearing the sender's typing throttle here: resuming inside the next second would otherwise be dropped as a repeat.
		if sentMessage != nil {
			sentMessage(userID, TypingTargetDM, req.ToID)
		}

		if hubPush != nil {
			hubPush(req.ToID, map[string]interface{}{"type": "message", "message": out, "temp_id": req.TempID})
			hubPush(userID, map[string]interface{}{"type": "message", "message": out, "temp_id": req.TempID})
		}

		notifyMessageReceived(db, req.ToID, userID, out.ID, hubPush)

		response.JSON(w, http.StatusCreated, out)
	}
}

func GetConversationHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserID(r)
		if !ok {
			response.Error(w, http.StatusUnauthorized, "not logged in")
			return
		}

		otherID, ok := idFromPath(w, r, "userID", "user")
		if !ok {
			return
		}

		limit, offset := paging.Parse(r)

		// id breaks ties: CURRENT_TIMESTAMP has one-second resolution, so without a total order same-second messages repeat or vanish across a page boundary.
		rows, err := db.Query(`
			SELECT id, sender_id, receiver_id, content, read_at, created_at
			FROM messages
			WHERE (sender_id = ? AND receiver_id = ?) OR (sender_id = ? AND receiver_id = ?)
			ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?`, userID, otherID, otherID, userID, limit, offset)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to load messages")
			return
		}
		defer rows.Close()

		out := []Message{}
		for rows.Next() {
			var m Message
			var readAt sql.NullTime
			if err := rows.Scan(&m.ID, &m.SenderID, &m.ReceiverID, &m.Content, &readAt, &m.CreatedAt); err != nil {
				response.Error(w, http.StatusInternalServerError, "failed to load messages")
				return
			}
			if readAt.Valid {
				t := readAt.Time
				m.ReadAt = &t
			}
			out = append(out, m)
		}
		// rows.Next() returns false on error as well as at the end, so a mid-iteration failure would otherwise look like a short page.
		if err := rows.Err(); err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to load messages")
			return
		}

		response.JSON(w, http.StatusOK, out)
	}
}

func ListConversationsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserID(r)
		if !ok {
			response.Error(w, http.StatusUnauthorized, "not logged in")
			return
		}

		conversations, err := ListConversations(db, userID)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to load conversations")
			return
		}

		response.JSON(w, http.StatusOK, conversations)
	}
}

func MarkReadHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserID(r)
		if !ok {
			response.Error(w, http.StatusUnauthorized, "not logged in")
			return
		}

		var req struct {
			MessageIDs []int64 `json:"message_ids"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			response.Error(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if len(req.MessageIDs) == 0 {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		tx, err := db.Begin()
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to mark messages read")
			return
		}
		stmt, err := tx.Prepare("UPDATE messages SET read_at = CURRENT_TIMESTAMP WHERE id = ? AND receiver_id = ?")
		if err != nil {
			tx.Rollback()
			response.Error(w, http.StatusInternalServerError, "failed to mark messages read")
			return
		}
		defer stmt.Close()

		for _, id := range req.MessageIDs {
			if _, err := stmt.Exec(id, userID); err != nil {
				tx.Rollback()
				response.Error(w, http.StatusInternalServerError, "failed to mark messages read")
				return
			}
		}
		// Commit is where a write failure surfaces; ignoring it reported 204 for updates that were never made durable.
		if err := tx.Commit(); err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to mark messages read")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func HandleSendGroupMessage(db *sql.DB, hubPush func(int64, interface{}), sentMessage SentMessageFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserID(r)
		if !ok {
			response.Error(w, http.StatusUnauthorized, "not logged in")
			return
		}

		var req struct {
			GroupID int64  `json:"group_id"`
			Content string `json:"content"`
			TempID  string `json:"temp_id,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			response.Error(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if req.GroupID < 1 {
			response.Error(w, http.StatusBadRequest, "group_id is required")
			return
		}

		// Existence before membership: answering an unknown group with the non-member 403 implied it exists.
		exists, err := groups.GroupExists(r.Context(), db, req.GroupID)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to load group")
			return
		}
		if !exists {
			response.Error(w, http.StatusNotFound, "group not found")
			return
		}

		content := strings.TrimSpace(req.Content)
		if content == "" {
			response.Error(w, http.StatusBadRequest, "content is required")
			return
		}
		if len([]rune(content)) > maxContentLength {
			response.Error(w, http.StatusBadRequest, "content is too long")
			return
		}

		getMembers := func(gID int64) ([]int64, error) {
			return groups.GroupMemberIDs(db, gID)
		}

		out, members, err := PersistGroupMessage(db, userID, req.GroupID, content, getMembers)
		if err != nil {
			if errors.Is(err, ErrNotGroupMember) {
				response.Error(w, http.StatusForbidden, "only group members can view or send group messages")
				return
			}
			response.Error(w, http.StatusInternalServerError, "failed to send message")
			return
		}
		out.TempID = req.TempID

		if sentMessage != nil {
			sentMessage(userID, TypingTargetGroup, req.GroupID)
		}

		if hubPush != nil {
			for _, memberID := range members {
				hubPush(memberID, map[string]interface{}{
					"type":    "group_message",
					"message": out,
					"temp_id": req.TempID,
				})
			}
		}

		response.JSON(w, http.StatusCreated, out)
	}
}

func GetGroupConversationHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserID(r)
		if !ok {
			response.Error(w, http.StatusUnauthorized, "not logged in")
			return
		}

		groupID, ok := idFromPath(w, r, "groupID", "group")
		if !ok {
			return
		}

		exists, err := groups.GroupExists(r.Context(), db, groupID)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to load group")
			return
		}
		if !exists {
			response.Error(w, http.StatusNotFound, "group not found")
			return
		}

		member, err := groups.IsGroupMember(db, groupID, userID)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to check membership")
			return
		}
		if !member {
			response.Error(w, http.StatusForbidden, "only group members can view or send group messages")
			return
		}

		limit, offset := paging.Parse(r)

		// id breaks same-second ties; see GetConversationHandler.
		rows, err := db.Query(`
			SELECT id, group_id, sender_id, content, created_at
			FROM group_messages
			WHERE group_id = ?
			ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?`, groupID, limit, offset)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to load messages")
			return
		}
		defer rows.Close()

		out := []OutgoingMessage{}
		for rows.Next() {
			var m OutgoingMessage
			if err := rows.Scan(&m.ID, &m.GroupID, &m.SenderID, &m.Content, &m.CreatedAt); err != nil {
				response.Error(w, http.StatusInternalServerError, "failed to load messages")
				return
			}
			out = append(out, m)
		}
		if err := rows.Err(); err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to load messages")
			return
		}

		response.JSON(w, http.StatusOK, out)
	}
}
