package groups

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"social-network/backend/pkg/middleware"
	"social-network/backend/pkg/response"
)

func RegisterRoutes(mux *http.ServeMux, db *sql.DB, hubPush func(int64, interface{})) {
	mux.HandleFunc("POST /api/groups", middleware.RequireAuth(db, createGroupHandler(db)))
	mux.HandleFunc("GET /api/groups", middleware.RequireAuth(db, browseGroupsHandler(db)))
	mux.HandleFunc("GET /api/groups/{id}", middleware.RequireAuth(db, getGroupHandler(db)))
	mux.HandleFunc("DELETE /api/groups/{id}", middleware.RequireAuth(db, deleteGroupHandler(db)))
	mux.HandleFunc("POST /api/groups/{id}/invite", middleware.RequireAuth(db, inviteToGroupHandler(db, hubPush)))
	mux.HandleFunc("POST /api/groups/{id}/join-request", middleware.RequireAuth(db, requestToJoinHandler(db, hubPush)))
	mux.HandleFunc("POST /api/group-invitations/{invID}/accept", middleware.RequireAuth(db, acceptInvitationHandler(db, hubPush)))
	mux.HandleFunc("POST /api/group-invitations/{invID}/decline", middleware.RequireAuth(db, declineInvitationHandler(db, hubPush)))
	mux.HandleFunc("POST /api/groups/{id}/posts", middleware.RequireAuth(db, createGroupPostHandler(db)))
	mux.HandleFunc("GET /api/groups/{id}/posts", middleware.RequireAuth(db, listGroupPostsHandler(db)))
	mux.HandleFunc("GET /api/groups/{id}/members", middleware.RequireAuth(db, listGroupMembersHandler(db)))
	mux.HandleFunc("GET /api/groups/{id}/join-requests", middleware.RequireAuth(db, listJoinRequestsHandler(db)))
	mux.HandleFunc("GET /api/groups/{id}/invitable-users", middleware.RequireAuth(db, listInvitableUsersHandler(db)))
	mux.HandleFunc("GET /api/my-invitations", middleware.RequireAuth(db, listMyInvitationsHandler(db)))
}

func createGroupHandler(db *sql.DB) http.HandlerFunc {
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

		var body struct {
			Title       string `json:"title"`
			Description string `json:"description"`
		}

		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			response.Error(
				w,
				http.StatusBadRequest,
				"invalid request body",
			)
			return
		}

		title := strings.TrimSpace(body.Title)
		description := strings.TrimSpace(body.Description)

		if title == "" || description == "" {
			response.Error(
				w,
				http.StatusBadRequest,
				"title and description are required",
			)
			return
		}

		if len([]rune(title)) > maxTitleLength {
			response.Error(
				w,
				http.StatusBadRequest,
				"title is too long",
			)
			return
		}

		if len([]rune(description)) > maxDescriptionLength {
			response.Error(
				w,
				http.StatusBadRequest,
				"description is too long",
			)
			return
		}

		groupID, err := createGroup(
			r.Context(),
			db,
			userID,
			title,
			description,
		)
		if err != nil {
			response.Error(
				w,
				http.StatusInternalServerError,
				"failed to create group",
			)
			return
		}

		group, err := getGroup(
			r.Context(),
			db,
			userID,
			groupID,
		)
		if err != nil {
			response.Error(
				w,
				http.StatusInternalServerError,
				"group created but could not be loaded",
			)
			return
		}

		response.JSON(
			w,
			http.StatusCreated,
			group,
		)
	}
}

func browseGroupsHandler(db *sql.DB) http.HandlerFunc {
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

		search := strings.TrimSpace(r.URL.Query().Get("search"))

		if len([]rune(search)) > maxSearchLength {
			response.Error(
				w,
				http.StatusBadRequest,
				"search query is too long",
			)
			return
		}

		// Only the exact string "true" filters, so a malformed param browses everything rather than silently hiding groups.
		memberOnly := r.URL.Query().Get("member") == "true"

		groups, err := listGroups(
			r.Context(),
			db,
			userID,
			search,
			memberOnly,
		)
		if err != nil {
			response.Error(
				w,
				http.StatusInternalServerError,
				"failed to load groups",
			)
			return
		}

		response.JSON(
			w,
			http.StatusOK,
			groups,
		)
	}
}

func getGroupHandler(db *sql.DB) http.HandlerFunc {
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

		groupID, ok := idFromPath(w, r, "id", "group")
		if !ok {
			return
		}

		group, err := getGroup(
			r.Context(),
			db,
			userID,
			groupID,
		)

		if errors.Is(err, sql.ErrNoRows) {
			response.Error(
				w,
				http.StatusNotFound,
				"group not found",
			)
			return
		}

		if err != nil {
			response.Error(
				w,
				http.StatusInternalServerError,
				"failed to load group",
			)
			return
		}

		response.JSON(
			w,
			http.StatusOK,
			group,
		)
	}
}

func deleteGroupHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		groupID, _, ok := requireGroupCreatorAccess(
			w,
			r,
			db,
			"only the group's creator can delete it",
		)
		if !ok {
			return
		}

		if err := deleteGroup(r.Context(), db, groupID); err != nil {
			response.Error(
				w,
				http.StatusInternalServerError,
				"failed to delete group",
			)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
