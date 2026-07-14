# 有界 node_exporter 查询参数进度记录

> status: active
> createdAt: 2026-07-14
> plan: [`bounded_node_exporter_query_parameters_execution_plan.md`](bounded_node_exporter_query_parameters_execution_plan.md)

## 执行状态

|阶段|状态|证据|
|-|-|-|
|G0：决策与计划落盘|完成|ADR-019、active execution plan、结构蓝图和文档路由已落盘；契约基线验证通过。|
|G1：契约先行|完成|CreateTask resolution、Task QueryPlan、Chart step、Execution actual range 和 MCP view/window 输入已定义并生成。|
|G2：QueryPlan 与持久化|完成|AI Core 已确定性解析并持久化时间范围、step 与 CPU rate window；SQLite `0004` 完成历史回填，Chart/Execution 记录有效 step 与实际样本范围。|
|G3：MCP 注册视图查询|完成|AI Core/MCP 查询边界已改为 view/window；注册表渲染并校验 CPU 30s/1m/5m PromQL，HTTP 使用动态 step，Mock 按范围/step 确定性重采样。|
|G4：极简 Agent 与可信回复|未开始|待缩减 Tool、使用 QueryPlan 和本地 formatter。|
|G5：Workbench 参数体验|未开始|待增加时间范围/resolution 控件和有效参数展示。|
|G6：端到端收口|未开始|待完成全量/E2E 验证和演进文档。|

## 当前边界

本切片保持 CPU、内存、load 三视图和 `prometheus-main` 不变。任何任意 PromQL、标签过滤、额外指标、
Dashboard 写入、Skill/Playbook 或模型读取原始 series 均不在本计划内。

## G0 验证证据

- `make validate-contracts`：两个 Plugin/AI Core OpenAPI、MCP OpenAPI、24 个 JSON Schema 与 Mock fixture 通过。
- `git diff --check`：通过。

## G1 验证证据

- `make generate`：Go/TypeScript AI Core、Plugin Resource 和 Grafana Tool 类型已重新生成。
- `make generated-client-diff`：生成物可复现。
- `make validate-contracts`：OpenAPI、24 个 JSON Schema、examples 与 fixture 通过。

## G2 验证证据

- `cd services/ai-core && go test ./...`：Domain、commands、SQLite migration/CRUD、HTTP API、Mock/Eino Adapter 与 workflow 全部通过。
- 五类有限自然语言解析已由 commands 测试覆盖：30 秒、1 分钟、30 分钟、5 分钟/每 5 秒和不应误识别的“三种”。
- SQLite migration 测试确认历史 Task 回填 `stepSeconds=300`、`cpuRateWindowSeconds=300`，历史 Chart query 回填 `stepSeconds=300`。
- Store 测试确认 `actualSampleRange` 可空并可持久化单点范围；SQLite 顶层写事务在进程内串行进入数据库，使活动 Task 唯一约束的并发结果稳定为一次成功、一次冲突。

## G3 验证证据

- `make test-assistant-mcp test-ai-mcp test-diagnostics`：assistant-mcp Port/namespace/Mock/HTTP/registry、AI Core MCP Gateway/workflow 和 35 个诊断测试全部通过。
- Registry 测试确认 CPU `30/60/300` 秒分别生成 `[30s]/[1m]/[5m]`，生成结果仍通过 PromQL AST/selector 上限；缺失 CPU window、非 CPU 携带 window 和未知 view 均在访问数据前拒绝。
- Mock Adapter 测试确认 30 秒范围、5 秒 step 精确返回 7 个首尾闭合的等间隔点；HTTP Adapter 测试确认将注册表生成的表达式和请求 step 传给 `query_range`。
- 诊断分析器接受三个注册 CPU window，但仍拒绝未知 Chart expression。
