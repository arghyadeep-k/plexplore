package store

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func requireSQLiteCLI(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("sqlite3"); err != nil {
		t.Skip("sqlite3 CLI not available")
	}
}

func TestMigrator_RecoversPartialAdditiveMigrationAndRecordsVersion(t *testing.T) {
	requireSQLiteCLI(t)

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	migrationsDir := filepath.Join(tmpDir, "migrations")
	if err := os.MkdirAll(migrationsDir, 0o755); err != nil {
		t.Fatalf("mkdir migrations: %v", err)
	}

	m := NewMigrator(dbPath, migrationsDir)
	if _, err := m.runSQL(createMigrationsTableSQL); err != nil {
		t.Fatalf("create schema_migrations: %v", err)
	}
	if _, err := m.runSQL(`CREATE TABLE IF NOT EXISTS devices (id INTEGER PRIMARY KEY, updated_at TEXT);`); err != nil {
		t.Fatalf("create devices table: %v", err)
	}

	migrationPath := filepath.Join(migrationsDir, "0002_devices_updated_at.sql")
	migrationSQL := "ALTER TABLE devices ADD COLUMN updated_at TEXT;"
	if err := os.WriteFile(migrationPath, []byte(migrationSQL), 0o644); err != nil {
		t.Fatalf("write migration file: %v", err)
	}

	if err := m.Apply(); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	output, err := m.runSQL("SELECT version FROM schema_migrations WHERE version='0002_devices_updated_at';")
	if err != nil {
		t.Fatalf("query schema_migrations: %v", err)
	}
	if strings.TrimSpace(output) != "0002_devices_updated_at" {
		t.Fatalf("expected recovered migration record, got %q", output)
	}

	if err := m.Apply(); err != nil {
		t.Fatalf("second Apply should remain idempotent, got %v", err)
	}
}

func TestMigrator_DoesNotRecordVersionWhenMigrationFails(t *testing.T) {
	requireSQLiteCLI(t)

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	migrationsDir := filepath.Join(tmpDir, "migrations")
	if err := os.MkdirAll(migrationsDir, 0o755); err != nil {
		t.Fatalf("mkdir migrations: %v", err)
	}

	migrationPath := filepath.Join(migrationsDir, "0008_bad.sql")
	if err := os.WriteFile(migrationPath, []byte("ALTER TABLE missing_table ADD COLUMN bad_col TEXT;"), 0o644); err != nil {
		t.Fatalf("write migration file: %v", err)
	}

	m := NewMigrator(dbPath, migrationsDir)
	if err := m.Apply(); err == nil {
		t.Fatalf("expected Apply to fail for invalid SQL")
	}

	if _, err := m.runSQL(createMigrationsTableSQL); err != nil {
		t.Fatalf("ensure schema_migrations table: %v", err)
	}
	output, err := m.runSQL("SELECT COUNT(*) FROM schema_migrations WHERE version='0008_bad';")
	if err != nil {
		t.Fatalf("count schema_migrations: %v", err)
	}
	if strings.TrimSpace(output) != "0" {
		t.Fatalf("expected failed migration not to be recorded, got %q", output)
	}
}
