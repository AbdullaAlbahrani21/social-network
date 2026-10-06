package groups

import (
	"context"
	"database/sql"
)

// email is matched on in the WHERE but deliberately never selected: returning it would turn the picker into an address harvester.
const invitableUserSelectColumns = `
	SELECT
		u.id,
		u.first_name || ' ' || u.last_name AS name,
		u.nickname
	FROM users u
`

func scanInvitableUser(
	scan func(dest ...any) error,
) (InvitableUserResponse, error) {
	var user InvitableUserResponse
	var nickname sql.NullString

	err := scan(
		&user.UserID,
		&user.Name,
		&nickname,
	)
	if err != nil {
		return InvitableUserResponse{}, err
	}

	if nickname.Valid {
		user.Nickname = &nickname.String
	}

	return user, nil
}

func listInvitableUsers(
	ctx context.Context,
	db *sql.DB,
	groupID int64,
	viewerID int64,
	search string,
) ([]InvitableUserResponse, error) {
	query := invitableUserSelectColumns + `
	WHERE u.id != ?
	  AND (
	           u.first_name LIKE ? ESCAPE '\'
	        OR u.last_name LIKE ? ESCAPE '\'
	        OR u.first_name || ' ' || u.last_name LIKE ? ESCAPE '\'
	        OR COALESCE(u.nickname, '') LIKE ? ESCAPE '\'
	        OR u.email LIKE ? ESCAPE '\'
	      )
	  AND NOT EXISTS (
	      SELECT 1
	      FROM group_members m
	      WHERE m.group_id = ?
	        AND m.user_id = u.id
	  )
	  AND NOT EXISTS (
	      SELECT 1
	      FROM group_invitations i
	      WHERE i.group_id = ?
	        AND i.invited_user_id = u.id
	        AND i.status = ?
	  )
	ORDER BY name ASC, u.id ASC
	LIMIT ?
`

	pattern := "%" + escapeLike(search) + "%"

	rows, err := db.QueryContext(
		ctx,
		query,
		viewerID,
		pattern,
		pattern,
		pattern,
		pattern,
		pattern,
		groupID,
		groupID,
		statusPending,
		maxInvitableUserResults,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []InvitableUserResponse{}

	for rows.Next() {
		user, err := scanInvitableUser(rows.Scan)
		if err != nil {
			return nil, err
		}

		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return users, nil
}
