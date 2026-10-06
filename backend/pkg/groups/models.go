package groups

type GroupResponse struct {
	ID          int64   `json:"id"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	CreatorID   int64   `json:"creatorId"`
	CreatorName string  `json:"creatorName"`
	CreatedAt   string  `json:"createdAt"`
	MemberCount int     `json:"memberCount"`
	IsMember    bool    `json:"isMember"`
	Role        *string `json:"role"`
}

type GroupPostResponse struct {
	ID             int64   `json:"id"`
	GroupID        int64   `json:"groupId"`
	UserID         int64   `json:"userId"`
	Content        string  `json:"content"`
	CreatedAt      string  `json:"createdAt"`
	AuthorName     string  `json:"authorName"`
	AuthorNickname *string `json:"authorNickname"`
	LikeCount      int64   `json:"likeCount"`
	ViewerHasLiked bool    `json:"viewerHasLiked"`
}

type InvitationResponse struct {
	ID            int64  `json:"id"`
	GroupID       int64  `json:"groupId"`
	InvitedUserID int64  `json:"invitedUserId"`
	InviterID     *int64 `json:"inviterId"`
	Type          string `json:"type"`
	Status        string `json:"status"`
	CreatedAt     string `json:"createdAt"`
}

type GroupMemberResponse struct {
	UserID   int64   `json:"userId"`
	Name     string  `json:"name"`
	Nickname *string `json:"nickname"`
	Role     string  `json:"role"`
	JoinedAt string  `json:"joinedAt"`
}

type PendingJoinRequestResponse struct {
	InvitationID int64   `json:"invitationId"`
	UserID       int64   `json:"userId"`
	Name         string  `json:"name"`
	Nickname     *string `json:"nickname"`
	CreatedAt    string  `json:"createdAt"`
}

type MyInvitationResponse struct {
	InvitationID int64   `json:"invitationId"`
	GroupID      int64   `json:"groupId"`
	GroupTitle   string  `json:"groupTitle"`
	InviterName  *string `json:"inviterName"`
	CreatedAt    string  `json:"createdAt"`
}

const (
	roleCreator = "creator"
	roleMember  = "member"

	typeInvite  = "invite"
	typeRequest = "request"

	statusPending  = "pending"
	statusAccepted = "accepted"
	statusDeclined = "declined"

	maxTitleLength       = 100
	maxDescriptionLength = 1000
	maxPostContentLength = 5000
	maxSearchLength      = 100
)

// Ten is a team decision: a group message is pushed to every member and a new event writes one notifications row per member.
const MaxGroupMembers = 10

// Deliberately no Email field: the query matches on email, but returning it would let any member harvest addresses.
type InvitableUserResponse struct {
	UserID   int64   `json:"userId"`
	Name     string  `json:"name"`
	Nickname *string `json:"nickname"`
}

const maxInvitableUserResults = 20
