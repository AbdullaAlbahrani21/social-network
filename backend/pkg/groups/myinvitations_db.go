package groups

import (
	"context"
	"database/sql"
)

// users is a LEFT join on inviter_id, which is nullable (ON DELETE SET NULL): an inner join would silently drop an invite whose sender is gone.
const myInvitationColumns = `
	SELECT
		i.id,
		i.group_id,
		g.title,
		u.first_name || ' ' || u.last_name AS inviter_name,
		i.created_at
	FROM group_invitations i
	JOIN groups g ON g.id = i.group_id
	LEFT JOIN users u ON u.id = i.inviter_id
`

func scanMyInvitation(
	scan func(dest ...any) error,
) (MyInvitationResponse, error) {
	var invitation MyInvitationResponse
	var inviterName sql.NullString

	err := scan(
		&invitation.InvitationID,
		&invitation.GroupID,
		&invitation.GroupTitle,
		&inviterName,
		&invitation.CreatedAt,
	)
	if err != nil {
		return MyInvitationResponse{}, err
	}

	if inviterName.Valid {
		invitation.InviterName = &inviterName.String
	}

	return invitation, nil
}

// type = 'invite' excludes the caller's own join requests: those record them as invited_user_id too, but the group's creator answers them.
func listMyPendingInvitations(
	ctx context.Context,
	db *sql.DB,
	userID int64,
) ([]MyInvitationResponse, error) {
	query := myInvitationColumns + `
	WHERE i.invited_user_id = ?
	  AND i.type = ?
	  AND i.status = ?
	ORDER BY i.created_at ASC, i.id ASC
`

	rows, err := db.QueryContext(
		ctx,
		query,
		userID,
		typeInvite,
		statusPending,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	invitations := []MyInvitationResponse{}

	for rows.Next() {
		invitation, err := scanMyInvitation(rows.Scan)
		if err != nil {
			return nil, err
		}

		invitations = append(invitations, invitation)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return invitations, nil
}
