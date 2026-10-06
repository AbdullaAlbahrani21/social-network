package events

type EventResponse struct {
	ID             int64   `json:"id"`
	GroupID        int64   `json:"groupId"`
	CreatorID      int64   `json:"creatorId"`
	CreatorName    string  `json:"creatorName"`
	Title          string  `json:"title"`
	Description    string  `json:"description"`
	EventTime      string  `json:"eventTime"`
	CreatedAt      string  `json:"createdAt"`
	GoingCount     int     `json:"goingCount"`
	NotGoingCount  int     `json:"notGoingCount"`
	ViewerResponse *string `json:"viewerResponse"`
}

type RSVPResponse struct {
	EventID  int64  `json:"eventId"`
	UserID   int64  `json:"userId"`
	Response string `json:"response"`
}

const (
	responseGoing    = "going"
	responseNotGoing = "not_going"

	maxTitleLength       = 100
	maxDescriptionLength = 1000

	maxEventYearsAhead = 2
)

// SQLite has no date type: event_time is text and ORDER BY is a string compare, so this fixed-width UTC layout is what makes lexical order chronological.
const sqliteDateTimeLayout = "2006-01-02 15:04:05"
