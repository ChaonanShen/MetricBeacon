# Grafana App Plugin

This module is the only browser-facing entrypoint.

- Frontend owns the workbench UI, generated Plugin Resource DTO consumption, SSE
  reconnection and Grafana DataFrame mapping.
- Backend owns Grafana request context extraction, resource API proxying, request/trace
  propagation and error mapping.
- Neither side owns Session/Task persistence, Agent workflow, MCP calls or fixtures.

The plugin ID is permanently `mini-torchbearing-app`. The G0 frontend only type-checks;
the Resource API and Grafana component dependencies are added after G1 contracts freeze.
