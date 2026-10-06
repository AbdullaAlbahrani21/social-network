package posts

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"

	"social-network/backend/pkg/middleware"
	"social-network/backend/pkg/notifications"
	"social-network/backend/pkg/response"
)

// It binds one "?", the viewer's id, which comes before any argument of the WHERE clause.
const likeColumns = `
	(SELECT COUNT(*) FROM post_likes pl WHERE pl.post_id = p.id) AS like_count,
	EXISTS (
	    SELECT 1 FROM post_likes pl WHERE pl.post_id = p.id AND pl.user_id = ?
	) AS viewer_has_liked`

type likeState struct {
	PostID         int64 `json:"post_id"`
	LikeCount      int64 `json:"like_count"`
	ViewerHasLiked bool  `json:"viewer_has_liked"`
}

// likePostHandler handles POST /api/posts/{id}/like: adds the viewer's like and notifies the author.
func likePostHandler(db *sql.DB, hubPush func(int64, interface{})) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		viewerID, _ := middleware.UserID(r)
		postID, ok := requireVisiblePost(w, r, db, viewerID)
		if !ok {
			return
		}

		// ON CONFLICT rather than checking first, so two likes sent together can't both decide the row is missing.
		res, err := db.Exec(`
			INSERT INTO post_likes (post_id, user_id) VALUES (?, ?)
			ON CONFLICT (post_id, user_id) DO NOTHING
		`, postID, viewerID)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to like post")
			return
		}

		// DO NOTHING reports 0 rows affected when the like was already there, which is what keeps a retry from notifying the author twice.
		inserted, err := res.RowsAffected()
		if err == nil && inserted > 0 {
			notifyPostLiked(db, postID, viewerID, hubPush)
		}

		writeLikeState(w, db, viewerID, postID)
	}
}

// notifyPostLiked sends the post author a "liked your post" notification (never for liking your own).
func notifyPostLiked(db *sql.DB, postID, likerID int64, hubPush func(int64, interface{})) {
	authorID, err := PostAuthor(db, postID)
	if err != nil {
		log.Printf("posts: failed to look up author of post %d to notify: %v", postID, err)
		return
	}
	if authorID == likerID {
		return
	}

	message := "Someone liked your post."
	if name, err := notifications.ActorName(context.Background(), db, likerID); err == nil {
		message = name + " liked your post."
	} else {
		log.Printf("posts: failed to load name of user %d for post_liked notification: %v", likerID, err)
	}

	if err := notifications.Create(
		db, authorID, notifications.TypePostLiked, likerID,
		notifyPostTargetType, postID, hubPush, message,
	); err != nil {
		log.Printf("posts: failed to create post_liked notification for user %d: %v", authorID, err)
	}
}

// unnotifyPostLiked deletes the like notification again when the like is taken back.
func unnotifyPostLiked(ctx context.Context, db *sql.DB, postID, likerID int64) {
	authorID, err := PostAuthor(db, postID)
	if err != nil {
		log.Printf("posts: failed to look up author of post %d to clear its like notification: %v", postID, err)
		return
	}
	if authorID == likerID {
		return
	}

	if err := notifications.DeleteForTargetFromActor(
		ctx, db, authorID, notifications.TypePostLiked,
		notifyPostTargetType, postID, likerID,
	); err != nil {
		log.Printf("posts: failed to delete post_liked notification for user %d: %v", authorID, err)
	}
}

// unlikePostHandler handles DELETE /api/posts/{id}/like: removes the viewer's like.
func unlikePostHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		viewerID, _ := middleware.UserID(r)
		postID, ok := requireVisiblePost(w, r, db, viewerID)
		if !ok {
			return
		}

		res, err := db.Exec(
			`DELETE FROM post_likes WHERE post_id = ? AND user_id = ?`,
			postID, viewerID,
		)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to unlike post")
			return
		}

		// Only a row actually removed is a real unlike: otherwise a no-op would take away a different user's like notification.
		removed, err := res.RowsAffected()
		if err == nil && removed > 0 {
			unnotifyPostLiked(r.Context(), db, postID, viewerID)
		}

		writeLikeState(w, db, viewerID, postID)
	}
}

// requireVisiblePost reads the post id from the URL and writes a 404 unless the viewer can see it.
func requireVisiblePost(w http.ResponseWriter, r *http.Request, db *sql.DB, viewerID int64) (int64, bool) {
	postID, ok := idFromPath(w, r, "id", "post")
	if !ok {
		return 0, false
	}

	canView, err := CanViewPost(db, viewerID, postID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to check post visibility")
		return 0, false
	}
	if !canView {
		response.Error(w, http.StatusNotFound, "post not found")
		return 0, false
	}

	return postID, true
}

// writeLikeState responds with the post's current like count and whether the viewer has liked it.
func writeLikeState(w http.ResponseWriter, db *sql.DB, viewerID, postID int64) {
	state := likeState{PostID: postID}

	err := db.QueryRow(`SELECT `+likeColumns+` FROM posts p WHERE p.id = ?`, viewerID, postID).
		Scan(&state.LikeCount, &state.ViewerHasLiked)
	if errors.Is(err, sql.ErrNoRows) {
		response.Error(w, http.StatusNotFound, "post not found")
		return
	}
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to load like count")
		return
	}

	response.JSON(w, http.StatusOK, state)
}
