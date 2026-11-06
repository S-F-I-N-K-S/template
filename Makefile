ifneq (,$(wildcard .env))
include .env
export
endif

.PHONY: generate migrate migrate-up migrate-down migrate-status run test require-env

generate:
	go tool oapi-codegen -generate types,chi-server -package api \
		-o internal/generated/api.gen.go contracts/openapi/trip-service.openapi.yaml

require-env:
	@test -n "$(DATABASE_URL)" || { \
		echo "DATABASE_URL не задан: выполните 'tripgoctl environment start' (он создаст .env) или экспортируйте переменные" >&2; \
		exit 1; }

migrate: migrate-up

migrate-up: require-env
	go tool goose -dir migrations postgres "$$DATABASE_URL" up

migrate-down: require-env
	go tool goose -dir migrations postgres "$$DATABASE_URL" down

migrate-status: require-env
	go tool goose -dir migrations postgres "$$DATABASE_URL" status

run: require-env
	go run ./cmd/trip-service

test:
	go test -race ./...
