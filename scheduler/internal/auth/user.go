package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// userRow is the full user shape including password_hash; kept internal.
type userRow struct {
	ID           int64
	Username     string
	Email        string
	PasswordHash string
	IsAdmin      bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (u *userRow) toPublic() *User {
	return &User{ID: u.ID, Username: u.Username, Email: u.Email, IsAdmin: u.IsAdmin}
}

// CreateUser inserts a new user with a pre-hashed password.
// Returns ErrUserExists if the username or email already exists.
func CreateUser(ctx context.Context, db *sql.DB, username, email, passwordHash string, isAdmin bool) (*User, error) {
	username = strings.TrimSpace(username)
	email = strings.TrimSpace(email)
	if username == "" || email == "" || passwordHash == "" {
		return nil, errors.New("username, email, and password are required")
	}

	var id int64
	var createdAt, updatedAt time.Time
	err := db.QueryRowContext(ctx, `
		INSERT INTO users (username, email, password_hash, is_admin)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, updated_at
	`, username, email, passwordHash, isAdmin).Scan(&id, &createdAt, &updatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrUserExists
		}
		return nil, fmt.Errorf("create user: %w", err)
	}

	return &User{ID: id, Username: username, Email: email, IsAdmin: isAdmin}, nil
}

// getUserByUsername fetches the full user row (including password hash) by username.
// Username comparison is case-insensitive (citext column).
func getUserByUsername(ctx context.Context, db *sql.DB, username string) (*userRow, error) {
	var u userRow
	err := db.QueryRowContext(ctx, `
		SELECT id, username, email, password_hash, is_admin, created_at, updated_at
		FROM users WHERE username = $1
	`, username).Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.IsAdmin, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// ChangeUserPassword verifies currentPlain against the stored hash for userID,
// then writes newHash. Returns ErrInvalidCredentials on a wrong current password
// so handlers can return 401 without distinguishing "no such user".
func ChangeUserPassword(ctx context.Context, db *sql.DB, userID int64, currentPlain, newHash string) error {
	var storedHash string
	err := db.QueryRowContext(ctx,
		`SELECT password_hash FROM users WHERE id = $1`, userID,
	).Scan(&storedHash)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrUserNotFound
	}
	if err != nil {
		return err
	}
	ok, err := VerifyPassword(currentPlain, storedHash)
	if err != nil {
		return err
	}
	if !ok {
		return ErrInvalidCredentials
	}
	_, err = db.ExecContext(ctx,
		`UPDATE users SET password_hash = $1, updated_at = now() WHERE id = $2`,
		newHash, userID)
	return err
}

// UpdatePassword rewrites the password hash for an existing user by username.
func UpdatePassword(ctx context.Context, db *sql.DB, username, newHash string) error {
	res, err := db.ExecContext(ctx, `
		UPDATE users SET password_hash = $1, updated_at = now()
		WHERE username = $2
	`, newHash, username)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrUserNotFound
	}
	return nil
}

// isUniqueViolation checks the pgx error code for "23505 unique_violation".
// We match via message substring to avoid a runtime dep on pgconn.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "SQLSTATE 23505") || strings.Contains(s, "unique_violation") || strings.Contains(s, "duplicate key")
}
