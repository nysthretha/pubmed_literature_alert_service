package httpapi

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

const (
	defaultLimit = 50
	maxLimit     = 500
)

type article struct {
	PMID             string          `json:"pmid"`
	Title            string          `json:"title"`
	Abstract         *string         `json:"abstract"`
	Journal          *string         `json:"journal"`
	PublicationDate  *string         `json:"publication_date"` // YYYY-MM-DD or null
	Authors          json.RawMessage `json:"authors"`
	PublicationTypes json.RawMessage `json:"publication_types"`
	FetchedAt        time.Time       `json:"fetched_at"`
	MatchedQueries   json.RawMessage `json:"matched_queries"`
}

func recentArticlesHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := defaultLimit
		if l := r.URL.Query().Get("limit"); l != "" {
			n, err := strconv.Atoi(l)
			if err != nil || n <= 0 {
				http.Error(w, "limit must be a positive integer", http.StatusBadRequest)
				return
			}
			if n > maxLimit {
				n = maxLimit
			}
			limit = n
		}

		rows, err := db.QueryContext(r.Context(), `
			SELECT
				a.pmid,
				a.title,
				a.abstract,
				a.journal,
				a.publication_date,
				a.authors,
				to_jsonb(a.publication_types) AS publication_types,
				a.fetched_at,
				COALESCE(
					json_agg(json_build_object('id', q.id, 'name', q.name) ORDER BY q.id)
						FILTER (WHERE q.id IS NOT NULL),
					'[]'::json
				) AS matched_queries
			FROM articles a
			LEFT JOIN query_matches qm ON qm.pmid = a.pmid
			LEFT JOIN queries q ON q.id = qm.query_id
			GROUP BY a.pmid
			ORDER BY a.fetched_at DESC
			LIMIT $1
		`, limit)
		if err != nil {
			slog.Error("recent articles query failed", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		articles := make([]article, 0, limit)
		for rows.Next() {
			var (
				a       article
				pubDate sql.NullTime
			)
			if err := rows.Scan(
				&a.PMID, &a.Title, &a.Abstract, &a.Journal, &pubDate,
				&a.Authors, &a.PublicationTypes, &a.FetchedAt, &a.MatchedQueries,
			); err != nil {
				slog.Error("scan failed", "err", err)
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			if pubDate.Valid {
				s := pubDate.Time.Format("2006-01-02")
				a.PublicationDate = &s
			}
			articles = append(articles, a)
		}
		if err := rows.Err(); err != nil {
			slog.Error("rows iteration failed", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{"articles": articles}); err != nil {
			slog.Error("encode failed", "err", err)
		}
	}
}
