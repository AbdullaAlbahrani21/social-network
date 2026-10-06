# --- build stage ---
#
# The tag matches go.mod's `go 1.26.6` exactly. go.mod carries no `toolchain`
# directive, so an older base image would either fail outright or silently
# download a second toolchain mid-build; pinning the patch version keeps the
# build hermetic and reproducible.
FROM golang:1.26.6-alpine AS build

# mattn/go-sqlite3 is a cgo package, and it is not only the driver: pkg/auth,
# pkg/groups and pkg/followers reference sqlite3.Error and
# sqlite3.ErrConstraintUnique directly to recognise a unique-constraint
# failure. Those symbols live behind the cgo build tag, so CGO_ENABLED=0 does
# not degrade to a pure-Go driver -- it fails to compile. build-base supplies
# the gcc and musl-dev that needs.
RUN apk add --no-cache build-base

WORKDIR /src

# Dependencies first, in their own layer: they change far less often than the
# source, so editing a handler does not re-download the module graph.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ENV CGO_ENABLED=1
RUN go build -trimpath -ldflags="-s -w" -o /out/server .

# --- runtime stage ---
#
# alpine, not scratch: a cgo binary is dynamically linked against musl, so it
# needs a libc present to start at all.
FROM alpine:3.22

# ca-certificates is not needed for anything today; wget (busybox) is, for the
# compose healthcheck.
RUN adduser -D -u 10001 app

# The migrations are compiled into the binary by go:embed (see
# pkg/db/migrations), so the binary really is the whole application -- there
# is no migrations directory, template or asset to copy alongside it.
COPY --from=build /out/server /usr/local/bin/server

# /app/data is the mount point. Created and owned by the unprivileged user so
# the server can write the database and uploads into a fresh named volume,
# which Docker initialises from the image and therefore with these
# permissions.
WORKDIR /app
RUN mkdir -p /app/data/uploads && chown -R app:app /app

USER app

# The same values the code defaults to, but relocated under the volume: the
# bare defaults are relative to the working directory, which would put the
# database and uploads inside the container's writable layer and lose them on
# every recreate.
ENV PORT=8080 \
    DB_PATH=/app/data/social_network.db \
    UPLOADS_DIR=/app/data/uploads

EXPOSE 8080

VOLUME ["/app/data"]

CMD ["server"]
