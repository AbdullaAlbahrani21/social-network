package posts

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"social-network/backend/pkg/followers"
	"social-network/backend/pkg/middleware"
	"social-network/backend/pkg/response"
)

// RegisterRoutes wires every post and like endpoint into the router, all behind RequireAuth.
func RegisterRoutes(mux *http.ServeMux, db *sql.DB, hubPush func(int64, interface{})) {
	mux.HandleFunc("POST /api/posts", middleware.RequireAuth(db, createPostHandler(db)))
	mux.HandleFunc("GET /api/posts", middleware.RequireAuth(db, feedHandler(db)))
	mux.HandleFunc("GET /api/posts/{id}", middleware.RequireAuth(db, getPostHandler(db)))
	mux.HandleFunc("DELETE /api/posts/{id}", middleware.RequireAuth(db, deletePostHandler(db)))
	mux.HandleFunc("POST /api/posts/{id}/like", middleware.RequireAuth(db, likePostHandler(db, hubPush)))
	mux.HandleFunc("DELETE /api/posts/{id}/like", middleware.RequireAuth(db, unlikePostHandler(db)))
}

const maxContentLength = 5000

var validPrivacy = map[string]bool{
	"public":         true,
	"almost_private": true,
	"private":        true,
}

// Takes 4 "?" args, all viewerID, and reads the author's users row: callers must join it as u.
const visibilityClause = `
	p.user_id = ?
	OR (p.group_id IS NULL AND p.privacy = 'public' AND u.is_public = 1)
	OR (p.group_id IS NULL AND p.privacy IN ('public', 'almost_private') AND p.user_id IN (
	    SELECT following_id FROM followers WHERE follower_id = ? AND status = 'accepted'
	))
	OR (p.group_id IS NULL AND p.privacy = 'private' AND p.id IN (
	    SELECT post_id FROM post_visibility WHERE user_id = ?
	))
	-- Group posts are visible to every member of the post's group. This must
	-- stay in sync with groups.IsGroupMember (pkg/groups/membership.go, whose
	-- query lives in isGroupMember in pkg/groups/invitations_db.go): any
	-- group_members row counts and role is irrelevant. It is inline SQL rather
	-- than a call to that function because this clause filters the multi-row
	-- feed query, where a per-row Go call would mean N+1 queries.
	OR (p.group_id IS NOT NULL AND EXISTS (
	    SELECT 1 FROM group_members gm WHERE gm.group_id = p.group_id AND gm.user_id = ?
	))
`

