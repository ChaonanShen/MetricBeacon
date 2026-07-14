#!/bin/sh
set -eu
project=${COMPOSE_PROJECT_NAME:-mini-torchbearing-mock}
export GRAFANA_HOST_PORT=${MTB_E2E_GRAFANA_PORT:-13000}
export AI_CORE_HOST_PORT=${MTB_E2E_AI_CORE_PORT:-18080}
export ASSISTANT_MCP_HOST_PORT=${MTB_E2E_MCP_PORT:-18081}
grafana_url="http://127.0.0.1:$GRAFANA_HOST_PORT"
root=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
compose_file="$root/compose.mock-e2e.yaml"
cleanup() { docker compose -p "$project" -f "$compose_file" down -v --remove-orphans; }
trap cleanup EXIT INT TERM
cd "$root/apps/grafana-plugin/frontend"
npm run build
cd "$root"
docker compose -p "$project" -f "$compose_file" up --build --wait
GRAFANA_URL=$grafana_url "$root/tests/e2e/mock/api-e2e.sh"
cd "$root/apps/grafana-plugin/frontend"
GRAFANA_URL=$grafana_url npm run test:e2e
