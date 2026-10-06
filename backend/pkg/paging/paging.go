package paging

import (
	"net/http"
	"strconv"
)

const (
	defaultPageSize = 50
	maxPageSize     = 100
)

// Clamped: a negative OFFSET is silently 0 in SQLite, but LIMIT -1 means "no limit" and turned a paged endpoint into a full-table dump.
func Parse(r *http.Request) (limit, offset int) {
	page := 1
	limit = defaultPageSize

	if p := r.URL.Query().Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			page = v
		}
	}
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil {
			limit = v
		}
	}

	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = defaultPageSize
	}
	if limit > maxPageSize {
		limit = maxPageSize
	}

	return limit, (page - 1) * limit
}
