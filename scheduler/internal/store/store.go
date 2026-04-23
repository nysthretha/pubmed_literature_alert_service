package store

import (
	"context"
	"database/sql"
	"time"
)

type Query struct {
	ID           int64
	Name         string
	QueryString  string
	LastPolledAt *time.Time
}

type Store struct {
	db *sql.DB
}

func New(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Queries(ctx context.Context) ([]Query, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, query_string, last_polled_at
		FROM queries
		ORDER BY id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Query
	for rows.Next() {
		var q Query
		if err := rows.Scan(&q.ID, &q.Name, &q.QueryString, &q.LastPolledAt); err != nil {
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
