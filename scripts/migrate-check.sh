#!/usr/bin/env bash
# Applies migrations up -> down -> up on a throwaway database to prove they are reversible.
set -euo pipefail
: "${DK_MIGRATE_DATABASE_URL:?set DK_MIGRATE_DATABASE_URL}"
base="${DK_MIGRATE_DATABASE_URL%/*}"
db="dk_migrate_check_$$"
psql "${base}/postgres" -qc "CREATE DATABASE ${db}"
trap 'psql "${base}/postgres" -qc "DROP DATABASE IF EXISTS ${db}"' EXIT
url="${base}/${db}?sslmode=disable"
migrate -path migrations -database "$url" up
migrate -path migrations -database "$url" down -all
migrate -path migrations -database "$url" up
echo "migrate-check OK"
