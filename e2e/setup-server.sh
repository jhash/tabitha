#!/bin/sh
# Builds the editor bundle + Go binary, migrates the e2e test database,
# seeds one predictable song, then execs the server. Playwright's
# webServer config runs this and waits on the healthcheck URL.
set -e

cd "$(dirname "$0")/.."

export DATABASE_URL="${E2E_DATABASE_URL:-postgres:///tabitha_e2e_test?sslmode=disable}"
export DEV_LOGIN_ENABLED=true
export PORT="${E2E_PORT:-8091}"

(cd editor && npm run build >/dev/null)
go build -o ./tmp/tabitha-e2e .

./tmp/tabitha-e2e migrate up
go run ./cmd/e2eseed

exec ./tmp/tabitha-e2e serve
