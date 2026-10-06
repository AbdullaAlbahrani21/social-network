package groups

import (
	"database/sql"
	"net/http"

	"social-network/backend/pkg/middleware"
	"social-network/backend/pkg/response"
)

func requireGroupCreatorAccess(
	w http.ResponseWriter,
	r *http.Request,
	db *sql.DB,
	forbidden string,
) (groupID int64, userID int64, ok bool) {
	userID, ok = middleware.UserID(r)
	if !ok {
		response.Error(
			w,
			http.StatusUnauthorized,
			"not logged in",
		)
		return 0, 0, false
	}

	groupID, ok = idFromPath(w, r, "id", "group")
	if !ok {
		return 0, 0, false
	}

	exists, err := groupExists(r.Context(), db, groupID)
	if err != nil {
		response.Error(
			w,
			http.StatusInternalServerError,
			"failed to load group",
		)
		return 0, 0, false
	}
	if !exists {
		response.Error(
			w,
			http.StatusNotFound,
			"group not found",
		)
		return 0, 0, false
	}

	creatorID, err := groupCreatorID(r.Context(), db, groupID)
	if err != nil {
		response.Error(
			w,
			http.StatusInternalServerError,
			"failed to load group",
		)
		return 0, 0, false
	}
	if creatorID != userID {
		response.Error(
			w,
			http.StatusForbidden,
			forbidden,
		)
		return 0, 0, false
	}

	return groupID, userID, true
}

func listJoinRequestsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		groupID, _, ok := requireGroupCreatorAccess(
			w,
			r,
			db,
			"only the group creator can view join requests",
		)
		if !ok {
			return
		}

		requests, err := listPendingJoinRequests(r.Context(), db, groupID)
		if err != nil {
			response.Error(
				w,
				http.StatusInternalServerError,
				"failed to load join requests",
			)
			return
		}

		response.JSON(
			w,
			http.StatusOK,
			requests,
		)
	}
}
