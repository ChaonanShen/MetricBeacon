# 产品化工作台 UI 移植进度记录

> status: completed
> createdAt: 2026-07-15
> plan: [`workbench_product_ui_migration_execution_plan.md`](workbench_product_ui_migration_execution_plan.md)

## 执行状态

|阶段|状态|证据|
|-|-|-|
|G0：计划、基线与边界冻结|已完成|执行计划、进度记录与文档路由已建立；冻结 3 份原型参考源码 SHA-256；`make validate-contracts` 通过 3 份 OpenAPI 与 25 个 JSON Schema；`make generated-client-diff`、`make test-frontend`（10 files / 30 tests + typecheck）、frontend `npm run build` 和 `git diff --check` 均通过。|
|G1：Controller/View seam 与主题基础|已完成|新增常驻 `WorkbenchShell`、页内唯一 h1 Header、Grafana theme 到 scoped CSS variables 的映射，以及无网络副作用的 `workbench-view`；旧三个真实 pane 先通过 typed ReactNode slot 接入。`Workbench.tsx` 的 Query/Mutation/SSE/ref controller 未迁出或复制。前端单元测试 11 files / 35 tests、typecheck、Rollup build、prototype 依赖/标识静态 guard 与 `git diff --check` 通过。|
|G2：Header、Context 与响应式工作区|已完成|页内 Header 提供唯一 h1、当前工作台/会话动作和四个原生 disabled 未来入口；新增只读 ContextPane，只展示 Session/Task/QueryPlan 真值和明确的未接入资源说明。根 container query 在宽屏形成 `Canvas / Context / Chat`，中宽折叠 Context，窄屏按 `Chat / Context / Canvas` 纵排。Playwright 已迁移到新 landmark/几何，验证 Context 真值、disabled nav、宽/窄/矮视口、图表两列和无水平溢出；`make e2e-mock` 的 6 组 API 链与 Playwright 1/1 通过。前端 11 files / 36 tests、typecheck、build 通过。|
|G3：真实 Session 与 Chat 整合|已完成|新增 `ChatPane` 与有界 `SessionMenu`，共用 controller 提供的 generated Session/Message/Task 与 runtime state；消息、assistant draft、Task/error/status、load-earlier、composer 和 Session 分页/选择均已接入。composer 只使用一个 form submit 路径，示例问题只填入 input。删除 `SessionPane`/`ConversationPane`，不保留双状态或双 composer。Playwright 证明 fresh 示例操作零 POST、Enter 首次提交恰好 1 个 Session POST + 1 个 Task POST，并继续通过多轮、A/B 切换、activity 置顶、reload replay、stale 404 与 theme route。`make e2e-mock` API 链及 Playwright 1/1 通过；前端 unit/typecheck/build 通过。|
|G4：Canvas 与真实图表视觉迁移|已完成|`ChartCanvas` 已迁移为产品化 Canvas header、真实计数、空画布与高密度 Task 分组；`ChartCard` 使用统一 scoped surface/PromQL 样式，同时原样保留 Grafana TimeSeries、DataFrame、ResizeObserver 和真实 execution。两列 container query、奇数尾图跨行、oldest-first、自动聚焦与 prepend 补偿保持不变。新增 `chart-view` 纯函数覆盖查询中/成功/失败/未知四种映射。前端 12 files / 40 tests、typecheck/build 通过；Mock API 链与 Playwright 1/1 通过图表数量、真实 legend、PromQL、plot 边界、两列几何、多轮和 reload replay。|
|G5：响应式、主题、可访问性与防泄漏验收|已完成|Playwright 扩为 3 个分层场景：真实纵向成功链；Task 503 后保留 pending payload/幂等键并成功重试；Shell 的唯一 h1、命名区域、disabled nav、Session focus、示例零 POST、dark/light token、1800/1366/900 布局、无水平溢出、无外部请求、无直连和无 `gaw_*`。新增 SSE malformed data 与 owner close 取消重连测试。前端 12 files / 42 tests、typecheck/build 通过；扩展后的 `make e2e-mock` 连续两轮均为 API 全链通过且 Playwright 3/3，通过后临时容器、镜像、网络和 volume 均清理。|
|G6：纵向回归与文档收口|已完成|计划、进度、文档路由、current overview/tree 与 code skeleton frontend 说明已同步。`git diff --check`、3 份 OpenAPI + 25 份 JSON Schema、generated client diff、`make check`（frontend 12 files / 42 tests、diagnostics 44 tests）、连续两轮 Mock API + Playwright 3/3、真实 Prometheus/node_exporter API + Playwright 3/3、`./scripts/mtb verify --full` 和有凭证 `make e2e-real-agent` 均通过；所有 E2E 临时资源均已清理。|

## 当前边界

当前只激活 frontend presentation migration。contracts、generated clients、Plugin Backend、AI Core、assistant-mcp、SQLite 和 Compose 均不在实现范围；任何真实 Folder/Dashboard/Service/权限、图表命令、HITL 或未来产品页面需求都必须另立切片。
