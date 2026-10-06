package followers

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/mattn/go-sqlite3"

	"social-network/backend/pkg/chat"
	"social-network/backend/pkg/middleware"
	"social-network/backend/pkg/notifications"
	"social-network/backend/pkg/response"
)

func actorName(ctx context.Context, db *sql.DB, userID int64, whatFor string) (string, bool) {
	name, err := notifications.ActorName(ctx, db, userID)
	if err != nil {
		log.Printf(
			"followers: failed to load name of user %d for %s notification: %v",
			userID,
			whatFor,
			err,
		)
		return "", false
	}

	return name, true
}

type userSummary struct {
	ID         int64   `json:"id"`
	FirstName  string  `json:"firstName"`
	LastName   string  `json:"lastName"`
	AvatarPath *string `json:"avatarPath"`
	Nickname   *string `json:"nickname"`
}

type followRequest struct {
	ID        int64       `json:"id"`
	Follower  userSummary `json:"follower"`
	CreatedAt string      `json:"createdAt"`
}

const followTargetType = "follower"

func RegisterRoutes(mux *http.ServeMux, db *sql.DB, hubPush func(int64, interface{})) {
	mux.HandleFunc("POST /api/users/{id}/follow", middleware.RequireAuth(db, sendFollowRequestHandler(db, hubPush)))
	mux.HandleFunc("POST /api/follow-requests/{id}/accept", middleware.RequireAuth(db, acceptFollowRequestHandler(db, hubPush)))
	mux.HandleFunc("POST /api/follow-requests/{id}/decline", middleware.RequireAuth(db, declineFollowRequestHandler(db)))
	mux.HandleFunc("DELETE /api/users/{id}/follow", middleware.RequireAuth(db, unfollowHandler(db)))
	mux.HandleFunc("GET /api/users/{id}/followers", middleware.RequireAuth(db, listFollowersHandler(db)))
	mux.HandleFunc("GET /api/users/{id}/following", middleware.RequireAuth(db, listFollowingHandler(db)))
	mux.HandleFunc("GET /api/follow-requests", middleware.RequireAuth(db, listFollowRequestsHandler(db)))
	mux.HandleFunc("GET /api/messageable-users", middleware.RequireAuth(db, listMessageableUsersHandler(db)))
}

func sendFollowRequestHandler(db *sql.DB, hubPush func(int64, interface{})) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		followerID, ok := middleware.UserID(r)
		if !ok {
			response.Error(w, http.StatusUnauthorized, "not logged in")
			return
		}

		followingID, err := parseID(r.PathValue("id"))
		if err != nil {
			response.Error(w, http.StatusBadRequest, "invalid user id")
			return
		}
		if followerID == followingID {
			response.Error(w, http.StatusBadRequest, "you cannot follow yourself")
			return
		}

		var isPublic bool
		err = db.QueryRowContext(r.Context(), `
			SELECT is_public
			FROM users
			WHERE id = ?
		`, followingID).Scan(&isPublic)
		if errors.Is(err, sql.ErrNoRows) {
			response.Error(w, http.StatusNotFound, "user not found")
			return
		}
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to load user")
			return
		}

		status := "pending"
		if isPublic {
			status = "accepted"
		}

		result, err := db.ExecContext(r.Context(), `
			INSERT INTO followers (follower_id, following_id, status)
			VALUES (?, ?, ?)
		`, followerID, followingID, status)
		if err != nil {
			var sqliteErr sqlite3.Error
			if errors.As(err, &sqliteErr) && sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique {
				var existingStatus string
				if queryErr := db.QueryRowContext(r.Context(), `
					SELECT status
					FROM followers
					WHERE follower_id = ? AND following_id = ?
				`, followerID, followingID).Scan(&existingStatus); queryErr == nil {
					if existingStatus == "accepted" {
						response.Error(w, http.StatusConflict, "already following this user")
					} else {
						response.Error(w, http.StatusConflict, "follow request already pending")
					}
					return
				}
			}

			response.Error(w, http.StatusInternalServerError, "failed to follow user")
			return
		}

		followID, err := result.LastInsertId()
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to read follow relationship")
			return
		}

		name, named := actorName(r.Context(), db, followerID, "follow")

		if status == "pending" {
			message := "Someone requested to follow you."
			if named {
				message = name + " requested to follow you."
			}

			_ = notifications.Create(db, followingID, notifications.TypeFollowRequest, followerID, followTargetType, followID, hubPush, message)
		} else {
			message := "Someone started following you."
			if named {
				message = name + " started following you."
			}

			_ = notifications.Create(db, followingID, notifications.TypeFollowAccepted, followerID, followTargetType, followID, hubPush, message)
		}

		response.JSON(w, http.StatusCreated, map[string]any{
			"id":     followID,
			"status": status,
		})
	}
}

