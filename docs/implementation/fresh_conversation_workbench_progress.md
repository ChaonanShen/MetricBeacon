# 新建对话与对话栏微调进度记录

> status: completed
> createdAt: 2026-07-15
> completedAt: 2026-07-15
> plan: [`fresh_conversation_workbench_execution_plan.md`](fresh_conversation_workbench_execution_plan.md)

## 执行状态

|阶段|状态|证据|
|-|-|-|
|G0：计划激活|已完成|用户已确认懒创建 Session，并允许从运行中的 Task 立即切换；执行计划和文档路由已提交。|
|G1：新建对话|已完成|Conversation header 已增加入口；Workbench 清理 Session/UI/route/ref/mutation 状态，旧 Task 迟到事件由 reducer 忽略；Mock browser E2E 验证新旧 Session 隔离。|
|G2：对话栏布局|已完成|桌面左栏/画布使用约 1:3 flex 比例，左栏限制 320..420px；消息区独立纵向滚动，header 和输入保持固定；宽屏、矮视口和窄屏几何 E2E 通过。|
|G3：全量验收|已完成|后续 IntentPlanner 结构化输出加固切片修复独立模型问题后，`make check`、Mock、真实指标和有凭证的真实 Agent E2E 全部通过。|

## 当前边界

本计划只修改 Grafana Plugin Frontend 和对应测试/文档。后端、契约、持久化模型、查询语义和已固定的图表画布保持不变。

## G1 验证目标

- reducer 覆盖完整 Session 清理以及清理后旧 TaskEvent no-op。
- route 继续覆盖删除 Workbench ID 且保留其他 Grafana 查询参数。
- browser E2E 覆盖清空、不同新 Session、旧 Session 仍可读取和新界面不混入旧内容。

## G1 验证证据

- `make test-frontend`：9 个测试文件、26 个用例和 TypeScript typecheck 通过；新增 late TaskEvent after clear 用例。
- `make e2e-mock`：有界 API E2E、frontend build 和 Playwright 全部通过；新建对话清空工作台，新 Session ID 与旧 ID 不同，旧 Session Resource 仍可读取，新界面只有新请求的一组图表。

## G2 验证证据

- `make test-frontend`：9 个测试文件、26 个用例和 TypeScript typecheck 通过。
- `make e2e-mock`：frontend build、API E2E 和 Playwright 通过。Grafana 桌面壳内左栏/画布实测比例保持在约三分之一容差内，右栏维持 280px；560px 高视口中消息区产生真实 overflow 且 `scrollTop` 可改变，输入仍位于 Pane 内；900px 窄屏纵向排列和无水平溢出继续通过。

## G3 验证证据

- `make check`：生成物 diff、OpenAPI/JSON Schema、Go domain/application/adapters/backend、26 个前端单测、35 个诊断单测、TypeScript 和依赖边界全部通过。
- `make e2e-real-metrics`：真实 Prometheus/node_exporter 的五种 API 输入与同一套 Playwright 新对话、栏宽、滚动、恢复和无溢出验收通过。
- 加载 `.env` 后 `make diagnose-deepseek` 通过，配置模型返回严格 pong，证明 key、base URL 和模型可用。
- `make e2e-real-agent` 已实际运行三次但未通过：两次概览正确选择 `cpu|memory|load`，却把未显式指定的计划生成为 `step=30s/window=30s`，与门禁要求的本地默认 `10s/60s` 不同；一次显式 `30m/10s` 请求的首轮三视图、真实查询和 durable events 通过，但后续 Planner 持续返回无效 JSON 并被安全映射为 retryable `dependency_unavailable`。试验性测试修改已全部撤回，未放宽 QueryPlan、工具、泄漏或持久化断言。

## 收口状态

新建对话、宽度和滚动功能本身已完成。独立的模型稳定性问题已由
[`intent_planner_structured_output_hardening_progress.md`](intent_planner_structured_output_hardening_progress.md)
收口，其 G3 重新通过 `make check`、Mock、真实指标和真实 Agent E2E；本计划据此完成。
