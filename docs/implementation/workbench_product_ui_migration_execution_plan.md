# 产品化工作台 UI 移植执行计划

> status: active
> createdAt: 2026-07-15
> implementationAuthorized: true
> decision: 仅重构 Grafana Plugin Frontend 展示层；以 2026-07-15 本机 `../grafana-ui` 当前源码为最新视觉基线，首版不改变跨进程契约、服务边界、权限模型或持久化结构，因此不新增 ADR
> productBaseline: [`../design/product_design_final.md`](../design/product_design_final.md)
> dependsOn: `session_history_workbench_execution_plan.md`、`grouped_chart_canvas_execution_plan.md`、`natural_language_query_input_execution_plan.md`、`current_codebase_overview.md`

## 1. 目标与结果形态

将本机平级 `../grafana-ui` 当前原型中的产品视觉，移植到当前真实 Grafana App Plugin，形成可生产演进的工作台界面；用户提供的旧版空状态截图用于补充理解空态和早期信息层级：

```text
Grafana 宿主 Chrome
└─ Torchbearing Workbench
   ├─ 页内产品标题与功能导航
   └─ 工作区
      ├─ Canvas：真实 TaskEvent 恢复的 Grafana TimeSeries
      ├─ Context：当前会话和真实 QueryPlan 上下文
      └─ Chat：私有 Session 切换、真实消息、流式状态与输入
```

本计划只把已经实现的 Session、Message、Task、TaskEvent、Chart 和 Execution 能力换成新的信息架构与视觉。原型中的 Mock 数据、模拟权限、模拟 SSE、图表写操作和未来产品页面不会进入正式业务状态。

完成后的首版应达到以下结果：

- 宽屏按当前原型源码采用 `Canvas | Context | Chat` 三栏，Canvas 占主要空间；
- 中等宽度折叠 Context，只保留 `Canvas | Chat`；
- 窄屏自然纵排且无水平溢出；
- 右侧 Chat 同时承载紧凑的 Session 选择、新建对话、消息时间线和 composer；
- 空状态、查询中、成功、无数据和失败状态均来自当前真实状态，不使用状态仿真器；
- 现有多轮、刷新恢复、有限事件重放、SSE 重连、Session 切换和图表分组行为不回退；
- 暗色视觉接近原型，同时完整支持 Grafana light/dark theme。

## 2. 依据优先级与冲突处理

实现时按以下优先级处理差异：

1. 当前 contracts、ADR、实际代码和测试决定可执行语义、安全和数据真值；
2. [`../design/product_design_final.md`](../design/product_design_final.md) 决定长期产品信息架构和术语；
3. 本机 `../grafana-ui` 当前源码决定 `Canvas | Context | Chat` 的列顺序、配色、密度、空态、卡片和交互风格，但不构成构建依赖；
4. 用户提供的旧版截图用于校验空状态和产品语言；截图与当前源码冲突时，以当前源码为准。

截图与 `../grafana-ui` 当前源码的列顺序、导航和默认内容存在差异，不要求逐像素复制。当前源码被视为后续设计演进，旧截图不再决定桌面列顺序；任何视觉参考都不得覆盖当前代码的真实行为。

由于 `../grafana-ui` 不是 Git 仓库，本计划冻结以下 2026-07-15 参考快照。后续原型若继续变化，不在当前实施中静默追随；应先审阅 diff 并明确是否修订 active plan：

|参考文件|SHA-256|用途|
|-|-|-|
|`../grafana-ui/src/components/ProductShell.tsx`|`393ea9424a0cc9134d2be8beeecb3171c4b89d849ade387f36abd4bcb9cdeb56`|产品密度、导航和 surface 语言；不复制独立站 shell。|
|`../grafana-ui/src/pages/Workbench.tsx`|`e3e79cb9cb9c90bea264f0e5fe51ce90676b360ca504c3044f928f0af96c7a3c`|最新 `Canvas | Context | Chat` 布局、空态和卡片参考。|
|`../grafana-ui/src/index.css`|`82edcad63aeea7f02f5cfa181591f253de5d18fcab55741c1188a50235ef493e`|色彩参考；不复制全局样式、字体或 Tailwind。|

若实现需要真实 Folder、Dashboard、Service、Grafana 权限、图表命令或未来产品页面，必须停止当前 Gate，先创建独立 Proposal/计划；涉及跨进程字段、权限或持久化时还必须契约先行并按需新增 ADR。

## 3. 范围清单

### 3.1 首版直接落地

