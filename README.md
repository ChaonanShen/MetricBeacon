# mini-torchbearing

Grafana 内嵌的自然语言指标分析工作台。本仓库当前实现的唯一切片是确定性的
`node_exporter` Mock 闭环，范围和 Gate 以
[`docs/basic_mock_skeleton_execution_plan.md`](docs/basic_mock_skeleton_execution_plan.md) 为准。

## 当前状态

基本 Mock 闭环已经实现：输入任意非空消息会产生固定的 node_exporter CPU、内存和负载三图。
仍未实现真实 Agent/LLM、真实 Prometheus 和 Grafana Dashboard 写入。

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
make test
make check
```

安装前端锁定依赖后再执行该命令：

```text
cd apps/grafana-plugin/frontend && npm ci
```

进度和每个 Gate 的验证证据保存在
[`docs/development/basic_mock_progress.md`](docs/development/basic_mock_progress.md)。

## 本地演示

分别启动 `assistant-mcp`（`:8081`）和 AI Core（`:8080`），或运行 `make e2e-mock` 构建 Compose；浏览器只通过 Grafana Plugin Resource API 访问系统。
