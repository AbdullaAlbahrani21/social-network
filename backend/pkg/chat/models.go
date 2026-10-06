package chat

import "time"

type OutgoingMessage struct {
	ID         int64      `json:"id"`
	SenderID   int64      `json:"sender_id"`
	ReceiverID int64      `json:"receiver_id,omitempty"`
	GroupID    int64      `json:"group_id,omitempty"`
	Content    string     `json:"content"`
	CreatedAt  time.Time  `json:"created_at"`
	ReadAt     *time.Time `json:"read_at,omitempty"`
	TempID     string     `json:"temp_id,omitempty"`
}

type Conversation struct {
	UserID        int64     `json:"userId"`
	Name          string    `json:"name"`
	Nickname      *string   `json:"nickname"`
	AvatarPath    *string   `json:"avatarPath"`
	LastMessage   string    `json:"lastMessage"`
	LastMessageAt time.Time `json:"lastMessageAt"`
	LastSenderID  int64     `json:"lastSenderId"`
	UnreadCount   int       `json:"unreadCount"`
	CanMessage    bool      `json:"canMessage"`
}

type Message struct {
	ID         int64      `json:"id"`
	SenderID   int64      `json:"sender_id"`
	ReceiverID int64      `json:"receiver_id"`
	Content    string     `json:"content"`
	ReadAt     *time.Time `json:"read_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}
