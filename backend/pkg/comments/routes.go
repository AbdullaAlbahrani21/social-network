package comments

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"

	"social-network/backend/pkg/middleware"
	"social-network/backend/pkg/posts"
	"social-network/backend/pkg/response"
)

const maxContentLength = 1000

// RegisterRoutes wires the comment endpoints into the router, all behind RequireAuth.
func RegisterRoutes(mux *http.ServeMux, db *sql.DB, hubPush func(int64, interface{})) {
	mux.HandleFunc("POST /api/posts/{id}/comments", middleware.RequireAuth(db, createCommentHandler(db, hubPush)))
	mux.HandleFunc("GET /api/posts/{id}/comments", middleware.RequireAuth(db, listCommentsHandler(db)))
	mux.HandleFunc("DELETE /api/posts/{id}/comments/{commentId}", middleware.RequireAuth(db, deleteCommentHandler(db)))
}

// idFromPath reads a positive numeric id from the URL path, or writes a 400 and returns false.
func idFromPath(w http.ResponseWriter, r *http.Request, name, label string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil || id < 1 {
		response.Error(w, http.StatusBadRequest, "invalid "+label+" id")
		return 0, false
	}
	return id, true
}

// createCommentHandler handles POST /api/posts/{id}/comments on a post the viewer can see.
func createCommentHandler(db *sql.DB, hubPush func(int64, interface{})) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		viewerID, _ := middleware.UserID(r)
		postID, ok := idFromPath(w, r, "id", "post")
		if !ok {
			return
		}

		canView, err := posts.CanViewPost(db, viewerID, postID)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to check post visibility")
			return
		}
		if !canView {
			response.Error(w, http.StatusNotFound, "post not found")
			return
		}

		if err := r.ParseMultipartForm(10 << 20); err != nil {
			response.Error(w, http.StatusBadRequest, "invalid form data")
			return
		}

		content := strings.TrimSpace(r.FormValue("content"))
		if content == "" {
			response.Error(w, http.StatusBadRequest, "content is required")
			return
		}
		if len([]rune(content)) > maxContentLength {
			response.Error(w, http.StatusBadRequest, "content is too long")
			return
		}

		imagePath, err := posts.SaveUploadedImage(r, "image")
		if err != nil {
			response.Error(w, http.StatusBadRequest, err.Error())
			return
		}
		// nil (not "") so an empty upload stores a real SQL NULL.
		var imagePathArg any
		if imagePath != "" {
			imagePathArg = imagePath
		}

		res, err := db.Exec(
			`INSERT INTO comments (post_id, user_id, content, image_path) VALUES (?, ?, ?, ?)`,
			postID, viewerID, content, imagePathArg,
		)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to create comment")
			return
		}
		commentID, _ := res.LastInsertId()

		notifyPostCommented(db, postID, viewerID, hubPush)

		result := map[string]any{
			"id":      commentID,
			"post_id": postID,
			"user_id": viewerID,
			"content": content,
		}
		if imagePath != "" {
			result["image_path"] = imagePath
		}
		response.JSON(w, http.StatusCreated, result)
	}
}

// listCommentsHandler handles GET /api/posts/{id}/comments: the post's comments, oldest first.
func listCommentsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		viewerID, _ := middleware.UserID(r)
		postID, ok := idFromPath(w, r, "id", "post")
		if !ok {
			return
		}

		canView, err := posts.CanViewPost(db, viewerID, postID)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to check post visibility")
			return
		}
		if !canView {
			response.Error(w, http.StatusNotFound, "post not found")
			return
		}

		rows, err := db.Query(`
			SELECT c.id, c.post_id, c.user_id, c.content, c.image_path, c.created_at,
			       u.first_name, u.last_name, u.nickname, u.avatar_path
			FROM comments c
			JOIN users u ON u.id = c.user_id
			WHERE c.post_id = ?
			ORDER BY c.created_at ASC
		`, postID)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to load comments")
			return
		}
		defer rows.Close()

		comments := []map[string]any{}
		for rows.Next() {
			var id, cPostID, userID int64
			var content, createdAt string
			var imagePath, nickname, avatarPath sql.NullString
			var firstName, lastName string

			if err := rows.Scan(&id, &cPostID, &userID, &content, &imagePath, &createdAt,
				&firstName, &lastName, &nickname, &avatarPath); err != nil {
				response.Error(w, http.StatusInternalServerError, "failed to read comments")
				return
			}

			author := map[string]any{
				"first_name": firstName,
				"last_name":  lastName,
			}
			if nickname.Valid {
				author["nickname"] = nickname.String
			}
			if avatarPath.Valid {
				author["avatar_path"] = avatarPath.String
			}

			comment := map[string]any{
				"id":         id,
				"post_id":    cPostID,
				"user_id":    userID,
				"content":    content,
				"created_at": createdAt,
				"author":     author,
			}
			if imagePath.Valid {
				comment["image_path"] = imagePath.String
			}
			comments = append(comments, comment)
		}
		if err := rows.Err(); err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to read comments")
			return
		}

		response.JSON(w, http.StatusOK, comments)
	}
}

// deleteCommentHandler handles DELETE on a comment, only if the viewer wrote it.
func deleteCommentHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		viewerID, _ := middleware.UserID(r)
		postID, ok := idFromPath(w, r, "id", "post")
		if !ok {
			return
		}
		commentID, ok := idFromPath(w, r, "commentId", "comment")
		if !ok {
			return
		}

		// post_id = ? scopes the delete to the post in the URL: without it, any post id deleted the comment.
		res, err := db.Exec(`DELETE FROM comments WHERE id = ? AND post_id = ? AND user_id = ?`, commentID, postID, viewerID)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to delete comment")
			return
		}
		affected, _ := res.RowsAffected()
		if affected == 0 {
			response.Error(w, http.StatusNotFound, "comment not found")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