|区域|正式实现|
|-|-|
|页内标题|展示 `AI Metrics Workbench / 指标分析工作台`，不重复伪造 Grafana 顶栏、用户头像或 breadcrumb。|
|产品导航|`工作台`为当前项；`会话`聚焦并展开右侧真实 Session 选择器；知识库、Playbook、Skill、晋升仅以明确 disabled/“尚未开放”状态出现，不改变 URL、不发请求。|
|Context|展示当前 Session 标题，以及当前活动 Task 或最近 Task 的 datasource、绝对时间范围、QueryPlan views、step 和真实状态。无 Task 时使用 `—` 或“提交后确定”。|
|Canvas|复用现有 `ChartCanvas -> ChartCard -> Grafana TimeSeries`，保留 Task 分组、真实 PromQL、执行状态、最多两列、奇数尾图跨整行和滚动补偿。|
|Chat|展示持久化 Message、流式 Assistant 文本、Task 状态、真实错误、恢复提示和自然语言输入。|
|Session|复用 owner-scoped、非空、keyset 分页的 Session list；支持新建本地空会话、切换、继续历史会话和加载更多。|
|空态|Canvas 展示真实空画布；Chat 展示当前支持的 CPU、memory、load 示例问题，点击只填入输入框，不直接创建 Task。|
|主题|使用 Grafana theme token、宿主字体、Grafana Icon 和 scoped styles；支持 dark/light。|

### 3.2 明确删除的原型痕迹

- `Proposal P1/P2`、角色/权限/类型/状态徽章条；
- `02 空状态 / 03 分析 + Canvas / 04 编辑 / 05 HITL` 状态演示页签；
- `重置演示`、`原型信息`、`PROTOTYPE / MOCK` 和 review-only 警示；
- 假 Grafana 顶栏、Folder 下拉、用户头像、通知数、告警 fingerprint、Agent 负载和假连接状态；
- `localStorage` 的 `gaw_*` 状态、HashRouter、模拟 SSE 定时器、关键词分支和随机 ID；
- `SVGChart` 及浏览器生成的 checkout/payment 假时序；
- 全局 `body`、全局 scrollbar、Google Fonts、Tailwind preflight 和 `select-none`。

### 3.3 首版不实现

- Folder、Dashboard、Service 的真实选择、推荐或权限结论；Context 只显示一条“Grafana 资源上下文尚未接入当前实现”的说明，不伪造具体值；
- `@datasource`、`@dashboard`、`@service`、`@folder` 和 `/skill` 的实体补全或 typed semantics；
- 会话搜索、重命名、删除、归档、分享、团队可见和 Fork；
- 图表拖动排序、布局持久化、全屏、关闭、删除、重跑、保存和 PromQL 编辑；
- Dashboard 写入、Diff、审批、HITL 和 Resume；
- 知识检索、Skill、Playbook、晋升、告警和审计页面；
- 原型 tool-call 卡片。当前事件虽包含 `tool.*`，但生成前端类型仍较弱；本计划不继续扩散手写 wire DTO，后续以事件类型硬化切片接入。

## 4. 必须保持不变的运行行为

以下行为属于 controller/data layer，不得因换壳修改：

1. 浏览器只通过 `/api/plugins/mini-torchbearing-app/resources` 调用 Plugin Resource API；不得直连 AI Core、assistant-mcp、Prometheus 或 Grafana HTTP API。
2. fresh Workbench 保持本地未保存状态，第一次提交时才懒创建 Session；打开 Session 菜单或点击“新建对话”不得创建或删除后端 Session。
3. Task 创建继续复用 `pendingTask` 和同一 idempotency key 进行安全重试；Enter 和发送按钮不得造成双提交。
4. 同一 Session 只允许一个非终态 Task；活动 Task 期间 composer 禁止二次提交，但用户仍可切换 Session。
5. Session 切换继续清理输入、route、reducer、sequence、replay、pending、幂等和聚焦 refs，并拒绝旧 history/replay/SSE 的迟到结果。
6. 历史 Task 必须先读取固定 `targetSequence` 的有限 replay，再为唯一非终态 Task 建立 SSE。
7. 所有 TaskEvent 都必须进入 reducer 推进连续 sequence；不能只保留 UI 可见事件。
8. duplicate event 忽略，sequence gap 和 transport failure 从最后连续序号重连，终态事件永久关闭连接。
9. URL 更新继续保留 `theme` 等 Grafana query 参数；只有明确 `resource_not_found` 才清除 stale Session ID。
10. Session list 保持当前 creator 私有、只含非空 Session，并按 activity 顺序分页；继续旧会话后它返回列表顶部。
11. 图表只来自 durable `chart.created` 和 `chart.execution_completed`，按 Task oldest-first 分组；不得预置假图。
12. Query composer 仍只提交自然语言和固定逻辑 datasource `prometheus-main`；范围、step 和 views 由现有 Planner/边界冻结。

