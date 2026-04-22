package main

import (
	"log"
	"os"

	"plexplore/internal/store"
)

func main() {
	dbPath := getenv("APP_SQLITE_PATH", "./data/plexplore.db")
	migrationsDir := getenv("APP_MIGRATIONS_DIR", "./migrations")

	migrator := store.NewMigrator(dbPath, migrationsDir)
	if err := migrator.Apply(); err != nil {
		log.Fatalf("migration failed: %v", err)
	}

	log.Printf("migrations applied successfully (db=%s)", dbPath)
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
