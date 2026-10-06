package groups

import (
	"context"
	"database/sql"
	"errors"

	"github.com/mattn/go-sqlite3"
)

type querier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func groupExists(
	ctx context.Context,
	q querier,
	groupID int64,
) (bool, error) {
	var exists int

	err := q.QueryRowContext(
		ctx,
		`SELECT EXISTS(SELECT 1 FROM groups WHERE id = ?)`,
		groupID,
	).Scan(&exists)

	return exists == 1, err
}

func userExists(
	ctx context.Context,
	q querier,
	userID int64,
) (bool, error) {
	var exists int

	err := q.QueryRowContext(
		ctx,
		`SELECT EXISTS(SELECT 1 FROM users WHERE id = ?)`,
		userID,
	).Scan(&exists)

	return exists == 1, err
}

func isGroupMember(
	ctx context.Context,
	q querier,
	groupID int64,
	userID int64,
) (bool, error) {
	var exists int

	err := q.QueryRowContext(
		ctx,
		`
		SELECT EXISTS(
			SELECT 1
			FROM group_members
			WHERE group_id = ?
			  AND user_id = ?
		)
		`,
		groupID,
		userID,
	).Scan(&exists)

	return exists == 1, err
}

func countGroupMembers(
	ctx context.Context,
	q querier,
	groupID int64,
) (int, error) {
	var count int

	err := q.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM group_members WHERE group_id = ?`,
		groupID,
	).Scan(&count)

	return count, err
}

func groupCreatorID(
	ctx context.Context,
	q querier,
	groupID int64,
) (int64, error) {
	var creatorID int64

	err := q.QueryRowContext(
		ctx,
		`SELECT creator_id FROM groups WHERE id = ?`,
		groupID,
	).Scan(&creatorID)

	return creatorID, err
}

func groupTitle(
	ctx context.Context,
	q querier,
	groupID int64,
) (string, error) {
	var title string

	err := q.QueryRowContext(
		ctx,
		`SELECT title FROM groups WHERE id = ?`,
		groupID,
	).Scan(&title)

	return title, err
}

func userFullName(
	ctx context.Context,
	q querier,
	userID int64,
) (string, error) {
	var name string

	err := q.QueryRowContext(
		ctx,
		`SELECT first_name || ' ' || last_name FROM users WHERE id = ?`,
		userID,
	).Scan(&name)

	return name, err
}

func hasPendingInvitation(
	ctx context.Context,
	q querier,
	groupID int64,
	invitedUserID int64,
) (bool, error) {
	var exists int

	err := q.QueryRowContext(
		ctx,
		`
		SELECT EXISTS(
			SELECT 1
			FROM group_invitations
			WHERE group_id = ?
			  AND invited_user_id = ?
			  AND status = ?
		)
		`,
		groupID,
		invitedUserID,
		statusPending,
	).Scan(&exists)

	return exists == 1, err
}

const invitationSelectColumns = `
	SELECT
		id,
		group_id,
		invited_user_id,
		inviter_id,
		type,
		status,
		created_at
	FROM group_invitations
`

func scanInvitation(scan func(dest ...any) error) (InvitationResponse, error) {
	var invitation InvitationResponse
	var inviterID sql.NullInt64

	err := scan(
		&invitation.ID,
		&invitation.GroupID,
		&invitation.InvitedUserID,
		&inviterID,
		&invitation.Type,
		&invitation.Status,
		&invitation.CreatedAt,
	)
	if err != nil {
		return InvitationResponse{}, err
	}

	if inviterID.Valid {
		invitation.InviterID = &inviterID.Int64
	}

	return invitation, nil
}

func getInvitation(
	ctx context.Context,
	q querier,
	invitationID int64,
) (InvitationResponse, error) {
	query := invitationSelectColumns + `
	WHERE id = ?
`

	return scanInvitation(
		q.QueryRowContext(ctx, query, invitationID).Scan,
	)
}

func createInvitation(
	ctx context.Context,
	db *sql.DB,
	groupID int64,
	invitedUserID int64,
	inviterID *int64,
	invitationType string,
) (InvitationResponse, error) {
	result, err := db.ExecContext(
		ctx,
		`
		INSERT INTO group_invitations (
			group_id,
			invited_user_id,
			inviter_id,
			type,
			status
		)
		VALUES (?, ?, ?, ?, ?)
		`,
		groupID,
		invitedUserID,
		inviterID,
		invitationType,
		statusPending,
	)
	if err != nil {
		return InvitationResponse{}, err
	}

	invitationID, err := result.LastInsertId()
	if err != nil {
		return InvitationResponse{}, err
	}

	return getInvitation(ctx, db, invitationID)
}

func resolveInvitation(
	ctx context.Context,
	db *sql.DB,
	invitation InvitationResponse,
	newStatus string,
) (InvitationResponse, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return InvitationResponse{}, err
	}
	defer tx.Rollback()

	// UNIQUE(group_id, invited_user_id, status) collides on a second row reaching the same terminal status, so the older resolved row is dropped.
	_, err = tx.ExecContext(
		ctx,
		`
		DELETE FROM group_invitations
		WHERE group_id = ?
		  AND invited_user_id = ?
		  AND status = ?
		  AND id != ?
		`,
		invitation.GroupID,
		invitation.InvitedUserID,
		newStatus,
		invitation.ID,
	)
	if err != nil {
		return InvitationResponse{}, err
	}

	// Conditional on status so two concurrent accepts can't both win.
	result, err := tx.ExecContext(
		ctx,
		`
		UPDATE group_invitations
		SET status = ?
		WHERE id = ?
		  AND status = ?
		`,
		newStatus,
		invitation.ID,
		statusPending,
	)
	if err != nil {
		return InvitationResponse{}, err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return InvitationResponse{}, err
	}

	if affected == 0 {
		return InvitationResponse{}, errNotPending
	}

	if newStatus == statusAccepted {
		// The cap check has to sit inside this transaction: counting on the pool first would let two accepts each read 9 and both write.
		alreadyMember, err := isGroupMember(
			ctx,
			tx,
			invitation.GroupID,
			invitation.InvitedUserID,
		)
		if err != nil {
			return InvitationResponse{}, err
		}

		if !alreadyMember {
			memberCount, err := countGroupMembers(
				ctx,
				tx,
				invitation.GroupID,
			)
			if err != nil {
				return InvitationResponse{}, err
			}

			if memberCount >= MaxGroupMembers {
				return InvitationResponse{}, errGroupFull
			}
		}

		_, err = tx.ExecContext(
			ctx,
			`
			INSERT INTO group_members (group_id, user_id, role)
			VALUES (?, ?, ?)
			ON CONFLICT (group_id, user_id) DO NOTHING
			`,
			invitation.GroupID,
			invitation.InvitedUserID,
			roleMember,
		)
		if err != nil {
			return InvitationResponse{}, err
		}
	}

	updated, err := getInvitation(ctx, tx, invitation.ID)
	if err != nil {
		return InvitationResponse{}, err
	}

	if err := tx.Commit(); err != nil {
		return InvitationResponse{}, err
	}

	return updated, nil
}

func isUniqueViolation(err error) bool {
	var sqliteErr sqlite3.Error

	return errors.As(err, &sqliteErr) &&
		sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique
}
