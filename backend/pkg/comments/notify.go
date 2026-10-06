package comments

import (
	"context"
	"database/sql"
	"log"

	"social-network/backend/pkg/notifications"
	"social-network/backend/pkg/posts"
)

const notifyTargetType = "post"

// notifyPostCommented tells the post author someone commented (never for commenting on your own post).
func notifyPostCommented(db *sql.DB, postID, commenterID int64, hubPush func(int64, interface{})) {
	authorID, err := posts.PostAuthor(db, postID)
	if err != nil {
		log.Printf("comments: failed to look up author of post %d to notify: %v", postID, err)
		return
	}
	if authorID == commenterID {
		return
	}

	message := "Someone commented on your post."
	if name, err := notifications.ActorName(context.Background(), db, commenterID); err == nil {
		message = name + " commented on your post."
	} else {
		log.Printf("comments: failed to load name of user %d for comment_created notification: %v", commenterID, err)
	}

	if err := notifications.Create(
		db, authorID, notifications.TypeCommentCreated, commenterID,
		notifyTargetType, postID, hubPush, message,
	); err != nil {
		log.Printf("comments: failed to create comment_created notification for user %d: %v", authorID, err)
	}
}
