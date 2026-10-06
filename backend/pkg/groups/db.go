package groups

import (
	"context"
	"database/sql"
	"strings"
)

const groupSelectColumns = `
	SELECT
		g.id,
		g.title,
		COALESCE(g.description, ''),
		g.creator_id,
		u.first_name || ' ' || u.last_name AS creator_name,
		g.created_at,
		(
			SELECT COUNT(*)
			FROM group_members m
			WHERE m.group_id = g.id
		) AS member_count,
		(
			SELECT m.role
			FROM group_members m
			WHERE m.group_id = g.id
			  AND m.user_id = ?
		) AS viewer_role
	FROM groups g
	JOIN users u ON u.id = g.creator_id
`

func scanGroup(scan func(dest ...any) error) (GroupResponse, error) {
	var group GroupResponse
	var viewerRole sql.NullString

	err := scan(
		&group.ID,
		&group.Title,
		&group.Description,
		&group.CreatorID,
		&group.CreatorName,
		&group.CreatedAt,
		&group.MemberCount,
		&viewerRole,
	)
	if err != nil {
		return GroupResponse{}, err
	}

	if viewerRole.Valid {
		group.IsMember = true
		group.Role = &viewerRole.String
	}

	return group, nil
}

func createGroup(
	ctx context.Context,
	db *sql.DB,
	creatorID int64,
	title string,
	description string,
) (int64, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(
		ctx,
		`
		INSERT INTO groups (creator_id, title, description)
		VALUES (?, ?, ?)
		`,
		creatorID,
		title,
		description,
	)
	if err != nil {
		return 0, err
	}

	groupID, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	_, err = tx.ExecContext(
		ctx,
		`
		INSERT INTO group_members (group_id, user_id, role)
		VALUES (?, ?, ?)
		`,
		groupID,
		creatorID,
		roleCreator,
	)
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return groupID, nil
}

func listGroups(
	ctx context.Context,
	db *sql.DB,
	viewerID int64,
	search string,
	memberOnly bool,
) ([]GroupResponse, error) {
	query := groupSelectColumns
	args := []any{viewerID}

	conditions := []string{}

	// Referring to a result alias in WHERE is a SQLite extension, and reusing viewer_role keeps the filter and the IsMember flag from disagreeing.
	if memberOnly {
		conditions = append(conditions, `viewer_role IS NOT NULL`)
	}

	if search != "" {
		// Parenthesised because the two LIKEs are one condition: without the brackets the member filter binds to the description arm alone.
		conditions = append(
			conditions,
			`(g.title LIKE ? ESCAPE '\' OR COALESCE(g.description, '') LIKE ? ESCAPE '\')`,
		)

		pattern := "%" + escapeLike(search) + "%"
		args = append(args, pattern, pattern)
	}

	if len(conditions) > 0 {
		query += "\n\tWHERE " + strings.Join(conditions, "\n\t  AND ") + "\n"
	}

	query += `
	ORDER BY g.created_at DESC, g.id DESC
`

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	groups := []GroupResponse{}

	for rows.Next() {
		group, err := scanGroup(rows.Scan)
		if err != nil {
			return nil, err
		}

		groups = append(groups, group)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return groups, nil
}

func getGroup(
	ctx context.Context,
	db *sql.DB,
	viewerID int64,
	groupID int64,
) (GroupResponse, error) {
	query := groupSelectColumns + `
	WHERE g.id = ?
`

	return scanGroup(
		db.QueryRowContext(ctx, query, viewerID, groupID).Scan,
	)
}

// The cascade that empties a group needs foreign key enforcement on; every clause is declared in the schema, not here.
func deleteGroup(
	ctx context.Context,
	db *sql.DB,
	groupID int64,
) error {
	_, err := db.ExecContext(
		ctx,
		`DELETE FROM groups WHERE id = ?`,
		groupID,
	)

	return err
}

// The escape character has to be escaped too, or \% turns back into a wildcard; NewReplacer's single pass avoids escaping it twice.
func escapeLike(value string) string {
	replacer := strings.NewReplacer(
		`\`, `\\`,
		`%`, `\%`,
		`_`, `\_`,
	)

	return replacer.Replace(value)
}
