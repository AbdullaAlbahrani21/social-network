package middleware

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"social-network/backend/pkg/response"
)

type contextKey string

const userIDKey contextKey = "userID"

const SessionCookieName = "session_token"

var ErrInvalidSession = errors.New("invalid or expired session")

func ValidateSession(
	ctx context.Context,
	db *sql.DB,
	token string,
) (int64, error) {
	if token == "" {
		return 0, ErrInvalidSession
	}

	var userID int64

	// datetime() on both sides is required, not cosmetic: expires_at is TEXT with a numeric offset while CURRENT_TIMESTAMP is UTC, so a bare comparison is lexicographic.
	err := db.QueryRowContext(
		ctx,
		`
	SELECT user_id
	FROM sessions
	WHERE id = ?
	  AND datetime(expires_at) > datetime('now')
	`,
		token,
	).Scan(&userID)

	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrInvalidSession
	}

	if err != nil {
		return 0, err
	}

	return userID, nil
}

func RequireAuth(
	db *sql.DB,
	next http.HandlerFunc,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(SessionCookieName)
		if err != nil {
			response.Error(
				w,
				http.StatusUnauthorized,
				"not logged in",
			)
			return
		}

		userID, err := ValidateSession(
			r.Context(),
			db,
			cookie.Value,
		)

		if errors.Is(err, ErrInvalidSession) {
			response.Error(
				w,
				http.StatusUnauthorized,
				"session expired or invalid",
			)
			return
		}

		if err != nil {
			response.Error(
				w,
				http.StatusInternalServerError,
				"failed to validate session",
			)
			return
		}

		ctx := context.WithValue(
			r.Context(),
			userIDKey,
			userID,
		)

		next(w, r.WithContext(ctx))
	}
}

func UserID(r *http.Request) (int64, bool) {
	userID, ok := r.Context().Value(userIDKey).(int64)
	return userID, ok
}
