.PHONY: build run test fmt migrate

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
