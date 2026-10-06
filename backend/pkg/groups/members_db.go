package groups

import (
	"context"
	"database/sql"
)

const memberSelectColumns = `
	SELECT
		m.user_id,
		u.first_name || ' ' || u.last_name AS name,
		u.nickname,
		m.role,
		m.joined_at
	FROM group_members m
	JOIN users u ON u.id = m.user_id
`

func scanGroupMember(
	scan func(dest ...any) error,
) (GroupMemberResponse, error) {
	var member GroupMemberResponse
	var nickname sql.NullString

	err := scan(
		&member.UserID,
		&member.Name,
		&nickname,
		&member.Role,
		&member.JoinedAt,
	)
	if err != nil {
		return GroupMemberResponse{}, err
	}

	if nickname.Valid {
		member.Nickname = &nickname.String
	}

	return member, nil
}

// The creator is sorted by role, not joined_at -- those agree only by accident -- and the id tiebreak settles same-second joins.
func listGroupMembers(
	ctx context.Context,
	db *sql.DB,
	groupID int64,
) ([]GroupMemberResponse, error) {
	query := memberSelectColumns + `
	WHERE m.group_id = ?
	ORDER BY
		CASE WHEN m.role = ? THEN 0 ELSE 1 END,
		m.joined_at ASC,
		m.id ASC
`

	rows, err := db.QueryContext(ctx, query, groupID, roleCreator)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	members := []GroupMemberResponse{}

	for rows.Next() {
		member, err := scanGroupMember(rows.Scan)
		if err != nil {
			return nil, err
		}

		members = append(members, member)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return members, nil
}
