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

wait_for_db_value() {
	query=$1
	wanted=$2
	limit=$3
	index=0
	while [ "$index" -lt "$limit" ]; do
		rm -rf "$database_dir"
		mkdir -p "$database_dir"
		docker cp "$ai_core_container:/var/lib/ai-core/." "$database_dir" >/dev/null
		value=$(sqlite3 "$database_dir/ai-core.sqlite" "$query")
		if [ "$value" = "$wanted" ]; then
			return 0
		fi
		index=$((index + 1))
		sleep 2
	done
	echo "database query did not reach $wanted: $query" >&2
	return 1
}

command -v sqlite3 >/dev/null
temporary_dir=$(mktemp -d)
database_dir=$temporary_dir/ai-core
trap 'rm -rf "$temporary_dir"' EXIT
ai_core_container=$(compose ps -q ai-core)
test -n "$ai_core_container"

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

node "$(dirname "$0")/golden-e2e.mjs"
wait_for_alert Normal 18

queue=$(compose exec -T order-service curl -fsS -H 'Authorization: Bearer incident-read-development-token' http://127.0.0.1:8091/ops/v1/queue)
echo "$queue" | grep -q '"depth":0'

wait_for_db_value "SELECT count(*) FROM alert_events WHERE status='resolved';" 1 20
wait_for_db_value "SELECT count(*) FROM approvals WHERE status='approved' AND version=2;" 1 1
wait_for_db_value "SELECT count(*) FROM remediation_executions WHERE state IN ('applied','already_applied') AND before_version + 1 = after_version;" 1 1
wait_for_db_value "SELECT count(*) FROM audit_records WHERE action='approval_decision' AND outcome='accepted';" 1 1
wait_for_db_value "SELECT count(*) FROM audit_records WHERE action='remediation_execute' AND outcome='accepted';" 1 1
wait_for_db_value "SELECT count(*) FROM audit_records WHERE action='remediation_execute' AND outcome='succeeded';" 1 1
wait_for_db_value "SELECT count(*) FROM audit_records WHERE action='remediation_verify' AND outcome='succeeded';" 1 1
test "$(sqlite3 "$database_dir/ai-core.sqlite" 'PRAGMA foreign_key_check;')" = ""

operation_id=$(sqlite3 "$database_dir/ai-core.sqlite" 'SELECT operation_id FROM remediation_executions LIMIT 1;')
test -n "$operation_id"
receipt=$(compose exec -T order-service curl -fsS -H 'Authorization: Bearer incident-read-development-token' "http://127.0.0.1:8091/ops/v1/operations/$operation_id")
echo "$receipt" | grep -q '"beforeConcurrency":0'
echo "$receipt" | grep -q '"afterConcurrency":2'

assistant_mcp_container=$(compose ps -q assistant-mcp)
docker cp "$assistant_mcp_container:/var/lib/assistant-mcp/execution-audit.jsonl" "$temporary_dir/execution-audit.jsonl" >/dev/null
test "$(wc -l < "$temporary_dir/execution-audit.jsonl" | tr -d ' ')" = 2
grep -q '"phase":"execute"' "$temporary_dir/execution-audit.jsonl"
grep -q '"outcome":"authorized"' "$temporary_dir/execution-audit.jsonl"
grep -q '"outcome":"succeeded"' "$temporary_dir/execution-audit.jsonl"
grep -q "\"operationId\":\"$operation_id\"" "$temporary_dir/execution-audit.jsonl"

if compose logs --no-color | grep -Eq 'incident-remediation-development-token|incident-approval-evidence-development-key-v1'; then
	echo "incident logs leaked a remediation credential" >&2
	exit 1
fi

echo "incident golden observability, remediation, persistence and audit E2E passed"
