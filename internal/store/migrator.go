package store

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
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
	DBPath         string
	MigrationsDir  string
	SQLiteBin      string
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
		if _, err := m.runSQL(string(sqlBytes)); err != nil {
			return fmt.Errorf("apply migration %q: %w", file, err)
		}
		if _, err := m.runSQL(fmt.Sprintf(
			"INSERT INTO schema_migrations(version) VALUES('%s');",
			escapeSingleQuotes(version),
		)); err != nil {
			return fmt.Errorf("record migration %q: %w", version, err)
		}
	}

	return nil
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
	cmd := exec.Command(m.SQLiteBin, "-batch", "-noheader", m.DBPath)
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
