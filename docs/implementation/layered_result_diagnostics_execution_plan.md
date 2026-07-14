# 分层结果合理性诊断执行计划

> status: active
> createdAt: 2026-07-14
> implementationAuthorized: true
> dependsOn: `real_backend_diagnostics_execution_plan.md`、`current_codebase_overview.md`

## 1. 目标

现有诊断已经能证明 Prometheus、assistant-mcp 和 DeepSeek 可连通且返回非空结果。本切片继续把“非空”细化为
code agent 可直接执行和解释的分层门禁：既检查 wire shape，也检查 CPU/内存/负载数值、时间戳、标签、
TaskEvent sequence、工具配对、Chart/Execution 关联和终态是否合理，并在安全摘要中输出计数与 min/max/latest。

## 2. 分层测试矩阵

|层级|入口|主要判断|
|-|-|-|
|L0 契约/解析|`make validate-contracts`、诊断单测|JSON、vector/matrix、字段和有限错误分类。|
|L1 指标语义|诊断单测、原始 Prometheus 探针|非空、有限数值、时间戳递增、instance 标签；CPU/内存在 0..100，load 非负。|
|L2 MCP Adapter|真实 assistant-mcp 探针|搜索/标签/三条规范查询，以及每个 view 的 series/samples/min/max/latest。|
|L3 Task/Event|Mock API E2E、分析器单测|连续 sequence、单一终态、工具 start/end 配对、Chart/Execution 一一对应、三视图语义合理。|
|L4 Agent|DeepSeek 直连、real-agent E2E|配置模型可用、严格最小回复；受限视图选择、真实 series、工具配对、持久化与泄漏检查。|
|L5 浏览器|Mock/real-metrics Playwright|恢复、提交、图表数量/布局和 stale Session 恢复；不重复承担后端数值真相。|

## 3. 执行切片

### G0：激活计划

提交：`docs: activate layered result diagnostics plan`

- 新增本计划和进度记录，并更新 `docs/CLAUDE.md`。
- 不修改契约、业务代码或持久化。

验证：`make validate-contracts`、`git diff --check`。

### P1：指标格式与语义分析

提交：`test(diagnostics): validate metric result semantics`

- 新增可复用的 Node 指标分析器及边界单测。
- 原始 Prometheus vector 除 shape 外，按视图验证合理区间并输出 min/max/latest。
- assistant-mcp live smoke 解码真实 point，验证标签、时间顺序、数值范围并输出安全统计摘要。
- 不固定真实机器的具体数值，不输出完整 series。

验证：`make test-diagnostics`、`make test-assistant-mcp`、`make diagnose-real-metrics`。

### P2：Task/Event/Chart 结果分析

提交：`test(e2e): analyze durable task results`

- 新增 TaskEvent 分析器及成功/失败单测。
- Mock API E2E 使用分析器检查 7 对工具事件、三图、Execution/series 和最终 Assistant Message。
- real-agent E2E 使用同一分析器检查动态工具调用、概览三图和 CPU 单图。
- 失败消息只输出事件类型计数和统计摘要，不打印完整时序或凭证。

验证：`make test-diagnostics`、`make e2e-mock`、`make e2e-real-metrics`、有凭证 `make e2e-real-agent`。

### P3：测试文档与收口

提交：`docs: publish layered backend test matrix`

- 新增面向 code agent 的测试矩阵、运行顺序、预期输出形式和故障定位表。
- 更新 README、当前代码概览/树、计划和进度状态。
- 记录实际验证统计，不把本机瞬时数值写成稳定契约。

验证：`make check`、`git diff --check`。

## 4. 边界

- 这是测试与诊断切片，不改变 OpenAPI、JSON Schema、MCP Tool Schema、SQLite 或运行时权限。
- 0..100 是当前规范 CPU 使用率和内存可用率表达式的语义约束；load 只要求有限且非负，不假设 CPU 数或固定阈值。
- 时间戳必须可解析且在单条 series 内严格递增；series 间不要求完全对齐。
- code agent 输出只能包含 view、计数、min/max/latest、事件类型计数和通过/失败阶段，不输出 key、内部 URL、模型 reasoning 或完整 raw series。

该切片不改变架构决策，不需要 ADR。
