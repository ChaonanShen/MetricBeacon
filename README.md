# mini-torchbearing

Grafana 内嵌的自然语言指标分析工作台。本仓库当前实现的唯一切片是确定性的
`node_exporter` Mock 闭环，范围和 Gate 以
[`docs/basic_mock_skeleton_execution_plan.md`](docs/basic_mock_skeleton_execution_plan.md) 为准。

## 当前状态

正在执行 G0（仓库脚手架）。还没有真实 Agent、Prometheus、Grafana Dashboard 写入或
业务 HTTP/MCP Handler；这些能力不得以临时代码绕过既定边界。

## 模块边界

- `apps/grafana-plugin`：Grafana App Plugin。Frontend 只调用 Plugin Resource API；
  Backend 只代理到 AI Core。
- `services/ai-core`：会话、任务、事件、工作流和 SQLite 的唯一所有者。
- `services/assistant-mcp`：MCP transport、`grafana.*` 只读工具与指标源 Adapter。
- `contracts`：跨进程 OpenAPI、JSON Schema、SSE 和 Tool Schema 的唯一来源。
- `data/mock-scenarios`：仅供 Mock Prometheus Adapter 读取的确定性 fixture。

## 开发入口

```text
make bootstrap-check
```

安装前端锁定依赖后再执行该命令：

```text
cd apps/grafana-plugin/frontend && npm ci
```

进度和每个 Gate 的验证证据保存在
[`docs/development/basic_mock_progress.md`](docs/development/basic_mock_progress.md)。
