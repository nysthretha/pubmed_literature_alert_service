package digests

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/apiutil"
	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/auth"
)

const (
	defaultLimit = 50
	maxLimit     = 200
)

func listHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, ok := auth.UserFromContext(r)
		if !ok {
			apiutil.WriteUnauthorized(w)
			return
		}
		limit, offset, err := apiutil.ParseLimitOffset(r, defaultLimit, maxLimit)
		if err != nil {
			apiutil.WriteBadRequest(w, err.Error())
			return
		}
		list, total, err := List(r.Context(), db, u.ID, limit, offset)
		if err != nil {
			apiutil.WriteInternal(w, err, "digests.list")
			return
		}
		apiutil.WriteJSON(w, http.StatusOK, map[string]any{
			"digests":  list,
			"total":    total,
			"has_more": offset+len(list) < total,
		})
	}
}

func getHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, ok := auth.UserFromContext(r)
		if !ok {
			apiutil.WriteUnauthorized(w)
			return
		}
		raw := r.PathValue("id")
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			apiutil.WriteBadRequest(w, "invalid id in path")
			return
		}
		d, err := GetDetail(r.Context(), db, u.ID, id)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				apiutil.WriteNotFound(w, "digest")
				return
			}
			apiutil.WriteInternal(w, err, "digests.get")
			return
		}
		apiutil.WriteJSON(w, http.StatusOK, d)
	}
}
