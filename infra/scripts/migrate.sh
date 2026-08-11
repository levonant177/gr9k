#!/usr/bin/env bash
set -euo pipefail

# Simple migration runner for local development
# Usage: ./migrate.sh up | down | status

MIGRATIONS_DIR="$(cd "$(dirname "$0")/../migrations" && pwd)"
DATABASE_URL="${DATABASE_URL:-postgres://ups:ups_secret@localhost:5432/ups_eco?sslmode=disable}"

case "${1:-up}" in
  up)
    echo "→ Applying migrations from $MIGRATIONS_DIR"
    for f in "$MIGRATIONS_DIR"/*.sql; do
      echo "  Applying $(basename "$f")..."
      psql "$DATABASE_URL" -f "$f" -v ON_ERROR_STOP=1
    done
    echo "✓ Migrations applied"
    ;;
  status)
    psql "$DATABASE_URL" -c "\dt"
    ;;
  *)
    echo "Usage: $0 {up|status}"
    exit 1
    ;;
esac
