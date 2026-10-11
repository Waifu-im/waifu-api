#!/usr/bin/env bash
#
# Runs the full Go test suite against a real Postgres with the frozen schema.
#
# It exists because the suite refuses to skip. An earlier version skipped when
# WAIFU_TEST_DATABASE_URL was unset, which meant "go test ./..." reported green
# while nothing had executed — and the service could not start at all, because
# of a duplicate route registration that only the first real boot exposed.
#
# This brings up the database if it is not already running, applies the frozen
# schema, runs the suite, and tears the container down again.
#
# Usage:
#   ./scripts/test.sh              # start a container, test, tear down
#   KEEP_DB=1 ./scripts/test.sh    # leave the container up for poking at
#
# CI does the same thing with a service container; see go-backend.yml.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CONTAINER="waifu-testdb"
PORT="${WAIFU_TEST_PORT:-55432}"
DATABASE="waifutest"
SCHEMA="${REPO_ROOT}/backend/go/internal/store/schema.sql"

export WAIFU_TEST_DATABASE_URL="${WAIFU_TEST_DATABASE_URL:-postgres://postgres:test@localhost:${PORT}/${DATABASE}}"

started_container=0

cleanup() {
  if [ "${started_container}" = "1" ] && [ -z "${KEEP_DB:-}" ]; then
    echo "==> stopping ${CONTAINER}"
    docker rm -f "${CONTAINER}" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

# Reuse an existing container if one is already listening, so repeated runs do
# not pay the image-pull and startup cost every time.
if docker ps --format '{{.Names}}' | grep -qx "${CONTAINER}"; then
  echo "==> reusing running ${CONTAINER}"
elif [ -z "${WAIFU_TEST_DATABASE_URL_EXTERNAL:-}" ]; then
  echo "==> starting ${CONTAINER} on port ${PORT}"
  docker rm -f "${CONTAINER}" >/dev/null 2>&1 || true
  docker run -d --name "${CONTAINER}" \
    -e POSTGRES_PASSWORD=test \
    -e POSTGRES_DB="${DATABASE}" \
    -p "${PORT}:5432" \
    pgvector/pgvector:pg18 >/dev/null
  started_container=1

  echo "==> waiting for postgres"
  for _ in $(seq 1 60); do
    # psql rather than pg_isready: pg_isready reports the *server* as accepting
    # connections before the database from POSTGRES_DB has been created, so it
    # succeeds early and the schema apply then fails on a missing database.
    if docker exec "${CONTAINER}" psql -q -U postgres -d "${DATABASE}" -c 'SELECT 1' >/dev/null 2>&1; then
      break
    fi
    sleep 1
  done
fi

# The schema is idempotent (every statement is CREATE ... IF NOT EXISTS), so
# applying it on every run also repairs a database left in a bad state.
echo "==> applying frozen schema"
docker exec -i "${CONTAINER}" psql -q -U postgres -d "${DATABASE}" < "${SCHEMA}"

echo "==> go vet"
(cd "${REPO_ROOT}/backend/go" && go vet ./...)

echo "==> gofmt"
unformatted=$(cd "${REPO_ROOT}/backend/go" && gofmt -l .)
if [ -n "${unformatted}" ]; then
  echo "these files need gofmt:" >&2
  echo "${unformatted}" >&2
  exit 1
fi

echo "==> go test -race ./..."
cd "${REPO_ROOT}/backend/go"
go test -race -timeout 10m ./...