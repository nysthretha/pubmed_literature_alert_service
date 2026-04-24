// Package queries implements user-scoped CRUD on the queries table.
//
// Every exported function takes userID as its first post-ctx parameter and
// includes `AND user_id = $N` in its SQL. See docs/ARCHITECTURE.md for the
// ownership rule.
package queries

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/lib/pq"
)

// Query is the full shape returned by the API for a query row.
type Query struct {
	ID                       int64      `json:"id"`
	Name                     string     `json:"name"`
	QueryString              string     `json:"query_string"`
	LastPolledAt             *time.Time `json:"last_polled_at"`
	PollIntervalSeconds      int        `json:"poll_interval_seconds"`
	IsActive                 bool       `json:"is_active"`
	MinAbstractLength        int        `json:"min_abstract_length"`
	PublicationTypeAllowlist []string   `json:"publication_type_allowlist"`
	PublicationTypeBlocklist []string   `json:"publication_type_blocklist"`
	Notes                    *string    `json:"notes"`
	CreatedAt                time.Time  `json:"created_at"`
	ArticleCount             int        `json:"article_count"`
}

// CreateInput carries the fully-defaulted values to INSERT. The handler
// applies defaults before calling Create so this struct doesn't need pointers.
type CreateInput struct {
	Name                     string
	QueryString              string
	PollIntervalSeconds      int
	IsActive                 bool
	MinAbstractLength        int
	PublicationTypeAllowlist []string // nil => allow all
	PublicationTypeBlocklist []string
	Notes                    *string
}

// UpdateInput carries optional updates for PATCH. Nil fields are not changed.
// Distinguishing "not provided" from "explicitly null" is not supported here;
// to clear a nullable field, delete + recreate the query.
type UpdateInput struct {
	Name                     *string
	QueryString              *string
	PollIntervalSeconds      *int
	IsActive                 *bool
	MinAbstractLength        *int
	PublicationTypeAllowlist *[]string
	PublicationTypeBlocklist *[]string
	Notes                    *string
}

var (
	ErrNotFound   = errors.New("query not found")
	ErrNameExists = errors.New("query with that name already exists for this user")
)

