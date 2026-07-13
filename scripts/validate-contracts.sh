#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
frontend="$root/apps/grafana-plugin/frontend"

"$frontend/node_modules/.bin/redocly" lint --config "$root/redocly.yaml" "$root/contracts/openapi/plugin-resource.yaml"
"$frontend/node_modules/.bin/redocly" lint --config "$root/redocly.yaml" "$root/contracts/openapi/plugin-ai-core.yaml"
"$frontend/node_modules/.bin/redocly" lint --config "$root/redocly.yaml" "$root/contracts/tools/grafana/tools.openapi.yaml"
node "$frontend/scripts/validate-contracts.mjs" "$root"
