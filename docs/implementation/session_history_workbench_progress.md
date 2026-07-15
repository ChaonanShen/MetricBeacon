# 历史会话与三栏工作台进度记录

> status: completed
> createdAt: 2026-07-15
> plan: [`session_history_workbench_execution_plan.md`](session_history_workbench_execution_plan.md)

## 执行状态

|阶段|状态|证据|
|-|-|-|
|G0：决策与执行记录|已完成|ADR-022、active plan、progress、文档路由与 ADR 索引已建立；`git diff --check` 和 `make validate-contracts` 通过。|
|G1：契约与生成物|已完成|SessionPage、两层 GET Session list、20/50 分页参数与响应示例已定义；Go/TypeScript Client、AI Core Server 和 Plugin types 已重新生成，`make validate-contracts`、`make generated-client-diff` 与 `git diff --check` 通过。|
|G2：AI Core|已完成|`Session.Touch`、owner-scoped/non-empty SQLite keyset page、`0006` 索引、CreateTask 原子 activity update、GET Session list 和 Session/Task/Event 全入口 creator 校验已实现；domain/application/SQLite/HTTP 测试覆盖空 Session、稳定游标、迁移、幂等/Planner 失败不 touch、跨用户 list/token/direct/SSE 访问。|
|G3：Plugin Backend|已完成|GET Session list route、20/50 分页参数和 generated Client 代理已实现；测试验证 Grafana identity 覆盖伪造头、query 原样转发、51 拒绝，`make test-plugin-backend` 与 `make generated-client-diff` 通过。|
|G4：Frontend|已完成|Resource infinite list、Unicode 首消息标题、Session-aware reducer/route、SessionPane 和统一切换已实现；布局改为“会话/对话/图表”，ContextPane、详情按钮和 selectedChart 状态已删除，Task-only 自动滚动保留。10 个文件 30 个单测、TypeScript 和 frontend build 通过。|
|G5：纵向验收与收口|已完成|API E2E 验证 owner history、标题、activity version/time；Playwright 覆盖 A/B 创建、恢复、继续旧会话置顶、刷新、stale Session 和桌面/窄屏布局。完整验收曾发现 history refresh 提前关闭本地活跃 Task SSE 的竞态，已以 reducer 终态事件边界修复并增加回归测试。`make check`、连续两轮 `make e2e-mock`、修复后 `./scripts/mtb verify --full`、`make e2e-real-metrics` 和有凭证的 `make e2e-real-agent` 全部通过。|

## 当前边界

只实现当前用户私有的非空 Session 列表、恢复和继续对话。不增加搜索、管理、共享或模型标题能力。
