.PHONY: test test-cover lint tidy build run docker-up docker-hub-up docker-down migrate

DATABASE_URL ?= postgres://paywatch:paywatch@localhost:5434/paywatch?sslmode=disable

test:
	go test ./...

test-cover:
	go test ./... -coverprofile=coverage.out -covermode=atomic
	go tool cover -func=coverage.out | tail -1

lint:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.4.0 run ./...

tidy:
	go mod tidy

build:
	go build -o bin/paywatch ./cmd/paywatch

run:
	go run ./cmd/paywatch

migrate:
	goose -dir db/migrations postgres "$(DATABASE_URL)" up

docker-up:
	docker compose -f docker/docker-compose.yml up --build -d

docker-hub-up:
	docker compose -f docker/docker-compose.hub.yml up -d

docker-down:
	docker compose -f docker/docker-compose.yml -f docker/docker-compose.hub.yml down
