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

# Each generator is skipped until its inputs exist (sqlc.yaml, *.templ, input.css),
# so `make run` works from T-101 onward without the generator tools installed.
sqlc:
	@if [ -f sqlc.yaml ]; then sqlc generate; else echo "sqlc: no sqlc.yaml yet, skipped"; fi

templ:
	@if [ -n "$$(find cmd internal web -name '*.templ' 2>/dev/null | head -n 1)" ]; then templ generate; else echo "templ: no .templ files yet, skipped"; fi

css:
	@if [ -f web/static/css/input.css ]; then tailwindcss -i web/static/css/input.css -o web/static/css/app.css --minify; else echo "css: no web/static/css/input.css yet, skipped"; fi

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