// List returns the user's queries ordered by created_at DESC, with article_count
// from LEFT JOIN query_matches.
func List(ctx context.Context, db *sql.DB, userID int64) ([]Query, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT q.id, q.name, q.query_string, q.last_polled_at,
		       q.poll_interval_seconds, q.is_active,
		       q.min_abstract_length,
		       q.publication_type_allowlist, q.publication_type_blocklist,
		       q.notes, q.created_at,
		       COUNT(qm.pmid) AS article_count
		FROM queries q
		LEFT JOIN query_matches qm ON qm.query_id = q.id
		WHERE q.user_id = $1
		GROUP BY q.id
		ORDER BY q.created_at DESC
	`, userID)
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
			&q.Notes, &q.CreatedAt, &q.ArticleCount,
		); err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

// GetByID fetches one query owned by userID. Returns ErrNotFound if the row
// doesn't exist or isn't owned by userID — attackers cannot distinguish.
func GetByID(ctx context.Context, db *sql.DB, userID, id int64) (*Query, error) {
	var q Query
	err := db.QueryRowContext(ctx, `
		SELECT q.id, q.name, q.query_string, q.last_polled_at,
		       q.poll_interval_seconds, q.is_active,
		       q.min_abstract_length,
		       q.publication_type_allowlist, q.publication_type_blocklist,
		       q.notes, q.created_at,
		       (SELECT COUNT(*) FROM query_matches WHERE query_id = q.id) AS article_count
		FROM queries q
		WHERE q.id = $1 AND q.user_id = $2
	`, id, userID).Scan(
		&q.ID, &q.Name, &q.QueryString, &q.LastPolledAt,
		&q.PollIntervalSeconds, &q.IsActive,
		&q.MinAbstractLength,
		pq.Array(&q.PublicationTypeAllowlist),
		pq.Array(&q.PublicationTypeBlocklist),
		&q.Notes, &q.CreatedAt, &q.ArticleCount,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &q, nil
}

// Create inserts a new query owned by userID.
// Returns ErrNameExists on unique-name collision for this user.
func Create(ctx context.Context, db *sql.DB, userID int64, in CreateInput) (*Query, error) {
	var id int64
	var createdAt time.Time
	err := db.QueryRowContext(ctx, `
		INSERT INTO queries
		  (user_id, name, query_string, poll_interval_seconds, is_active,
		   min_abstract_length, publication_type_allowlist,
		   publication_type_blocklist, notes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at
	`, userID, in.Name, in.QueryString, in.PollIntervalSeconds, in.IsActive,
		in.MinAbstractLength, pq.Array(in.PublicationTypeAllowlist),
		pq.Array(in.PublicationTypeBlocklist), in.Notes,
	).Scan(&id, &createdAt)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrNameExists
		}
		return nil, err
	}
	return GetByID(ctx, db, userID, id)
}

// Update applies a partial patch. Returns ErrNotFound if the row doesn't
// exist or isn't owned. Returns ErrNameExists on unique-name collision.
// Changing query_string does NOT clear last_polled_at — see ClearLastPolled
// for that.
func Update(ctx context.Context, db *sql.DB, userID, id int64, in UpdateInput) (*Query, error) {
	sets := []string{}
	args := []any{id, userID}
	nextArg := 3

	add := func(col string, val any) {
		sets = append(sets, col+" = $"+itoa(nextArg))
		args = append(args, val)
		nextArg++
	}

	if in.Name != nil {
		add("name", *in.Name)
	}
	if in.QueryString != nil {
		add("query_string", *in.QueryString)
	}
	if in.PollIntervalSeconds != nil {
		add("poll_interval_seconds", *in.PollIntervalSeconds)
	}
	if in.IsActive != nil {
		add("is_active", *in.IsActive)
	}
	if in.MinAbstractLength != nil {
		add("min_abstract_length", *in.MinAbstractLength)
	}
	if in.PublicationTypeAllowlist != nil {
		add("publication_type_allowlist", pq.Array(*in.PublicationTypeAllowlist))
	}
	if in.PublicationTypeBlocklist != nil {
		add("publication_type_blocklist", pq.Array(*in.PublicationTypeBlocklist))
	}
	if in.Notes != nil {
		add("notes", *in.Notes)
	}

	if len(sets) == 0 {
		// No changes — just return current row.
		return GetByID(ctx, db, userID, id)
	}

	sql := "UPDATE queries SET " + strings.Join(sets, ", ") +
		" WHERE id = $1 AND user_id = $2"
	res, err := db.ExecContext(ctx, sql, args...)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrNameExists
		}
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, ErrNotFound
	}
	return GetByID(ctx, db, userID, id)
}

// Delete removes a query owned by userID. FK CASCADE on query_matches.query_id
// (from migration 00001) handles the join table.
func Delete(ctx context.Context, db *sql.DB, userID, id int64) error {
	res, err := db.ExecContext(ctx, `DELETE FROM queries WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// CountMatches returns the number of query_matches rows that would cascade
// if this query were deleted. Used for audit logging before the DELETE.
func CountMatches(ctx context.Context, db *sql.DB, userID, id int64) (int, error) {
	var n int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM query_matches qm
		JOIN queries q ON q.id = qm.query_id
		WHERE qm.query_id = $1 AND q.user_id = $2
	`, id, userID).Scan(&n)
	return n, err
}

// ClearLastPolled sets last_polled_at = NULL so the next scheduler tick
// re-polls the query from the 30-day lookback window.
func ClearLastPolled(ctx context.Context, db *sql.DB, userID, id int64) error {
	res, err := db.ExecContext(ctx,
		`UPDATE queries SET last_polled_at = NULL WHERE id = $1 AND user_id = $2`,
		id, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "SQLSTATE 23505") || strings.Contains(s, "unique") || strings.Contains(s, "duplicate")
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
