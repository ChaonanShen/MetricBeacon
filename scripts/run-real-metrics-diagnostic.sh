#!/bin/sh
set -eu

project=${COMPOSE_PROJECT_NAME:-mini-torchbearing-real-diagnostics}
root=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
base_compose="$root/compose.mock-e2e.yaml"
real_compose="$root/compose.real-metrics-e2e.yaml"
compose() { docker compose -p "$project" -f "$base_compose" -f "$real_compose" "$@"; }
cleanup() { compose down -v --remove-orphans; }
trap cleanup EXIT INT TERM

compose up --build --wait prometheus node-exporter assistant-mcp
"$root/scripts/wait-for-real-metrics.sh" "$project" "$base_compose" "$real_compose"
"$root/scripts/probe-real-prometheus.sh" "$project" "$base_compose" "$real_compose"

cd "$root/services/assistant-mcp"
MTB_RUN_LIVE_MCP_DIAGNOSTIC=1 \
	MTB_LIVE_MCP_ENDPOINT=http://127.0.0.1:8081/mcp \
	go test ./internal/bootstrap -run '^TestLivePrometheusMCPDiagnostic$' -count=1 -v
