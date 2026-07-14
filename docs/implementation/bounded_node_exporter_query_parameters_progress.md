# 有界 node_exporter 查询参数进度记录

> status: completed
> createdAt: 2026-07-14
> plan: [`bounded_node_exporter_query_parameters_execution_plan.md`](bounded_node_exporter_query_parameters_execution_plan.md)

## 执行状态

|阶段|状态|证据|
|-|-|-|
|G0：决策与计划落盘|完成|ADR-019、execution plan、结构蓝图和文档路由已落盘；契约基线验证通过。|
|G1：契约先行|完成|CreateTask resolution、Task QueryPlan、Chart step、Execution actual range 和 MCP view/window 输入已定义并生成。|
|G2：QueryPlan 与持久化|完成|AI Core 已确定性解析并持久化时间范围、step 与 CPU rate window；SQLite `0004` 完成历史回填，Chart/Execution 记录有效 step 与实际样本范围。|
|G3：MCP 注册视图查询|完成|AI Core/MCP 查询边界已改为 view/window；注册表渲染并校验 CPU 30s/1m/5m PromQL，HTTP 使用动态 step，Mock 按范围/step 确定性重采样。|
|G4：极简 Agent 与可信回复|完成|Eino query Tool 仅接受 view；成功 proposal 是权威视图选择且模型终态文本被忽略，零 proposal 的 unsupported 路径严格校验；Mock/Eino 共用本地 formatter。|
|G5：Workbench 参数体验|完成|左栏可选择 30s..6h 默认范围和 auto/注册 step，并提交结构化 analysisContext；右栏展示 Task/Chart 有效参数与 actual sample range。|
|G6：端到端收口|完成|五类语句已通过 Mock 精确验收、真实指标语义验收和浏览器恢复/布局验收；有凭证的真实 Agent view-only/local-answer smoke 通过，文档完成收口。|

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

## G4 验证证据

- `make test-ai-agent test-ai-core-domain test-ai-mcp`：Eino/Mock/Profile/local formatter、Bootstrap、Domain/Application 和 MCP Gateway/workflow 全部通过。
- Eino 测试确认模型 Tool 携带 `expression` 时在 QueryEngine 前拒绝；有成功 proposal 时模型终态文本会被忽略而非持久化，零 proposal 的 unsupported 终态仍严格校验。
- 本地 formatter 测试确认回答包含有效范围、step、CPU window、series/sample 数、first/latest/min/max/mean/delta 和实际数据范围，同时不包含 label value。
- 模型 ToolSummary 只包含有效 QueryPlan 与聚合统计/实际范围，不包含逐点 timestamp、raw points、真实 label、内部 URL、身份或上游 warning 文本。

## G5 验证证据

- `cd apps/grafana-plugin/frontend && npm run typecheck`：通过。
- `cd apps/grafana-plugin/frontend && npm test -- --run`：8 个测试文件、20 个用例通过。
- `query-options` 测试确认 auto 与显式 5 秒均生成契约规定的结构化 `analysisContext`；UI 不复制服务端范围、点数预算或自然语言解析规则。
- ContextPane 展示 Task `queryPlan.stepSeconds/cpuRateWindowSeconds`、Chart step 和 Execution `actualSampleRange`；没有实际样本时明确显示无可用样本。

## G6 验证证据

- `make check`：生成物可复现、3 份 OpenAPI、24 份 JSON Schema/fixture、Go/前端/35 个诊断测试、lint、边界与 secret scan 全部通过。
- `make e2e-mock`：五条验收语句精确得到 `30s/5/30`、`1m/5/30`、`30m/10/60`、`5m/5/30`、默认 `30m/10/60`；每条 series 的点数、5/10 秒间隔、首尾范围、Chart/Execution/actual range 和本地回复均一致。Playwright 控件、三栏/窄屏布局、连续任务、刷新 replay 与 stale Session 恢复通过。
- `make e2e-real-metrics`：相同五条语句均返回非空、有限、按有效 step 间隔的真实 CPU/内存/load series；不固定瞬时值或理论点数，实际短历史由 `actualSampleRange` 如实记录。Prometheus 浮点秒边界只在测试中允许 1 秒序列化容差；Mock 仍要求严格首尾相等。浏览器验收通过。
- 加载本地 `.env` 后执行 `make e2e-real-agent`：DeepSeek 概览只发起 3 个 view 查询并生成三图，CPU 追问只发起 1 个 view 查询；默认 30 分钟计划为 step 10 秒/CPU window 60 秒，本地回复、durable tool 配对、replay 和 API/日志/SQLite 泄漏检查通过，key 未输出。
- 三个 E2E 脚本退出后均删除其 Compose 容器、network 与 AI Core volume。
