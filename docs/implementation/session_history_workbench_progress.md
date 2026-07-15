# 历史会话与三栏工作台进度记录

> status: active
> createdAt: 2026-07-15
> plan: [`session_history_workbench_execution_plan.md`](session_history_workbench_execution_plan.md)

## 执行状态

|阶段|状态|证据|
|-|-|-|
|G0：决策与执行记录|已完成|ADR-022、active plan、progress、文档路由与 ADR 索引已建立；`git diff --check` 和 `make validate-contracts` 通过。|
|G1：契约与生成物|已完成|SessionPage、两层 GET Session list、20/50 分页参数与响应示例已定义；Go/TypeScript Client、AI Core Server 和 Plugin types 已重新生成，`make validate-contracts`、`make generated-client-diff` 与 `git diff --check` 通过。|
|G2：AI Core|进行中|领域 `Session.Touch`、owner-scoped/non-empty SQLite keyset page、`0006` 复合索引和 CreateTask 事务 activity update 已实现；domain/application/SQLite 测试通过，待接入 HTTP list 与全入口 owner 校验。|
|G3：Plugin Backend|待开始|待实现 Session list 代理。|
|G4：Frontend|待开始|待实现会话栏、切换和三栏重排。|
|G5：纵向验收与收口|待开始|待完成 API/browser E2E、全量检查和文档快照。|

## 当前边界

只实现当前用户私有的非空 Session 列表、恢复和继续对话。不增加搜索、管理、共享或模型标题能力。
