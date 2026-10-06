# Backend structure

This compiles and runs as-is. Every route is implemented -- there are no
`501` placeholders left -- and the routing, DB connection, and migrations all
work end to end.

## Run it

```bash
cd backend
go mod tidy       # fetches deps (google/uuid, gorilla/websocket, go-sqlite3, bcrypt)
go run .
```

`go.mod` declares Go 1.26.6; an older local toolchain downloads it
automatically unless `GOTOOLCHAIN=local` is set. go-sqlite3 uses cgo, so a C
compiler must be available.

It listens on `:8080` and creates `social_network.db` in the working directory
on first run. On every start, `sqlite.Connect` applies each
`pkg/db/migrations/sqlite/*.up.sql` not yet recorded in its
`schema_migrations` table. Uploaded avatars and images are saved under
`uploads/` in the working directory (`media.Dir`).

## Layout

```
backend/
├── server.go                    # wires every package's routes together — should barely change
├── pkg/
│   ├── db/
│   │   ├── migrations/
│   │   │   ├── migrations.go    # embeds the .sql files (see note below)
│   │   │   └── sqlite/*.sql     # *.up.sql are embedded and applied; *.down.sql are not used
│   │   └── sqlite/sqlite.go     # connects + runs migrations on startup
│   ├── response/response.go     # shared JSON helpers — import this, don't roll your own
│   ├── middleware/              # auth.go: session check (RequireAuth) + cookie name; cors.go
│   ├── paging/paging.go         # shared ?page/?limit parsing for paged lists
│   ├── media/media.go           # serves uploads/ to logged-in users; owns the uploads path
│   ├── auth/          } Member 1
│   ├── profile/       }
│   ├── followers/     }
│   ├── posts/          } Member 2
│   ├── comments/        }
│   ├── groups/           } Member 3
│   ├── events/             }
│   ├── chat/                 } Member 4
│   ├── websocket/             }
│   └── notifications/          }
```

Every feature package exposes one `RegisterRoutes(mux, db, ...)` function
that `server.go` calls. The extra arguments differ: followers, groups, events
and chat take the hub's `Push` for live delivery, auth takes
`hub.CloseSession` so logout closes that session's sockets, and media takes
the uploads directory.

## Things worth knowing

**Why `migrations.go` exists as its own file.** Go's `//go:embed` can only
reach files in the same directory or a subdirectory of the file containing
the directive. Since `migrations/` and `sqlite/` (the connection code) are
siblings under `pkg/db/`, the embed had to live in `pkg/db/migrations/migrations.go`
instead of inside `sqlite.go` directly. This doesn't change how you write or
name migration files, it's purely plumbing.

**The shared piece:** `notifications.Create(...)` in
`pkg/notifications/notifications.go`. followers, groups and events call it
(follow requests, group invites, join requests, event creation), passing the
hub's `Push` and, when they have names in scope, the exact message text. If
you change its signature, say so in the group chat first since three other
packages call it.

**`middleware.RequireAuth`** wraps a handler and makes the logged-in user's
ID available inside it via `middleware.UserID(r)`. A protected route looks
like this:

```go
mux.HandleFunc("PUT /api/profile/visibility",
    middleware.RequireAuth(db, updateVisibilityHandler(db)))
```

Every `/api` route is wrapped except register, login and logout. `/ws`
checks the session cookie itself through `middleware.ValidateSession`, and
`/uploads/` is wrapped inside `pkg/media`.