// CanViewPost reports whether the viewer may see the post, using the shared visibilityClause.
func CanViewPost(db *sql.DB, viewerID, postID int64) (bool, error) {
	var exists bool
	err := db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM posts p JOIN users u ON u.id = p.user_id
			WHERE p.id = ? AND (`+visibilityClause+`)
		)
	`, postID, viewerID, viewerID, viewerID, viewerID).Scan(&exists)
	return exists, err
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

// Kept apart from likeColumns because writeLikeState scans exactly those two columns.
const commentCountColumn = `
	(SELECT COUNT(*) FROM comments c WHERE c.post_id = p.id) AS comment_count`

type scanner interface {
	Scan(dest ...any) error
}

// scanPost turns one row of the post SELECT into the JSON shape the frontend expects.
func scanPost(s scanner) (map[string]any, error) {
	var id, userID int64
	var groupID sql.NullInt64
	var content, privacy, createdAt string
	var imagePath, nickname, avatarPath sql.NullString
	var firstName, lastName string
	var likeCount, commentCount int64
	var viewerHasLiked bool

	if err := s.Scan(&id, &userID, &groupID, &content, &imagePath, &privacy, &createdAt,
		&firstName, &lastName, &nickname, &avatarPath, &likeCount, &viewerHasLiked, &commentCount); err != nil {
		return nil, err
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

	post := map[string]any{
		"id":               id,
		"user_id":          userID,
		"content":          content,
		"privacy":          privacy,
		"created_at":       createdAt,
		"author":           author,
		"like_count":       likeCount,
		"viewer_has_liked": viewerHasLiked,
		"comment_count":    commentCount,
	}
	if groupID.Valid {
		post["group_id"] = groupID.Int64
	}
	if imagePath.Valid {
		post["image_path"] = imagePath.String
	}
	return post, nil
}

// createPostHandler handles POST /api/posts: validates the form, saves the image and inserts the post.
func createPostHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := middleware.UserID(r)

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

		privacy := r.FormValue("privacy")
		if privacy == "" {
			privacy = "public"
		}
		if !validPrivacy[privacy] {
			response.Error(w, http.StatusBadRequest, "privacy must be public, almost_private, or private")
			return
		}

		var audience []int64
		if privacy == "private" {
			ids, err := resolveAudience(db, userID, r.Form["visibility_user_ids"])
			var invalid audienceError
			if errors.As(err, &invalid) {
				response.Error(w, http.StatusBadRequest, invalid.message)
				return
			}
			if err != nil {
				response.Error(w, http.StatusInternalServerError, "failed to check post audience")
				return
			}
			audience = ids
		}

		imagePath, err := SaveUploadedImage(r, "image")
		if err != nil {
			response.Error(w, http.StatusBadRequest, err.Error())
			return
		}

		postID, err := insertPost(db, userID, content, privacy, imagePath, audience)
		if err != nil {
			removeUploadedImage(imagePath)
			response.Error(w, http.StatusInternalServerError, "failed to create post")
			return
		}

		result := map[string]any{
			"id":      postID,
			"user_id": userID,
			"content": content,
			"privacy": privacy,
		}
		if imagePath != "" {
			result["image_path"] = imagePath
		}
		response.JSON(w, http.StatusCreated, result)
	}
}

type audienceError struct {
	message string
}

// Error lets audienceError be returned as a normal Go error.
func (e audienceError) Error() string {
	return e.message
}

// resolveAudience checks the chosen viewers of a private post are all real followers of the author.
// The first bad value fails the whole request, so a mistyped or stale id is reported rather than silently narrowing the audience.
func resolveAudience(db *sql.DB, authorID int64, values []string) ([]int64, error) {
	seen := map[int64]bool{}
	ids := []int64{}

	for _, value := range values {
		id, err := strconv.ParseInt(value, 10, 64)
		if err != nil || id < 1 {
			return nil, audienceError{fmt.Sprintf("visibility user id %q is not a valid id", value)}
		}
		if seen[id] {
			continue
		}
		seen[id] = true

		following, err := followers.IsFollowing(db, id, authorID)
		if err != nil {
			return nil, err
		}
		if !following {
			var exists bool
			if err := db.QueryRow(`SELECT EXISTS (SELECT 1 FROM users WHERE id = ?)`, id).Scan(&exists); err != nil {
				return nil, err
			}
			if !exists {
				return nil, audienceError{fmt.Sprintf("user %d does not exist", id)}
			}
			return nil, audienceError{fmt.Sprintf("user %d is not one of your followers", id)}
		}

		ids = append(ids, id)
	}

	if len(ids) == 0 {
		return nil, audienceError{"select at least one recipient for a private post"}
	}

	return ids, nil
}

// insertPost saves the post and its private-audience rows in one transaction.
func insertPost(db *sql.DB, userID int64, content, privacy, imagePath string, audience []int64) (int64, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	res, err := tx.Exec(
		`INSERT INTO posts (user_id, content, privacy, image_path) VALUES (?, ?, ?, ?)`,
		userID, content, privacy, nullable(imagePath),
	)
	if err != nil {
		return 0, err
	}
	postID, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	for _, viewerID := range audience {
		if _, err := tx.Exec(
			`INSERT INTO post_visibility (post_id, user_id) VALUES (?, ?)`,
			postID, viewerID,
		); err != nil {
			return 0, err
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return postID, nil
}

// feedHandler handles GET /api/posts: every post the viewer may see, newest first.
func feedHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		viewerID, _ := middleware.UserID(r)

		rows, err := db.Query(`
			SELECT p.id, p.user_id, p.group_id, p.content, p.image_path, p.privacy, p.created_at,
			       u.first_name, u.last_name, u.nickname, u.avatar_path,`+likeColumns+`,`+commentCountColumn+`
			FROM posts p
			JOIN users u ON u.id = p.user_id
			WHERE `+visibilityClause+`
			ORDER BY p.created_at DESC
		`, viewerID, viewerID, viewerID, viewerID, viewerID)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to load feed")
			return
		}
		defer rows.Close()

		posts := []map[string]any{}
		for rows.Next() {
			post, err := scanPost(rows)
			if err != nil {
				response.Error(w, http.StatusInternalServerError, "failed to read feed")
				return
			}
			posts = append(posts, post)
		}
		// rows.Next() returning false could mean "done" or "errored mid-read" -- rows.Err() is what tells those two apart.
		if err := rows.Err(); err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to read feed")
			return
		}

		response.JSON(w, http.StatusOK, posts)
	}
}

// getPostHandler handles GET /api/posts/{id}: one post, or 404 if it is missing or hidden.
func getPostHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		viewerID, _ := middleware.UserID(r)
		postID, ok := idFromPath(w, r, "id", "post")
		if !ok {
			return
		}

		// 404 either way, so a private post's existence isn't leaked to someone who can't see it.
		row := db.QueryRow(`
			SELECT p.id, p.user_id, p.group_id, p.content, p.image_path, p.privacy, p.created_at,
			       u.first_name, u.last_name, u.nickname, u.avatar_path,`+likeColumns+`,`+commentCountColumn+`
			FROM posts p
			JOIN users u ON u.id = p.user_id
			WHERE p.id = ? AND (`+visibilityClause+`)
		`, viewerID, postID, viewerID, viewerID, viewerID, viewerID)

		post, err := scanPost(row)
		if err == sql.ErrNoRows {
			response.Error(w, http.StatusNotFound, "post not found")
			return
		}
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to load post")
			return
		}

		response.JSON(w, http.StatusOK, post)
	}
}

// deletePostHandler handles DELETE /api/posts/{id}: removes the post only if the viewer wrote it.
func deletePostHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		viewerID, _ := middleware.UserID(r)
		postID, ok := idFromPath(w, r, "id", "post")
		if !ok {
			return
		}

		// user_id = ? means this only ever deletes your own post; 0 rows affected covers both "doesn't exist" and "not yours".
		res, err := db.Exec(`DELETE FROM posts WHERE id = ? AND user_id = ?`, postID, viewerID)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to delete post")
			return
		}
		affected, _ := res.RowsAffected()
		if affected == 0 {
			response.Error(w, http.StatusNotFound, "post not found")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
