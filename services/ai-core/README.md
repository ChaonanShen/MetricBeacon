# AI Core

AI Core is the only owner of analysis Sessions, Messages, Tasks, durable TaskEvents,
Charts and the SQLite database. It will expose the AI Core HTTP API, orchestrate the
deterministic Agent through Ports and persist state before notifying SSE consumers.

It does not own Grafana authentication, MCP server transport, Prometheus connectivity or
fixture loading. Domain and application packages may not import database, MCP, Grafana or
model SDKs; `scripts/check-boundaries.sh` enforces the initial rule.

G0 creates a buildable module only. Domain, Port and SQLite work starts in G2.
