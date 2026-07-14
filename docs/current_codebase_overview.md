# 当前骨架代码说明

> 本文以 2026-07-14 的实际代码为准，说明已可工作的基本 Mock 闭环；长期设计请参照 `docs/code_skeleton_design.md`。

## 当前可演示的能力

用户在 Grafana App Plugin 中提交任意非空分析请求后，系统会创建持久化的 Session 和 Task，并通过 SSE 返回固定的 `node_exporter` CPU、内存可用率和系统负载三张时序图。数据与 Agent 计划都是确定性的 Mock，因此该闭环用于验证模块边界、协议、持久化和前端恢复能力，不代表自然语言理解或真实 Prometheus 查询已经实现。

```text
Grafana 浏览器前端
  -> Plugin Resource API
  -> Grafana Plugin Backend（认证上下文、受控代理）
  -> AI Core（Session/Task 工作流、SQLite、SSE）
  -> assistant-mcp（MCP transport、grafana.* 只读工具）
  -> Mock Prometheus Adapter -> node_exporter fixture

SSE TaskEvent 按原路径回传到前端，前端恢复状态并渲染三张图。
```

## 模块与职责

|位置|职责|当前实现边界|
|-|-|-|
|`contracts/`|跨进程协议的单一来源：Plugin Resource API、AI Core API、SSE 事件、MCP Tool Schema、错误码和示例。|通过 OpenAPI/JSON Schema 校验，并据此生成 Go/TypeScript 类型；业务模块不应另写重复 DTO。|
|`packages/generated-clients`、`packages/generated-contracts`|契约生成物。前者是 AI Core HTTP Client，后者是 Grafana MCP 工具类型。|由 `scripts/generate-clients.sh` 生成，`make generated-client-diff` 检查生成结果可复现。|
|`packages/request-context-go`|跨服务传递的租户、组织、用户、角色、权限、请求与 Trace 上下文。|Plugin Backend 从 Grafana 请求上下文生成它；浏览器传入的身份头不会被信任。|
|`packages/testkit-go`|测试用的确定性时钟与 ID 生成器。|仅为测试可重复性服务。|
|`services/ai-core/internal/domain`|核心领域模型和状态规则：Session/Message、AnalysisTask/TaskEvent/ToolCall、Chart/Execution、时间范围与领域错误。|不依赖数据库、MCP、Grafana 或模型 SDK。|
|`services/ai-core/internal/application` 与 `internal/ports`|命令服务和分析工作流；Port 定义存储、事件通知、Agent、工具、时钟和 ID 等外部能力。|工作流先持久化状态/事件，再通知 SSE；重启后将不能安全续跑的任务标记为失败。|
|`services/ai-core/internal/adapters`|将 Port 接到具体实现：SQLite、内存通知器、MCP 客户端、系统时钟/随机 ID 与确定性 Mock Agent。HTTP 入站 Adapter 暴露会话、任务、读取与 SSE 重放接口。|AI Core 是 Session、Task、Event、Chart 和 SQLite 数据的唯一所有者。它不直接读取 fixture，也不承载 Grafana 鉴权。|
|`services/ai-core/internal/bootstrap`|组装依赖：SQLite Store、Mock Agent、MCP Gateway、工作流、HTTP API。|`/readyz` 会同时检查 SQLite 及 MCP 工具是否就绪。|
|`services/assistant-mcp`|以 Streamable HTTP（`/mcp`）暴露只读的 `grafana.*` MCP 工具：`search_metrics`、`get_metric_labels`、`query_prometheus`。|工具先做权限和 Schema 校验，再调用 Prometheus Port；该服务不拥有 AI Core 的任务或数据库。|
|`services/assistant-mcp/internal/adapters/prometheus/mock`|Mock Prometheus Port 实现。|唯一允许读取 `data/mock-scenarios` 的代码；按请求的 PromQL 返回固定 fixture。真实 HTTP Adapter 目前显式 `not_implemented`。|
|`apps/grafana-plugin/frontend`|React/Grafana 工作台：创建 Session/Task、消费 Resource API、以连续 sequence 消费/重连 SSE、还原 URL、把执行结果映射为 Grafana DataFrame 与时序图。|当前页面只展示输入、状态、助手文本与三张图；未实现图表编辑和 Dashboard 写入。|
|`apps/grafana-plugin/backend`|Grafana Plugin SDK 的薄 Resource API 层。|从 Grafana 上下文提取身份、读取 `aiCoreEndpoint` 配置、代理请求与 SSE 字节流，并映射错误；不持久化业务数据、不调用 MCP。|
|`data/mock-scenarios/node_exporter_overview`|确定性场景数据：指标搜索、标签、三条查询结果、期望事件。|只供 MCP 的 Mock Prometheus Adapter 使用，并受 Schema 校验。|
|`scripts/`、`Makefile`、`tests/e2e/`|工程门禁、代码生成、契约/边界检查与端到端验收入口。|`compose.mock-e2e.yaml` 启动 assistant-mcp、AI Core 与 Grafana 三个容器。|

