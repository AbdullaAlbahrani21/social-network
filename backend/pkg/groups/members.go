package groups

import (
	"database/sql"
	"net/http"

	"social-network/backend/pkg/response"
)

func listGroupMembersHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		groupID, _, ok := requireGroupMemberAccess(
			w,
			r,
			db,
			"only group members can view the member list",
		)
		if !ok {
			return
		}

		members, err := listGroupMembers(r.Context(), db, groupID)
		if err != nil {
			response.Error(
				w,
				http.StatusInternalServerError,
				"failed to load members",
			)
			return
		}

		response.JSON(
			w,
			http.StatusOK,
			members,
		)
	}
}
