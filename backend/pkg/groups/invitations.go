package groups

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	"social-network/backend/pkg/middleware"
	"social-network/backend/pkg/notifications"
	"social-network/backend/pkg/response"
)

var errNotPending = errors.New("invitation is no longer pending")

var errGroupFull = errors.New("group is full")

const notifyTargetType = "group_invitation"

func notify(
	db *sql.DB,
	userID int64,
	notifType notifications.Type,
	actorID int64,
	targetID int64,
	message string,
	hubPush func(int64, interface{}),
) {
	err := notifications.Create(
		db,
		userID,
		notifType,
		actorID,
		notifyTargetType,
		targetID,
		hubPush,
		message,
	)
	if err != nil {
		log.Printf(
			"groups: failed to create %s notification for user %d: %v",
			notifType,
			userID,
			err,
		)
	}
}

func inviteMessage(r *http.Request, db *sql.DB, groupID int64) string {
	title, err := groupTitle(r.Context(), db, groupID)
	if err != nil {
		log.Printf(
			"groups: failed to load title of group %d for invite notification: %v",
			groupID,
			err,
		)
		return "You were invited to join a group."
	}

	return "You were invited to join " + title + "."
}

func joinRequestMessage(
	r *http.Request,
	db *sql.DB,
	groupID int64,
	requesterID int64,
) string {
	title, err := groupTitle(r.Context(), db, groupID)
	if err != nil {
		log.Printf(
			"groups: failed to load title of group %d for join-request notification: %v",
			groupID,
			err,
		)
		return "Someone requested to join your group."
	}

	name, err := userFullName(r.Context(), db, requesterID)
	if err != nil {
		log.Printf(
			"groups: failed to load name of user %d for join-request notification: %v",
			requesterID,
			err,
		)
		return "Someone requested to join " + title + "."
	}

	return name + " requested to join " + title + "."
}

func declineMessage(
	r *http.Request,
	db *sql.DB,
	groupID int64,
	declinerID int64,
) string {
	title, err := groupTitle(r.Context(), db, groupID)
	if err != nil {
		log.Printf(
			"groups: failed to load title of group %d for decline notification: %v",
			groupID,
			err,
		)
		return "Your group invitation was declined."
	}

	name, err := userFullName(r.Context(), db, declinerID)
	if err != nil {
		log.Printf(
			"groups: failed to load name of user %d for decline notification: %v",
			declinerID,
			err,
		)
		return "Your invitation to " + title + " was declined."
	}

	return name + " declined your invitation to " + title + "."
}

// inviter_id is NULL for a join request and for an invite whose sender deleted their account (ON DELETE SET NULL), so neither has a recipient.
func notifyInviterOfDecline(
	r *http.Request,
	db *sql.DB,
	invitation InvitationResponse,
	declinerID int64,
	hubPush func(int64, interface{}),
) {
	if invitation.Type != typeInvite || invitation.InviterID == nil {
		return
	}

	notify(
		db,
		*invitation.InviterID,
		notifications.TypeInvitationDeclined,
		declinerID,
		invitation.ID,
		declineMessage(r, db, invitation.GroupID, declinerID),
		hubPush,
	)
}

func idFromPath(
	w http.ResponseWriter,
	r *http.Request,
	name string,
	label string,
) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil || id < 1 {
		response.Error(
			w,
			http.StatusBadRequest,
			"invalid "+label+" id",
		)
		return 0, false
	}

	return id, true
}

func inviteToGroupHandler(db *sql.DB, hubPush func(int64, interface{})) http.HandlerFunc {
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

		var body struct {
			UserID int64 `json:"userId"`
		}

		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			response.Error(
				w,
				http.StatusBadRequest,
				"invalid request body",
			)
			return
		}

		if body.UserID < 1 {
			response.Error(
				w,
				http.StatusBadRequest,
				"userId is required",
			)
			return
		}

		exists, err := groupExists(r.Context(), db, groupID)
		if err != nil {
			response.Error(
				w,
				http.StatusInternalServerError,
				"failed to load group",
			)
			return
		}
		if !exists {
			response.Error(
				w,
				http.StatusNotFound,
				"group not found",
			)
			return
		}

		callerIsMember, err := isGroupMember(
			r.Context(),
			db,
			groupID,
			userID,
		)
		if err != nil {
			response.Error(
				w,
				http.StatusInternalServerError,
				"failed to check membership",
			)
			return
		}
		if !callerIsMember {
			response.Error(
				w,
				http.StatusForbidden,
				"only group members can invite others",
			)
			return
		}

		targetExists, err := userExists(r.Context(), db, body.UserID)
		if err != nil {
			response.Error(
				w,
				http.StatusInternalServerError,
				"failed to load user",
			)
			return
		}
		if !targetExists {
			response.Error(
				w,
				http.StatusNotFound,
				"user not found",
			)
			return
		}

		targetIsMember, err := isGroupMember(
			r.Context(),
			db,
			groupID,
			body.UserID,
		)
		if err != nil {
			response.Error(
				w,
				http.StatusInternalServerError,
				"failed to check membership",
			)
			return
		}
		if targetIsMember {
			response.Error(
				w,
				http.StatusConflict,
				"user is already a member of this group",
			)
			return
		}

		pending, err := hasPendingInvitation(
			r.Context(),
			db,
			groupID,
			body.UserID,
		)
		if err != nil {
			response.Error(
				w,
				http.StatusInternalServerError,
				"failed to check invitations",
			)
			return
		}
		if pending {
			response.Error(
				w,
				http.StatusConflict,
				"there is already a pending invitation or request for this user",
			)
			return
		}

		invitation, err := createInvitation(
			r.Context(),
			db,
			groupID,
			body.UserID,
			&userID,
			typeInvite,
		)
		if err != nil {
			// A concurrent invite can still lose the UNIQUE race after the check above.
			if isUniqueViolation(err) {
				response.Error(
					w,
					http.StatusConflict,
					"there is already a pending invitation or request for this user",
				)
				return
			}

			response.Error(
				w,
				http.StatusInternalServerError,
				"failed to create invitation",
			)
			return
		}

		notify(
			db,
			body.UserID,
			notifications.TypeGroupInvite,
			userID,
			invitation.ID,
			inviteMessage(r, db, groupID),
			hubPush,
		)

		response.JSON(
			w,
			http.StatusCreated,
			invitation,
		)
	}
}

