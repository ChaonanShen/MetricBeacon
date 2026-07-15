# 新建对话与对话栏微调执行计划

> status: completed
> createdAt: 2026-07-15
> completedAt: 2026-07-15
> implementationAuthorized: true
> decision: local Workbench UI refinement; no ADR or cross-process contract change required
> dependsOn: `three_pane_workbench_execution_plan.md`、`grouped_chart_canvas_execution_plan.md`、`current_codebase_overview.md`

## 1. 目标

在不改变现有图表画布、查询链路或后端边界的前提下，为 Workbench 增加明确的“新建对话”入口，并改善左侧对话栏的宽度与独立滚动体验。

- 点击“新建对话”立即清空当前 Workbench；下一次提交才使用现有 API 创建新的持久化 Session，避免空 Session。
- 旧 Session、Message、Task 和 Chart 保留在 AI Core；本切片不增加历史会话列表、删除或切换界面。
- 运行中的 Task 允许切走并继续在后台执行；前端关闭旧订阅并拒绝迟到事件污染新工作台。
- 桌面对话栏约为分析画布宽度的三分之一，并具有独立纵向滚动；窄屏继续自然纵向排列。

## 2. 范围与非目标

本切片只修改 Grafana Plugin Frontend 的 Workbench 编排、ConversationPane 布局、前端测试和实现文档。它不修改 AI Core、Plugin Backend、assistant-mcp、OpenAPI/Schema、生成客户端、SQLite、QueryPlan、ChartCard、ChartCanvas 分组/网格或右侧上下文语义，也不增加依赖和 ADR。

## 3. 已锁定行为

### 3.1 新建对话

- 按钮位于对话栏 header 右侧，始终可见；空白状态点击是无后端请求的幂等清理。
- 清理范围包含输入、Session ID、选中图表、提示/错误、Session reducer、SSE sequence、replay、幂等请求、pending request 和自动聚焦状态，同时删除 URL 的 `sessionId/taskId` 并保留其他参数。
- 下一次提交沿用现有懒创建逻辑，必须得到不同 Session ID；旧 Session 不删除且仍可通过原 URL/API 恢复。
- 已创建且正在执行的 Task 不禁用按钮。切换后 effect cleanup 关闭旧 SSE；已经进入回调的旧事件因 reducer 中不存在旧 Task 而被忽略。
- CreateSession/CreateTask 或“加载更早记录”请求尚未返回时短暂禁用按钮，避免不可取消 mutation 的迟到成功结果重新选中旧 Session。

### 3.2 宽度与滚动

- 桌面左栏和中心画布使用约 `1:3` 的 flex grow 比例；左栏限制约 `320px..420px`，右栏仍为 `280px`。
- 中等宽度受左栏最小值保护时允许偏离精确比例，中心画布继续依靠既有 container query 降为单列。
- 对话 Pane 在桌面填满三栏可用高度；header 与输入区固定，只有消息区 `overflow-y:auto`，不产生水平滚动。
- 窄屏维持对话、画布、上下文纵向排列和页面自然滚动，避免嵌套滚动。

## 4. 实施与提交

1. 激活计划、进度记录和文档路由。
2. 实现新建对话清理与迟到事件隔离，补 reducer、route 和 browser E2E。
3. 实现左栏比例、桌面高度和消息滚动，补几何与实际滚动 E2E。
4. 运行前端、Mock、真实指标、真实 Agent 和全量质量门禁，记录证据并完成计划。

每个独立切片小步提交；相关 current snapshot/progress 与代码同提交。不 push、不建 PR、不 amend 或重写历史。

## 5. 验收标准

- 新建对话后消息、Task、图表、上下文、输入、错误和 URL 标识均为空；旧事件不能恢复旧状态。
- 新提交创建不同 Session，界面只显示新会话；旧 Session 仍可读取。
- 宽屏左栏约为中心画布三分之一，右栏不变；既有图表最多两列和无水平溢出断言继续成立。
- 较矮桌面视口中消息区 `scrollHeight > clientHeight` 且 `scrollTop` 可改变，header/输入保持可见。
- `make test-frontend`、frontend build、`make e2e-mock`、`make e2e-real-metrics`、`make e2e-real-agent` 与 `make check` 通过。

## 6. 收口记录

本切片曾因外部模型结构化输出不稳定而暂留 active。后续
[`intent_planner_structured_output_hardening_execution_plan.md`](intent_planner_structured_output_hardening_execution_plan.md)
已修复该独立 Planner 问题，并重新通过 `make check`、Mock、真实指标和有凭证的真实 Agent E2E。
因此本计划的 UI 行为和完整回归门均已满足，不再保留为活动切片。