func acceptFollowRequestHandler(db *sql.DB, hubPush func(int64, interface{})) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		followingID, ok := middleware.UserID(r)
		if !ok {
			response.Error(w, http.StatusUnauthorized, "not logged in")
			return
		}

		requestID, err := parseID(r.PathValue("id"))
		if err != nil {
			response.Error(w, http.StatusBadRequest, "invalid follow request id")
			return
		}

		// One transaction, with the write conditional on the request still being pending: several simultaneous accepts would otherwise all succeed and each notify.
		tx, err := db.BeginTx(r.Context(), nil)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to accept follow request")
			return
		}
		defer tx.Rollback()

		var followerID int64
		err = tx.QueryRowContext(r.Context(), `
			UPDATE followers
			SET status = 'accepted'
			WHERE id = ?
			  AND following_id = ?
			  AND status = 'pending'
			RETURNING follower_id
		`, requestID, followingID).Scan(&followerID)
		if errors.Is(err, sql.ErrNoRows) {
			response.Error(w, http.StatusNotFound, "pending follow request not found")
			return
		}
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to accept follow request")
			return
		}

		if err := tx.Commit(); err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to accept follow request")
			return
		}

		// After the commit: Create also pushes over the websocket, which a rollback could not take back.
		message := "Your follow request was accepted."
		if name, named := actorName(r.Context(), db, followingID, "follow_accepted"); named {
			message = name + " accepted your follow request."
		}

		_ = notifications.Create(db, followerID, notifications.TypeFollowAccepted, followingID, followTargetType, requestID, hubPush, message)

		response.JSON(w, http.StatusOK, map[string]any{
			"id":     requestID,
			"status": "accepted",
		})
	}
}

func declineFollowRequestHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		followingID, ok := middleware.UserID(r)
		if !ok {
			response.Error(w, http.StatusUnauthorized, "not logged in")
			return
		}

		requestID, err := parseID(r.PathValue("id"))
		if err != nil {
			response.Error(w, http.StatusBadRequest, "invalid follow request id")
			return
		}

		result, err := db.ExecContext(r.Context(), `
			DELETE FROM followers
			WHERE id = ?
			  AND following_id = ?
			  AND status = 'pending'
		`, requestID, followingID)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to decline follow request")
			return
		}

		rows, err := result.RowsAffected()
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to decline follow request")
			return
		}
		if rows == 0 {
			response.Error(w, http.StatusNotFound, "pending follow request not found")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func unfollowHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		followerID, ok := middleware.UserID(r)
		if !ok {
			response.Error(w, http.StatusUnauthorized, "not logged in")
			return
		}

		followingID, err := parseID(r.PathValue("id"))
		if err != nil {
			response.Error(w, http.StatusBadRequest, "invalid user id")
			return
		}

		// The delete and what goes with it share a transaction, so a cancelled request cannot leave its notification behind and an unfollow cannot leave the access.
		tx, err := db.BeginTx(r.Context(), nil)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to unfollow user")
			return
		}
		defer tx.Rollback()

		var followID int64
		var status string
		err = tx.QueryRowContext(r.Context(), `
			DELETE FROM followers
			WHERE follower_id = ?
			  AND following_id = ?
			  AND status IN ('accepted', 'pending')
			RETURNING id, status
		`, followerID, followingID).Scan(&followID, &status)
		if errors.Is(err, sql.ErrNoRows) {
			response.Error(w, http.StatusConflict, "you are not following or requesting to follow this user")
			return
		}
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to unfollow user")
			return
		}

		// Ending an accepted follow also ends the follower's places on that author's private-post audiences -- only that author's.
		if status == "accepted" {
			if _, err := tx.ExecContext(r.Context(), `
				DELETE FROM post_visibility
				WHERE user_id = ?
				  AND post_id IN (SELECT id FROM posts WHERE user_id = ?)
			`, followerID, followingID); err != nil {
				response.Error(w, http.StatusInternalServerError, "failed to unfollow user")
				return
			}
		}

		if status == "pending" {
			if err := notifications.DeleteForTarget(
				r.Context(),
				tx,
				followingID,
				notifications.TypeFollowRequest,
				followTargetType,
				followID,
			); err != nil {
				response.Error(w, http.StatusInternalServerError, "failed to cancel follow request")
				return
			}
		}

		if err := tx.Commit(); err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to unfollow user")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func listFollowersHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requesterID, _ := middleware.UserID(r)
		profileID, err := parseID(r.PathValue("id"))
		if err != nil {
			response.Error(w, http.StatusBadRequest, "invalid user id")
			return
		}

		allowed, err := canViewUser(db, requesterID, profileID)
		if errors.Is(err, sql.ErrNoRows) {
			response.Error(w, http.StatusNotFound, "user not found")
			return
		}
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to check profile access")
			return
		}
		if !allowed {
			response.Error(w, http.StatusForbidden, "this profile is private")
			return
		}

		users, err := loadUsers(db, `
			SELECT u.id, u.first_name, u.last_name, u.avatar_path, u.nickname
			FROM followers f
			JOIN users u ON u.id = f.follower_id
			WHERE f.following_id = ? AND f.status = 'accepted'
			ORDER BY u.first_name, u.last_name
		`, profileID)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to load followers")
			return
		}

		response.JSON(w, http.StatusOK, users)
	}
}

func listFollowingHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requesterID, _ := middleware.UserID(r)
		profileID, err := parseID(r.PathValue("id"))
		if err != nil {
			response.Error(w, http.StatusBadRequest, "invalid user id")
			return
		}

		allowed, err := canViewUser(db, requesterID, profileID)
		if errors.Is(err, sql.ErrNoRows) {
			response.Error(w, http.StatusNotFound, "user not found")
			return
		}
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to check profile access")
			return
		}
		if !allowed {
			response.Error(w, http.StatusForbidden, "this profile is private")
			return
		}

		users, err := loadUsers(db, `
			SELECT u.id, u.first_name, u.last_name, u.avatar_path, u.nickname
			FROM followers f
			JOIN users u ON u.id = f.following_id
			WHERE f.follower_id = ? AND f.status = 'accepted'
			ORDER BY u.first_name, u.last_name
		`, profileID)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to load following")
			return
		}

		response.JSON(w, http.StatusOK, users)
	}
}

func listFollowRequestsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserID(r)
		if !ok {
			response.Error(w, http.StatusUnauthorized, "not logged in")
			return
		}

		rows, err := db.QueryContext(r.Context(), `
			SELECT
				f.id,
				u.id,
				u.first_name,
				u.last_name,
				u.avatar_path,
				u.nickname,
				f.created_at
			FROM followers f
			JOIN users u ON u.id = f.follower_id
			WHERE f.following_id = ?
			  AND f.status = 'pending'
			ORDER BY f.created_at DESC
		`, userID)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to load follow requests")
			return
		}
		defer rows.Close()

		requests := make([]followRequest, 0)
		for rows.Next() {
			var request followRequest
			if err := rows.Scan(
				&request.ID,
				&request.Follower.ID,
				&request.Follower.FirstName,
				&request.Follower.LastName,
				&request.Follower.AvatarPath,
				&request.Follower.Nickname,
				&request.CreatedAt,
			); err != nil {
				returnError(w, "failed to load follow requests")
				return
			}
			requests = append(requests, request)
		}
		if err := rows.Err(); err != nil {
			returnError(w, "failed to load follow requests")
			return
		}

		response.JSON(w, http.StatusOK, requests)
	}
}

func parseID(value string) (int64, error) {
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("invalid id")
	}
	return id, nil
}

func canViewUser(db *sql.DB, requesterID, profileID int64) (bool, error) {
	var isPublic bool
	if err := db.QueryRow(`SELECT is_public FROM users WHERE id = ?`, profileID).Scan(&isPublic); err != nil {
		return false, err
	}
	if requesterID == profileID || isPublic {
		return true, nil
	}

	return IsFollowing(db, requesterID, profileID)
}

func IsFollowing(db *sql.DB, followerID, followingID int64) (bool, error) {
	var isFollower bool
	err := db.QueryRow(`
		SELECT EXISTS (
			SELECT 1
			FROM followers
			WHERE follower_id = ?
			  AND following_id = ?
			  AND status = 'accepted'
		)
	`, followerID, followingID).Scan(&isFollower)
	return isFollower, err
}

func loadUsers(db *sql.DB, query string, id int64) ([]userSummary, error) {
	rows, err := db.Query(query, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]userSummary, 0)
	for rows.Next() {
		var user userSummary
		if err := rows.Scan(&user.ID, &user.FirstName, &user.LastName, &user.AvatarPath, &user.Nickname); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func returnError(w http.ResponseWriter, message string) {
	response.Error(w, http.StatusInternalServerError, message)
}

func listMessageableUsersHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserID(r)
		if !ok {
			response.Error(w, http.StatusUnauthorized, "not logged in")
			return
		}

		// CanMessageCondition binds the sender as ?1 and the self-exclusion reuses it, so this still binds a single value.
		users, err := loadUsers(db, `
			SELECT u.id, u.first_name, u.last_name, u.avatar_path, u.nickname
			FROM users u
			WHERE u.id != ?1
			  AND `+chat.CanMessageCondition+`
			ORDER BY u.first_name, u.last_name
		`, userID)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to load messageable users")
			return
		}

		response.JSON(w, http.StatusOK, users)
	}
}
