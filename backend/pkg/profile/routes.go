package profile

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"social-network/backend/pkg/auth"
	"social-network/backend/pkg/middleware"
	"social-network/backend/pkg/response"
)

type userSummary struct {
	ID         int64   `json:"id"`
	FirstName  string  `json:"firstName"`
	LastName   string  `json:"lastName"`
	AvatarPath *string `json:"avatarPath"`
	Nickname   *string `json:"nickname"`
}

type profilePost struct {
	ID        int64   `json:"id"`
	Content   string  `json:"content"`
	ImagePath *string `json:"imagePath"`
	Privacy   string  `json:"privacy"`
	CreatedAt string  `json:"createdAt"`
}

type profileResponse struct {
	User         auth.UserResponse `json:"user"`
	Posts        []profilePost     `json:"posts"`
	Followers    []userSummary     `json:"followers"`
	Following    []userSummary     `json:"following"`
	IsOwn        bool              `json:"isOwn"`
	IsFollower   bool              `json:"isFollower"`
	FollowStatus string            `json:"followStatus"`
	IsRestricted bool              `json:"isRestricted"`

	PostCount      int `json:"postCount"`
	FollowerCount  int `json:"followerCount"`
	FollowingCount int `json:"followingCount"`
}

func RegisterRoutes(mux *http.ServeMux, db *sql.DB) {
	mux.HandleFunc(
		"GET /api/profile/{id}",
		middleware.RequireAuth(db, getProfileHandler(db)),
	)
	mux.HandleFunc(
		"PUT /api/profile/visibility",
		middleware.RequireAuth(db, updateVisibilityHandler(db)),
	)
}

func getProfileHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requesterID, ok := middleware.UserID(r)
		if !ok {
			response.Error(w, http.StatusUnauthorized, "not logged in")
			return
		}

		profileID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || profileID <= 0 {
			response.Error(w, http.StatusBadRequest, "invalid profile id")
			return
		}

		var user auth.UserResponse
		var isPublic int
		err = db.QueryRowContext(r.Context(), `
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
        `, profileID).Scan(
			&user.ID,
			&user.Email,
			&user.FirstName,
			&user.LastName,
			&user.DateOfBirth,
			&user.AvatarPath,
			&user.Nickname,
			&user.AboutMe,
			&isPublic,
		)
		if errors.Is(err, sql.ErrNoRows) {
			response.Error(w, http.StatusNotFound, "profile not found")
			return
		}
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to load profile")
			return
		}
		user.IsPublic = isPublic == 1

		isOwn := requesterID == profileID
		followStatus := "none"
		isFollower := false

		if !isOwn {
			var status string
			err = db.QueryRowContext(r.Context(), `
                SELECT status
                FROM followers
                WHERE follower_id = ?
                  AND following_id = ?
            `, requesterID, profileID).Scan(&status)

			if err == nil {
				followStatus = status
				isFollower = status == "accepted"
			} else if !errors.Is(err, sql.ErrNoRows) {
				response.Error(w, http.StatusInternalServerError, "failed to check follow status")
				return
			}
		}

		isRestricted := !user.IsPublic && !isOwn && !isFollower

		var postCount, followerCount, followingCount int
		err = db.QueryRowContext(r.Context(), `
            SELECT
                (SELECT COUNT(*) FROM posts
                 WHERE user_id = ?1 AND group_id IS NULL),
                (SELECT COUNT(*) FROM followers
                 WHERE following_id = ?1 AND status = 'accepted'),
                (SELECT COUNT(*) FROM followers
                 WHERE follower_id = ?1 AND status = 'accepted')
        `, profileID).Scan(&postCount, &followerCount, &followingCount)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to load profile counts")
			return
		}

		posts := make([]profilePost, 0)
		followers := make([]userSummary, 0)
		following := make([]userSummary, 0)

		if isRestricted {
			user.Email = ""
			user.DateOfBirth = ""
			user.AboutMe = nil
		} else {
			posts, err = loadVisiblePosts(db, requesterID, profileID, isOwn, isFollower)
			if err != nil {
				response.Error(w, http.StatusInternalServerError, "failed to load profile posts")
				return
			}

			followers, err = loadUserList(db, `
                SELECT u.id, u.first_name, u.last_name, u.avatar_path, u.nickname
                FROM followers f
                JOIN users u ON u.id = f.follower_id
                WHERE f.following_id = ?
                  AND f.status = 'accepted'
                ORDER BY u.first_name, u.last_name
            `, profileID)
			if err != nil {
				response.Error(w, http.StatusInternalServerError, "failed to load followers")
				return
			}

			following, err = loadUserList(db, `
                SELECT u.id, u.first_name, u.last_name, u.avatar_path, u.nickname
                FROM followers f
                JOIN users u ON u.id = f.following_id
                WHERE f.follower_id = ?
                  AND f.status = 'accepted'
                ORDER BY u.first_name, u.last_name
            `, profileID)
			if err != nil {
				response.Error(w, http.StatusInternalServerError, "failed to load following")
				return
			}
		}

		response.JSON(w, http.StatusOK, profileResponse{
			User:         user,
			Posts:        posts,
			Followers:    followers,
			Following:    following,
			IsOwn:        isOwn,
			IsFollower:   isFollower,
			FollowStatus: followStatus,
			IsRestricted: isRestricted,

			PostCount:      postCount,
			FollowerCount:  followerCount,
			FollowingCount: followingCount,
		})
	}
}

func updateVisibilityHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserID(r)
		if !ok {
			response.Error(w, http.StatusUnauthorized, "not logged in")
			return
		}

		var body struct {
			IsPublic *bool `json:"isPublic"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.IsPublic == nil {
			response.Error(w, http.StatusBadRequest, "isPublic is required")
			return
		}

		value := 0
		if *body.IsPublic {
			value = 1
		}

		if _, err := db.ExecContext(r.Context(), `
            UPDATE users
            SET is_public = ?
            WHERE id = ?
        `, value, userID); err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to update profile visibility")
			return
		}

		response.JSON(w, http.StatusOK, map[string]bool{
			"isPublic": *body.IsPublic,
		})
	}
}

func loadVisiblePosts(
	db *sql.DB,
	requesterID int64,
	profileID int64,
	isOwn bool,
	isFollower bool,
) ([]profilePost, error) {
	rows, err := db.Query(`
        SELECT p.id, p.content, p.image_path, p.privacy, p.created_at
        FROM posts p
        WHERE p.user_id = ?
          AND p.group_id IS NULL
          AND (
                ? = 1
             OR p.privacy = 'public'
             OR (p.privacy = 'almost_private' AND ? = 1)
             OR (
                    p.privacy = 'private'
                AND EXISTS (
                    SELECT 1
                    FROM post_visibility pv
                    WHERE pv.post_id = p.id
                      AND pv.user_id = ?
                )
             )
          )
        ORDER BY p.created_at DESC
    `, profileID, isOwn, isFollower, requesterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	posts := make([]profilePost, 0)
	for rows.Next() {
		var post profilePost
		if err := rows.Scan(
			&post.ID,
			&post.Content,
			&post.ImagePath,
			&post.Privacy,
			&post.CreatedAt,
		); err != nil {
			return nil, err
		}
		posts = append(posts, post)
	}

	return posts, rows.Err()
}

func loadUserList(db *sql.DB, query string, profileID int64) ([]userSummary, error) {
	rows, err := db.Query(query, profileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]userSummary, 0)
	for rows.Next() {
		var user userSummary
		if err := rows.Scan(
			&user.ID,
			&user.FirstName,
			&user.LastName,
			&user.AvatarPath,
			&user.Nickname,
		); err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	return users, rows.Err()
}
