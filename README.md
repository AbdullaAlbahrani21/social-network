# Social Network

A Facebook-like social network with profiles, followers, posts, groups, events, real-time private and group chat, and live notifications.

- **Backend:** Go (standard library `net/http`), SQLite with embedded migrations, Gorilla WebSocket
- **Frontend:** Vue 3, Vue Router, Pinia, built with Vite
- **Deployment:** Docker Compose, with the Go API in one container and nginx serving the SPA in the other

## Features

- **Authentication:** register, log in and log out with session cookies. Passwords are hashed with bcrypt.
- **Profiles:** public or private profiles, with an avatar, a nickname and an "about me" section.
- **Followers:** follow users. Following a private profile sends a follow request that the owner accepts or declines.
- **Posts:** text and image posts with three privacy levels: `public`, `almost_private` (followers only) and `private` (chosen followers only). Posts support comments and likes.
- **Groups:** create groups, invite members, request to join, and share posts inside a group.
- **Events:** group events that members answer with an RSVP.
- **Chat:** real-time private messages and group chat over WebSockets, with typing indicators and emoji.
- **Notifications:** live notifications for follow requests, group invitations, join requests and new events.

## Quick start (Docker)

You need Docker with the Compose plugin.

```bash
docker compose up --build
```

| Service  | URL                                              |
| -------- | ------------------------------------------------ |
| App      | http://localhost:3000                            |
| API      | http://localhost:3001 (for direct API access)    |

The database and uploaded images are kept in the `backend-data` volume, so they survive `docker compose down`. To start from an empty database, run `docker compose down -v`.

## Running locally (development)

### Requirements

- **Go 1.26.6 or newer.** An older Go downloads the right toolchain automatically.
- **A C compiler** (`gcc` or `clang`), because `go-sqlite3` uses cgo. On macOS, run `xcode-select --install`.
- **Node.js 20.19+ or 22.12+**, which Vite 8 requires.

### 1. Backend

```bash
cd backend
go run .
```

The API listens on http://localhost:8080. On first run it creates `social_network.db` and applies every migration in `pkg/db/migrations/sqlite/`. Uploaded images are saved in `uploads/`.

### 2. Frontend

In a second terminal:

```bash
cd frontend
npm install
npm run dev
```

Open http://localhost:5173. The Vite dev server proxies `/api` and `/ws` to the backend on port 8080.

> If you see `sh: vite: command not found`, the dependencies aren't installed yet. Run `npm install` first.

## Configuration

The backend reads these environment variables:

| Variable         | Default                   | Purpose                                                                 |
| ---------------- | ------------------------- | ----------------------------------------------------------------------- |
| `PORT`           | `8080`                    | Port the API listens on                                                 |
| `DB_PATH`        | `./social_network.db`     | Path of the SQLite database file                                        |
| `UPLOADS_DIR`    | `uploads`                 | Directory for uploaded avatars and images                               |
| `ALLOWED_ORIGIN` | `http://localhost:5173`   | Origin allowed by CORS and the WebSocket handshake                      |

The frontend reads `VITE_API_BASE_URL` when it is built. It defaults to `http://localhost:8080`. Setting it to an empty string makes the app send requests to its own origin, which the Docker build uses so that nginx can proxy them.

## Project structure

```
social-network/
├── docker-compose.yml
├── backend/
│   ├── server.go              # wires every package's routes together
│   ├── Dockerfile
│   └── pkg/
│       ├── db/                # SQLite connection and migrations
│       ├── middleware/        # session auth, CORS
│       ├── auth/  profile/  followers/
│       ├── posts/  comments/
│       ├── groups/  events/
│       ├── chat/  websocket/  notifications/
│       ├── media/             # serves uploaded files to logged-in users
│       └── response/  paging/  config/
└── frontend/
    ├── Dockerfile
    ├── nginx.conf
    └── src/
        ├── views/             # pages (feed, profile, groups, chat, ...)
        ├── components/
        ├── stores/            # Pinia stores (auth, chat, notifications, websocket)
        ├── router/
        └── api.js
```

For more detail on the backend, see [backend/STRUCTURE.md](backend/STRUCTURE.md) and [backend/pkg/db/SCHEMA.md](backend/pkg/db/SCHEMA.md).

## Authors

- Abdulrahman Albraiki
- Abdulla Albahrani
- Isa Mubarak
- Mohamed Alaali
