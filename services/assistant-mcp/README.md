# assistant-mcp

This service will expose the `grafana.*` read-only MCP tools over Streamable HTTP. Its
namespace service calls a Prometheus Port; only the Mock Prometheus Adapter may load
deterministic scenario files.

It does not own AI Core tasks or SQLite, and Tool handlers must not bypass the namespace
service to read fixtures. G0 provides a buildable service shell; the real transport and
Mock Adapter arrive in G3.
