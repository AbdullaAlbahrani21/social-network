-- Reverts 000016: nickname is nullable again and no longer unique.
--
-- Reference only, like every down migration here: only *.up.sql files are
-- embedded, and nothing runs these. Run by hand, it needs PRAGMA foreign_keys
-- = OFF first, outside any transaction, for the reason 000016 gives, or
-- dropping users cascades into every table that references it.
--
-- Placeholder nicknames ('user_' || id) stay as they are rather than going
-- back to NULL.
CREATE TABLE users_old (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    first_name TEXT NOT NULL,
    last_name TEXT NOT NULL,
    date_of_birth DATE NOT NULL,
    avatar_path TEXT,
    nickname TEXT,
    about_me TEXT,
    is_public INTEGER NOT NULL DEFAULT 1 CHECK (is_public IN (0,1)),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO users_old (
    id, email, password_hash, first_name, last_name, date_of_birth,
    avatar_path, nickname, about_me, is_public, created_at
)
SELECT
    id, email, password_hash, first_name, last_name, date_of_birth,
    avatar_path, nickname, about_me, is_public, created_at
FROM users;

DELETE FROM sqlite_sequence WHERE name = 'users_old';

INSERT INTO sqlite_sequence (name, seq)
SELECT 'users_old', seq FROM sqlite_sequence WHERE name = 'users';

DROP TABLE users;

ALTER TABLE users_old RENAME TO users;

CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
