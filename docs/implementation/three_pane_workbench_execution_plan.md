# 基础三栏工作台执行计划

> status: draft-review
> createdAt: 2026-07-14
> implementationAuthorized: false
> dependsOn: `../design/arch_design_detail.md` P2、`code_skeleton_design.md`、
> `current_codebase_overview.md`

## 1. 目标与范围

在现有持久化多轮 Workbench 上建立 P2 的基础三栏结构：

```text
┌──────────────┬─────────────────────────────┬──────────────┐
│ 对话栏 280px │ 图表画布 minmax(0, 1fr)     │ 上下文 280px │
│ 消息与输入   │ 自适应多图网格              │ 只读分析信息 │
└──────────────┴─────────────────────────────┴──────────────┘
```

本切片只调整前端展示和本地 UI 状态：

- 左栏保留当前 Session 的消息、Task 进度、错误、输入框和历史加载。
- 中栏展示所有已恢复或新生成的图表。
- 右栏展示数据源、时间范围、Task 状态及当前选中图表的只读详情。
- 宽屏显示三栏；小于 Grafana `xl` 断点时纵向堆叠，保证无水平溢出。

### 1.1 本计划不包含

- Canvas OpenAPI、Schema、SQLite 或服务端持久化。
- 拖动、删除、缩放、排序、Pin、全屏和布局编辑。
- PromQL、标题、时间范围或图表类型编辑。
- Session 列表、搜索、分享和 Fork。
- `PanelChrome` 替换、Dashboard 保存或审批。
- 新 CSS 框架、拖拽库或运行时依赖。
- AI Core、assistant-mcp、Plugin Backend、契约和生成客户端修改。
- real-metrics、real-agent 问题诊断或修复；两种模式仅登记为后续独立任务，不进入本计划验收。

## 2. 锁定的实现设计

### 2.1 三栏组件

保持 `Workbench.tsx` 为 API、SSE、reducer 和 mutation 编排入口，将展示拆成：

|组件|固定职责|
|-|-|
|`ConversationPane`|显示 Session 标题、消息、流式助手文本、Task 状态和错误；消息区顶部加载更早记录；输入框和提交按钮位于栏底，保留现有禁用条件、幂等重试和活动 Task 限制。|
|`ChartCanvas`|显示“分析画布”和图表数量；无图时显示说明性空态；使用 Grafana `Grid minColumnWidth={34}` 展示 ChartCard；每张图提供“详情”按钮。|
|`ContextPane`|显示 Session 标题、数据源、时间范围和 Task 状态；存在选中图表时显示标题、查询状态、序列数、单位和完整只读 PromQL；无图时显示空态。|

左栏是当前对话，而不是 Session 列表：现有 Plugin Resource API 没有列出 Session 的接口，不能在本切片伪造或新增该能力。

### 2.2 布局和响应式

- 仅使用 Grafana `Stack`、`Box`、`Grid` 和 `ScrollContainer`，不增加样式依赖。
- 宽屏采用横向 `Stack`：左栏和右栏固定 `280px` 且不收缩；中栏 `grow=1`、`minWidth=0`；栏间距使用 token `2`。
- 宽屏工作区高度为 `calc(100dvh - 112px)`，三栏各自纵向滚动，输入框不随消息滚走。
- `xs` 至 `lg` 采用纵向 Stack，三栏宽度为 `100%`；页面负责纵向滚动，对话栏最小高度为 `420px`，画布和上下文按内容展开。
- Pane 使用 Grafana theme 背景、弱边框和圆角，不写死暗色主题颜色。
- 图表网格改为实际容器宽度驱动，移除当前视口级 `xl: 6` 配置；同一中心画布在常见桌面宽度为两列，空间足够时可为三列。

### 2.3 选中图表与数据边界