## 关键数据与依赖边界

- AI Core 独占业务持久化。SQLite 迁移在 `services/ai-core/migrations/sqlite/`，Plugin 和 MCP 都不能直接读写它。
- Mock 只位于 Adapter 层：Mock Agent 在 AI Core 的出站 Adapter，Mock Prometheus 在 MCP 的出站 Adapter；领域和工作流中没有 `mockMode` 分支。
- SSE 事件带有 Task、Session 和单调递增 sequence。事件先写入 durable store，客户端可通过 `afterSequence` 或 `Last-Event-ID` 获取断线后的后缀。
- 前端只能访问 Grafana Plugin Resource API；它不直连 AI Core、MCP 或 Prometheus。
- `scripts/check-boundaries.sh` 会阻止 AI Core 的 domain/application/ports import 外部 SDK、Adapter 或 Mock fixture。

## 尚未实现的范围

当前只覆盖一个固定 Mock 场景。以下是明确保留的后续能力，而非现有功能：真实 Agent/LLM、真实 Prometheus、PromQL 或图表编辑/重跑、Dashboard 写入与审批、真实 Grafana 写权限、知识库/Skill/Playbook、会话分享/Fork、告警和其他数据源。对应的部分 Port 或 Schema 已预留，但不能按“已实现”理解。

## 测试与检查入口

|命令|覆盖内容|本次结果（2026-07-14）|
|-|-|-|
|`make bootstrap-check`|固定 Go/Node/npm 版本；三个运行时 Go 模块全量编译测试；前端 typecheck；依赖边界。|通过。|
|`make test-ai-core-domain`|AI Core 领域、应用和 Port 的单元测试。|由 `make check` 通过。|
|`make test-sqlite`|SQLite Store 与内存事件通知器：CRUD、租户隔离、事务/幂等、sequence 与重放。|由 `make check` 通过。|
|`make test-assistant-mcp`|Mock Prometheus Adapter、MCP 接线和工具调用。|由 `make check` 通过。|
|`make test-ai-mcp`|AI Core MCP Gateway/查询 Adapter、HTTP API、分析工作流与 SSE。|由 `make check` 通过。|
|`make test-plugin-backend`|Grafana Resource API 代理、身份上下文、错误与 SSE 转发。|由 `make check` 通过。|
|`make test-frontend`|Vitest 工作台状态、SSE、路由、时间范围和 DataFrame mapper；随后 TypeScript typecheck。|通过：5 个测试文件、9 个用例。|
|`make test`|上述 Go 和前端测试的聚合入口。|由 `make check` 通过。|
|`make validate-contracts`|3 份 OpenAPI、21 份 JSON Schema 与 node_exporter fixture。|通过。|
|`make generated-client-diff`|重新生成 Client/类型后确认 Git 无差异。|通过。|
|`make lint`|Go 格式检查和前端 typecheck。|通过。|
|`make boundary-check`、`make secret-scan`|AI Core 依赖边界和常见私钥/AKIA 模式扫描。|通过。|
|`make check`|除容器 E2E 外的完整质量门禁：生成物、契约、lint、`make test`、边界与密钥扫描。|通过。|
|`make e2e-mock`|构建前端与三个容器；API E2E 校验幂等、事件 sequence 连续性、7 次工具调用、3 张图和 SSE 重放；Playwright 再验证浏览器提交和刷新恢复。|通过：容器健康后，API 脚本与 Playwright 用例均通过（1/1）。|

运行前端测试前若未安装依赖，先执行：

```sh
cd apps/grafana-plugin/frontend && npm ci
```

建议日常使用 `make check`；需要确认完整 Grafana 链路时再执行 `make e2e-mock`。完整 E2E 脚本会在结束时移除自己创建的 Compose 项目和 volume。
