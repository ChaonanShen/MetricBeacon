#!/bin/sh
set -eu

if [ "$#" -lt 3 ] || [ "$#" -gt 4 ]; then
	echo "usage: $0 <compose-project> <base-compose> <real-metrics-compose> [timeout-seconds]" >&2
	exit 2
fi

project=$1
base_compose=$2
real_compose=$3
timeout_seconds=${4:-90}
compose() { docker compose -p "$project" -f "$base_compose" -f "$real_compose" "$@"; }

deadline=$(( $(date +%s) + timeout_seconds ))
while :; do
	up=$(compose exec -T prometheus wget -qO- 'http://127.0.0.1:9090/api/v1/query?query=up%7Bjob%3D%22node-exporter%22%7D%20%3D%3D%201' 2>/dev/null || true)
	cpu=$(compose exec -T prometheus wget -qO- 'http://127.0.0.1:9090/api/v1/query?query=min%28count_over_time%28node_cpu_seconds_total%7Bjob%3D%22node-exporter%22%2Cmode%3D%22idle%22%7D%5B1m%5D%29%29%20%3E%3D%20bool%202' 2>/dev/null || true)
	if node -e 'const ready = (raw) => { const response = JSON.parse(raw); return response.status === "success" && response.data?.result?.some((item) => Number(item.value?.[1]) === 1); }; process.exit(ready(process.argv[1]) && ready(process.argv[2]) ? 0 : 1)' "$up" "$cpu" 2>/dev/null; then
		echo "[prometheus] node_exporter target and CPU scrape history are ready"
		break
	fi
	if [ "$(date +%s)" -ge "$deadline" ]; then
		echo "Timed out waiting for node-exporter up=1 and two CPU idle scrapes" >&2
		exit 1
	fi
	sleep 1
done
