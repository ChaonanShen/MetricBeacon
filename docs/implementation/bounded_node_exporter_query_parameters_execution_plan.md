# 有界 node_exporter 查询参数执行计划

> status: completed
> createdAt: 2026-07-14
> implementationAuthorized: true
> decision: [`../adr/ADR-019-bounded-node-exporter-query-parameters.md`](../adr/ADR-019-bounded-node-exporter-query-parameters.md)
> dependsOn: `node_exporter_real_analysis_execution_plan.md`、`current_codebase_overview.md`

## 1. 目标

保持真实 Agent 简单且可控：模型只选择 `cpu`、`memory`、`load` 视图；时间范围、step、CPU rate 窗口、
规范 PromQL 和最终事实性回复全部由本地确定性代码产生。用户可通过结构化 Workbench 控件或有限自然语言表达
查询最近 30 秒至 6 小时的数据，并得到与实际执行参数一致、具有足够采样密度的时序图。

完成后的链路为：

```text
用户消息 + analysisContext 默认值
  -> application-local QueryIntentResolver
  -> durable Task.timeRange + Task.queryPlan
  -> Mock/Eino Agent 只选择 view keys
  -> assistant-mcp 注册表渲染 canonical PromQL
  -> Prometheus query_range(start, end, step)
  -> local accumulator + deterministic Assistant answer
  -> durable Chart/Execution/Event -> Grafana Workbench
```

## 2. 范围与非目标

本切片包含：

- 相对/绝对时间范围、`auto`/显式 step、CPU 自动 rate 窗口的契约、解析、持久化和展示。
- CPU `[30s]`、`[1m]`、`[5m]` 三个注册变体；内存和 load 表达式保持不变。
- Agent Tool 输入从 `{view, expression}` 缩减为 `{view}`。
- MCP 查询输入改为注册视图参数，由 assistant-mcp 返回规范 expression。
- Mock/Real Adapter 一致性、可信本地摘要和五条真实用户语句的回归测试。

本切片不包含：任意 PromQL、额外 node_exporter 指标、instance/label 过滤、分组维度、跨数据源、图表编辑/重跑、
Dashboard 写入、Skill/Playbook、并行工具或模型读取原始时间序列。

## 3. 决策完成的参数模型

### 3.1 请求参数

`CreateTask.analysisContext` 保留 `datasourceUid` 和 `timeRange`，新增可选 `resolution`：

```json
{"resolution":{"mode":"auto"}}
```

或：

```json
{"resolution":{"stepSeconds":5}}
```

允许的显式 step 为 `5|10|15|30|60|120|300`。省略时等价于 `auto`。

### 3.2 有效 QueryPlan

Task 新增必需字段：

```json
{"queryPlan":{"stepSeconds":5,"cpuRateWindowSeconds":30}}
```

- 时间范围最小 30 秒、最大 6 小时。
- auto step 从 `[5,10,15,30,60,120,300]` 选择第一个使
  `floor(duration/step)+1 <= 300` 的值。
- 显式 step 若产生超过 1,000 个理论点，返回 `invalid_argument`，不静默改写。
- CPU auto window：范围不超过 10 分钟用 30 秒；不超过 1 小时用 60 秒；其余用 300 秒。
- 历史 Task migration 回填 `300/300`，保持旧查询语义。

Chart Query 增加必需 `stepSeconds`。Execution 保留 `sampleRange` 表示发给 Prometheus 的查询范围，新增可空
`actualSampleRange` 表示返回点的最早/最晚时间。

### 3.3 有限自然语言解析

解析器位于 AI Core Application，使用标准库和注入 Clock，不依赖 Eino/模型：

- 时间：`近|最近|过去 + 数字 + 秒|分钟|小时`。
- resolution：`每 + 数字 + 秒|分钟 + 一个点|画一个点`。
- 数字支持阿拉伯数字和常用中文整数（至少覆盖一、五、十、三十、六十）。
- 绝对 `from/to` 始终优先；否则当前消息中的明确相对时间覆盖请求中的相对默认值。
- 当前消息中的明确 resolution 覆盖 `analysisContext.resolution`；未识别则使用请求值或 auto。
- 单独出现“三种”“五分钟平均”等不满足完整受支持结构时不得误解析；失败时使用结构化值，不猜测。

解析与相对时间冻结在 Task 事务前完成；idempotency hash 继续绑定原始 caller intent，同一 key 重试返回同一 Task。

### 3.4 MCP 与注册表

`grafana.query_prometheus` 输入从 `expression` 改为：

```json
{
  "datasourceUid":"prometheus-main",
  "view":"cpu",
  "cpuRateWindowSeconds":30,
  "start":"...",
  "end":"...",
  "stepSeconds":5,
  "mode":"execute"
}
```

`cpuRateWindowSeconds` 必需但可为 `null`：CPU 必须是 `30|60|300`，memory/load 必须为 `null`。assistant-mcp
注册表渲染并返回 `validation.canonicalExpression`；AI Core 不再向 Tool 提交 expression。Adapter 仍执行 PromQL AST、
范围、step、series、sample 和响应大小上限。