## 5. 目标组件与状态边界

### 5.1 组件树

```text
module.tsx
└─ QueryClientProvider
   └─ Workbench.tsx                       # 唯一 controller，保持常驻
      └─ WorkbenchShell.tsx               # 纯展示编排和局部展开/折叠状态
         ├─ WorkbenchHeader.tsx           # 页内标题与产品导航
         ├─ ContextPane.tsx               # 真实只读上下文
         ├─ ChartCanvas.tsx               # 现有真实画布，调整外观
         │  └─ ChartCard.tsx              # 现有 Grafana TimeSeries，调整外观
         └─ ChatPane.tsx                  # 右栏壳、消息、状态和 composer
            └─ SessionMenu.tsx            # Session 选择、分页与新建入口
```

### 5.2 Controller/View seam

`Workbench.tsx` 继续独占：

- React Query 的 Session snapshot/history/list；
- create Task 与 load-more mutation；
- reducer、SSE、有限 replay 和 sequence refs；
- Session 切换、stale recovery、URL 和 idempotency；
- chart groups 与自动聚焦/滚动编排。

`WorkbenchShell` 只接收分组后的 view props，不发请求、不读取 `localStorage`、不创建 EventSource，也不复制服务端状态：

```ts
type WorkbenchShellProps = {
  context: WorkbenchContextView;
  sessions: SessionControls;
  conversation: ConversationControls;
  chartGroups: ChartGroup[];
  chartCanvasRef: React.RefObject<ChartCanvasHandle>;
};
```

- `SessionControls` 和 `ConversationControls` 是组件 props 分组，内部字段继续引用 generated `Session`、`Message`、`Task` 和现有 `WorkbenchState`，不是新的 wire DTO；
- Shell 的 Session menu/context 折叠状态可以是局部 UI state；不得用 `key`、路由或条件根节点重新挂载 `Workbench` controller；
- 顶部“会话”只调用 Shell 内的 `openSessionMenu()` 并转移焦点，不增加第二套 Session 选择状态；
- 未来产品导航项使用真实 `disabled` 或 `aria-disabled`，没有空 route 和点击副作用。

### 5.3 纯派生 View Model

新增 `workbench-view.ts` 及单元测试，集中处理：

- `contextTask = activeTask ?? newest task`；
- datasource、time range、views、step、Task status 的安全格式化；
- fresh、loading、running、completed、failed 的显示标签；
- 示例问题列表；
- 不完整历史的稳定 fallback，绝不输出 `undefined`。

该文件不得包含网络调用、时间随机数、Mock 分支或复制 Chart/Execution Schema。

## 6. 视觉、主题与响应式约束

### 6.1 样式实现

- 使用 `@grafana/ui` 的 `useTheme2`、Grafana theme 和 Icon/Button/Input 等现有组件；
- `workbench-styles.ts` 输出根节点 scoped CSS 和由 theme 映射的 CSS variables，不新增 CSS-in-JS 或构建依赖；
- 不引入 Tailwind、Vite、React Router、Lucide 或远程字体；Rollup 配置保持不变；
- 所有 CSS 作用域限定在 Workbench root，不修改 `body`、Grafana chrome 或全局 scrollbar；
- surface/text/border/success/warning/error/info 使用 Grafana token；橙色只作为产品 active/focus accent，并在 light/dark 下保证对比度；
- 消息、错误、PromQL 和图表文本必须可选择复制；
- icon-only 按钮必须有可访问名称，`:focus-visible` 不得被移除。

### 6.2 桌面与窄屏

|可用容器宽度|布局|
|-|-|
|`>= 1366px`|`minmax(560px, 1fr) minmax(240px, 280px) minmax(340px, 380px)` 三栏，gap 16；三个内容区独立滚动。|
|`1024..1365px`|Context 折叠为摘要/抽屉入口；主体为 Canvas 自适应 + 约 340–360px Chat。|
|`< 1024px`|页内纵向滚动，顺序为 Header、Chat、Context summary、Canvas；Session 使用 popover/picker，composer sticky 在 Chat 容器底部。|

