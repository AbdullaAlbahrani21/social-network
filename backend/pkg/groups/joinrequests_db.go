package groups

import (
	"context"
	"database/sql"
)

const pendingJoinRequestColumns = `
	SELECT
		i.id,
		i.invited_user_id,
		u.first_name || ' ' || u.last_name AS name,
		u.nickname,
		i.created_at
	FROM group_invitations i
	JOIN users u ON u.id = i.invited_user_id
`

func scanPendingJoinRequest(
	scan func(dest ...any) error,
) (PendingJoinRequestResponse, error) {
	var request PendingJoinRequestResponse
	var nickname sql.NullString

	err := scan(
		&request.InvitationID,
		&request.UserID,
		&request.Name,
		&nickname,
		&request.CreatedAt,
	)
	if err != nil {
		return PendingJoinRequestResponse{}, err
	}

	if nickname.Valid {
		request.Nickname = &nickname.String
	}

	return request, nil
}

// type = 'request' excludes invites: they sit in the same table but are answered by the invited user, not the creator.
func listPendingJoinRequests(
	ctx context.Context,
	db *sql.DB,
	groupID int64,
) ([]PendingJoinRequestResponse, error) {
	query := pendingJoinRequestColumns + `
	WHERE i.group_id = ?
	  AND i.type = ?
	  AND i.status = ?
	ORDER BY i.created_at ASC, i.id ASC
`

	rows, err := db.QueryContext(
		ctx,
		query,
		groupID,
		typeRequest,
		statusPending,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	requests := []PendingJoinRequestResponse{}

	for rows.Next() {
		request, err := scanPendingJoinRequest(rows.Scan)
		if err != nil {
			return nil, err
		}

		requests = append(requests, request)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return requests, nil
}
