.PHONY: build run test fmt migrate backup restore verify-backup-restore

build:
	go build ./...

run:
	go run ./cmd/server

test:
	go test ./...

fmt:
	go fmt ./...

migrate:
	go run ./cmd/migrate

backup:
	./scripts/backup.sh

restore:
	@if [ -z "$(ARCHIVE)" ]; then echo "ARCHIVE is required, e.g. make restore ARCHIVE=./backups/plexplore-backup-YYYYMMDD-HHMMSS.tar.gz"; exit 1; fi
	./scripts/restore.sh --archive "$(ARCHIVE)"

verify-backup-restore:
	bash ./scripts/verify_backup_restore.sh
