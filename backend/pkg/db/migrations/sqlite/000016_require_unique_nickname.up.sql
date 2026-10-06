-- Nickname becomes required and unique, because login now accepts it in place
-- of the email. SQLite cannot add NOT NULL or UNIQUE to an existing column, so
-- the table is rebuilt: create users_new, copy every row into it, drop users,
-- and give users_new the name.
--
-- sqlite.Connect runs every migration with foreign key enforcement switched
-- off, and runs PRAGMA foreign_key_check before committing. That matters here.
-- With enforcement on, DROP TABLE users would first delete every row, and ON
-- DELETE CASCADE would take every session, follow, post, comment, message and
-- membership with it. With it off, the fourteen tables that reference users(id)
-- keep all their rows, and their references resolve to the rebuilt table once
-- it is called users again. Rows are copied with their ids, so each reference
-- still points at the same person.
--
-- A nickname is copied as it is, unless the row gets the placeholder
-- 'user_' || id. That happens when the nickname is
--   * missing or blank,
--   * a repeat of a nickname a lower id already has (the lowest id keeps it), or
--   * exactly user_ and digits naming some other id, which a placeholder could
--     otherwise collide with.
-- Placeholders are distinct because ids are, and no nickname that is kept can
-- equal one, since a kept nickname of that shape is its own row's placeholder.
-- Nicknames are not otherwise rewritten, including any the registration
-- format rule would now refuse.
--
-- The id sequence is carried over as well, so an id freed by a deleted user
-- is still never handed out again.
CREATE TABLE users_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    first_name TEXT NOT NULL,
    last_name TEXT NOT NULL,
    date_of_birth DATE NOT NULL,
    avatar_path TEXT,
    nickname TEXT NOT NULL UNIQUE,
    about_me TEXT,
    is_public INTEGER NOT NULL DEFAULT 1 CHECK (is_public IN (0,1)),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO users_new (
    id, email, password_hash, first_name, last_name, date_of_birth,
    avatar_path, nickname, about_me, is_public, created_at
)
SELECT
    u.id, u.email, u.password_hash, u.first_name, u.last_name, u.date_of_birth,
    u.avatar_path,
    CASE
        WHEN u.nickname IS NULL
          OR trim(u.nickname) = ''
          OR EXISTS (SELECT 1 FROM users d WHERE d.nickname = u.nickname AND d.id < u.id)
          OR (u.nickname GLOB 'user_[0-9]*'
              AND u.nickname NOT GLOB 'user_*[^0-9]*'
              AND u.nickname <> 'user_' || u.id)
        THEN 'user_' || u.id
        ELSE u.nickname
    END,
    u.about_me, u.is_public, u.created_at
FROM users u;

DELETE FROM sqlite_sequence WHERE name = 'users_new';

INSERT INTO sqlite_sequence (name, seq)
SELECT 'users_new', seq FROM sqlite_sequence WHERE name = 'users';

DROP TABLE users;

ALTER TABLE users_new RENAME TO users;

CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
