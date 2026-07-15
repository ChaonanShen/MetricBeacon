#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
frontend="$root/apps/grafana-plugin/frontend"
generated_openapi="$root/build/generated/openapi"

mkdir -p "$generated_openapi" \
  "$root/packages/generated-clients/go" \
  "$root/packages/generated-clients/typescript" \
  "$root/packages/generated-contracts/go" \
  "$root/packages/generated-contracts/typescript" \
  "$root/services/ai-core/internal/adapters/inbound/http/generated" \
  "$root/packages/generated-clients/go/orderdemo" \
  "$root/services/order-demo/internal/adapters/inbound/http/generated" \
  "$root/services/order-demo/internal/adapters/inbound/http/faultgenerated" \
  "$frontend/src/api/generated"

"$frontend/node_modules/.bin/redocly" bundle --config "$root/redocly.yaml" \
  "$root/contracts/openapi/plugin-ai-core.yaml" \
  -o "$generated_openapi/plugin-ai-core.codegen.source.yaml"
node "$frontend/scripts/project-oas31-to-oas30.mjs" \
  "$generated_openapi/plugin-ai-core.codegen.source.yaml" \
  "$generated_openapi/plugin-ai-core.codegen.yaml"
"$frontend/node_modules/.bin/redocly" bundle --config "$root/redocly.yaml" \
  "$root/contracts/tools/grafana/tools.openapi.yaml" \
  -o "$generated_openapi/grafana-tools.codegen.source.yaml"
node "$frontend/scripts/project-oas31-to-oas30.mjs" \
  "$generated_openapi/grafana-tools.codegen.source.yaml" \
  "$generated_openapi/grafana-tools.codegen.yaml"
"$frontend/node_modules/.bin/redocly" bundle --config "$root/redocly.yaml" \
  "$root/contracts/tools/incident/tools.openapi.yaml" \
  -o "$generated_openapi/incident-tools.codegen.source.yaml"
node "$frontend/scripts/project-oas31-to-oas30.mjs" \
  "$generated_openapi/incident-tools.codegen.source.yaml" \
  "$generated_openapi/incident-tools.codegen.yaml"
for name in order-demo order-demo-fault
do
  "$frontend/node_modules/.bin/redocly" bundle --config "$root/redocly.yaml" \
    "$root/contracts/openapi/$name.yaml" \
    -o "$generated_openapi/$name.codegen.source.yaml"
  node "$frontend/scripts/project-oas31-to-oas30.mjs" \
    "$generated_openapi/$name.codegen.source.yaml" \
    "$generated_openapi/$name.codegen.yaml"
done

"$frontend/node_modules/.bin/openapi-typescript" "$root/contracts/openapi/plugin-resource.yaml" \
  -o "$frontend/src/api/generated/plugin-resource.ts"
"$frontend/node_modules/.bin/openapi-typescript" "$root/contracts/openapi/plugin-ai-core.yaml" \
  -o "$root/packages/generated-clients/typescript/plugin-ai-core.ts"
"$frontend/node_modules/.bin/openapi-typescript" "$root/contracts/tools/grafana/tools.openapi.yaml" \
  -o "$root/packages/generated-contracts/typescript/grafana-tools.ts"
"$frontend/node_modules/.bin/openapi-typescript" "$root/contracts/tools/incident/tools.openapi.yaml" \
  -o "$root/packages/generated-contracts/typescript/incident-tools.ts"

go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.7.2 \
  -generate types,client -package aicore \
  -o "$root/packages/generated-clients/go/aicore.gen.go" \
  "$generated_openapi/plugin-ai-core.codegen.yaml"
go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.7.2 \
  -generate types -package grafanatools \
  -o "$root/packages/generated-contracts/go/grafana_tools.gen.go" \
  "$generated_openapi/grafana-tools.codegen.yaml"
go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.7.2 \
  -generate types -package grafanatools \
  -o "$root/packages/generated-contracts/go/incident_tools.gen.go" \
  "$generated_openapi/incident-tools.codegen.yaml"
go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.7.2 \
  -generate types,client -package orderdemo \
  -o "$root/packages/generated-clients/go/orderdemo/order_demo.gen.go" \
  "$generated_openapi/order-demo.codegen.yaml"
(cd "$root/services/order-demo" && \
  go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.7.2 \
    -generate types,std-http -package generated \
    -o internal/adapters/inbound/http/generated/order_api.gen.go \
    "$generated_openapi/order-demo.codegen.yaml")
(cd "$root/services/order-demo" && \
  go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.7.2 \
    -generate types,std-http -package faultgenerated \
    -o internal/adapters/inbound/http/faultgenerated/fault_api.gen.go \
    "$generated_openapi/order-demo-fault.codegen.yaml")
(cd "$root/services/ai-core" && \
  go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.7.2 \
    -generate types,std-http -package httpapi \
    -o internal/adapters/inbound/http/generated/server.gen.go \
    "$generated_openapi/plugin-ai-core.codegen.yaml")
