-- Reverts 000018: notifications.type goes back to the eight values 000017
-- allowed.
--
-- Reference only, like every down migration here: only *.up.sql files are
-- embedded, and nothing runs these. Run by hand, it needs PRAGMA foreign_keys
-- = OFF first, outside any transaction, for the reason 000016 gives.
--
-- Narrowing the constraint cannot keep rows the narrower list refuses, so any
-- group_invite_declined notification is dropped rather than rewritten to some
-- other type: no older type means "your invitation was declined", and
-- inventing one would show users a notification that misdescribes what
-- happened. The id sequence is carried over, so the ids those rows used are
-- not handed out again.
CREATE TABLE notifications_old (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    actor_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
    type TEXT NOT NULL CHECK (type IN (
        'follow_request',
        'follow_accepted',
        'group_invite',
        'group_join_request',
        'group_event_created',
        'post_liked',
        'comment_created',
        'message_received'
    )),
    target_type TEXT,
    target_id INTEGER,
    message TEXT,
    is_read INTEGER NOT NULL DEFAULT 0 CHECK (is_read IN (0,1)),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO notifications_old (
    id, user_id, actor_id, type, target_type, target_id, message, is_read, created_at
)
SELECT
    id, user_id, actor_id, type, target_type, target_id, message, is_read, created_at
FROM notifications
WHERE type IN (
    'follow_request',
    'follow_accepted',
    'group_invite',
    'group_join_request',
    'group_event_created',
    'post_liked',
    'comment_created',
    'message_received'
);

DELETE FROM sqlite_sequence WHERE name = 'notifications_old';

INSERT INTO sqlite_sequence (name, seq)
SELECT 'notifications_old', seq FROM sqlite_sequence WHERE name = 'notifications';

DROP TABLE notifications;

ALTER TABLE notifications_old RENAME TO notifications;

CREATE INDEX IF NOT EXISTS idx_notifications_user_id ON notifications(user_id);
CREATE INDEX IF NOT EXISTS idx_notifications_user_unread ON notifications(user_id, is_read);
