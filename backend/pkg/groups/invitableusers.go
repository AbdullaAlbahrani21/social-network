package groups

import (
	"database/sql"
	"net/http"
	"strings"

	"social-network/backend/pkg/response"
)

func listInvitableUsersHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		groupID, userID, ok := requireGroupMemberAccess(
			w,
			r,
			db,
			"only group members can search for people to invite",
		)
		if !ok {
			return
		}

		// After the access checks rather than before: a 400 here would confirm the group exists to a caller who may not see it.
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

		users, err := listInvitableUsers(
			r.Context(),
			db,
			groupID,
			userID,
			search,
		)
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