Canvas 内部继续使用现有 container query：实际可用宽度达到约 736px 时最多两列，否则单列；三图时尾图跨整行。所有 grid child 保持 `min-width: 0`，页面必须满足 `scrollWidth <= clientWidth`。

### 6.3 独立滚动

- 宽屏 Workbench root 可以 `overflow: hidden`，但 Context、Canvas 和 Chat 各自提供可见滚动容器；
- Chat header、Session selector 和 composer 固定，只有 message timeline 滚动；
- Session menu 展开高度限制在 160–220px，并保留“加载更多”；
- 矮视口中 composer 仍位于 Chat pane 内且可访问；
- 窄屏恢复页面自然滚动，不能用桌面 overflow 规则锁死页面。

## 7. 逐文件改动纲领

|文件|计划改动|
|-|-|
|`frontend/package.json`、`package-lock.json`|保持不变；不引入原型依赖或新的样式运行时。|
|`src/module.tsx`|保持单一 `QueryClientProvider -> Workbench` 入口，不增加 HashRouter。|
|`src/workbench/Workbench.tsx`|保留 controller；将末尾 JSX 替换为 typed `WorkbenchShell` props；派生 `contextTask` 和 Context view。|
|`src/workbench/WorkbenchShell.tsx`|新增页内 header、导航、响应式三栏和 Session menu/context collapse 局部状态。|
|`src/workbench/WorkbenchHeader.tsx`|新增标题、当前 Workbench、真实“会话”动作和明确禁用的未来能力。|
|`src/workbench/ContextPane.tsx`|新增只读上下文和 unsupported context 说明；不读取 Grafana API 或伪造权限。|
|`src/workbench/ChatPane.tsx`|新增右栏；整合消息时间线、草稿、Task 状态、错误、恢复提示、示例问题和 composer。|
|`src/workbench/SessionMenu.tsx`|从现有 SessionPane 提取真实列表、选择、分页、loading/error 和新建行为。|
|`src/workbench/SessionPane.tsx`|ChatPane 接线完成后删除；不保留两套 Session UI。|
|`src/workbench/ConversationPane.tsx`|ChatPane 接线完成后删除；不保留两套 composer 或消息状态。|
|`src/workbench/WorkbenchPane.tsx`|调整为新的 shared pane shell，保持 theme-aware 和语义化 region/aside。|
|`src/workbench/ChartCanvas.tsx`|保留 imperative scroll API、Task 分组和 container grid；迁移 header、空态和 pane 视觉。|
|`src/workbench/ChartCard.tsx`|保留 DataFrame/TimeSeries/ResizeObserver/PromQL；调整卡片标题、状态和边框，不增加假统计或命令按钮。|
|`src/workbench/workbench-view.ts`、`*.test.ts`|新增纯 Context/状态/示例派生与 fallback 测试。|
|`tests/e2e/mock/browser-e2e.spec.ts`|把旧“会话/对话/图表”几何断言迁移到新 landmark/testid；增加空态、主题、导航禁用、Context 真值、网络和 a11y 断言。|
|`docs/implementation/*`、`docs/CLAUDE.md`|维护本计划、progress、current overview/tree 和完成证据。|

`resource.ts`、generated types、contracts、Plugin Backend、AI Core、assistant-mcp、SQLite migration 和 Compose 不属于本计划改动范围。

## 8. Gate、提交与验证

### G0：计划、基线与边界冻结

- 建立本计划与 progress，并更新 `docs/CLAUDE.md`、implementation README；
- 记录 prototype 元素的“接真实能力 / disabled / 删除 / 延后”分类；
- 确认主仓基线的 frontend unit/typecheck/build、contracts 和 generated diff；
- 不修改 completed 历史计划正文。

验证：

```bash
make validate-contracts
make generated-client-diff
make test-frontend
(cd apps/grafana-plugin/frontend && npm run build)
git diff --check
```

提交：`docs: plan product workbench UI migration`

### G1：建立 Controller/View seam 与主题基础

- 添加 `WorkbenchShell`、`WorkbenchHeader`、`workbench-view` 和 shared theme styles；
- `Workbench.tsx` 保持唯一 controller，所有 request/SSE/ref 仍在该组件中；
- 新 Shell 首先承载现有三个 pane，保证结构迁移时行为不变；
- 增加 Context/status 纯派生测试；
- 添加静态 guard，禁止原型依赖或 Mock 标识进入正式前端。

验证：

