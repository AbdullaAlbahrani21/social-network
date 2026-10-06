CREATE TABLE IF NOT EXISTS post_visibility (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    post_id INTEGER NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    UNIQUE (post_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_post_visibility_post_id ON post_visibility(post_id);
