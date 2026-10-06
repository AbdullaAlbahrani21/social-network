package auth

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/mail"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"

	"social-network/backend/pkg/middleware"
	"social-network/backend/pkg/response"
)

// The maximum counts bytes: bcrypt reads only the first 72 and ignores the rest.
const (
	minPasswordLength = 8
	maxPasswordBytes  = 72

	passwordTooShortMessage = "password must be at least 8 characters"
	passwordTooLongMessage  = "password must be at most 72 bytes"
)

// Shared by the nickname and the first and last names, so the three cannot drift apart; only the lengths differ.
const nameCharacterClass = `[A-Za-z0-9._-]`

var (
	nicknamePattern   = regexp.MustCompile(`^` + nameCharacterClass + `{3,30}$`)
	personNamePattern = regexp.MustCompile(`^` + nameCharacterClass + `+$`)
)

const (
	nicknameRequiredMessage = "nickname is required"
	nicknameFormatMessage   = "nickname must be 3 to 30 characters: letters, digits, dots, hyphens or underscores"

	firstNameFormatMessage = "first name may only contain English letters, numbers, and . _ -"
	lastNameFormatMessage  = "last name may only contain English letters, numbers, and . _ -"
)

// Registration only: login must keep accepting any password, or accounts created before these rules could not sign in.
const visibleASCIIClass = `[!-~]`

var (
	visibleASCIIPattern = regexp.MustCompile(`^` + visibleASCIIClass + `+$`)
	aboutMePattern      = regexp.MustCompile(`^(?:` + visibleASCIIClass + `|[ \r\n])*$`)
)

const (
	emailFormatMessage    = "email may only contain English letters, numbers, and standard symbols"
	passwordFormatMessage = "password may only contain English letters, numbers, and symbols (no spaces or emoji)"
	aboutMeFormatMessage  = "about me may only contain English letters, numbers, symbols, and spaces"
)

const (
	maxFirstNameLength = 50
	maxLastNameLength  = 50
	maxAboutMeLength   = 500
)

const (
	minimumAge = 16

	minimumAgeMessage = "You'll be able to use the app when you're older — for now, focus on your studies!"

	maximumAge = 120

	maximumAgeMessage = "please enter a valid date of birth"
)

// Both sides are reduced to a calendar date first: a date of birth is midnight UTC while now carries the server's zone offset.
func isUnderMinimumAge(birthDate, now time.Time) bool {
	eligibleOn := birthDate.AddDate(minimumAge, 0, 0)

	year, month, day := now.Date()
	today := time.Date(year, month, day, 0, 0, 0, 0, eligibleOn.Location())

	return eligibleOn.After(today)
}

// Same calendar-date reduction as isUnderMinimumAge; the 120th birthday itself is still accepted.
func isOverMaximumAge(birthDate, now time.Time) bool {
	lastEligibleOn := birthDate.AddDate(maximumAge, 0, 0)

	year, month, day := now.Date()
	today := time.Date(year, month, day, 0, 0, 0, 0, lastEligibleOn.Location())

	return lastEligibleOn.Before(today)
}

// Wire contract: src/stores/websocket.js matches these exactly, and a close frame's payload is capped at 125 bytes.
const (
	closeReasonLoggedOut         = "logged out"
	closeReasonLoggedInElsewhere = "logged in elsewhere"
)

func RegisterRoutes(mux *http.ServeMux, db *sql.DB, closeSession func(token string, reason string)) {
	mux.HandleFunc("POST /api/register", registerHandler(db))
	mux.HandleFunc("POST /api/login", loginHandler(db, closeSession))
	mux.HandleFunc("POST /api/logout", logoutHandler(db, closeSession))

	mux.HandleFunc(
		"GET /api/me",
		middleware.RequireAuth(
			db,
			meHandler(db),
		),
	)

	// Deliberately not behind RequireAuth -- see sessionHandler.
	mux.HandleFunc("GET /api/session", sessionHandler(db))

	mux.HandleFunc(
		"GET /api/users",
		middleware.RequireAuth(
			db,
			searchUsersHandler(db),
		),
	)
}

func registerHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(
			w,
			r.Body,
			maxAvatarSize+(1<<20),
		)

		if err := r.ParseMultipartForm(maxAvatarSize); err != nil {
			response.Error(
				w,
				http.StatusBadRequest,
				"invalid registration form",
			)
			return
		}

		email := strings.TrimSpace(r.FormValue("email"))
		password := r.FormValue("password")
		firstName := strings.TrimSpace(r.FormValue("first_name"))
		lastName := strings.TrimSpace(r.FormValue("last_name"))
		dateOfBirth := strings.TrimSpace(r.FormValue("date_of_birth"))
		nickname := strings.TrimSpace(r.FormValue("nickname"))
		aboutMe := strings.TrimSpace(r.FormValue("about_me"))

		if email == "" ||
			password == "" ||
			firstName == "" ||
			lastName == "" ||
			dateOfBirth == "" {

			response.Error(
				w,
				http.StatusBadRequest,
				"missing required fields",
			)
			return
		}

		if len([]rune(firstName)) > maxFirstNameLength {
			response.Error(
				w,
				http.StatusBadRequest,
				"first name is too long",
			)
			return
		}

		if !personNamePattern.MatchString(firstName) {
			response.Error(
				w,
				http.StatusBadRequest,
				firstNameFormatMessage,
			)
			return
		}

		if len([]rune(lastName)) > maxLastNameLength {
			response.Error(
				w,
				http.StatusBadRequest,
				"last name is too long",
			)
			return
		}

		if !personNamePattern.MatchString(lastName) {
			response.Error(
				w,
				http.StatusBadRequest,
				lastNameFormatMessage,
			)
			return
		}

		if len([]rune(aboutMe)) > maxAboutMeLength {
			response.Error(
				w,
				http.StatusBadRequest,
				"about me is too long",
			)
			return
		}

		if !aboutMePattern.MatchString(aboutMe) {
			response.Error(
				w,
				http.StatusBadRequest,
				aboutMeFormatMessage,
			)
			return
		}

		if nickname == "" {
			response.Error(
				w,
				http.StatusBadRequest,
				nicknameRequiredMessage,
			)
			return
		}

		if !nicknamePattern.MatchString(nickname) {
			response.Error(
				w,
				http.StatusBadRequest,
				nicknameFormatMessage,
			)
			return
		}

		address, err := mail.ParseAddress(email)
		if err != nil || address.Address != email {
			response.Error(
				w,
				http.StatusBadRequest,
				"invalid email address",
			)
			return
		}

		// ParseAddress accepts UTF-8 local parts and domains, so this narrows what it lets through.
		if !visibleASCIIPattern.MatchString(email) {
			response.Error(
				w,
				http.StatusBadRequest,
				emailFormatMessage,
			)
			return
		}

		birthDate, err := time.Parse(
			"2006-01-02",
			dateOfBirth,
		)
		if err != nil {
			response.Error(
				w,
				http.StatusBadRequest,
				"invalid date of birth",
			)
			return
		}

		if birthDate.After(time.Now()) {
			response.Error(
				w,
				http.StatusBadRequest,
				"date of birth cannot be in the future",
			)
			return
		}

		// After the future check, the more specific complaint about a date that is also under sixteen years ago.
		if isUnderMinimumAge(birthDate, time.Now()) {
			response.Error(
				w,
				http.StatusBadRequest,
				minimumAgeMessage,
			)
			return
		}

		if isOverMaximumAge(birthDate, time.Now()) {
			response.Error(
				w,
				http.StatusBadRequest,
				maximumAgeMessage,
			)
			return
		}

		// Before the length checks, so "😂😂" gets the character error rather than "too short".
		if !visibleASCIIPattern.MatchString(password) {
			response.Error(
				w,
				http.StatusBadRequest,
				passwordFormatMessage,
			)
			return
		}

		if utf8.RuneCountInString(password) < minPasswordLength {
			response.Error(
				w,
				http.StatusBadRequest,
				passwordTooShortMessage,
			)
			return
		}

		if len(password) > maxPasswordBytes {
			response.Error(
				w,
				http.StatusBadRequest,
				passwordTooLongMessage,
			)
			return
		}

		passwordHash, err := bcrypt.GenerateFromPassword(
			[]byte(password),
			bcrypt.DefaultCost,
		)
		if err != nil {
			response.Error(
				w,
				http.StatusInternalServerError,
				"failed to process password",
			)
			return
		}

		tx, err := db.Begin()
		if err != nil {
			response.Error(
				w,
				http.StatusInternalServerError,
				"failed to create account",
			)
			return
		}
		defer tx.Rollback()

		result, err := tx.Exec(`
			INSERT INTO users (
				email,
				password_hash,
				first_name,
				last_name,
				date_of_birth,
				nickname,
				about_me
			)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`,
			email,
			string(passwordHash),
			firstName,
			lastName,
			dateOfBirth,
			nickname,
			nullableString(aboutMe),
		)

		if err != nil {
			var sqliteErr sqlite3.Error

			if errors.As(err, &sqliteErr) &&
				sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique {

				// The UNIQUE constraint error names the column: "UNIQUE constraint failed: users.nickname".
				message := "email already registered"
				if strings.Contains(sqliteErr.Error(), "users.nickname") {
					message = "nickname already registered"
				}

				response.Error(
					w,
					http.StatusConflict,
					message,
				)
				return
			}

			response.Error(
				w,
				http.StatusInternalServerError,
				"failed to create account",
			)
			return
		}

		userID, err := result.LastInsertId()
		if err != nil {
			response.Error(
				w,
				http.StatusInternalServerError,
				"failed to create account",
			)
			return
		}

		var avatarPath *string
		var savedAvatarPath string

		file, _, err := r.FormFile("avatar")

		if err == nil {
			defer file.Close()

			header := make([]byte, 512)

			n, err := file.Read(header)
			if err != nil && err != io.EOF {
				response.Error(
					w,
					http.StatusBadRequest,
					"could not read avatar",
				)
				return
			}

			contentType := http.DetectContentType(
				header[:n],
			)

			if _, allowed := allowedAvatarTypes[contentType]; !allowed {
				response.Error(
					w,
					http.StatusBadRequest,
					"avatar must be JPEG, PNG or GIF",
				)
				return
			}

			if _, err := file.Seek(0, io.SeekStart); err != nil {
				response.Error(
					w,
					http.StatusInternalServerError,
					"could not process avatar",
				)
				return
			}

			path, err := saveAvatar(
				file,
				userID,
				contentType,
			)
			if err != nil {
				response.Error(
					w,
					http.StatusInternalServerError,
					"failed to save avatar",
				)
				return
			}

			savedAvatarPath = path
			avatarPath = &path

			_, err = tx.Exec(`
				UPDATE users
				SET avatar_path = ?
				WHERE id = ?
			`,
				path,
				userID,
			)
			if err != nil {
				removeAvatar(path)

				response.Error(
					w,
					http.StatusInternalServerError,
					"failed to save avatar",
				)
				return
			}

		} else if !errors.Is(err, http.ErrMissingFile) {
			response.Error(
				w,
				http.StatusBadRequest,
				"invalid avatar upload",
			)
			return
		}

		token, err := createSession(tx, userID)
		if err != nil {
			if savedAvatarPath != "" {
				removeAvatar(savedAvatarPath)
			}

			response.Error(
				w,
				http.StatusInternalServerError,
				"failed to create session",
			)
			return
		}

		if err := tx.Commit(); err != nil {
			if savedAvatarPath != "" {
				removeAvatar(savedAvatarPath)
			}

			response.Error(
				w,
				http.StatusInternalServerError,
				"failed to create account",
			)
			return
		}

		setSessionCookie(w, token)

		// Formatted rather than echoed, so register returns the same RFC3339 date login, /api/me and profile read back.
		response.JSON(
			w,
			http.StatusCreated,
			UserResponse{
				ID:          userID,
				Email:       email,
				FirstName:   firstName,
				LastName:    lastName,
				DateOfBirth: birthDate.Format(time.RFC3339),
				AvatarPath:  avatarPath,
				Nickname:    stringPointer(nickname),
				AboutMe:     stringPointer(aboutMe),
				IsPublic:    true,
			},
		)
	}
}

