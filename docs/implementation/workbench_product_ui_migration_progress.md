# 产品化工作台 UI 移植进度记录

> status: active
> createdAt: 2026-07-15
> plan: [`workbench_product_ui_migration_execution_plan.md`](workbench_product_ui_migration_execution_plan.md)

## 执行状态

|阶段|状态|证据|
|-|-|-|
|G0：计划、基线与边界冻结|已完成|执行计划、进度记录与文档路由已建立；冻结 3 份原型参考源码 SHA-256；`make validate-contracts` 通过 3 份 OpenAPI 与 25 个 JSON Schema；`make generated-client-diff`、`make test-frontend`（10 files / 30 tests + typecheck）、frontend `npm run build` 和 `git diff --check` 均通过。|
|G1：Controller/View seam 与主题基础|已完成|新增常驻 `WorkbenchShell`、页内唯一 h1 Header、Grafana theme 到 scoped CSS variables 的映射，以及无网络副作用的 `workbench-view`；旧三个真实 pane 先通过 typed ReactNode slot 接入。`Workbench.tsx` 的 Query/Mutation/SSE/ref controller 未迁出或复制。前端单元测试 11 files / 35 tests、typecheck、Rollup build、prototype 依赖/标识静态 guard 与 `git diff --check` 通过。|
|G2：Header、Context 与响应式工作区|未开始|—|
|G3：真实 Session 与 Chat 整合|未开始|—|
|G4：Canvas 与真实图表视觉迁移|未开始|—|
|G5：响应式、主题、可访问性与防泄漏验收|未开始|—|
|G6：纵向回归与文档收口|未开始|—|

## 当前边界

当前只激活 frontend presentation migration。contracts、generated clients、Plugin Backend、AI Core、assistant-mcp、SQLite 和 Compose 均不在实现范围；任何真实 Folder/Dashboard/Service/权限、图表命令、HITL 或未来产品页面需求都必须另立切片。
