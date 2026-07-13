# assistant-mcp

This service will expose the `grafana.*` read-only MCP tools over Streamable HTTP. Its
namespace service calls a Prometheus Port; only the Mock Prometheus Adapter may load
deterministic scenario files.

It does not own AI Core tasks or SQLite, and Tool handlers must not bypass the namespace
service to read fixtures.

## 运行

```text
go run ./cmd/server
```

MCP 为 `/mcp`，健康端点为 `/healthz`、`/readyz`。可配置 `ASSISTANT_MCP_LISTEN_ADDRESS`、`ASSISTANT_MCP_FIXTURE_DIR`、`ASSISTANT_MCP_SCHEMA_DIR`；只注册三个只读 `grafana.*` 工具。真实 Prometheus HTTP Adapter 明确返回 `not_implemented`。
