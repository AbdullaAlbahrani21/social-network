package chat

import (
	"context"
	"database/sql"
	"log"

	"social-network/backend/pkg/notifications"
)

const notifyTargetType = "message"

func notifyMessageReceived(db *sql.DB, recipientID, senderID, messageID int64, hubPush func(int64, interface{})) {
	if recipientID == senderID {
		return
	}

	message := "Someone sent you a message."
	if name, err := notifications.ActorName(context.Background(), db, senderID); err == nil {
		message = name + " sent you a message."
	} else {
		log.Printf("chat: failed to load name of user %d for message_received notification: %v", senderID, err)
	}

	if err := notifications.CreateOrRefresh(
		db, recipientID, notifications.TypeMessageReceived, senderID,
		notifyTargetType, messageID, hubPush, message,
	); err != nil {
		log.Printf("chat: failed to create message_received notification for user %d: %v", recipientID, err)
	}
}
