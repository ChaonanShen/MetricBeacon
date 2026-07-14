#!/bin/sh
set -eu

project=${COMPOSE_PROJECT_NAME:-mini-torchbearing-real-metrics}
export GRAFANA_HOST_PORT=${MTB_E2E_GRAFANA_PORT:-13000}
export AI_CORE_HOST_PORT=${MTB_E2E_AI_CORE_PORT:-18080}
export ASSISTANT_MCP_HOST_PORT=${MTB_E2E_MCP_PORT:-18081}
grafana_url="http://127.0.0.1:$GRAFANA_HOST_PORT"
root=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
base_compose="$root/compose.mock-e2e.yaml"
real_compose="$root/compose.real-metrics-e2e.yaml"
compose() { docker compose -p "$project" -f "$base_compose" -f "$real_compose" "$@"; }
cleanup() { compose down -v --remove-orphans; }
trap cleanup EXIT INT TERM

cd "$root/apps/grafana-plugin/frontend"
npm run build
cd "$root"
compose up --build --wait
"$root/scripts/wait-for-real-metrics.sh" "$project" "$base_compose" "$real_compose"

GRAFANA_URL=$grafana_url REAL_METRICS=1 "$root/tests/e2e/mock/api-e2e.sh"
cd "$root/apps/grafana-plugin/frontend"
GRAFANA_URL=$grafana_url REAL_METRICS=1 npm run test:e2e
