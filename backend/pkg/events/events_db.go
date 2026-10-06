package events

import (
	"context"
	"database/sql"
	"time"
)

func eventGroupID(
	ctx context.Context,
	db *sql.DB,
	eventID int64,
) (int64, error) {
	var groupID int64

	err := db.QueryRowContext(
		ctx,
		`SELECT group_id FROM events WHERE id = ?`,
		eventID,
	).Scan(&groupID)

	return groupID, err
}

const eventSelectColumns = `
	SELECT
		e.id,
		e.group_id,
		e.creator_id,
		u.first_name || ' ' || u.last_name AS creator_name,
		e.title,
		COALESCE(e.description, ''),
		e.event_time,
		e.created_at,
		(
			SELECT COUNT(*)
			FROM event_responses r
			WHERE r.event_id = e.id
			  AND r.response = 'going'
		) AS going_count,
		(
			SELECT COUNT(*)
			FROM event_responses r
			WHERE r.event_id = e.id
			  AND r.response = 'not_going'
		) AS not_going_count,
		(
			SELECT r.response
			FROM event_responses r
			WHERE r.event_id = e.id
			  AND r.user_id = ?
		) AS viewer_response
	FROM events e
	JOIN users u ON u.id = e.creator_id
`

func scanEvent(scan func(dest ...any) error) (EventResponse, error) {
	var event EventResponse
	var eventTime, createdAt time.Time
	var viewerResponse sql.NullString

	err := scan(
		&event.ID,
		&event.GroupID,
		&event.CreatorID,
		&event.CreatorName,
		&event.Title,
		&event.Description,
		&eventTime,
		&createdAt,
		&event.GoingCount,
		&event.NotGoingCount,
		&viewerResponse,
	)
	if err != nil {
		return EventResponse{}, err
	}

	event.EventTime = eventTime.UTC().Format(time.RFC3339)
	event.CreatedAt = createdAt.UTC().Format(time.RFC3339)

	if viewerResponse.Valid {
		event.ViewerResponse = &viewerResponse.String
	}

	return event, nil
}

func createEvent(
	ctx context.Context,
	db *sql.DB,
	groupID int64,
	creatorID int64,
	title string,
	description string,
	eventTime time.Time,
) (EventResponse, error) {
	var storedDescription any
	if description != "" {
		storedDescription = description
	}

	result, err := db.ExecContext(
		ctx,
		`
		INSERT INTO events (
			group_id,
			creator_id,
			title,
			description,
			event_time
		)
		VALUES (?, ?, ?, ?, ?)
		`,
		groupID,
		creatorID,
		title,
		storedDescription,
		eventTime.UTC().Format(sqliteDateTimeLayout),
	)
	if err != nil {
		return EventResponse{}, err
	}

	eventID, err := result.LastInsertId()
	if err != nil {
		return EventResponse{}, err
	}

	return getEvent(ctx, db, creatorID, eventID)
}

func getEvent(
	ctx context.Context,
	db *sql.DB,
	viewerID int64,
	eventID int64,
) (EventResponse, error) {
	query := eventSelectColumns + `
	WHERE e.id = ?
`

	return scanEvent(
		db.QueryRowContext(ctx, query, viewerID, eventID).Scan,
	)
}

func listEvents(
	ctx context.Context,
	db *sql.DB,
	viewerID int64,
	groupID int64,
) ([]EventResponse, error) {
	query := eventSelectColumns + `
	WHERE e.group_id = ?
	ORDER BY e.event_time ASC, e.id ASC
`

	rows, err := db.QueryContext(ctx, query, viewerID, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := []EventResponse{}

	for rows.Next() {
		event, err := scanEvent(rows.Scan)
		if err != nil {
			return nil, err
		}

		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return events, nil
}

func saveResponse(
	ctx context.Context,
	db *sql.DB,
	eventID int64,
	userID int64,
	response string,
) error {
	_, err := db.ExecContext(
		ctx,
		`
		INSERT INTO event_responses (event_id, user_id, response)
		VALUES (?, ?, ?)
		ON CONFLICT (event_id, user_id)
		DO UPDATE SET response = excluded.response
		`,
		eventID,
		userID,
		response,
	)

	return err
}
