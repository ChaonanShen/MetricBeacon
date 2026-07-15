# assistant-mcp

This service will expose the `grafana.*` read-only MCP tools over Streamable HTTP. Its
namespace service calls a Prometheus Port; only the Mock Prometheus Adapter may load
deterministic scenario files.

The incident slice also defines a typed, read-only order-demo Port with deterministic
Mock scenarios and an HTTP Adapter generated from the Operational OpenAPI. It is
registered only by the opt-in Incident profile. The Port has no fault or write
method; the HTTP Adapter uses a deployment read token and enforces time, body, cardinality,
redirect, and response-semantic limits.

`internal/adapters/assets/filesystem` loads deployment-provided operational assets
read-only. Startup validates their JSON Schemas and cross-references, restricts Playbook
capabilities to the referenced Skill and the compiled read registry, and pins raw file
bytes with SHA-256. Alert resolution requires exactly one source/name/required-label
match; zero or multiple matches fail closed.

Set `ASSISTANT_MCP_INCIDENT_ENABLED=true` to opt into the Incident namespaces. The
profile requires asset/tool schema directories, a checkpoint key of at least 32 bytes,
and either the bounded Mock order driver or the HTTP driver with
`ASSISTANT_MCP_ORDER_URL` plus `ASSISTANT_MCP_ORDER_READ_TOKEN`. The default profile
still exposes exactly the original three `grafana.*` tools. Incident mode adds eleven
closed-world read-only tools; none exposes Fault Injection, shell, arbitrary HTTP, or a
write/execute operation.

It does not own AI Core tasks or SQLite, and Tool handlers must not bypass the namespace
service to read fixtures.

## 运行

```text
go run ./cmd/server
```

MCP 为 `/mcp`，健康端点为 `/healthz`、`/readyz`；只注册三个只读 `grafana.*` 工具。`/healthz` 只表示进程可响应；使用 HTTP driver 时 `/readyz` 会以短超时探测 Prometheus `/-/ready`。

默认 `ASSISTANT_MCP_PROMETHEUS_DRIVER=mock`，读取 fixture。将 driver 设为 `http` 时，Adapter 只使用环境中的 `ASSISTANT_MCP_PROMETHEUS_URL`（默认 `http://prometheus:9090`）和 `ASSISTANT_MCP_PROMETHEUS_TIMEOUT`（默认 `10s`）；逻辑数据源 UID 固定为 `ASSISTANT_MCP_PROMETHEUS_DATASOURCE_UID=prometheus-main`。真实模式仍只允许 CPU、内存可用率和系统负载的注册表查询，不能通过 MCP 请求指定 endpoint 或任意 PromQL。
