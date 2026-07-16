#!/usr/bin/env bash
# Setup script for tabitha worktrees.
# Copies shared .env/.env.local from the main repo checkout and verifies
# Go tooling.

set -e

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$REPO_ROOT"

get_main_repo_root() {
    if [ -n "$CONDUCTOR_ROOT_PATH" ]; then
        echo "$CONDUCTOR_ROOT_PATH"
    else
        (cd "$(dirname "$(git rev-parse --git-common-dir)")" && pwd)
    fi
}

MAIN_ROOT="$(get_main_repo_root)"

for f in .env .env.local; do
    if [ ! -f "$REPO_ROOT/$f" ] && [ -f "$MAIN_ROOT/$f" ]; then
        cp "$MAIN_ROOT/$f" "$REPO_ROOT/$f"
        echo "Copied $f from $MAIN_ROOT"
    fi
done

if ! command -v go &> /dev/null; then
    echo "Error: Go toolchain not found."
    exit 1
fi

if ! command -v air &> /dev/null; then
    echo "Warning: air not found. Installing..."
    go install github.com/air-verse/air@latest
fi

go mod download

echo "Setup complete. All worktrees share DATABASE_URL from .env; run will auto-assign a port."
