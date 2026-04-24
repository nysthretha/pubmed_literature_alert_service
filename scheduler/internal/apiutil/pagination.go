package apiutil

import (
	"errors"
	"net/http"
	"strconv"
)

// ParseLimitOffset reads `limit` and `offset` from query params with defaults
// and bounds. Returns an error describing the failing parameter if invalid.
func ParseLimitOffset(r *http.Request, defaultLimit, maxLimit int) (limit, offset int, err error) {
	limit = defaultLimit
	offset = 0

	if l := r.URL.Query().Get("limit"); l != "" {
		n, e := strconv.Atoi(l)
		if e != nil || n < 1 {
			return 0, 0, errors.New("limit must be a positive integer")
		}
		if n > maxLimit {
			n = maxLimit
		}
		limit = n
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		n, e := strconv.Atoi(o)
		if e != nil || n < 0 {
			return 0, 0, errors.New("offset must be >= 0")
		}
		offset = n
	}
	return limit, offset, nil
}
