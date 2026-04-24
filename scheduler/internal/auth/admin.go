package auth

// Admin-bypass methods: these operate across users and must only be called
// behind AdminRequired middleware. See docs/ARCHITECTURE.md.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// AdminUserRow is the list-response shape: public fields plus aggregated counts.
type AdminUserRow struct {
	ID           int64      `json:"id"`
	Username     string     `json:"username"`
	Email        string     `json:"email"`
	IsAdmin      bool       `json:"is_admin"`
	CreatedAt    time.Time  `json:"created_at"`
	QueriesCount int        `json:"queries_count"`
	LastLoginAt  *time.Time `json:"last_login_at"`
}

// AdminListUsers returns all users with aggregated queries_count and last_login_at.
// Paginated. Ordered by created_at DESC.
func AdminListUsers(ctx context.Context, db *sql.DB, limit, offset int) ([]AdminUserRow, int, error) {
	var total int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := db.QueryContext(ctx, `
		SELECT u.id, u.username, u.email, u.is_admin, u.created_at,
		       (SELECT COUNT(*) FROM queries WHERE user_id = u.id) AS queries_count,
		       (SELECT MAX(created_at) FROM sessions WHERE user_id = u.id) AS last_login_at
		FROM users u
		ORDER BY u.created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]AdminUserRow, 0, limit)
	for rows.Next() {
		var (
			u    AdminUserRow
			last sql.NullTime
		)
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.IsAdmin, &u.CreatedAt, &u.QueriesCount, &last); err != nil {
			return nil, 0, err
		}
		if last.Valid {
			t := last.Time
			u.LastLoginAt = &t
		}
		out = append(out, u)
	}
	return out, total, rows.Err()
}

// AdminCreateUser inserts a user; admins can set is_admin.
// Returns ErrUserExists on unique-violation.
func AdminCreateUser(ctx context.Context, db *sql.DB, username, email, passwordHash string, isAdmin bool) (*User, error) {
	return CreateUser(ctx, db, username, email, passwordHash, isAdmin)
}

// AdminUpdateUser applies a partial update. Only is_admin and email are
// supported; passwords use AdminResetPassword. Returns ErrUserNotFound
// when no row matches, ErrUserExists on email collision.
func AdminUpdateUser(ctx context.Context, db *sql.DB, id int64, email *string, isAdmin *bool) (*User, error) {
	sets := []string{"updated_at = now()"}
	args := []any{id}
	nextArg := 2
	if email != nil {
		sets = append(sets, "email = $"+itoaAdmin(nextArg))
		args = append(args, *email)
		nextArg++
	}
	if isAdmin != nil {
		sets = append(sets, "is_admin = $"+itoaAdmin(nextArg))
		args = append(args, *isAdmin)
		nextArg++
	}
	if len(sets) == 1 {
		// only updated_at — fetch and return
		return adminGetUserByID(ctx, db, id)
	}

	query := "UPDATE users SET " + strings.Join(sets, ", ") +
		" WHERE id = $1 RETURNING id, username, email, is_admin"
	var u User
	err := db.QueryRowContext(ctx, query, args...).Scan(&u.ID, &u.Username, &u.Email, &u.IsAdmin)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrUserExists
		}
		return nil, err
	}
	return &u, nil
}

// AdminDeleteUser removes a user. Cascades to queries, sessions (via FK).
// query_matches, digests, digest_articles cascade via their own FKs.
// Articles themselves stay.
func AdminDeleteUser(ctx context.Context, db *sql.DB, id int64) error {
	res, err := db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrUserNotFound
	}
	return nil
}

// AdminResetPassword writes a new password hash for the given user id.
func AdminResetPassword(ctx context.Context, db *sql.DB, id int64, newHash string) error {
	res, err := db.ExecContext(ctx,
		`UPDATE users SET password_hash = $1, updated_at = now() WHERE id = $2`,
		newHash, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrUserNotFound
	}
	return nil
}

// adminGetUserByID is an internal helper used by AdminUpdateUser when no
// columns were actually updated (so we still return a consistent User shape).
func adminGetUserByID(ctx context.Context, db *sql.DB, id int64) (*User, error) {
	var u User
	err := db.QueryRowContext(ctx,
		`SELECT id, username, email, is_admin FROM users WHERE id = $1`, id,
	).Scan(&u.ID, &u.Username, &u.Email, &u.IsAdmin)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func itoaAdmin(n int) string {
	return fmt.Sprintf("%d", n)
}
