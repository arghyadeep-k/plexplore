package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrSessionNotFound = errors.New("session not found")

const defaultSessionTTL = 7 * 24 * time.Hour

type Session struct {
	Token     string
	UserID    int64
	ExpiresAt time.Time
	CreatedAt time.Time
}

func (s *SQLiteStore) CreateSession(ctx context.Context, userID int64) (Session, error) {
	if userID <= 0 {
		return Session{}, fmt.Errorf("user id is required")
	}

	token, err := generateSessionToken()
	if err != nil {
		return Session{}, fmt.Errorf("generate session token: %w", err)
	}
	expiresAt := time.Now().UTC().Add(defaultSessionTTL)
	createdAt := time.Now().UTC()

	_, err = s.db.ExecContext(ctx, `
INSERT INTO sessions(token, user_id, expires_at, created_at)
VALUES (?, ?, ?, ?);
`, token, userID, expiresAt.Format(time.RFC3339Nano), createdAt.Format(time.RFC3339Nano))
	if err != nil {
		return Session{}, fmt.Errorf("create session: %w", err)
	}

	return Session{
		Token:     token,
		UserID:    userID,
		ExpiresAt: expiresAt,
		CreatedAt: createdAt,
	}, nil
}

func (s *SQLiteStore) GetSession(ctx context.Context, token string) (Session, error) {
	normalizedToken := strings.TrimSpace(token)
	if normalizedToken == "" {
		return Session{}, ErrSessionNotFound
	}

	var out Session
	var expiresAtRaw string
	var createdAtRaw string
	err := s.db.QueryRowContext(ctx, `
SELECT token, user_id, expires_at, created_at
FROM sessions
WHERE token = ?
LIMIT 1;
`, normalizedToken).Scan(
		&out.Token,
		&out.UserID,
		&expiresAtRaw,
		&createdAtRaw,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Session{}, ErrSessionNotFound
		}
		return Session{}, fmt.Errorf("get session: %w", err)
	}

	out.ExpiresAt, err = parseDBTime(expiresAtRaw)
	if err != nil {
		return Session{}, fmt.Errorf("parse session expires_at: %w", err)
	}
	out.CreatedAt, err = parseDBTime(createdAtRaw)
	if err != nil {
		return Session{}, fmt.Errorf("parse session created_at: %w", err)
	}
	if !out.ExpiresAt.After(time.Now().UTC()) {
		_ = s.DeleteSession(ctx, normalizedToken)
		return Session{}, ErrSessionNotFound
	}

	return out, nil
}

func (s *SQLiteStore) DeleteSession(ctx context.Context, token string) error {
	normalizedToken := strings.TrimSpace(token)
	if normalizedToken == "" {
		return nil
	}

	_, err := s.db.ExecContext(ctx, `
DELETE FROM sessions
WHERE token = ?;
`, normalizedToken)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func generateSessionToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
