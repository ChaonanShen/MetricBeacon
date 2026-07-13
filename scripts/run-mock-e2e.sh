#!/bin/sh
set -eu
project=${COMPOSE_PROJECT_NAME:-mini-torchbearing-mock}
cleanup() { docker compose -p "$project" -f compose.mock-e2e.yaml down -v --remove-orphans; }
trap cleanup EXIT INT TERM
cd apps/grafana-plugin/frontend
npm run build
cd ../../..
docker compose -p "$project" -f compose.mock-e2e.yaml up --build --wait
tests/e2e/mock/api-e2e.sh