```bash
make test-frontend
(cd apps/grafana-plugin/frontend && npm run build)
! rg -n 'lucide-react|react-router-dom|tailwind|fonts.googleapis.com|gaw_|mockData|SVGChart' apps/grafana-plugin/frontend/src apps/grafana-plugin/frontend/package.json
```

提交：`refactor(frontend): establish workbench view shell`

### G2：迁移 Header、Context 与响应式工作区

- 实现页内产品标题、真实/禁用导航、三档响应式 layout；
- 实现 ContextPane，显示当前真实 Task 上下文；
- 实现 Canvas/Chat/Context 的语义 landmark、focus 和折叠行为；
- fresh page 只允许现有 Session list GET，不产生 Session/Task POST；
- 删除 Proposal、演示状态、假权限和假连接等 prototype chrome。

验证：frontend unit/typecheck/build，加定向 Playwright 验证 empty state、dark/light、disabled navigation、无外部请求和无水平溢出。

提交：`feat(frontend): migrate product workbench shell`

### G3：整合真实 Session 与 Chat

- 用 `SessionMenu` 和 `ChatPane` 替换旧 SessionPane/ConversationPane；
- Session menu 使用同一份 query data 和 `selectConversation` handler；
- 消息时间线保留持久化消息、流式 Assistant draft、Task status/error 和 load-earlier；
- composer 保留输入、busy/active Task 禁用与幂等提交；
- 示例问题只填充 input，不自动提交；
- 删除旧两个 pane，避免两套选择和 composer。

验证：

```bash
make test-frontend
(cd apps/grafana-plugin/frontend && npm run build)
make e2e-mock
```

Browser 断言覆盖：单次提交只产生一个 Session/Task、A/B 会话切换、继续旧会话置顶、新建不写后端、旧迟到结果不污染、route 保留 `theme`、stale 404 与其他错误区分。

提交：`feat(frontend): integrate sessions into chat pane`

### G4：迁移 Canvas 与真实图表视觉

- 将 ChartCanvas 改为原型风格的 Canvas header、计数、空态和高密度卡片空间；
- 将 ChartCard 改为产品化卡片，但保留真实 TimeSeries、PromQL、状态和 ResizeObserver；
- 保留 Task oldest-first、两列上限、奇数尾图跨行、自动聚焦和 prepend scroll compensation；
- 不增加全屏、编辑、删除、排序、保存或 Dashboard 操作。

验证：unit/typecheck/build 和 Browser 图表几何、PromQL 展开、plot/card 边界、多轮/reload replay、无假预置图。

提交：`style(frontend): migrate the real chart canvas`

### G5：响应式、主题、可访问性与防泄漏验收

- 在 1800×900、1366×768、900×900、1800×560 视口验证布局；390×844 记录 best-effort，不作为当前硬性 mobile 承诺；
- dark/light 均验证空态和已加载态；
- 检查唯一 h1、命名 landmark/nav、icon label、Tab 顺序、Enter 单次提交、focus-visible、`aria-current`、`aria-disabled`、alert/status/live region；
- 迁移 E2E selector 到 role 或稳定 testid，不依赖样式 class；
- 记录浏览器请求，禁止 Google Fonts、外部 Web、AI Core/MCP/Prometheus 直连；
- 检查 `localStorage` 不出现 `gaw_*`。

验证：连续两轮 `make e2e-mock`，确保重构没有掩盖 history/SSE 竞态。

提交：`test(frontend): verify migrated workbench experience`

### G6：纵向回归与文档收口

- 更新 current codebase overview、current code tree、必要的 code skeleton frontend 说明、本 progress 和文档路由；
- 将计划与 progress 改为 completed；
- 记录未执行的凭证依赖测试，不把外部模型凭证当成 UI 必需条件。

验证：

```bash
git diff --check
make validate-contracts
make generated-client-diff
make check
make e2e-mock
make e2e-mock
make e2e-real-metrics
./scripts/mtb verify --full
# 有 DeepSeek 凭证时：make e2e-real-agent
```

提交：`docs: complete product workbench UI migration`

## 9. Browser 验收矩阵

### 9.1 空工作台

- 页面只有一个 h1，Context、Canvas、Chat 均具有可访问名称；
- Session history 可以 GET 并展开，但 fresh 状态没有选中 Session；
- 不出现图表或假数据，Session/Task POST 数均为 0；
- Canvas 和 Chat 的空态文案明确下一步；
- Knowledge/Playbook/Skill/晋升不产生 route 或网络行为；
- 不出现 prototype/reset/simulator 文案。

