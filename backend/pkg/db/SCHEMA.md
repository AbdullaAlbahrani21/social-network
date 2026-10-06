# Social Network — Database Schema

15 tables, defined by the migrations in `backend/pkg/db/migrations/sqlite/`. They are applied by `sqlite.Connect` (`pkg/db/sqlite/sqlite.go`) on every startup, not by an external tool: it embeds the `*.up.sql` files, runs each one not yet recorded in its own `schema_migrations` table in filename order, one transaction per file, and records it. It splits a file on `;`, so a migration must not contain a `;` inside a string literal or trigger body. The `*.down.sql` files are not embedded, and nothing runs them.

## Tables

| # | Table | Purpose |
|---|-------|---------|
| 1 | `users` | Account + profile info. `is_public` drives the follow-request bypass logic. |
| 2 | `sessions` | Session token → user_id, with `expires_at` for cookie-based auth. |
| 3 | `followers` | Follow relationships/requests. `status` is `pending` or `accepted`. |
| 4 | `groups` | Group title/description + creator. |
| 5 | `group_members` | Confirmed membership only (accepted invites/requests end up here). |
| 6 | `group_invitations` | Invite-to-join (`type = 'invite'`) and request-to-join (`'request'`) records, `pending` / `accepted` / `declined`. Accepting one also writes the `group_members` row. |
| 7 | `posts` | Feed + group posts (`group_id` NULL = normal feed post). `privacy` is public / almost_private / private; group posts store `'public'` only to satisfy the CHECK, since membership decides who sees them. |
| 8 | `post_visibility` | Only used when `posts.privacy = 'private'` — explicit list of user_ids allowed to see it. The API does not currently check that they follow the author. |
| 9 | `comments` | Comments on posts, can include an image. |
| 10 | `events` | Group events with title/description/time. |
| 11 | `event_responses` | One row per user per event: `going` / `not_going`. |
| 12 | `messages` | Private 1-to-1 chat messages. |
| 13 | `group_messages` | Group chat room messages. |
| 14 | `notifications` | Generic notification feed — `type` + optional `target_type`/`target_id` point back at whatever triggered it; `message` is the display text. |
| 15 | `post_likes` | One row per user per liked post (`UNIQUE (post_id, user_id)`). The feed and single-post responses count them per post. |

The runner also creates a 16th table, `schema_migrations`, for its own bookkeeping.

## Key design decisions

**Followers as one table, not two.** Rather than separate "follow requests" and "follows" tables, `followers` uses a `status` column. A public profile still gets a row — just auto-inserted as `accepted` instead of `pending` — so your follower/following counts always come from the same query.

**Group membership vs. invitations are separate tables.** `group_invitations` holds the messy, temporary state (pending/accepted/declined, invite vs. self-request). Once accepted, a row gets written to `group_members` and is what your "is this user in the group" checks should query — keeps that check a simple join instead of a status filter.

**Post privacy has three tiers, but only one needs a join table.** `public` and `almost_private` (followers-only) can be resolved from `posts.privacy` + the `followers` table alone. Only `private` (specific followers) needs the extra `post_visibility` table — so most reads stay cheap.

**Notifications are generic on purpose.** Rather than a table per notification type, `type` + `target_type`/`target_id` let one table cover follow requests, group invites, join requests, and event creation (plus whatever else you add) without new migrations each time.

**IDs are `INTEGER AUTOINCREMENT`,** not UUIDs. Simpler for a project this size — swap to `google/uuid`/`gofrs/uuid` text IDs later if you want unguessable public URLs (e.g. `/profile/:id`); the allowed-packages list includes both.

## Not yet covered (add as needed)
- Full-text search (group browse and the invite picker use plain `LIKE` over titles, descriptions and names)
- Refresh/cleanup job for expired `sessions` rows
