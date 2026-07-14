# 对话分组宽版图表画布进度记录

> status: draft-review
> createdAt: 2026-07-15
> plan: [`grouped_chart_canvas_execution_plan.md`](grouped_chart_canvas_execution_plan.md)

## 执行状态

|阶段|状态|证据|
|-|-|-|
|G0：计划激活|待授权|计划已保存为 draft-review，尚未授权业务代码修改。|
|G1：分组派生与画布结构|未开始|待实现与验证。|
|G2：响应式宽版布局与滚动|未开始|待实现与验证。|
|G3：端到端与文档收口|未开始|待实现与验证。|

## 当前边界

本计划仅修改 Grafana Plugin Frontend 的 Workbench 分组、布局、选择和滚动体验。后端、跨进程契约、持久化、
查询语义和自然语言输入计划均保持不变。

## 草案基线

- 当前 `ChartCanvas` 将所有 Task 的 Chart 扁平化后放入一个 Grid。
- 当前桌面 `minColumnWidth.xl=21` 约为 168px，中心画布可形成三至四个窄列。
- 当前 reducer 的 `taskOrder` 为 newest-first，而对话 Message 为 oldest-first；新计划只在画布派生层反转 Task 顺序。
- 当前 ChartCard 已通过 ResizeObserver 适配卡片宽度，plot 高度为 260px，无需重写绘图组件。
