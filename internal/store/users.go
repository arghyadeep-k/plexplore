package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrUserNotFound = errors.New("user not found")

type User struct {
	ID           int64
	Name         string
	Email        string
	PasswordHash string
	IsAdmin      bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type CreateUserParams struct {
	Name         string
	Email        string
	PasswordHash string
	IsAdmin      bool
}

func (s *SQLiteStore) CreateUser(ctx context.Context, params CreateUserParams) (User, error) {
	email := strings.ToLower(strings.TrimSpace(params.Email))
	if email == "" {
		return User{}, fmt.Errorf("email is required")
	}
	passwordHash := strings.TrimSpace(params.PasswordHash)
	if passwordHash == "" {
		return User{}, fmt.Errorf("password hash is required")
	}

	name := strings.TrimSpace(params.Name)
	if name == "" {
		name = email
	}
	nowUTC := time.Now().UTC().Format(time.RFC3339Nano)

	result, err := s.db.ExecContext(ctx, `
INSERT INTO users(name, email, password_hash, is_admin, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?);
`, name, email, passwordHash, boolToSQLiteInt(params.IsAdmin), nowUTC, nowUTC)
	if err != nil {
		return User{}, fmt.Errorf("create user: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return User{}, fmt.Errorf("user last insert id: %w", err)
	}
	return s.GetUserByID(ctx, id)
}

func (s *SQLiteStore) GetUserByEmail(ctx context.Context, email string) (User, error) {
	normalized := strings.ToLower(strings.TrimSpace(email))
	if normalized == "" {
		return User{}, ErrUserNotFound
	}

	var id int64
	err := s.db.QueryRowContext(ctx, `
SELECT id
FROM users
WHERE LOWER(email) = ?
ORDER BY id ASC
LIMIT 1;
`, normalized).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrUserNotFound
		}
		return User{}, fmt.Errorf("lookup user by email: %w", err)
	}
	return s.GetUserByID(ctx, id)
}

func (s *SQLiteStore) GetUserByID(ctx context.Context, id int64) (User, error) {
	if id <= 0 {
		return User{}, ErrUserNotFound
	}

	var user User
	var isAdminRaw int
	var createdAtRaw string
	var updatedAtRaw string
	err := s.db.QueryRowContext(ctx, `
SELECT id, name, email, password_hash, is_admin, created_at, updated_at
FROM users
WHERE id = ?;
`, id).Scan(
		&user.ID,
		&user.Name,
		&user.Email,
		&user.PasswordHash,
		&isAdminRaw,
		&createdAtRaw,
		&updatedAtRaw,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrUserNotFound
		}
		return User{}, fmt.Errorf("lookup user by id: %w", err)
	}

	user.IsAdmin = isAdminRaw != 0
	var parseErr error
	user.CreatedAt, parseErr = parseDBTime(createdAtRaw)
	if parseErr != nil {
		return User{}, fmt.Errorf("parse user created_at: %w", parseErr)
	}
	user.UpdatedAt, parseErr = parseDBTime(updatedAtRaw)
	if parseErr != nil {
		return User{}, fmt.Errorf("parse user updated_at: %w", parseErr)
	}

	return user, nil
}

func (s *SQLiteStore) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, name, email, password_hash, is_admin, created_at, updated_at
FROM users
ORDER BY id ASC;
`)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	users := make([]User, 0)
	for rows.Next() {
		var user User
		var isAdminRaw int
		var createdAtRaw string
		var updatedAtRaw string
		if err := rows.Scan(
			&user.ID,
			&user.Name,
			&user.Email,
			&user.PasswordHash,
			&isAdminRaw,
			&createdAtRaw,
			&updatedAtRaw,
		); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}

		user.IsAdmin = isAdminRaw != 0
		user.CreatedAt, err = parseDBTime(createdAtRaw)
		if err != nil {
			return nil, fmt.Errorf("parse user created_at: %w", err)
		}
		user.UpdatedAt, err = parseDBTime(updatedAtRaw)
		if err != nil {
			return nil, fmt.Errorf("parse user updated_at: %w", err)
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate users: %w", err)
	}
	return users, nil
}

func boolToSQLiteInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
