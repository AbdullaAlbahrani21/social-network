package groups

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"social-network/backend/pkg/middleware"
	"social-network/backend/pkg/response"
)

// Existence is checked before membership, so a bad group id reads as 404 rather than a 403 that would imply the group is real.
func requireGroupMemberAccess(
	w http.ResponseWriter,
	r *http.Request,
	db *sql.DB,
	forbidden string,
) (groupID int64, userID int64, ok bool) {
	userID, ok = middleware.UserID(r)
	if !ok {
		response.Error(
			w,
			http.StatusUnauthorized,
			"not logged in",
		)
		return 0, 0, false
	}

	groupID, ok = idFromPath(w, r, "id", "group")
	if !ok {
		return 0, 0, false
	}

	exists, err := groupExists(r.Context(), db, groupID)
	if err != nil {
		response.Error(
			w,
			http.StatusInternalServerError,
			"failed to load group",
		)
		return 0, 0, false
	}
	if !exists {
		response.Error(
			w,
			http.StatusNotFound,
			"group not found",
		)
		return 0, 0, false
	}

	member, err := isGroupMember(r.Context(), db, groupID, userID)
	if err != nil {
		response.Error(
			w,
			http.StatusInternalServerError,
			"failed to check membership",
		)
		return 0, 0, false
	}
	if !member {
		response.Error(
			w,
			http.StatusForbidden,
			forbidden,
		)
		return 0, 0, false
	}

	return groupID, userID, true
}

func requireGroupPostAccess(
	w http.ResponseWriter,
	r *http.Request,
	db *sql.DB,
) (groupID int64, userID int64, ok bool) {
	return requireGroupMemberAccess(
		w,
		r,
		db,
		"only group members can view or write group posts",
	)
}

func createGroupPostHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		groupID, userID, ok := requireGroupPostAccess(w, r, db)
		if !ok {
			return
		}

		var body struct {
			Content string `json:"content"`
		}

		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			response.Error(
				w,
				http.StatusBadRequest,
				"invalid request body",
			)
			return
		}

		content := strings.TrimSpace(body.Content)

		if content == "" {
			response.Error(
				w,
				http.StatusBadRequest,
				"content is required",
			)
			return
		}

		if len([]rune(content)) > maxPostContentLength {
			response.Error(
				w,
				http.StatusBadRequest,
				"content is too long",
			)
			return
		}

		post, err := createGroupPost(
			r.Context(),
			db,
			groupID,
			userID,
			content,
		)
		if err != nil {
			response.Error(
				w,
				http.StatusInternalServerError,
				"failed to create post",
			)
			return
		}

		response.JSON(
			w,
			http.StatusCreated,
			post,
		)
	}
}

func listGroupPostsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		groupID, userID, ok := requireGroupPostAccess(w, r, db)
		if !ok {
			return
		}

		posts, err := listGroupPosts(r.Context(), db, userID, groupID)
		if err != nil {
			response.Error(
				w,
				http.StatusInternalServerError,
				"failed to load posts",
			)
			return
		}

		response.JSON(
			w,
			http.StatusOK,
			posts,
		)
	}
}
