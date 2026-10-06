package auth

import (
	"context"
	"database/sql"
	"net/http"
	"strings"

	"social-network/backend/pkg/response"
)

func searchUsersHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		search := strings.TrimSpace(r.URL.Query().Get("search"))
		if search == "" {
			response.Error(
				w,
				http.StatusBadRequest,
				"search query is required",
			)
			return
		}

		if len([]rune(search)) > maxSearchLength {
			response.Error(
				w,
				http.StatusBadRequest,
				"search query is too long",
			)
			return
		}

		users, err := searchUsers(r.Context(), db, search)
		if err != nil {
			response.Error(
				w,
				http.StatusInternalServerError,
				"failed to search users",
			)
			return
		}

		response.JSON(
			w,
			http.StatusOK,
			users,
		)
	}
}

// No email arm: matching on an address would let any logged-in user test whether it has an account.
func searchUsers(ctx context.Context, db *sql.DB, search string) ([]UserSearchResult, error) {
	pattern := "%" + escapeLike(search) + "%"

	rows, err := db.QueryContext(ctx, `
		SELECT
			u.id,
			u.first_name || ' ' || u.last_name AS name,
			u.nickname,
			u.avatar_path
		FROM users u
		WHERE u.first_name LIKE ?1 ESCAPE '\'
		   OR u.last_name LIKE ?1 ESCAPE '\'
		   OR u.first_name || ' ' || u.last_name LIKE ?1 ESCAPE '\'
		   OR COALESCE(u.nickname, '') LIKE ?1 ESCAPE '\'
		ORDER BY name ASC, u.id ASC
		LIMIT ?2
	`, pattern, maxUserSearchResults)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []UserSearchResult{}

	for rows.Next() {
		var user UserSearchResult
		if err := rows.Scan(
			&user.ID,
			&user.Name,
			&user.Nickname,
			&user.AvatarPath,
		); err != nil {
			return nil, err
		}

		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return users, nil
}

// The backslash has to be escaped too, or \% turns back into a wildcard; NewReplacer's single pass keeps it from being escaped twice.
func escapeLike(value string) string {
	replacer := strings.NewReplacer(
		`\`, `\\`,
		`%`, `\%`,
		`_`, `\_`,
	)

	return replacer.Replace(value)
}
