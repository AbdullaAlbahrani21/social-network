package groups

import (
	"context"
	"database/sql"
)

func IsGroupMember(db *sql.DB, groupID int64, userID int64) (bool, error) {
	return isGroupMember(context.Background(), db, groupID, userID)
}

func IsGroupMemberTx(ctx context.Context, tx *sql.Tx, groupID int64, userID int64) (bool, error) {
	return isGroupMember(ctx, tx, groupID, userID)
}

func GroupExists(ctx context.Context, db *sql.DB, groupID int64) (bool, error) {
	return groupExists(ctx, db, groupID)
}

func GroupTitle(ctx context.Context, db *sql.DB, groupID int64) (string, error) {
	return groupTitle(ctx, db, groupID)
}

func GroupMemberIDs(db *sql.DB, groupID int64) ([]int64, error) {
	rows, err := db.QueryContext(
		context.Background(),
		`
		SELECT user_id
		FROM group_members
		WHERE group_id = ?
		ORDER BY user_id
		`,
		groupID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	memberIDs := []int64{}

	for rows.Next() {
		var userID int64

		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}

		memberIDs = append(memberIDs, userID)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return memberIDs, nil
}