func requestToJoinHandler(db *sql.DB, hubPush func(int64, interface{})) http.HandlerFunc {
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

		exists, err := groupExists(r.Context(), db, groupID)
		if err != nil {
			response.Error(
				w,
				http.StatusInternalServerError,
				"failed to load group",
			)
			return
		}
		if !exists {
			response.Error(
				w,
				http.StatusNotFound,
				"group not found",
			)
			return
		}

		alreadyMember, err := isGroupMember(
			r.Context(),
			db,
			groupID,
			userID,
		)
		if err != nil {
			response.Error(
				w,
				http.StatusInternalServerError,
				"failed to check membership",
			)
			return
		}
		if alreadyMember {
			response.Error(
				w,
				http.StatusConflict,
				"you are already a member of this group",
			)
			return
		}

		pending, err := hasPendingInvitation(
			r.Context(),
			db,
			groupID,
			userID,
		)
		if err != nil {
			response.Error(
				w,
				http.StatusInternalServerError,
				"failed to check invitations",
			)
			return
		}
		if pending {
			response.Error(
				w,
				http.StatusConflict,
				"you already have a pending invitation or request for this group",
			)
			return
		}

		creatorID, err := groupCreatorID(r.Context(), db, groupID)
		if err != nil {
			response.Error(
				w,
				http.StatusInternalServerError,
				"failed to load group",
			)
			return
		}

		invitation, err := createInvitation(
			r.Context(),
			db,
			groupID,
			userID,
			nil,
			typeRequest,
		)
		if err != nil {
			if isUniqueViolation(err) {
				response.Error(
					w,
					http.StatusConflict,
					"you already have a pending invitation or request for this group",
				)
				return
			}

			response.Error(
				w,
				http.StatusInternalServerError,
				"failed to create join request",
			)
			return
		}

		notify(
			db,
			creatorID,
			notifications.TypeGroupJoinRequest,
			userID,
			invitation.ID,
			joinRequestMessage(r, db, groupID, userID),
			hubPush,
		)

		response.JSON(
			w,
			http.StatusCreated,
			invitation,
		)
	}
}

func acceptInvitationHandler(db *sql.DB, hubPush func(int64, interface{})) http.HandlerFunc {
	return resolveInvitationHandler(db, statusAccepted, hubPush)
}

func declineInvitationHandler(db *sql.DB, hubPush func(int64, interface{})) http.HandlerFunc {
	return resolveInvitationHandler(db, statusDeclined, hubPush)
}

func resolveInvitationHandler(
	db *sql.DB,
	newStatus string,
	hubPush func(int64, interface{}),
) http.HandlerFunc {
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

		invitationID, ok := idFromPath(w, r, "invID", "invitation")
		if !ok {
			return
		}

		invitation, err := getInvitation(r.Context(), db, invitationID)

		if errors.Is(err, sql.ErrNoRows) {
			response.Error(
				w,
				http.StatusNotFound,
				"invitation not found",
			)
			return
		}

		if err != nil {
			response.Error(
				w,
				http.StatusInternalServerError,
				"failed to load invitation",
			)
			return
		}

		if invitation.Status != statusPending {
			response.Error(
				w,
				http.StatusConflict,
				"invitation has already been "+invitation.Status,
			)
			return
		}

		allowed, err := canResolveInvitation(r, db, invitation, userID)
		if err != nil {
			response.Error(
				w,
				http.StatusInternalServerError,
				"failed to load group",
			)
			return
		}
		if !allowed {
			response.Error(
				w,
				http.StatusForbidden,
				"you are not allowed to respond to this invitation",
			)
			return
		}

		updated, err := resolveInvitation(
			r.Context(),
			db,
			invitation,
			newStatus,
		)

		if errors.Is(err, errNotPending) {
			response.Error(
				w,
				http.StatusConflict,
				"invitation is no longer pending",
			)
			return
		}

		if errors.Is(err, errGroupFull) {
			response.Error(
				w,
				http.StatusConflict,
				"group is full",
			)
			return
		}

		if err != nil {
			response.Error(
				w,
				http.StatusInternalServerError,
				"failed to update invitation",
			)
			return
		}

		if newStatus == statusDeclined {
			notifyInviterOfDecline(r, db, invitation, userID, hubPush)
		}

		response.JSON(
			w,
			http.StatusOK,
			updated,
		)
	}
}

func canResolveInvitation(
	r *http.Request,
	db *sql.DB,
	invitation InvitationResponse,
	userID int64,
) (bool, error) {
	if invitation.Type == typeInvite {
		return invitation.InvitedUserID == userID, nil
	}

	creatorID, err := groupCreatorID(
		r.Context(),
		db,
		invitation.GroupID,
	)
	if err != nil {
		return false, err
	}

	return creatorID == userID, nil
}
