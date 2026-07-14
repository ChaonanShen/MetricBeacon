#!/bin/sh
set -eu

if [ "$#" -ne 3 ]; then
	echo "usage: $0 <compose-project> <base-compose> <real-metrics-compose>" >&2
	exit 2
fi

project=$1
base_compose=$2
real_compose=$3
root=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
compose() { docker compose -p "$project" -f "$base_compose" -f "$real_compose" "$@"; }

probe() {
	view=$1
	expression=$2
	encoded=$(node -e 'process.stdout.write(encodeURIComponent(process.argv[1]))' "$expression")
	response=$(compose exec -T prometheus wget -qO- "http://127.0.0.1:9090/api/v1/query?query=$encoded")
	printf '%s' "$response" | node "$root/tests/diagnostics/prometheus-response.mjs" "$view"
}

probe cpu '100 * (1 - avg by (instance) (rate(node_cpu_seconds_total{mode="idle"}[5m])))'
probe memory '100 * node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes'
probe load 'node_load1'
