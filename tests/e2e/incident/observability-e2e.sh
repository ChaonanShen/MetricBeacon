#!/bin/sh
set -eu

if [ "$#" -ne 4 ]; then
	echo "usage: $0 <project> <base-compose> <metrics-compose> <incident-compose>" >&2
	exit 2
fi

project=$1
base=$2
metrics=$3
incident=$4

compose() {
	docker compose -p "$project" -f "$base" -f "$metrics" -f "$incident" "$@"
}

grafana_api() {
	compose exec -T grafana curl -fsS -u "${GRAFANA_ADMIN_USER:-admin}:${GRAFANA_ADMIN_PASSWORD:-admin}" "http://127.0.0.1:3000$1"
}

wait_for_alert() {
	wanted=$1
	limit=$2
	index=0
	while [ "$index" -lt "$limit" ]; do
		alerts=$(grafana_api /api/prometheus/grafana/api/v1/alerts)
		case "$wanted:$alerts" in
			Alerting:*'"state":"Alerting"'*) return 0 ;;
			Normal:*'"state":"Alerting"'*|Normal:*'"state":"Pending"'*) ;;
			Normal:*) return 0 ;;
		esac
		index=$((index + 1))
		sleep 5
	done
	echo "alert did not reach $wanted" >&2
	return 1
}

fault_container=$(compose ps -q fault-controller)
test -n "$fault_container"
test "$(docker inspect -f '{{.HostConfig.NetworkMode}}' "$fault_container")" = none

business_ops_status=$(compose exec -T order-service curl -sS -o /dev/null -w '%{http_code}' http://127.0.0.1:8090/ops/v1/config/worker)
test "$business_ops_status" = 404

up=$(compose exec -T prometheus wget -qO- 'http://127.0.0.1:9090/api/v1/query?query=up%7Bjob%3D%22order-demo%22%7D')
echo "$up" | grep -q '"value":\[[^]]*,"1"\]'

grafana_api /api/v1/provisioning/alert-rules/order_queue_backlog | grep -q '"title":"OrderQueueBacklog"'
if compose logs --no-color grafana | grep -q 'Failed to provision alerting'; then
	echo "Grafana alert provisioning failed" >&2
	exit 1
fi

compose exec -T fault-controller curl -fsS -X POST http://127.0.0.1:8092/faults/v1/scenarios/worker-stopped/activate >/dev/null
wait_for_alert Alerting 18

backlog=$(compose exec -T prometheus wget -qO- 'http://127.0.0.1:9090/api/v1/query?query=mtb_demo_order_queue_depth')
echo "$backlog" | grep -Eq '"value":\[[^]]*,"([1-9][0-9]|[1-9][0-9][0-9])"\]'

compose exec -T fault-controller curl -fsS -X POST http://127.0.0.1:8092/faults/v1/reset >/dev/null
wait_for_alert Normal 18

queue=$(compose exec -T order-service curl -fsS -H 'Authorization: Bearer incident-read-development-token' http://127.0.0.1:8091/ops/v1/queue)
echo "$queue" | grep -q '"depth":0'

echo "incident observability E2E passed"

