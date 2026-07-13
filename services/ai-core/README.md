# AI Core

AI Core is the only owner of analysis Sessions, Messages, Tasks, durable TaskEvents,
Charts and the SQLite database. It will expose the AI Core HTTP API, orchestrate the
deterministic Agent through Ports and persist state before notifying SSE consumers.

It does not own Grafana authentication, MCP server transport, Prometheus connectivity or
fixture loading. Domain and application packages may not import database, MCP, Grafana or
model SDKs; `scripts/check-boundaries.sh` enforces the initial rule.

## 运行

```text
AI_CORE_SQLITE_PATH=data/ai-core.sqlite ASSISTANT_MCP_ENDPOINT=http://127.0.0.1:8081/mcp go run ./cmd/server
```

`/readyz` 同时检查 SQLite 与三个 MCP 工具。SSE 可用 `afterSequence` 或 `Last-Event-ID` 重放；`Resume`、真实 Agent/模型仍返回结构化未实现错误。
