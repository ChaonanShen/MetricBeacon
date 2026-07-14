# 对话分组宽版图表画布进度记录

> status: completed
> createdAt: 2026-07-15
> plan: [`grouped_chart_canvas_execution_plan.md`](grouped_chart_canvas_execution_plan.md)

## 执行状态

|阶段|状态|证据|
|-|-|-|
|G0：计划激活|已完成|用户已授权按顺序执行；计划、进度、文档路由和当前快照已更新。|
|G1：分组派生与画布结构|已完成|纯 `deriveChartGroups` 产生 oldest-first 非空分组；ChartCanvas 渲染 Task section、prompt/时间/数量与总计。|
|G2：响应式宽版布局与滚动|已完成|viewport + 736px container query 启用最多两列，奇数尾图跨行；新 Task/恢复聚焦和 prepend 滚动补偿已实现。|
|G3：端到端与文档收口|已完成|Mock browser E2E、current overview/tree、文档路由与完整质量门禁已收口。|

## 当前边界

本计划仅修改 Grafana Plugin Frontend 的 Workbench 分组、布局、选择和滚动体验。后端、跨进程契约、持久化、
查询语义和自然语言输入计划均保持不变。

## 草案基线

- 当前 `ChartCanvas` 将所有 Task 的 Chart 扁平化后放入一个 Grid。
- 当前桌面 `minColumnWidth.xl=21` 约为 168px，中心画布可形成三至四个窄列。
- 当前 reducer 的 `taskOrder` 为 newest-first，而对话 Message 为 oldest-first；新计划只在画布派生层反转 Task 顺序。
- 当前 ChartCard 已通过 ResizeObserver 适配卡片宽度，plot 高度为 260px，无需重写绘图组件。

## G1 验证证据

- `make test-frontend`：9 个测试文件、22 个用例和 TypeScript typecheck 通过。
- 新增测试覆盖 newest-first 输入反转、Chart 归属、prompt 三级 fallback、空 Task、增量 Chart 和最新默认选择。

## G2 验证证据

- `make test-frontend`：9 个测试文件、25 个用例与 typecheck 通过；新增单次自动聚焦、恢复选择和 prepend scroll 公式测试。
- `make e2e-mock`：通过；1800px 宽画布验证两列上限与 `2 + 1 full-width`，900px 验证单列与无水平溢出。

## G3 验证证据

- frontend `npm run build`：通过。
- `make check`：通过。
- Browser E2E 连续两轮分析，验证分组正序、prompt/图表归属、宽窄布局、Context 选择、刷新 replay、PromQL 展开和无水平溢出。
