package chat

import (
	"database/sql"
	"encoding/json"
	"log"
	"sync"
	"time"

	"social-network/backend/pkg/groups"
)

const TypingFrameType = "typing"

const (
	TypingTargetDM    = "dm"
	TypingTargetGroup = "group"
)

const typingMinInterval = time.Second

type TypingEvent struct {
	Type       string `json:"type"`
	FromUserID int64  `json:"fromUserId"`
	TargetType string `json:"targetType"`
	TargetID   int64  `json:"targetId"`
}

type TypingRelay struct {
	db   *sql.DB
	push func(int64, interface{})
	now  func() time.Time

	mu   sync.Mutex
	last map[typingKey]time.Time
}

type typingKey struct {
	senderID   int64
	targetType string
	targetID   int64
}

func NewTypingRelay(db *sql.DB, push func(int64, interface{})) *TypingRelay {
	return &TypingRelay{
		db:   db,
		push: push,
		now:  time.Now,
		last: make(map[typingKey]time.Time),
	}
}

func (t *TypingRelay) HandleFrame(senderID int64, frame []byte) {
	var req struct {
		TargetType string `json:"targetType"`
		TargetID   int64  `json:"targetId"`
	}
	if err := json.Unmarshal(frame, &req); err != nil || req.TargetID < 1 {
		return
	}
	if req.TargetType != TypingTargetDM && req.TargetType != TypingTargetGroup {
		return
	}

	key := typingKey{senderID: senderID, targetType: req.TargetType, targetID: req.TargetID}
	if !t.allow(key) {
		return
	}

	recipients, err := t.recipients(key)
	if err != nil {
		log.Printf("typing: checking %s %d for user %d: %v", key.targetType, key.targetID, senderID, err)
		return
	}

	event := TypingEvent{
		Type:       TypingFrameType,
		FromUserID: senderID,
		TargetType: key.targetType,
		TargetID:   key.targetID,
	}
	for _, id := range recipients {
		t.push(id, event)
	}
}

// Runs before the database checks, so a client sending too fast costs no queries.
func (t *TypingRelay) allow(key typingKey) bool {
	now := t.now()

	t.mu.Lock()
	defer t.mu.Unlock()

	if last, ok := t.last[key]; ok && now.Sub(last) < typingMinInterval {
		return false
	}
	t.last[key] = now

	// Stale entries are swept only once the map is big enough to be worth it.
	if len(t.last) > 1024 {
		for k, at := range t.last {
			if now.Sub(at) >= typingMinInterval {
				delete(t.last, k)
			}
		}
	}
	return true
}

// Without this reset the pre-send timestamp swallows the first keystroke after a send, leaving the sender looking idle for up to a second.
func (t *TypingRelay) SentMessage(senderID int64, targetType string, targetID int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.last, typingKey{senderID: senderID, targetType: targetType, targetID: targetID})
}

func (t *TypingRelay) recipients(key typingKey) ([]int64, error) {
	switch key.targetType {
	case TypingTargetDM:
		if key.targetID == key.senderID {
			return nil, nil
		}
		allowed, err := CheckCanMessage(t.db, key.senderID, key.targetID)
		if err != nil || !allowed {
			return nil, err
		}
		return []int64{key.targetID}, nil

	case TypingTargetGroup:
		member, err := groups.IsGroupMember(t.db, key.targetID, key.senderID)
		if err != nil || !member {
			return nil, err
		}
		members, err := groups.GroupMemberIDs(t.db, key.targetID)
		if err != nil {
			return nil, err
		}
		others := make([]int64, 0, len(members))
		for _, id := range members {
			if id != key.senderID {
				others = append(others, id)
			}
		}
		return others, nil
	}
	return nil, nil
}
