# 历史会话与三栏工作台执行计划

> status: active
> createdAt: 2026-07-15
> implementationAuthorized: true
> decision: [`../adr/ADR-022-private-session-history.md`](../adr/ADR-022-private-session-history.md)
> dependsOn: `fresh_conversation_workbench_execution_plan.md`、`grouped_chart_canvas_execution_plan.md`

## 1. 目标

在现有 durable Session 恢复链路上增加当前用户的历史会话列表、切换和继续对话，并将工作台调整为“会话 / 对话 / 图表”三栏。空白工作台仍在首次提交时懒创建 Session；已提交的 Message、Task、TaskEvent、Chart 和 Execution 继续由 AI Core 自动持久化。

## 2. 锁定范围

- AI Core 与 Plugin Resource API 增加 owner-scoped `GET /sessions`，返回按 `updatedAt DESC,id DESC` 排列的 `SessionPage`，默认 20、最大 50 条。
- 列表只包含当前 tenant/creator 且至少有一个 Task 的 Session；Session 及其 Task/Event 的浏览器访问统一校验 creator。
- Task 接受事务同时更新 Session `updatedAt/version`；幂等重试和 Planner 失败不重复更新。
- 新 Session 标题由首条消息压缩空白后截为最多 50 个 Unicode code point；不调用模型，不回填旧标题。
- 前端新增 SessionPane 和无限分页；切换时关闭旧 SSE、清理当前 reducer/refs/route，并以 Session 身份拒绝迟到 history/replay/event。
- 删除 ContextPane、图表“详情”和本地 selectedChart 状态；保留卡片标题、状态、PromQL 和图表。
- 桌面为约 260px 会话栏、320..420px 对话栏、自适应图表栏；窄屏按会话、对话、图表纵向排列。

首版不增加搜索、重命名、删除、归档、收藏、标签、分享、团队可见、Fork 或草稿保存。

## 3. Gate 与提交

### G0：决策与执行记录

建立 ADR、本计划和 progress，更新文档路由。验证 `git diff --check` 与契约校验。

### G1：契约与生成物

增加 SessionPage、两个 GET Session list operation、分页参数和示例；重新生成 Go/TypeScript Client 与 Server。验证契约及生成物可复现。

### G2：AI Core

增加 Session Touch、owner get/list Port、SQLite `0006` 复合索引和 keyset 查询；在 CreateTask 事务内 touch Session；所有浏览器入口校验 Session owner。补 domain/application/SQLite/HTTP 单测。

### G3：Plugin Backend

代理 Session list query，继续由 Grafana Context 覆盖浏览器身份。补路由、参数、身份和错误映射测试。

### G4：Frontend

增加 SessionPane、标题 helper、无限分页、Session-aware reducer 和 Session-only route；实现统一切换；移除 ContextPane 与图表选择；调整三栏布局和纯逻辑测试。

### G5：纵向验收与收口

API E2E 验证 Session list；Browser E2E 验证创建 A/B、切换恢复、继续 A 后置顶、刷新、桌面/窄屏布局和旧上下文 UI 消失。更新 current snapshots 与验证证据后完成计划。

## 4. 验收门

- 当前用户只能列出和读取自己的非空 Session；外用户直接访问返回 404。
- 静态分页无重复遗漏，非法/跨资源/跨用户 token 返回 `invalid_argument`。
- 新 Task 原子更新 Session；Planner/事务失败和幂等重试不错误 touch。
- 切换可恢复完整多轮消息与图表，旧异步结果不能污染目标 Session。
- 继续历史 Session 后它回到列表顶部；无 ID 入口仍为空白新对话。
- 桌面和窄屏无水平溢出；图表仍最多两列且滚动恢复不回退。
- `make check`、`make e2e-mock`、`make e2e-real-metrics` 和 `./scripts/mtb verify --full` 通过；有凭证时补 `make e2e-real-agent`。

## 5. 提交与保护

每个 Gate 小步提交，仅包含对应代码、测试和必要演进文档。不 push、不创建 PR、不 amend、不重写历史；保留并排除用户未跟踪的 `docs/design/product_design_final.md`。

