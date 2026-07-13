#!/bin/sh
set -eu
base=${GRAFANA_URL:-http://127.0.0.1:3000}
headers='-H Content-Type:application/json'
session=$(curl -fsS "$base/api/plugins/mini-torchbearing-app/resources/sessions" -X POST $headers --data '{"title":"Mock E2E"}')
session_id=$(node -e 'console.log(JSON.parse(process.argv[1]).id)' "$session")
task=$(curl -fsS "$base/api/plugins/mini-torchbearing-app/resources/tasks" -X POST $headers -H 'Idempotency-Key: mock-e2e-task' --data "{\"sessionId\":\"$session_id\",\"message\":\"show node exporter\",\"analysisContext\":{\"datasourceUid\":\"mock-prometheus\",\"timeRange\":{\"relativeDuration\":\"30m\"}}}")
task_id=$(node -e 'console.log(JSON.parse(process.argv[1]).id)' "$task")
events=$(curl -sS --max-time 3 "$base/api/plugins/mini-torchbearing-app/resources/tasks/$task_id/events?afterSequence=0" || true)
printf '%s' "$events" | grep -q 'assistant.message.completed'
printf '%s' "$events" | grep -q 'CPU 使用率'
printf '%s' "$events" | grep -q '内存可用率'
printf '%s' "$events" | grep -q '系统负载'
printf '%s' "$events" | grep -q 'chart.execution_completed'
