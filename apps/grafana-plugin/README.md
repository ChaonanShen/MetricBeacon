# Grafana App Plugin

This module is the only browser-facing entrypoint.

- Frontend owns the workbench UI, generated Plugin Resource DTO consumption, SSE
  reconnection and Grafana DataFrame mapping.
- Backend owns Grafana request context extraction, resource API proxying, request/trace
  propagation and error mapping.
- Neither side owns Session/Task persistence, Agent workflow, MCP calls or fixtures.

The plugin ID is permanently `mini-torchbearing-app`.

## Build

```text
cd frontend && npm ci && npm run build
cd ../backend && go test ./...
```

Backend reads `aiCoreEndpoint` from the provisioned Grafana App `jsonData`; `AI_CORE_TIMEOUT` and `AI_CORE_MAX_RESPONSE_BYTES` remain optional process-level limits. It ignores browser-provided `X-MTB-*` identity headers and derives identity from Grafana context; its SSE proxy copies bytes and IDs without buffering or renumbering.
