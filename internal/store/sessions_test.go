package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSQLiteStore_CreateGetDeleteSession(t *testing.T) {
	s := openStoreWithSchema(t)
	ctx := context.Background()

	user, err := s.CreateUser(ctx, CreateUserParams{
		Email:        "session-user@example.com",
		PasswordHash: "hash-user",
	})
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	created, err := s.CreateSession(ctx, user.ID)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	if created.Token == "" || created.UserID != user.ID {
		t.Fatalf("unexpected created session: %+v", created)
	}
	if !created.ExpiresAt.After(time.Now().UTC()) {
		t.Fatalf("expected non-expired session, got %+v", created)
	}

	loaded, err := s.GetSession(ctx, created.Token)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}
	if loaded.Token != created.Token || loaded.UserID != user.ID {
		t.Fatalf("unexpected loaded session: %+v", loaded)
	}

	if err := s.DeleteSession(ctx, created.Token); err != nil {
		t.Fatalf("DeleteSession failed: %v", err)
	}
	if _, err := s.GetSession(ctx, created.Token); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("expected ErrSessionNotFound after delete, got %v", err)
	}
}

func TestSQLiteStore_GetSession_Expired(t *testing.T) {
	s := openStoreWithSchema(t)
	ctx := context.Background()

	user, err := s.CreateUser(ctx, CreateUserParams{
		Email:        "expired-user@example.com",
		PasswordHash: "hash-user",
	})
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	session, err := s.CreateSession(ctx, user.ID)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	expired := time.Now().UTC().Add(-1 * time.Minute).Format(time.RFC3339Nano)
	if _, err := s.db.ExecContext(ctx, `
UPDATE sessions SET expires_at = ? WHERE token = ?;
`, expired, session.Token); err != nil {
		t.Fatalf("expire session update failed: %v", err)
	}

	if _, err := s.GetSession(ctx, session.Token); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("expected ErrSessionNotFound for expired session, got %v", err)
	}
}
