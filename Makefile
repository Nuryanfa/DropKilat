# DropKilat — developer commands. Targets for code that does not exist yet will fail until its task lands.
SHELL := /bin/bash
-include .env
export

MIGRATE := migrate -path migrations -database "$(DK_MIGRATE_DATABASE_URL)"
COMPOSE := docker compose -f deploy/docker-compose.dev.yml

.PHONY: db-up db-down migrate migrate-down migrate-check run build test test-unit test-concurrency e2e load lint sqlc templ css gen

db-up:            ## start PostgreSQL for development
	$(COMPOSE) up -d db

db-down:
	$(COMPOSE) down

migrate:          ## apply all migrations
	$(MIGRATE) up

migrate-down:
	$(MIGRATE) down 1

migrate-check:    ## up -> down -> up on a throwaway database
	./scripts/migrate-check.sh

gen: sqlc templ css

sqlc:
	sqlc generate

templ:
	templ generate

css:
	tailwindcss -i web/static/css/input.css -o web/static/css/app.css --minify

run: gen
	go run ./cmd/dropkilat

build: gen
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/dropkilat ./cmd/dropkilat

test-unit:
	go test -short ./...

test:             ## unit + integration (needs Docker for testcontainers)
	go test -race ./...

test-concurrency: ## stock invariants, repeated
	go test -race -run 'Concurrency|Invariant' -count=50 ./internal/order/... ./internal/waitlist/...

e2e:
	cd tests/e2e && npx playwright test

load:
	k6 run tests/load/claim_burst.js

lint:
	golangci-lint run ./...
	govulncheck ./...