- Workbench 增加本地 `selectedChartId?: string`，不写入 reducer、URL 或服务端。
- 当前选择仍存在于图表集合时保持不变；首次出现图表且尚未选择时，默认选择当前展示顺序中的第一张图；新图不抢占已有选择。
- 刷新后不恢复选择；有限 replay 完成后重新选择第一张可见图。
- ChartCard 的“详情”按钮使用 `aria-pressed`，选中卡片使用更强边框提示；不把整张卡片变为按钮，以免与 PromQL `<details>` 冲突。
- 图表继续从各 Task 的 durable TaskEvent runtime 聚合，不创建前端 Canvas 真相源；消息、Task 和图表当前排序均保持不变。
- 不增加 `mockMode`、`realMode` 或 Adapter 判断，也不在页面展示虚构的服务、环境、Dashboard 或运行模式信息。

真正加入拖拽、删除或布局恢复时，必须另立计划，先定义 Canvas 契约并由 AI Core 持久化，再实现前端操作。

## 3. 执行切片与提交

### G0：激活执行记录

提交：`docs: activate three-pane workbench execution plan`

1. 收到明确实现指令后，将本计划改为 `status: active`、`implementationAuthorized: true`。
2. 新建对应 progress 文档，记录 P1、P2 和最终验收状态。
3. 更新文档路由，将本计划标记为当前活动 UI 切片。
4. 不修改已完成的 Chart Trio 和 real-analysis 历史计划。
5. 不新增 ADR：本切片不改变契约、所有权、权限或持久化边界。

验证：`make validate-contracts` 与 `git diff --check`。

### P1：实现三栏工作台

提交：`feat(frontend): add three-pane workbench shell`

1. 从 Workbench 提取 `ConversationPane`、`ChartCanvas` 和 `ContextPane`。
2. 增加响应式三栏壳、独立滚动区和空态。
3. 将图表网格改为容器宽度驱动。
4. 增加 `selectedChartId`、详情按钮、选中样式和右栏只读详情。
5. 保持现有请求、SSE、reducer、Chart mapper 和 TimeSeries 行为。
6. 在同一提交更新 progress、current-code overview 和 code tree，说明三栏结构及未实现的 Canvas 编辑边界。

验证：`make test-frontend` 和 `make check`。

### P2：补齐 Mock 浏览器验收并收口

提交：`test(e2e): verify three-pane workbench layout`

1. 更新现有 browser E2E，不再断言宽屏六张图全部位于同一行。
2. 在 `1440×900` 下验证：
   - “对话”“分析画布”“分析上下文”三栏同排，几何顺序为左栏、画布、右栏；
   - 中栏至少形成两列，六张图形成多行；
   - plot 位于所属卡片和画布边界内；
   - 点击第二张图的“详情”后，右栏展示对应标题、PromQL 和序列数。
3. 在 `900×900` 下验证三栏纵向排列、图表可见且无水平溢出。
4. 刷新后继续验证消息、六张图和有限 replay 恢复；选中状态允许恢复为默认第一张图。
5. 通过最终验证后，将计划与 progress 标记 completed，并同步文档路由和当前代码快照。

最终验证：

```text
make test-frontend
make check
make e2e-mock
```

如果 `127.0.0.1:3000`、`8080` 或 `8081` 被用户管理的 Compose 栈占用，不得主动停止该栈，也不得用 real-metrics 替代 Mock 验收；应记录端口阻塞，待端口释放后再完成 `make e2e-mock`，计划在此之前不得标记 completed。

## 4. 完成标准与回滚

完成标准：

1. `1440px` 宽视口中明确显示左对话、中画布、右上下文三栏。
2. 左栏保留消息恢复、活动 Task、错误、输入和加载历史能力。
3. 中栏能展示至少六张历史图表，形成多列多行，不出现六个极窄列。
4. 右栏能根据“详情”选择展示正确图表信息，不提供编辑入口。
5. `900px` 宽视口无水平滚动，三栏按既定顺序纵向排列。
6. Chart、PromQL、图例、状态、刷新恢复和 SSE 行为不回退。
7. 不产生 OpenAPI、Schema、生成客户端、SQLite 或后端代码差异。
8. `make check` 与 `make e2e-mock` 通过；real-metrics 和 real-agent 不属于本计划完成条件。
9. 每个提交仅包含对应 UI 切片和必须同步的演进文档。

回滚只需反向恢复前端与测试提交；本计划没有数据库迁移、协议兼容或持久化数据风险。