### 3.5 Agent 与回复

- Eino `query_prometheus` Tool schema 只含 `view`；datasource、范围、step/window 由 run closure 注入。
- Mock Agent 使用同一个 Task QueryPlan，继续固定三视图，保证默认回归稳定。
- 模型可以看到 Profile、历史消息、当前消息和有界统计摘要，但不能改变 QueryPlan。
- 最终 `AssistantText` 由本地 formatter 根据成功 proposals、有效 QueryPlan、actual range、series/sample 数、
  first/latest/min/max/mean/delta 生成；模型自由文本不进入持久化事实回复。
- ToolSummary 不包含逐点 timestamps、raw points、真实 label values、URL、身份、secret 或 reasoning。

## 4. 分提交执行顺序

### G0：决策与计划落盘

提交：`docs: plan bounded node exporter query parameters`

- 新增 ADR-019、本计划和进度记录。
- 更新 ADR/文档路由和结构蓝图。
- 验证：`make validate-contracts`、`git diff --check`。

### G1：契约先行

提交：`feat(contracts): define bounded query plans`

- 更新 CreateTask、Task、Chart、Execution、MCP Tool Schema、examples 和 OpenAPI 引用。
- 生成 Go/TypeScript 类型并验证生成差异。
- 验证：`make generate generated-client-diff validate-contracts`。

### G2：QueryPlan 与持久化

提交：`feat(ai-core): persist resolved query plans`

- 增加 QueryPlan Domain 值对象、有限解析器和 CreateTask 解析。
- SQLite forward migration、历史回填、repository/API/event wire 更新。
- Workflow 将有效 step 写入 Chart，并计算 Execution actual range。
- 验证：Domain、commands、SQLite、HTTP 和 workflow 定向测试。

### G3：MCP 注册视图查询

提交：`feat(mcp): execute bounded node exporter views`

- MCP namespace/Port/Adapter 改用 view/window 输入。
- 注册表渲染三个 CPU window 变体并继续 AST allowlist。
- HTTP Adapter 传递动态 step；Mock Adapter 在范围内确定性重采样。
- 更新 fixture、契约测试、诊断脚本及 Mock/Real 一致性覆盖。
- 验证：`make test-assistant-mcp test-ai-mcp test-diagnostics`。

### G4：极简 Agent 与可信回复

提交：`refactor(agent): constrain model to view selection`

- Mock/Eino Runtime 使用 Task QueryPlan；Eino Tool 仅接受 view。
- Profile 移除让模型提交表达式的要求，保留三视图解释和禁止项。
- 增加安全统计与本地事实 formatter，删除 plain-text final 持久化 fallback。
- 验证：`make test-ai-agent test-ai-core-domain test-ai-mcp`。

### G5：Workbench 参数体验

提交：`feat(frontend): control node exporter query resolution`

- 顶部/上下文区增加时间范围和 auto/显式 step 控件。
- 提交结构化 analysisContext；Task/Chart 详情显示有效 step/window 和 actual range。
- 不在前端复制权威解析或安全限制。
- 验证：`make test-frontend`、Playwright 定向测试。

### G6：端到端收口

提交：`test(e2e): verify bounded node exporter queries`

- 覆盖 30s、1m、30m、5m/5s 和三视图请求。
- Mock 精确检查有效范围、step/window、point 间隔和可信回复；Real 只检查非空、范围、间隔、有限值和实际
  data range，不固定瞬时数值。
- 更新 README、runbook、current overview/tree、本计划/进度和文档路由。
- 验证：`make check`、`make e2e-mock`、`make e2e-real-metrics`；有凭证时 `make e2e-real-agent`。

## 5. 验收标准

- “查看近30s里 node exporter 中 CPU 数据”执行 30 秒范围、step 5 秒、CPU window 30 秒。
- “查看近一分钟 CPU 变化图”执行 1 分钟范围、step 5 秒、CPU window 30 秒。
- “查看近30分钟 CPU 变化数据”执行 30 分钟范围、auto step 10 秒、CPU window 60 秒。
- “最近五分钟 CPU，每5s一个点”执行 5 分钟范围、step 5 秒、CPU window 30 秒。
- “画出三种 node exporter 监测数据”仍生成三图，并使用默认 30 分钟/auto step。
- 持久化 Task、Chart、Execution、SSE replay、刷新恢复与 Assistant 文本对有效参数一致。
- Agent/MCP/API/日志/SQLite 不泄漏 key、内部 URL、身份、raw points 或 reasoning。
- 任意 expression、额外 view/window、越界范围/step 和超过点数预算的请求在访问 Prometheus 前失败。

## 6. 完成结果

本计划于 2026-07-14 完成。最终实现保持模型只通过成功的 `query_prometheus {view}` 调用选择视图；当至少一个
查询成功时，模型终态内容不参与结果判定或持久化，AI Core 直接按成功 proposals 和本地统计生成回复。零 proposal
的 unsupported 路径仍要求严格 JSON。完整 Gate 与三种 Compose 模式的验证证据见
[`bounded_node_exporter_query_parameters_progress.md`](bounded_node_exporter_query_parameters_progress.md)。
