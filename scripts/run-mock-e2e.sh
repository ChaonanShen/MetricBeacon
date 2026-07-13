#!/bin/sh
set -eu
project=${COMPOSE_PROJECT_NAME:-mini-torchbearing-mock}
root=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
compose_file="$root/compose.mock-e2e.yaml"
cleanup() { docker compose -p "$project" -f "$compose_file" down -v --remove-orphans; }
trap cleanup EXIT INT TERM
cd "$root/apps/grafana-plugin/frontend"
npm run build
cd "$root"
docker compose -p "$project" -f "$compose_file" up --build --wait
"$root/tests/e2e/mock/api-e2e.sh"
cd "$root/apps/grafana-plugin/frontend"
npm run test:e2e
