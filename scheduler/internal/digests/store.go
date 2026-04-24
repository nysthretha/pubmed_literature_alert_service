// Package digests implements user-scoped read-only endpoints over the digests
// and digest_articles tables.
package digests

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

// DigestSummary is the row shape for list responses.
type DigestSummary struct {
	ID               int64      `json:"id"`
	SentAt           time.Time  `json:"sent_at"`
	SentLocalDate    *string    `json:"sent_local_date"`
	Status           string     `json:"status"`
	ArticlesIncluded int        `json:"articles_included"`
	Manual           bool       `json:"manual"`
}

// DigestArticle is a lightweight article record embedded in digest detail.
type DigestArticle struct {
	PMID            string          `json:"pmid"`
	Title           string          `json:"title"`
	Journal         *string         `json:"journal"`
	PublicationDate *string         `json:"publication_date"`
	MatchedQueries  json.RawMessage `json:"matched_queries"`
}

// DigestDetail is the response for /api/digests/:id.
type DigestDetail struct {
	DigestSummary
	ErrorMessage *string         `json:"error_message"`
	Articles     []DigestArticle `json:"articles"`
}

var ErrNotFound = errors.New("digest not found")

// List returns the user's digests sorted by sent_at DESC, with total count.
func List(ctx context.Context, db *sql.DB, userID int64, limit, offset int) ([]DigestSummary, int, error) {
	var total int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM digests WHERE user_id = $1`, userID,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := db.QueryContext(ctx, `
		SELECT id, sent_at, sent_local_date, status, articles_included, manual
		FROM digests
		WHERE user_id = $1
		ORDER BY sent_at DESC
		LIMIT $2 OFFSET $3
	`, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]DigestSummary, 0, limit)
	for rows.Next() {
		var (
			d       DigestSummary
			localDt sql.NullTime
		)
		if err := rows.Scan(
			&d.ID, &d.SentAt, &localDt, &d.Status, &d.ArticlesIncluded, &d.Manual,
		); err != nil {
			return nil, 0, err
		}
		if localDt.Valid {
			s := localDt.Time.Format("2006-01-02")
			d.SentLocalDate = &s
		}
		out = append(out, d)
	}
	return out, total, rows.Err()
}

// GetDetail returns a digest owned by userID with its articles array.
// matched_queries on each article is scoped to userID's queries only.
func GetDetail(ctx context.Context, db *sql.DB, userID, id int64) (*DigestDetail, error) {
	var (
		d       DigestDetail
		localDt sql.NullTime
	)
	err := db.QueryRowContext(ctx, `
		SELECT id, sent_at, sent_local_date, status, articles_included, manual,
		       error_message
		FROM digests
		WHERE id = $1 AND user_id = $2
	`, id, userID).Scan(
		&d.ID, &d.SentAt, &localDt, &d.Status, &d.ArticlesIncluded, &d.Manual,
		&d.ErrorMessage,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if localDt.Valid {
		s := localDt.Time.Format("2006-01-02")
		d.SentLocalDate = &s
	}

	rows, err := db.QueryContext(ctx, `
		SELECT a.pmid, a.title, a.journal, a.publication_date,
		       COALESCE((
		         SELECT json_agg(json_build_object('id', q.id, 'name', q.name) ORDER BY q.id)
		         FROM query_matches qm
		         JOIN queries q ON q.id = qm.query_id
		         WHERE qm.pmid = a.pmid AND q.user_id = $2
		       ), '[]'::json) AS matched_queries
		FROM digest_articles da
		JOIN articles a ON a.pmid = da.pmid
		WHERE da.digest_id = $1
		ORDER BY a.publication_date DESC NULLS LAST, a.pmid
	`, id, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	d.Articles = make([]DigestArticle, 0)
	for rows.Next() {
		var (
			a       DigestArticle
			pubDate sql.NullTime
		)
		if err := rows.Scan(&a.PMID, &a.Title, &a.Journal, &pubDate, &a.MatchedQueries); err != nil {
			return nil, err
		}
		if pubDate.Valid {
			s := pubDate.Time.Format("2006-01-02")
			a.PublicationDate = &s
		}
		d.Articles = append(d.Articles, a)
	}
	return &d, rows.Err()
}
