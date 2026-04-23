package store

import (
	"context"
	"errors"
	"testing"
)

func TestSQLiteStore_CreateAndGetUserByEmail(t *testing.T) {
	s := openStoreWithSchema(t)
	ctx := context.Background()

	created, err := s.CreateUser(ctx, CreateUserParams{
		Name:         "Admin User",
		Email:        "admin@example.com",
		PasswordHash: "hash-admin",
		IsAdmin:      true,
	})
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	if created.ID == 0 {
		t.Fatal("expected non-zero user id")
	}
	if !created.IsAdmin {
		t.Fatal("expected admin user")
	}
	if created.Email != "admin@example.com" {
		t.Fatalf("unexpected email: %q", created.Email)
	}
	if created.PasswordHash != "hash-admin" {
		t.Fatalf("unexpected password hash: %q", created.PasswordHash)
	}
	if created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
		t.Fatalf("expected timestamps set, got %+v", created)
	}

	loaded, err := s.GetUserByEmail(ctx, "ADMIN@example.com")
	if err != nil {
		t.Fatalf("GetUserByEmail failed: %v", err)
	}
	if loaded.ID != created.ID {
		t.Fatalf("expected same user id, got %d vs %d", loaded.ID, created.ID)
	}
}

func TestSQLiteStore_ListUsers(t *testing.T) {
	s := openStoreWithSchema(t)
	ctx := context.Background()

	_, err := s.CreateUser(ctx, CreateUserParams{
		Email:        "user-a@example.com",
		PasswordHash: "hash-a",
		IsAdmin:      false,
	})
	if err != nil {
		t.Fatalf("CreateUser A failed: %v", err)
	}
	_, err = s.CreateUser(ctx, CreateUserParams{
		Email:        "user-b@example.com",
		PasswordHash: "hash-b",
		IsAdmin:      true,
	})
	if err != nil {
		t.Fatalf("CreateUser B failed: %v", err)
	}

	users, err := s.ListUsers(ctx)
	if err != nil {
		t.Fatalf("ListUsers failed: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}
	if users[1].Email != "user-b@example.com" || !users[1].IsAdmin {
		t.Fatalf("unexpected second user: %+v", users[1])
	}
}

func TestSQLiteStore_GetUserNotFound(t *testing.T) {
	s := openStoreWithSchema(t)
	ctx := context.Background()

	_, err := s.GetUserByEmail(ctx, "missing@example.com")
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound by email, got %v", err)
	}
	_, err = s.GetUserByID(ctx, 42)
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound by id, got %v", err)
	}
}

func TestSQLiteStore_UsersSchemaHasAuthFields(t *testing.T) {
	s := openStoreWithSchema(t)

	rows, err := s.db.Query(`PRAGMA table_info(users);`)
	if err != nil {
		t.Fatalf("PRAGMA table_info(users) failed: %v", err)
	}
	defer rows.Close()

	columns := map[string]bool{}
	for rows.Next() {
		var (
			cid        int
			name       string
			colType    string
			notNull    int
			defaultV   any
			primaryKey int
		)
		if err := rows.Scan(&cid, &name, &colType, &notNull, &defaultV, &primaryKey); err != nil {
			t.Fatalf("scan table_info row failed: %v", err)
		}
		columns[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate table_info rows failed: %v", err)
	}

	required := []string{"id", "email", "password_hash", "is_admin", "created_at", "updated_at"}
	for _, column := range required {
		if !columns[column] {
			t.Fatalf("expected users column %q to exist", column)
		}
	}
}