func loginHandler(db *sql.DB, closeSession func(token string, reason string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}

		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			response.Error(
				w,
				http.StatusBadRequest,
				"invalid request body",
			)
			return
		}

		identifier := strings.TrimSpace(body.Email)
		password := body.Password

		if identifier == "" || password == "" {
			response.Error(
				w,
				http.StatusBadRequest,
				"email/nickname and password are required",
			)
			return
		}

		// Rejected before the lookup: CompareHashAndPassword reads only the first 72 bytes, so a 72-byte password would match any longer string beginning with it.
		if len(password) > maxPasswordBytes {
			response.Error(
				w,
				http.StatusBadRequest,
				passwordTooLongMessage,
			)
			return
		}

		var user UserResponse
		var passwordHash string

		// ORDER BY settles an identifier that is one person's email and another's nickname: the exact email match wins.
		err := db.QueryRow(`
			SELECT
				id,
				email,
				password_hash,
				first_name,
				last_name,
				date_of_birth,
				avatar_path,
				nickname,
				about_me,
				is_public
			FROM users
			WHERE email = ?1 OR nickname = ?1
			ORDER BY email = ?1 DESC
			LIMIT 1
		`,
			identifier,
		).Scan(
			&user.ID,
			&user.Email,
			&passwordHash,
			&user.FirstName,
			&user.LastName,
			&user.DateOfBirth,
			&user.AvatarPath,
			&user.Nickname,
			&user.AboutMe,
			&user.IsPublic,
		)

		if errors.Is(err, sql.ErrNoRows) {
			response.Error(
				w,
				http.StatusUnauthorized,
				"invalid email/nickname or password",
			)
			return
		}

		if err != nil {
			response.Error(
				w,
				http.StatusInternalServerError,
				"failed to login",
			)
			return
		}

		if err := bcrypt.CompareHashAndPassword(
			[]byte(passwordHash),
			[]byte(password),
		); err != nil {
			response.Error(
				w,
				http.StatusUnauthorized,
				"invalid email/nickname or password",
			)
			return
		}

		// The DELETE runs first so the transaction takes SQLite's write lock before reading: two simultaneous logins then serialize instead of deadlocking on a lock upgrade.
		tx, err := db.BeginTx(r.Context(), nil)
		if err != nil {
			response.Error(
				w,
				http.StatusInternalServerError,
				"failed to create session",
			)
			return
		}
		defer tx.Rollback()

		replaced, err := deleteUserSessions(tx, user.ID)
		if err != nil {
			response.Error(
				w,
				http.StatusInternalServerError,
				"failed to create session",
			)
			return
		}

		token, err := createSession(tx, user.ID)
		if err != nil {
			response.Error(
				w,
				http.StatusInternalServerError,
				"failed to create session",
			)
			return
		}

		if err := tx.Commit(); err != nil {
			response.Error(
				w,
				http.StatusInternalServerError,
				"failed to create session",
			)
			return
		}

		// After the commit: a closed socket cannot be reopened by a rollback.
		for _, old := range replaced {
			closeSession(old, closeReasonLoggedInElsewhere)
		}

		setSessionCookie(w, token)

		response.JSON(
			w,
			http.StatusOK,
			user,
		)
	}
}

