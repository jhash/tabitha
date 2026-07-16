#!/usr/bin/env bash
# Run script for tabitha worktrees.
# Auto-detects an available port and starts the dev server (air).
# DATABASE_URL is shared across all worktrees via .env (same Postgres).

set -e

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$REPO_ROOT"

find_available_port() {
    local port=8080
    local max_attempts=100
    local attempt=0

    while [ $attempt -lt $max_attempts ]; do
        if ! nc -z localhost "$port" 2>/dev/null; then
            echo "$port"
            return 0
        fi
        port=$((port + 1))
        attempt=$((attempt + 1))
    done

    echo "Error: Could not find available port after $max_attempts attempts" >&2
    return 1
}

PORT="$(find_available_port)"

export PORT
export APP_URL="http://localhost:${PORT}"

echo "Starting dev server on port $PORT"
echo "Database: ${DATABASE_URL:-(from .env)}"

exec air
