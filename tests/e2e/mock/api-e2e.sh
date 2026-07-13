#!/bin/sh
set -eu
base=${GRAFANA_URL:-http://127.0.0.1:3000}
grafana_user=${GRAFANA_ADMIN_USER:-admin}
grafana_password=${GRAFANA_ADMIN_PASSWORD:-admin}
curl -fsS -u "$grafana_user:$grafana_password" "$base/api/plugins/mini-torchbearing-app/settings" >/dev/null
session=$(curl -fsS -u "$grafana_user:$grafana_password" "$base/api/plugins/mini-torchbearing-app/resources/sessions" -X POST -H 'Content-Type: application/json' --data '{"title":"Mock E2E"}')
session_id=$(node -e 'console.log(JSON.parse(process.argv[1]).id)' "$session")
task=$(curl -fsS -u "$grafana_user:$grafana_password" "$base/api/plugins/mini-torchbearing-app/resources/tasks" -X POST -H 'Content-Type: application/json' -H 'Idempotency-Key: mock-e2e-task' --data "{\"sessionId\":\"$session_id\",\"message\":\"show node exporter\",\"analysisContext\":{\"datasourceUid\":\"mock-prometheus\",\"timeRange\":{\"relativeDuration\":\"30m\"}}}")
task_id=$(node -e 'console.log(JSON.parse(process.argv[1]).id)' "$task")
events=$(curl -sS -u "$grafana_user:$grafana_password" --max-time 5 "$base/api/plugins/mini-torchbearing-app/resources/tasks/$task_id/events?afterSequence=0" || true)
printf '%s' "$events" | grep -q 'assistant.message.completed'
printf '%s' "$events" | grep -q 'CPU 使用率'
printf '%s' "$events" | grep -q '内存可用率'
printf '%s' "$events" | grep -q '系统负载'
printf '%s' "$events" | grep -q 'chart.execution_completed'
