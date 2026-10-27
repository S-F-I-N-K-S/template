     1	-include .env
     2	export
     3	
     4	export DATABASE_URL ?= postgres://tripgo:tripgo@localhost:5432/tripgo?sslmode=disable
     5	
     6	.PHONY: generate migrate migrate-up migrate-down migrate-status run fmt lint test test-integration
     7	
     8	generate:
     9		go tool oapi-codegen -generate types,chi-server -package api \
    10			-o internal/generated/api.gen.go contracts/openapi/trip-service.openapi.yaml
    11		go generate ./...
    12	
    13	migrate: migrate-up
    14	
    15	migrate-up:
    16		go tool goose -dir migrations postgres "$$DATABASE_URL" up
    17	
    18	migrate-down:
    19		go tool goose -dir migrations postgres "$$DATABASE_URL" down
    20	
    21	migrate-status:
    22		go tool goose -dir migrations postgres "$$DATABASE_URL" status
    23	
    24	run:
    25		go run ./cmd/trip-service
    26	
    27	fmt:
    28		golangci-lint fmt
    29	
    30	lint:
    31		golangci-lint run ./...
    32	
    33	test:
    34		go test -race -count=1 ./...
    35		go test -race -count=1 -coverprofile=coverage.out ./internal/usecase/... ./internal/domain/...
    36		go tool cover -func=coverage.out | tail -n 1
    37	
    38	test-integration:
    39		go test -race -count=1 -tags integration ./...
    40	
