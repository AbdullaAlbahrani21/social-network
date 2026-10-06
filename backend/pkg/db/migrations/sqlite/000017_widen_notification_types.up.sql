-- Widens the notifications.type CHECK constraint with the three types that
-- likes, comments and direct messages need: post_liked, comment_created and
-- message_received. The five values 000014 allowed are all kept.
--
-- SQLite cannot alter a CHECK constraint in place, so the table is rebuilt the
-- way 000016 rebuilds users: create notifications_new, copy every row, drop
-- notifications, and give notifications_new the name.
--
-- sqlite.Connect runs every migration with foreign key enforcement switched
-- off and runs PRAGMA foreign_key_check before committing, which is what makes
-- the drop safe here. notifications references users(id) twice -- user_id ON
-- DELETE CASCADE and actor_id ON DELETE SET NULL -- but nothing references
-- notifications, so dropping it cascades into nothing either way. Rows are
-- copied with their ids, and the id sequence is carried over, so an id freed
-- by a deleted notification is still never handed out again.
--
-- Every existing row satisfies the new constraint, since the new list is the
-- old one plus three values. No row is rewritten.
CREATE TABLE notifications_new (
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

INSERT INTO notifications_new (
    id, user_id, actor_id, type, target_type, target_id, message, is_read, created_at
)
SELECT
    id, user_id, actor_id, type, target_type, target_id, message, is_read, created_at
FROM notifications;

DELETE FROM sqlite_sequence WHERE name = 'notifications_new';

INSERT INTO sqlite_sequence (name, seq)
SELECT 'notifications_new', seq FROM sqlite_sequence WHERE name = 'notifications';

DROP TABLE notifications;

ALTER TABLE notifications_new RENAME TO notifications;

CREATE INDEX IF NOT EXISTS idx_notifications_user_id ON notifications(user_id);
CREATE INDEX IF NOT EXISTS idx_notifications_user_unread ON notifications(user_id, is_read);
