package auth

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"net/http"
	"time"

	"social-network/backend/pkg/middleware"
)

const sessionDuration = 7 * 24 * time.Hour

type sessionExecutor interface {
	Exec(query string, args ...any) (sql.Result, error)
}

func createSession(
	db sessionExecutor,
	userID int64,
) (string, error) {
	tokenBytes := make([]byte, 32)

	if _, err := rand.Read(tokenBytes); err != nil {
		return "", err
	}

	token := base64.RawURLEncoding.EncodeToString(tokenBytes)
	expiresAt := time.Now().Add(sessionDuration)

	_, err := db.Exec(`
		INSERT INTO sessions (
			id,
			user_id,
			expires_at
		)
		VALUES (?, ?, ?)
	`,
		token,
		userID,
		expiresAt,
	)
	if err != nil {
		return "", err
	}

	return token, nil
}

func setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     middleware.SessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionDuration.Seconds()),
	})
}

func deleteSession(db *sql.DB, token string) error {
	_, err := db.Exec(
		`DELETE FROM sessions WHERE id = ?`,
		token,
	)

	return err
}

// RETURNING makes the read part of the delete: a separate SELECT first would let a session created between the two statements be deleted unreported.
func deleteUserSessions(tx *sql.Tx, userID int64) ([]string, error) {
	rows, err := tx.Query(
		`DELETE FROM sessions WHERE user_id = ? RETURNING id`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tokens []string
	for rows.Next() {
		var token string
		if err := rows.Scan(&token); err != nil {
			return nil, err
		}
		tokens = append(tokens, token)
	}

	return tokens, rows.Err()
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     middleware.SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}
