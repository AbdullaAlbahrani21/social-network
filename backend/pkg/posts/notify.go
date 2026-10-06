package posts

import (
	"database/sql"
)

const notifyPostTargetType = "post"

// PostAuthor returns the id of the user who wrote the post.
func PostAuthor(db *sql.DB, postID int64) (int64, error) {
	var authorID int64
	err := db.QueryRow(`SELECT user_id FROM posts WHERE id = ?`, postID).Scan(&authorID)
	return authorID, err
}
