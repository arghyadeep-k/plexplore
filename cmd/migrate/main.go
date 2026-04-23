package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"plexplore/internal/api"
	"plexplore/internal/store"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		log.Fatalf("migration failed: %v", err)
	}
}

func run(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("migrate", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	dbPath := fs.String("db", getenv("APP_SQLITE_PATH", "./data/plexplore.db"), "sqlite database path")
	migrationsDir := fs.String("migrations", getenv("APP_MIGRATIONS_DIR", "./migrations"), "migrations directory")
	createAdmin := fs.Bool("create-admin", false, "bootstrap admin user")
	email := fs.String("email", "", "admin email for --create-admin")
	password := fs.String("password", "", "admin password for --create-admin")
	isAdmin := fs.Bool("is-admin", true, "admin flag for --create-admin (must be true)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	migrator := store.NewMigrator(*dbPath, *migrationsDir)
	if err := migrator.Apply(); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}

	if !*createAdmin {
		_, _ = fmt.Fprintf(stdout, "migrations applied successfully (db=%s)\n", *dbPath)
		return nil
	}

	if !*isAdmin {
		return fmt.Errorf("--is-admin must be true with --create-admin")
	}
	normalizedEmail := strings.TrimSpace(*email)
	if normalizedEmail == "" {
		return fmt.Errorf("--email is required with --create-admin")
	}
	normalizedPassword := strings.TrimSpace(*password)
	if normalizedPassword == "" {
		return fmt.Errorf("--password is required with --create-admin")
	}

	sqliteStore, err := store.OpenSQLiteStore(*dbPath)
	if err != nil {
		return fmt.Errorf("open sqlite store: %w", err)
	}
	defer sqliteStore.Close()

	existing, err := sqliteStore.GetUserByEmail(context.Background(), normalizedEmail)
	if err == nil {
		if existing.IsAdmin {
			return fmt.Errorf("admin already exists for email %s", normalizedEmail)
		}
		return fmt.Errorf("user already exists for email %s and is not admin", normalizedEmail)
	}
	if !errors.Is(err, store.ErrUserNotFound) {
		return fmt.Errorf("lookup existing user: %w", err)
	}

	passwordHash, err := api.HashPassword(normalizedPassword)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	created, err := sqliteStore.CreateUser(context.Background(), store.CreateUserParams{
		Name:         normalizedEmail,
		Email:        normalizedEmail,
		PasswordHash: passwordHash,
		IsAdmin:      true,
	})
	if err != nil {
		return fmt.Errorf("create admin user: %w", err)
	}

	_, _ = fmt.Fprintf(stdout, "created admin: id=%d email=%s is_admin=%t\n", created.ID, created.Email, created.IsAdmin)
	return nil
}

func getenv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}
