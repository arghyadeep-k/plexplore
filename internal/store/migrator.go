package store

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

const createMigrationsTableSQL = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version TEXT PRIMARY KEY,
    applied_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
`

const sqlitePiPragmasSQL = `
PRAGMA foreign_keys = ON;
PRAGMA journal_mode = WAL;
PRAGMA synchronous = NORMAL;
PRAGMA wal_autocheckpoint = 1000;
PRAGMA busy_timeout = 5000;
PRAGMA cache_size = -4096;
PRAGMA temp_store = MEMORY;
`

type Migrator struct {
	DBPath        string
	MigrationsDir string
	SQLiteBin     string
}

func NewMigrator(dbPath, migrationsDir string) Migrator {
	return Migrator{
		DBPath:        dbPath,
		MigrationsDir: migrationsDir,
		SQLiteBin:     "sqlite3",
	}
}

func (m Migrator) Apply() error {
	if strings.TrimSpace(m.DBPath) == "" {
		return fmt.Errorf("empty sqlite db path")
	}
	if strings.TrimSpace(m.MigrationsDir) == "" {
		return fmt.Errorf("empty migrations directory")
	}

	if err := os.MkdirAll(filepath.Dir(m.DBPath), 0o755); err != nil {
		return fmt.Errorf("create db directory: %w", err)
	}

	if _, err := m.runSQL(sqlitePiPragmasSQL); err != nil {
		return fmt.Errorf("apply sqlite pragmas: %w", err)
	}
	if _, err := m.runSQL(createMigrationsTableSQL); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	applied, err := m.appliedVersions()
	if err != nil {
		return err
	}

	files, err := migrationFiles(m.MigrationsDir)
	if err != nil {
		return err
	}

	for _, file := range files {
		version := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
		if applied[version] {
			continue
		}

		sqlBytes, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("read migration %q: %w", file, err)
		}
		if err := m.applyMigration(version, string(sqlBytes)); err != nil {
			return fmt.Errorf("apply migration %q: %w", file, err)
		}
	}

	return nil
}

func (m Migrator) applyMigration(version, sqlText string) error {
	quotedVersion := "'" + escapeSingleQuotes(version) + "'"
	txScript := strings.TrimSpace(sqlText) + ";\n" +
		"INSERT INTO schema_migrations(version) VALUES(" + quotedVersion + ");\n"
	wrapped := "BEGIN IMMEDIATE;\n" + txScript + "COMMIT;\n"
	if _, err := m.runSQL(wrapped); err == nil {
		return nil
	} else {
		alreadyApplied, checkErr := m.migrationSchemaAlreadyApplied(version)
		if checkErr != nil {
			return checkErr
		}
		if !alreadyApplied {
			return fmt.Errorf("migration SQL failed and schema check says not applied: %w", err)
		}

		if _, err := m.runSQL("INSERT OR IGNORE INTO schema_migrations(version) VALUES(" + quotedVersion + ");"); err != nil {
			return fmt.Errorf("record recovered migration %q: %w", version, err)
		}
		return nil
	}
}

func (m Migrator) migrationSchemaAlreadyApplied(version string) (bool, error) {
	switch version {
	case "0002_devices_updated_at":
		return m.columnExists("devices", "updated_at")
	case "0005_users_auth_fields":
		hasPasswordHash, err := m.columnExists("users", "password_hash")
		if err != nil {
			return false, err
		}
		hasIsAdmin, err := m.columnExists("users", "is_admin")
		if err != nil {
			return false, err
		}
		hasUpdatedAt, err := m.columnExists("users", "updated_at")
		if err != nil {
			return false, err
		}
		if !(hasPasswordHash && hasIsAdmin && hasUpdatedAt) {
			return false, nil
		}
		hasIndex, err := m.indexExists("idx_users_email_unique_nonempty")
		if err != nil {
			return false, err
		}
		if !hasIndex {
			if _, err := m.runSQL(`
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_unique_nonempty
ON users(email)
WHERE email <> '';
`); err != nil {
				return false, fmt.Errorf("recover users email index: %w", err)
			}
		}
		return true, nil
	case "0007_device_api_key_hash":
		hasHash, err := m.columnExists("devices", "api_key_hash")
		if err != nil {
			return false, err
		}
		hasPreview, err := m.columnExists("devices", "api_key_preview")
		if err != nil {
			return false, err
		}
		if !(hasHash && hasPreview) {
			return false, nil
		}
		hasIndex, err := m.indexExists("idx_devices_api_key_hash")
		if err != nil {
			return false, err
		}
		if !hasIndex {
			if _, err := m.runSQL(`
CREATE UNIQUE INDEX IF NOT EXISTS idx_devices_api_key_hash
ON devices(api_key_hash)
WHERE api_key_hash IS NOT NULL AND trim(api_key_hash) <> '';
`); err != nil {
				return false, fmt.Errorf("recover device api key hash index: %w", err)
			}
		}
		return true, nil
	default:
		return false, nil
	}
}

func (m Migrator) columnExists(tableName, columnName string) (bool, error) {
	output, err := m.runSQL("PRAGMA table_info(" + tableName + ");")
	if err != nil {
		return false, fmt.Errorf("check table_info for %s: %w", tableName, err)
	}
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		fields := strings.Split(line, "|")
		if len(fields) < 2 {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(fields[1]), strings.TrimSpace(columnName)) {
			return true, nil
		}
	}
	return false, nil
}

func (m Migrator) indexExists(indexName string) (bool, error) {
	query := "SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name='" + escapeSingleQuotes(indexName) + "';"
	output, err := m.runSQL(query)
	if err != nil {
		return false, fmt.Errorf("check index %s: %w", indexName, err)
	}
	count, parseErr := strconv.Atoi(strings.TrimSpace(output))
	if parseErr != nil {
		return false, fmt.Errorf("parse index count output: %w", parseErr)
	}
	return count > 0, nil
}

func (m Migrator) appliedVersions() (map[string]bool, error) {
	output, err := m.runSQL("SELECT version FROM schema_migrations ORDER BY version;")
	if err != nil {
		return nil, fmt.Errorf("load applied migration versions: %w", err)
	}

	versions := make(map[string]bool)
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		version := strings.TrimSpace(line)
		if version == "" {
			continue
		}
		versions[version] = true
	}
	return versions, nil
}

func migrationFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read migrations dir %q: %w", dir, err)
	}

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(strings.ToLower(entry.Name()), ".sql") {
			files = append(files, filepath.Join(dir, entry.Name()))
		}
	}
	slices.Sort(files)
	return files, nil
}

func (m Migrator) runSQL(sql string) (string, error) {
	cmd := exec.Command(m.SQLiteBin, "-batch", "-bail", "-noheader", m.DBPath)
	cmd.Stdin = strings.NewReader(sql)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("sqlite3 failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}

func escapeSingleQuotes(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}
