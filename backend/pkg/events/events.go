package events

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"social-network/backend/pkg/groups"
	"social-network/backend/pkg/middleware"
	"social-network/backend/pkg/notifications"
	"social-network/backend/pkg/response"
)

const notifyTargetType = "event"

// A package-level variable rather than a direct call, so tests can record and fail the fan-out; swapping it makes those tests unsafe under t.Parallel().
var createNotification = notifications.Create

// The frame type is a wire contract with frontend/src/stores/websocket.js, matched exactly.
const eventCreatedFrameType = "group_event"

type eventCreatedFrame struct {
	Type    string        `json:"type"`
	GroupID int64         `json:"groupId"`
	Event   EventResponse `json:"event"`
}

// One pass over the members: every member (the creator included, whose other tabs know nothing of the POST) gets the live event, and everyone but the creator also gets a notification.
func announceEvent(
	db *sql.DB,
	event EventResponse,
	creatorID int64,
	hubPush func(int64, interface{}),
) {
	groupID := event.GroupID

	memberIDs, err := groups.GroupMemberIDs(db, groupID)
	if err != nil {
		log.Printf(
			"events: failed to load members of group %d to notify: %v",
			groupID,
			err,
		)
		return
	}

	frame := eventCreatedFrame{
		Type:    eventCreatedFrameType,
		GroupID: groupID,
		Event:   viewerNeutral(event),
	}

	message := eventCreatedMessage(db, groupID)

	for _, memberID := range memberIDs {
		hubPush(memberID, frame)

		if memberID == creatorID {
			continue
		}

		err := createNotification(
			db,
			memberID,
			notifications.TypeGroupEventCreated,
			creatorID,
			notifyTargetType,
			event.ID,
			hubPush,
			message,
		)
		if err != nil {
			log.Printf(
				"events: failed to notify user %d about event %d: %v",
				memberID,
				event.ID,
				err,
			)
		}
	}
}

// The stored event was read as the creator, but one frame goes to every member: a brand new event has no responses, so stripping the viewer's columns is what makes it true for all of them.
func viewerNeutral(event EventResponse) EventResponse {
	event.GoingCount = 0
	event.NotGoingCount = 0
	event.ViewerResponse = nil

	return event
}

func eventCreatedMessage(db *sql.DB, groupID int64) string {
	title, err := groups.GroupTitle(context.Background(), db, groupID)
	if err != nil {
		log.Printf(
			"events: failed to load title of group %d for notification: %v",
			groupID,
			err,
		)
		return "A new event was posted in one of your groups."
	}

	return "A new event was posted in " + title + "."
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

// Existence is checked before membership, so a bad group id reads as 404 rather than a 403 that would confirm the group exists.
func requireGroupMembership(
	w http.ResponseWriter,
	r *http.Request,
	db *sql.DB,
) (groupID int64, userID int64, ok bool) {
	userID, ok = middleware.UserID(r)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "not logged in")
		return 0, 0, false
	}

	groupID, ok = idFromPath(w, r, "id", "group")
	if !ok {
		return 0, 0, false
	}

	exists, err := groups.GroupExists(r.Context(), db, groupID)
	if err != nil {
		response.Error(
			w,
			http.StatusInternalServerError,
			"failed to load group",
		)
		return 0, 0, false
	}
	if !exists {
		response.Error(w, http.StatusNotFound, "group not found")
		return 0, 0, false
	}

	if !checkMembership(w, db, groupID, userID) {
		return 0, 0, false
	}

	return groupID, userID, true
}

func checkMembership(
	w http.ResponseWriter,
	db *sql.DB,
	groupID int64,
	userID int64,
) bool {
	member, err := groups.IsGroupMember(db, groupID, userID)
	if err != nil {
		response.Error(
			w,
			http.StatusInternalServerError,
			"failed to check membership",
		)
		return false
	}
	if !member {
		response.Error(
			w,
			http.StatusForbidden,
			"only group members can view or manage group events",
		)
		return false
	}

	return true
}

func createEventHandler(db *sql.DB, hubPush func(int64, interface{})) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		groupID, userID, ok := requireGroupMembership(w, r, db)
		if !ok {
			return
		}

		var body struct {
			Title       string `json:"title"`
			Description string `json:"description"`
			EventTime   string `json:"eventTime"`
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

		if title == "" {
			response.Error(w, http.StatusBadRequest, "title is required")
			return
		}

		if len([]rune(title)) > maxTitleLength {
			response.Error(w, http.StatusBadRequest, "title is too long")
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

		if strings.TrimSpace(body.EventTime) == "" {
			response.Error(
				w,
				http.StatusBadRequest,
				"eventTime is required",
			)
			return
		}

		eventTime, err := time.Parse(
			time.RFC3339,
			strings.TrimSpace(body.EventTime),
		)
		if err != nil {
			response.Error(
				w,
				http.StatusBadRequest,
				"eventTime must be an RFC3339 timestamp",
			)
			return
		}

		if !eventTime.After(time.Now()) {
			response.Error(
				w,
				http.StatusBadRequest,
				"Event date and time must be in the future.",
			)
			return
		}

		if eventTime.After(time.Now().AddDate(maxEventYearsAhead, 0, 0)) {
			response.Error(
				w,
				http.StatusBadRequest,
				"Event date and time is too far in the future: it must be within 2 years.",
			)
			return
		}

		event, err := createEvent(
			r.Context(),
			db,
			groupID,
			userID,
			title,
			description,
			eventTime,
		)
		if err != nil {
			response.Error(
				w,
				http.StatusInternalServerError,
				"failed to create event",
			)
			return
		}

		announceEvent(db, event, userID, hubPush)

		response.JSON(w, http.StatusCreated, event)
	}
}

func listEventsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		groupID, userID, ok := requireGroupMembership(w, r, db)
		if !ok {
			return
		}

		events, err := listEvents(r.Context(), db, userID, groupID)
		if err != nil {
			response.Error(
				w,
				http.StatusInternalServerError,
				"failed to load events",
			)
			return
		}

		response.JSON(w, http.StatusOK, events)
	}
}

func rsvpHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserID(r)
		if !ok {
			response.Error(w, http.StatusUnauthorized, "not logged in")
			return
		}

		eventID, ok := idFromPath(w, r, "id", "event")
		if !ok {
			return
		}

		groupID, err := eventGroupID(r.Context(), db, eventID)

		if errors.Is(err, sql.ErrNoRows) {
			response.Error(w, http.StatusNotFound, "event not found")
			return
		}

		if err != nil {
			response.Error(
				w,
				http.StatusInternalServerError,
				"failed to load event",
			)
			return
		}

		if !checkMembership(w, db, groupID, userID) {
			return
		}

		var body struct {
			Response string `json:"response"`
		}

		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			response.Error(
				w,
				http.StatusBadRequest,
				"invalid request body",
			)
			return
		}

		answer := strings.TrimSpace(body.Response)
		if answer != responseGoing && answer != responseNotGoing {
			response.Error(
				w,
				http.StatusBadRequest,
				"response must be going or not_going",
			)
			return
		}

		if err := saveResponse(
			r.Context(),
			db,
			eventID,
			userID,
			answer,
		); err != nil {
			response.Error(
				w,
				http.StatusInternalServerError,
				"failed to save response",
			)
			return
		}

		response.JSON(w, http.StatusOK, RSVPResponse{
			EventID:  eventID,
			UserID:   userID,
			Response: answer,
		})
	}
}