### 9.2 单轮与多轮分析

- 首轮提交 payload 仍为自然语言和 `{ datasourceUid: 'prometheus-main' }`；
- Assistant 文本、Task 状态、真实 PromQL、CPU/memory/load 图表和 Grafana legend 可见；
- Context 显示 Task 的实际 datasource、time range、views、step 和 status；
- 第二轮仍属于同一 Session，并形成下一组 oldest-first 图表；
- 三张图最多两个 x 坐标，尾图跨整行；
- 页面 reload 后有限 replay 恢复相同消息、分组和图表，不重复。

### 9.3 Session 生命周期

- 新建对话清空输入、消息、Task、Context 和 Canvas，但不删除旧 Session；
- 新提交产生不同 Session ID；
- 选择 A/B 时消息、图表和 Context 不串线；
- 继续旧 Session 后 activity 排序更新；
- 切换期间迟到 history/replay/event 被忽略；
- missing Session 的 404 清 route 并显示 status，依赖/权限错误保留上下文并显示 alert。

### 9.4 主题、布局与交互

- dark/light 下 surface、正文、边框、focus 和状态均可辨认；
- 各视口无页面水平溢出，TimeSeries plot 位于卡片内；
- 矮桌面中 Chat timeline、Session menu 和 Canvas 可以独立滚动，composer 保持可见；
- 窄屏可通过键盘访问 Context、Chat、Canvas 和 Session picker；
- 消息和 PromQL 可选择复制；
- Enter 只提交一次，图标按钮均有 label，当前 Session 使用 `aria-current`。

## 10. 风险与控制

|风险|控制|
|-|-|
|Shell 或 nav 导致 controller remount，丢失 SSE/idempotency refs|Workbench 保持唯一常驻 controller；Shell 不使用 HashRouter、动态 root key 或重复 provider。|
|把原型 View Model 当 wire contract|组件 props 直接引用 generated types；只新增纯 UI 派生类型，不复制网络 DTO。|
|为了填满 Context 伪造 Folder/权限/Service|只显示现有 Task 真值；unsupported context 使用统一说明。|
|硬编码 dark 色导致 light theme 不可用|surface/text/border 使用 Grafana token；两主题均加入 Browser gate。|
|固定宽度和 overflow 裁掉图表、PromQL 或 focus ring|container-aware grid、`min-width:0`、多视口几何和 scroll gate。|
|Chat 内 Session UI 挤压消息区|默认折叠选择器，展开高度有界，timeline 独立滚动。|
|E2E 因 CSS class 改名失去行为覆盖|改用 role/label/testid；保留原业务断言，只更新布局断言。|
|原型依赖、远程字体或直连请求泄漏|静态 rg guard + Browser request guard + localStorage guard。|
|未来导航看似可用却没有后端|使用 disabled/aria-disabled 和“尚未开放”，无 route、无空页面。|

## 11. 停止条件与回滚

出现以下任一需求时停止当前计划并另立切片：

- 要求 future nav 进入可操作页面；
- 要求展示可信 Grafana Folder/Dashboard/Service/权限；
- 要求编辑、重跑、保存、关闭或持久化 Canvas 布局；
- 要求 tool-call、clarification、HITL、approval 或 resume 新状态；
- 要求修改 OpenAPI、TaskEvent、generated types、Plugin Backend 或 AI Core。

本计划没有数据库迁移和跨进程协议变化。回滚只需按 Gate revert 前端展示提交；不得加入运行时 `mockMode` 或双 UI 分支作为长期回滚机制。controller/data layer 保持不变，因此回滚不会影响已持久化 Session、Task、Event 和 Chart。

## 12. 完成定义

只有同时满足以下条件才能将计划标为 completed：

- 新产品化 Workbench 已完全替代旧“会话 / 对话 / 图表”三栏展示；
- 当前所有真实分析和 Session 行为通过新界面完成；
- prototype Mock、假上下文、假图表和全局样式没有进入主仓；
- dark/light、宽屏/窄屏、键盘、滚动和无溢出验收通过；
- frontend unit/typecheck/build、连续两轮 Mock E2E、real-metrics、`make check` 和 full verify 通过；
- 有凭证时 real-agent 通过，无凭证时明确记录未运行；
- contracts/generated 保持无差异；
- progress、current overview、current tree、文档路由和必要 blueprint 与实际代码一致；
- 每个 Gate 使用独立、可验证的小提交完成，未 push、未建 PR、未 amend 或改写历史。
