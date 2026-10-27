-include .env
export

export DATABASE_URL ?= postgres://tripgo:tripgo@localhost:5432/tripgo?sslmode=disable

.PHONY: generate migrate migrate-up migrate-down migrate-status run fmt lint test test-integration

generate:
	go tool oapi-codegen -generate types,chi-server -package api \
		-o internal/generated/api.gen.go contracts/openapi/trip-service.openapi.yaml
	go generate ./...

migrate: migrate-up

migrate-up:
	go tool goose -dir migrations postgres "$$DATABASE_URL" up

migrate-down:
	go tool goose -dir migrations postgres "$$DATABASE_URL" down

migrate-status:
	go tool goose -dir migrations postgres "$$DATABASE_URL" status

run:
	go run ./cmd/trip-service

fmt:
	golangci-lint fmt

lint:
	golangci-lint run ./...

test:
	go test -race -count=1 ./...
	go test -race -count=1 -coverprofile=coverage.out ./internal/usecase/... ./internal/domain/...
	go tool cover -func=coverage.out | tail -n 1

test-integration:
	go test -race -count=1 -tags integration ./...
