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

deadline=$(( $(date +%s) + 90 ))
while :; do
	up=$(compose exec -T prometheus wget -qO- 'http://127.0.0.1:9090/api/v1/query?query=up%7Bjob%3D%22node-exporter%22%7D%20%3D%3D%201' 2>/dev/null || true)
	cpu=$(compose exec -T prometheus wget -qO- 'http://127.0.0.1:9090/api/v1/query?query=min%28count_over_time%28node_cpu_seconds_total%7Bjob%3D%22node-exporter%22%2Cmode%3D%22idle%22%7D%5B1m%5D%29%29%20%3E%3D%20bool%202' 2>/dev/null || true)
	if node -e 'const ready = (raw) => { const response = JSON.parse(raw); return response.status === "success" && response.data?.result?.some((item) => Number(item.value?.[1]) === 1); }; process.exit(ready(process.argv[1]) && ready(process.argv[2]) ? 0 : 1)' "$up" "$cpu" 2>/dev/null; then
		break
	fi
	if [ "$(date +%s)" -ge "$deadline" ]; then
		echo "Timed out waiting for node-exporter up=1 and two CPU idle scrapes" >&2
		exit 1
	fi
	sleep 1
done

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
