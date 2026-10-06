package groups

import (
	"context"
	"database/sql"
)

// privacy is written as 'public' only to satisfy the NOT NULL + CHECK constraint; membership, not that column, is what makes a group post visible.

const groupPostSelectColumns = `
	SELECT
		p.id,
		p.group_id,
		p.user_id,
		p.content,
		p.created_at,
		u.first_name || ' ' || u.last_name AS author_name,
		u.nickname,
		(
			SELECT COUNT(*)
			FROM post_likes l
			WHERE l.post_id = p.id
		) AS like_count,
		EXISTS (
			SELECT 1
			FROM post_likes l
			WHERE l.post_id = p.id
			  AND l.user_id = ?
		) AS viewer_has_liked
	FROM posts p
	JOIN users u ON u.id = p.user_id
`

func scanGroupPost(scan func(dest ...any) error) (GroupPostResponse, error) {
	var post GroupPostResponse
	var nickname sql.NullString

	err := scan(
		&post.ID,
		&post.GroupID,
		&post.UserID,
		&post.Content,
		&post.CreatedAt,
		&post.AuthorName,
		&nickname,
		&post.LikeCount,
		&post.ViewerHasLiked,
	)
	if err != nil {
		return GroupPostResponse{}, err
	}

	if nickname.Valid {
		post.AuthorNickname = &nickname.String
	}

	return post, nil
}

func createGroupPost(
	ctx context.Context,
	db *sql.DB,
	groupID int64,
	userID int64,
	content string,
) (GroupPostResponse, error) {
	result, err := db.ExecContext(
		ctx,
		`
		INSERT INTO posts (user_id, group_id, content, image_path, privacy)
		VALUES (?, ?, ?, NULL, 'public')
		`,
		userID,
		groupID,
		content,
	)
	if err != nil {
		return GroupPostResponse{}, err
	}

	postID, err := result.LastInsertId()
	if err != nil {
		return GroupPostResponse{}, err
	}

	return getGroupPost(ctx, db, userID, groupID, postID)
}

// Constraining on group_id as well as id lets the scan take it as a plain int64, though the column is nullable.
func getGroupPost(
	ctx context.Context,
	db *sql.DB,
	viewerID int64,
	groupID int64,
	postID int64,
) (GroupPostResponse, error) {
	query := groupPostSelectColumns + `
	WHERE p.id = ?
	  AND p.group_id = ?
`

	return scanGroupPost(
		db.QueryRowContext(ctx, query, viewerID, postID, groupID).Scan,
	)
}

// The id tiebreak is load-bearing: created_at defaults to CURRENT_TIMESTAMP, which SQLite stores at one-second resolution.
func listGroupPosts(
	ctx context.Context,
	db *sql.DB,
	viewerID int64,
	groupID int64,
) ([]GroupPostResponse, error) {
	query := groupPostSelectColumns + `
	WHERE p.group_id = ?
	ORDER BY p.created_at DESC, p.id DESC
`

	rows, err := db.QueryContext(ctx, query, viewerID, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	posts := []GroupPostResponse{}

	for rows.Next() {
		post, err := scanGroupPost(rows.Scan)
		if err != nil {
			return nil, err
		}

		posts = append(posts, post)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return posts, nil
}
