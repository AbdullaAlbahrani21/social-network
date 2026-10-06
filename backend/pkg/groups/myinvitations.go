package groups

import (
	"database/sql"
	"net/http"

	"social-network/backend/pkg/middleware"
	"social-network/backend/pkg/response"
)

func listMyInvitationsHandler(db *sql.DB) http.HandlerFunc {
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

		invitations, err := listMyPendingInvitations(r.Context(), db, userID)
		if err != nil {
			response.Error(
				w,
				http.StatusInternalServerError,
				"failed to load invitations",
			)
			return
		}

		response.JSON(
			w,
			http.StatusOK,
			invitations,
		)
	}
}
