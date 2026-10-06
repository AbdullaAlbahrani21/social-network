package chat

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"social-network/backend/pkg/groups"
)

var ErrNotGroupMember = errors.New("not a group member")

// Callers must alias users as u and bind the sender as ?1.
const CanMessageCondition = `(
	u.is_public = 1
	OR EXISTS (
		SELECT 1
		FROM followers f
		WHERE f.status = 'accepted'
		  AND (
			(f.follower_id = ?1 AND f.following_id = u.id)
			OR (f.follower_id = u.id AND f.following_id = ?1)
		  )
	)
)`

func CheckCanMessage(db *sql.DB, senderID, receiverID int64) (bool, error) {
	var ok int
	err := db.QueryRow(`
		SELECT 1
		FROM users u
		WHERE u.id = ?2
		  AND `+CanMessageCondition,
		senderID, receiverID,
	).Scan(&ok)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// A pair's latest message is its highest id: ids only grow, and created_at is written at insert.
func ListConversations(db *sql.DB, userID int64) ([]Conversation, error) {
	rows, err := db.Query(`
		WITH latest AS (
			SELECT CASE WHEN sender_id = ?1 THEN receiver_id ELSE sender_id END AS partner_id,
			       MAX(id) AS last_message_id
			FROM messages
			WHERE (sender_id = ?1 OR receiver_id = ?1)
			  AND sender_id != receiver_id
			GROUP BY partner_id
		)
		SELECT u.id, u.first_name || ' ' || u.last_name, u.nickname, u.avatar_path,
		       m.content, m.created_at, m.sender_id,
		       (SELECT COUNT(*) FROM messages x
		        WHERE x.sender_id = u.id AND x.receiver_id = ?1 AND x.read_at IS NULL),
		       `+CanMessageCondition+`
		FROM latest l
		JOIN users u ON u.id = l.partner_id
		JOIN messages m ON m.id = l.last_message_id
		ORDER BY m.created_at DESC, m.id DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	conversations := []Conversation{}
	for rows.Next() {
		var c Conversation
		if err := rows.Scan(
			&c.UserID, &c.Name, &c.Nickname, &c.AvatarPath,
			&c.LastMessage, &c.LastMessageAt, &c.LastSenderID,
			&c.UnreadCount, &c.CanMessage,
		); err != nil {
			return nil, err
		}
		conversations = append(conversations, c)
	}
	return conversations, rows.Err()
}

func PersistOneToOne(db *sql.DB, fromID, toID int64, content string) (OutgoingMessage, error) {
	var out OutgoingMessage
	tx, err := db.Begin()
	if err != nil {
		return out, err
	}

	// RETURNING reads back the stored created_at: a Go time.Now() is a different clock domain and made the message jump position on reload.
	var msgID int64
	var createdAt time.Time
	err = tx.QueryRow(`
		INSERT INTO messages (sender_id, receiver_id, content)
		VALUES (?, ?, ?)
		RETURNING id, created_at`,
		fromID, toID, content,
	).Scan(&msgID, &createdAt)
	if err != nil {
		tx.Rollback()
		return out, err
	}

	if err := tx.Commit(); err != nil {
		return out, err
	}
	out = OutgoingMessage{
		ID:         msgID,
		SenderID:   fromID,
		ReceiverID: toID,
		Content:    content,
		CreatedAt:  createdAt,
	}
	return out, nil
}

func PersistGroupMessage(db *sql.DB, fromID, groupID int64, content string, getMembers func(int64) ([]int64, error)) (OutgoingMessage, []int64, error) {
	var out OutgoingMessage
	tx, err := db.Begin()
	if err != nil {
		return out, nil, err
	}
	// Membership is checked inside the transaction, before the INSERT, so a non-member's message is never written.
	member, err := groups.IsGroupMemberTx(context.Background(), tx, groupID, fromID)
	if err != nil {
		tx.Rollback()
		return out, nil, err
	}
	if !member {
		tx.Rollback()
		return out, nil, ErrNotGroupMember
	}

	var msgID int64
	var createdAt time.Time
	err = tx.QueryRow(`
		INSERT INTO group_messages (group_id, sender_id, content)
		VALUES (?, ?, ?)
		RETURNING id, created_at`,
		groupID, fromID, content,
	).Scan(&msgID, &createdAt)
	if err != nil {
		tx.Rollback()
		return out, nil, err
	}

	members, err := getMembers(groupID)
	if err != nil {
		tx.Rollback()
		return out, nil, err
	}

	if err := tx.Commit(); err != nil {
		return out, nil, err
	}
	out = OutgoingMessage{
		ID:        msgID,
		SenderID:  fromID,
		GroupID:   groupID,
		Content:   content,
		CreatedAt: createdAt,
	}
	return out, members, nil
}
