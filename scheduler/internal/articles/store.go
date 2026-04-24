// Package articles implements user-scoped read-only endpoints over the
// articles table. Articles are global (deduplicated across users); ownership
// is derived via query_matches -> queries.user_id.
package articles

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
)

// Article is the API shape returned by list and detail endpoints.
type Article struct {
	PMID             string          `json:"pmid"`
	Title            string          `json:"title"`
	Abstract         *string         `json:"abstract"`
	Journal          *string         `json:"journal"`
	PublicationDate  *string         `json:"publication_date"` // YYYY-MM-DD
	Authors          json.RawMessage `json:"authors"`
	PublicationTypes json.RawMessage `json:"publication_types"`
	FetchedAt        time.Time       `json:"fetched_at"`
	MatchedQueries   json.RawMessage `json:"matched_queries"`
}

type ListParams struct {
	UserID  int64
	Limit   int
	Offset  int
	QueryID *int64     // optional — filter to one of the user's queries
	Since   *time.Time // optional — fetched_at >= Since
	Search  string     // optional — plainto_tsquery on title + abstract
}

var ErrNotFound = errors.New("article not found")

// List returns articles matched by the user's queries, paginated, with a
// total count. matched_queries is scoped to the user's queries.
//
// If QueryID is set, it is validated to belong to the user via the same
// JOIN — passing another user's query id returns no rows (and total=0).
func List(ctx context.Context, db *sql.DB, p ListParams) ([]Article, int, error) {
	whereClauses := []string{"q.user_id = $1"}
	args := []any{p.UserID}
	nextArg := 2

	if p.QueryID != nil {
		whereClauses = append(whereClauses, "qm.query_id = $"+strconv.Itoa(nextArg))
		args = append(args, *p.QueryID)
		nextArg++
	}
	if p.Since != nil {
		whereClauses = append(whereClauses, "a.fetched_at >= $"+strconv.Itoa(nextArg))
		args = append(args, *p.Since)
		nextArg++
	}
	if p.Search != "" {
		whereClauses = append(whereClauses,
			"to_tsvector('english', a.title || ' ' || COALESCE(a.abstract, '')) @@ plainto_tsquery('english', $"+
				strconv.Itoa(nextArg)+")")
		args = append(args, p.Search)
		nextArg++
	}

	where := strings.Join(whereClauses, " AND ")

	// Count (distinct pmid because an article may match multiple of the
	// user's queries via the join).
	var total int
	countSQL := `
		SELECT COUNT(DISTINCT a.pmid)
		FROM articles a
		JOIN query_matches qm ON qm.pmid = a.pmid
		JOIN queries q ON q.id = qm.query_id
		WHERE ` + where
	if err := db.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Pagination args.
	args = append(args, p.Limit, p.Offset)
	limitPlaceholder := "$" + strconv.Itoa(nextArg)
	offsetPlaceholder := "$" + strconv.Itoa(nextArg+1)

	listSQL := `
		SELECT a.pmid, a.title, a.abstract, a.journal, a.publication_date,
		       a.authors, to_jsonb(a.publication_types) AS publication_types,
		       a.fetched_at,
		       COALESCE((
		         SELECT json_agg(json_build_object('id', q2.id, 'name', q2.name) ORDER BY q2.id)
		         FROM query_matches qm2
		         JOIN queries q2 ON q2.id = qm2.query_id
		         WHERE qm2.pmid = a.pmid AND q2.user_id = $1
		       ), '[]'::json) AS matched_queries
		FROM articles a
		JOIN query_matches qm ON qm.pmid = a.pmid
		JOIN queries q ON q.id = qm.query_id
		WHERE ` + where + `
		GROUP BY a.pmid
		ORDER BY MAX(a.fetched_at) DESC
		LIMIT ` + limitPlaceholder + ` OFFSET ` + offsetPlaceholder

	rows, err := db.QueryContext(ctx, listSQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	articles := make([]Article, 0, p.Limit)
	for rows.Next() {
		a, err := scanArticle(rows)
		if err != nil {
			return nil, 0, err
		}
		articles = append(articles, a)
	}
	return articles, total, rows.Err()
}

// GetByPMID returns one article iff the user has a query_match linking them
// to it. Otherwise ErrNotFound (no existence leak).
func GetByPMID(ctx context.Context, db *sql.DB, userID int64, pmid string) (*Article, error) {
	row := db.QueryRowContext(ctx, `
		SELECT a.pmid, a.title, a.abstract, a.journal, a.publication_date,
		       a.authors, to_jsonb(a.publication_types) AS publication_types,
		       a.fetched_at,
		       COALESCE((
		         SELECT json_agg(json_build_object('id', q.id, 'name', q.name) ORDER BY q.id)
		         FROM query_matches qm
		         JOIN queries q ON q.id = qm.query_id
		         WHERE qm.pmid = a.pmid AND q.user_id = $2
		       ), '[]'::json) AS matched_queries
		FROM articles a
		WHERE a.pmid = $1
		  AND EXISTS (
		    SELECT 1 FROM query_matches qm
		    JOIN queries q ON q.id = qm.query_id
		    WHERE qm.pmid = a.pmid AND q.user_id = $2
		  )
	`, pmid, userID)

	a, err := scanArticle(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// scanArticle handles both *sql.Rows and *sql.Row.
func scanArticle(s scanner) (Article, error) {
	var (
		a       Article
		pubDate sql.NullTime
	)
	if err := s.Scan(
		&a.PMID, &a.Title, &a.Abstract, &a.Journal, &pubDate,
		&a.Authors, &a.PublicationTypes, &a.FetchedAt, &a.MatchedQueries,
	); err != nil {
		return a, err
	}
	if pubDate.Valid {
		s := pubDate.Time.Format("2006-01-02")
		a.PublicationDate = &s
	}
	return a, nil
}

type scanner interface {
	Scan(dest ...any) error
}
