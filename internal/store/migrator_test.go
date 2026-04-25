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

func TestMigrator_VisitGenerationStateMigrationBackfillsDeviceID(t *testing.T) {
	requireSQLiteCLI(t)

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	migrationsDir := filepath.Join(tmpDir, "migrations")
	if err := os.MkdirAll(migrationsDir, 0o755); err != nil {
		t.Fatalf("mkdir migrations: %v", err)
	}

	m0001 := `
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS devices (
    id INTEGER PRIMARY KEY,
    user_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    source_type TEXT NOT NULL,
    api_key TEXT NOT NULL
);
`
	m0008 := `
CREATE TABLE IF NOT EXISTS visit_generation_state (
    device_name TEXT PRIMARY KEY,
    last_processed_seq INTEGER NOT NULL DEFAULT 0,
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
`
	m0009 := `
CREATE TABLE IF NOT EXISTS visit_generation_state_new (
    device_id INTEGER PRIMARY KEY,
    last_processed_seq INTEGER NOT NULL DEFAULT 0,
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    FOREIGN KEY(device_id) REFERENCES devices(id) ON DELETE CASCADE
);
INSERT OR IGNORE INTO visit_generation_state_new(device_id, last_processed_seq, updated_at)
SELECT d.id, s.last_processed_seq, s.updated_at
FROM visit_generation_state s
JOIN devices d ON d.name = s.device_name;
DROP TABLE IF EXISTS visit_generation_state;
ALTER TABLE visit_generation_state_new RENAME TO visit_generation_state;
`

	writeMigration := func(filename, sql string) {
		if err := os.WriteFile(filepath.Join(migrationsDir, filename), []byte(sql), 0o644); err != nil {
			t.Fatalf("write migration %s: %v", filename, err)
		}
	}
	writeMigration("0001_init.sql", m0001)
	writeMigration("0008_visit_generation_state.sql", m0008)

	m := NewMigrator(dbPath, migrationsDir)
	if err := m.Apply(); err != nil {
		t.Fatalf("initial Apply failed: %v", err)
	}

	if _, err := m.runSQL(`
INSERT INTO users(id, name) VALUES (1, 'u1');
INSERT INTO devices(id, user_id, name, source_type, api_key) VALUES (7, 1, 'phone-main', 'owntracks', 'x');
INSERT INTO visit_generation_state(device_name, last_processed_seq, updated_at) VALUES ('phone-main', 123, '2026-04-24T12:00:00Z');
`); err != nil {
		t.Fatalf("seed pre-migration visit_generation_state failed: %v", err)
	}

	writeMigration("0009_visit_generation_state_device_id.sql", m0009)

	if err := m.Apply(); err != nil {
		t.Fatalf("second Apply with 0009 rerun failed: %v", err)
	}

	out, err := m.runSQL("SELECT device_id, last_processed_seq FROM visit_generation_state ORDER BY device_id;")
	if err != nil {
		t.Fatalf("query migrated visit_generation_state failed: %v", err)
	}
	if strings.TrimSpace(out) != "7|123" {
		t.Fatalf("expected backfilled device_id watermark row '7|123', got %q", strings.TrimSpace(out))
	}
}
