#!/bin/sh
set -eu

if [ -z "${DEEPSEEK_API_KEY:-}" ]; then
	echo "DEEPSEEK_API_KEY is required for make e2e-real-agent" >&2
	exit 2
fi

project=${COMPOSE_PROJECT_NAME:-mini-torchbearing-real-agent}
root=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
base_compose="$root/compose.mock-e2e.yaml"
metrics_compose="$root/compose.real-metrics-e2e.yaml"
agent_compose="$root/compose.real-agent-e2e.yaml"
compose() { docker compose -p "$project" -f "$base_compose" -f "$metrics_compose" -f "$agent_compose" "$@"; }
cleanup() { compose down -v --remove-orphans; }
trap cleanup EXIT INT TERM

cd "$root/apps/grafana-plugin/frontend"
npm run build
cd "$root"
compose up --build --wait
"$root/scripts/wait-for-real-metrics.sh" "$project" "$base_compose" "$metrics_compose"

node "$root/tests/e2e/real-agent/api-smoke.mjs"
log_file=$(mktemp "${TMPDIR:-/tmp}/mini-torchbearing-real-agent-log.XXXXXX")
database_file=$(mktemp "${TMPDIR:-/tmp}/mini-torchbearing-real-agent-db.XXXXXX")
key_file=$(mktemp "${TMPDIR:-/tmp}/mini-torchbearing-real-agent-key.XXXXXX")
trap 'rm -f "$log_file" "$database_file" "$key_file"; cleanup' EXIT INT TERM
umask 077
printf '%s' "$DEEPSEEK_API_KEY" >"$key_file"
compose logs --no-color >"$log_file"
rm -f "$database_file"
compose cp ai-core:/var/lib/ai-core/ai-core.sqlite "$database_file"
if grep -Eq 'http://prometheus:9090|reasoning-marker|raw-series-marker' "$log_file" || strings "$database_file" | grep -Eq 'http://prometheus:9090|reasoning-marker|raw-series-marker'; then
	echo "real-agent logs included a prohibited marker" >&2
	exit 1
fi
if grep -F -f "$key_file" "$log_file" >/dev/null || strings "$database_file" | grep -F -f "$key_file" >/dev/null; then
	echo "real-agent output persisted the DeepSeek API key" >&2
	exit 1
fi
