#!/bin/sh
set -eu

project=${COMPOSE_PROJECT_NAME:-mini-torchbearing-real-metrics}
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

deadline=$(( $(date +%s) + 90 ))
while :; do
	up=$(compose exec -T prometheus wget -qO- --post-data='query=up%7Bjob%3D%22node-exporter%22%7D+%3D%3D+1' http://127.0.0.1:9090/api/v1/query 2>/dev/null || true)
	cpu=$(compose exec -T prometheus wget -qO- --post-data='query=min%28count_over_time%28node_cpu_seconds_total%7Bjob%3D%22node-exporter%22%2Cmode%3D%22idle%22%7D%5B1m%5D%29%29+%3E%3D+2' http://127.0.0.1:9090/api/v1/query 2>/dev/null || true)
	if node -e 'const ready = (raw) => { const response = JSON.parse(raw); return response.status === "success" && response.data?.result?.some((item) => Number(item.value?.[1]) === 1); }; process.exit(ready(process.argv[1]) && ready(process.argv[2]) ? 0 : 1)' "$up" "$cpu" 2>/dev/null; then
		break
	fi
	if [ "$(date +%s)" -ge "$deadline" ]; then
		echo "Timed out waiting for node-exporter up=1 and two CPU idle scrapes" >&2
		exit 1
	fi
	sleep 1
done

REAL_METRICS=1 "$root/tests/e2e/mock/api-e2e.sh"
cd "$root/apps/grafana-plugin/frontend"
npm run test:e2e
