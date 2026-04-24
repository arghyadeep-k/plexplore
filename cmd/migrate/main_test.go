package main

import (
	"bytes"
	"context"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"plexplore/internal/api"
	"plexplore/internal/store"
)

func migrationsDirForTests(t *testing.T) string {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to resolve caller path")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
	return filepath.Join(root, "migrations")
}

func TestRun_CreateAdmin_SuccessAndDuplicateBlocked(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	migrationsDir := migrationsDirForTests(t)

	var output bytes.Buffer
	err := run([]string{
		"--db", dbPath,
		"--migrations", migrationsDir,
		"--create-admin",
		"--email", "admin@example.com",
		"--password", "test-pass-123",
	}, &output)
	if err != nil {
		t.Fatalf("run create-admin failed: %v", err)
	}
	if !strings.Contains(output.String(), "created admin:") {
		t.Fatalf("expected create-admin output, got %q", output.String())
	}

	sqliteStore, err := store.OpenSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("OpenSQLiteStore failed: %v", err)
	}
	defer sqliteStore.Close()

	user, err := sqliteStore.GetUserByEmail(context.Background(), "admin@example.com")
	if err != nil {
		t.Fatalf("GetUserByEmail failed: %v", err)
	}
	if !user.IsAdmin {
		t.Fatalf("expected admin user, got %+v", user)
	}
	if user.PasswordHash == "test-pass-123" {
		t.Fatalf("password hash stored as plaintext: %+v", user)
	}
	if !api.VerifyPassword(user.PasswordHash, "test-pass-123") {
		t.Fatalf("stored hash does not verify test password")
	}

	output.Reset()
	err = run([]string{
		"--db", dbPath,
		"--migrations", migrationsDir,
		"--create-admin",
		"--email", "admin@example.com",
		"--password", "test-pass-123",
	}, &output)
	if err == nil || !strings.Contains(err.Error(), "admin already exists") {
		t.Fatalf("expected duplicate admin error, got %v", err)
	}
}

func TestRun_CreateAdmin_Validation(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	migrationsDir := migrationsDirForTests(t)

	var output bytes.Buffer
	cases := [][]string{
		{"--db", dbPath, "--migrations", migrationsDir, "--create-admin"},
		{"--db", dbPath, "--migrations", migrationsDir, "--create-admin", "--email", "admin@example.com"},
		{"--db", dbPath, "--migrations", migrationsDir, "--create-admin", "--password", "test-pass-123"},
		{"--db", dbPath, "--migrations", migrationsDir, "--create-admin", "--email", "admin@example.com", "--password", "test-pass-123", "--is-admin=false"},
	}
	for _, args := range cases {
		output.Reset()
		if err := run(args, &output); err == nil {
			t.Fatalf("expected validation error for args=%v", args)
		}
	}
}
