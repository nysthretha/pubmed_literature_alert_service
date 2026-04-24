package articles

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"time"

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

		p := ListParams{UserID: u.ID, Limit: limit, Offset: offset}

		if qs := r.URL.Query().Get("query_id"); qs != "" {
			n, err := strconv.ParseInt(qs, 10, 64)
			if err != nil || n <= 0 {
				apiutil.WriteBadRequest(w, "query_id must be a positive integer")
				return
			}
			p.QueryID = &n
		}
		if s := r.URL.Query().Get("since"); s != "" {
			t, err := time.Parse(time.RFC3339, s)
			if err != nil {
				apiutil.WriteBadRequest(w, "since must be an RFC3339 timestamp")
				return
			}
			p.Since = &t
		}
		if s := r.URL.Query().Get("search"); s != "" {
			if len(s) > 200 {
				apiutil.WriteBadRequest(w, "search must be <= 200 characters")
				return
			}
			p.Search = s
		}

		list, total, err := List(r.Context(), db, p)
		if err != nil {
			apiutil.WriteInternal(w, err, "articles.list")
			return
		}

		hasMore := offset+len(list) < total
		apiutil.WriteJSON(w, http.StatusOK, map[string]any{
			"articles": list,
			"total":    total,
			"has_more": hasMore,
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
		pmid := r.PathValue("pmid")
		if pmid == "" || len(pmid) > 20 {
			apiutil.WriteBadRequest(w, "invalid pmid in path")
			return
		}
		a, err := GetByPMID(r.Context(), db, u.ID, pmid)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				apiutil.WriteNotFound(w, "article")
				return
			}
			apiutil.WriteInternal(w, err, "articles.get")
			return
		}
		apiutil.WriteJSON(w, http.StatusOK, a)
	}
}
