package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/lib/pq"
)

type Query struct {
	ID                        int64
	Name                      string
	QueryString               string
	LastPolledAt              *time.Time
	PollIntervalSeconds       int
	IsActive                  bool
	MinAbstractLength         int
	PublicationTypeAllowlist  []string // nil means "allow all"
	PublicationTypeBlocklist  []string
	Notes                     *string
}

type Store struct {
	db *sql.DB
}

func New(db *sql.DB) *Store {
	return &Store{db: db}
}

// DueQueries returns active queries whose last_polled_at is older than
// their per-row poll_interval_seconds (or null). Stalest first.
//
// Single-scheduler deployment. If we ever scale horizontally, add
// FOR UPDATE SKIP LOCKED here and update last_polled_at inside the
// same transaction to prevent two schedulers picking the same query.
func (s *Store) DueQueries(ctx context.Context) ([]Query, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, query_string, last_polled_at,
		       poll_interval_seconds, is_active,
		       min_abstract_length,
		       publication_type_allowlist,
		       publication_type_blocklist,
		       notes
		FROM queries
		WHERE is_active
		  AND (last_polled_at IS NULL
		       OR now() - last_polled_at >= (poll_interval_seconds || ' seconds')::interval)
		ORDER BY COALESCE(last_polled_at, 'epoch'::timestamptz)
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Query
	for rows.Next() {
		var q Query
		if err := rows.Scan(
			&q.ID, &q.Name, &q.QueryString, &q.LastPolledAt,
			&q.PollIntervalSeconds, &q.IsActive,
			&q.MinAbstractLength,
			pq.Array(&q.PublicationTypeAllowlist),
			pq.Array(&q.PublicationTypeBlocklist),
			&q.Notes,
		); err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

func (s *Store) UpdateLastPolled(ctx context.Context, id int64, at time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE queries SET last_polled_at = $1 WHERE id = $2`, at, id)
	return err
}