func logoutHandler(db *sql.DB, closeSession func(token string, reason string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(middleware.SessionCookieName)

		if err == nil {
			if err := deleteSession(db, cookie.Value); err != nil {
				response.Error(
					w,
					http.StatusInternalServerError,
					"failed to log out",
				)
				return
			}

			// Only once the delete has committed: closing first would drop sockets for a session that is still valid if the delete then fails.
			closeSession(cookie.Value, closeReasonLoggedOut)
		}

		clearSessionCookie(w)

		response.JSON(
			w,
			http.StatusOK,
			map[string]string{
				"message": "logged out",
			},
		)
	}
}

func loadUser(db *sql.DB, userID int64) (UserResponse, error) {
	var user UserResponse

	err := db.QueryRow(`
		SELECT
			id,
			email,
			first_name,
			last_name,
			date_of_birth,
			avatar_path,
			nickname,
			about_me,
			is_public
		FROM users
		WHERE id = ?
	`,
		userID,
	).Scan(
		&user.ID,
		&user.Email,
		&user.FirstName,
		&user.LastName,
		&user.DateOfBirth,
		&user.AvatarPath,
		&user.Nickname,
		&user.AboutMe,
		&user.IsPublic,
	)

	return user, err
}

// Deliberately not behind RequireAuth: a 4xx here is printed to the console by the browser's network stack, where no catch can reach it, on every cold load of a public page.
func sessionHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		loggedOut := func() {
			response.JSON(w, http.StatusOK, map[string]any{"user": nil})
		}

		cookie, err := r.Cookie(middleware.SessionCookieName)
		if err != nil {
			loggedOut()
			return
		}

		userID, err := middleware.ValidateSession(r.Context(), db, cookie.Value)
		if errors.Is(err, middleware.ErrInvalidSession) {
			loggedOut()
			return
		}
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to validate session")
			return
		}

		user, err := loadUser(db, userID)
		if errors.Is(err, sql.ErrNoRows) {
			loggedOut()
			return
		}
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to load user")
			return
		}

		response.JSON(w, http.StatusOK, map[string]any{"user": user})
	}
}

func meHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserID(r)
		if !ok {
			response.Error(
				w,
				http.StatusUnauthorized,
				"not logged in",
			)
			return
		}

		user, err := loadUser(db, userID)

		if errors.Is(err, sql.ErrNoRows) {
			response.Error(
				w,
				http.StatusUnauthorized,
				"user no longer exists",
			)
			return
		}

		if err != nil {
			response.Error(
				w,
				http.StatusInternalServerError,
				"failed to load user",
			)
			return
		}

		response.JSON(
			w,
			http.StatusOK,
			user,
		)
	}
}
