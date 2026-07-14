#!/bin/sh
set -eu

project=${COMPOSE_PROJECT_NAME:-mini-torchbearing-real-diagnostics}
mcp_host_port=${MTB_DIAGNOSTIC_MCP_PORT:-18081}
export ASSISTANT_MCP_HOST_PORT=$mcp_host_port
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
	MTB_LIVE_MCP_ENDPOINT="http://127.0.0.1:$mcp_host_port/mcp" \
	go test ./internal/bootstrap -run '^TestLivePrometheusMCPDiagnostic$' -count=1 -v
