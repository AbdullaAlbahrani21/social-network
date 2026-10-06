package media

import (
	"database/sql"
	"io/fs"
	"net/http"

	"social-network/backend/pkg/config"
	"social-network/backend/pkg/middleware"
)

var Dir = config.Env("UPLOADS_DIR", "uploads")

const URLPrefix = "/uploads/"

// http.FileServer renders an HTML index for any directory without an index.html, which handed out every uploaded filename, so a directory is a 404.
func RegisterRoutes(mux *http.ServeMux, db *sql.DB, dir string) {
	mux.HandleFunc(
		"GET "+URLPrefix,
		middleware.RequireAuth(db, Handler(dir).ServeHTTP),
	)
}

func Handler(dir string) http.Handler {
	files := http.StripPrefix(
		URLPrefix,
		http.FileServer(noDirectories{http.Dir(dir)}),
	)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Everything here is served only to a logged-in user, so a shared cache must never store a response and replay it to someone else.
		w.Header().Set("Cache-Control", "private")
		files.ServeHTTP(w, r)
	})
}

type noDirectories struct {
	fs http.FileSystem
}

func (n noDirectories) Open(name string) (http.File, error) {
	file, err := n.fs.Open(name)
	if err != nil {
		return nil, err
	}

	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}

	if info.IsDir() {
		file.Close()
		return nil, fs.ErrNotExist
	}

	return file, nil
}
