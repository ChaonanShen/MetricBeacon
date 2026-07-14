# 自然语言单输入查询进度记录

> status: active
> createdAt: 2026-07-15
> plan: [`natural_language_query_input_execution_plan.md`](natural_language_query_input_execution_plan.md)

## 执行状态

|阶段|状态|证据|
|-|-|-|
|G0：决策与计划|完成|ADR-020、execution plan、进度记录、ADR 索引、文档路由与结构蓝图已落盘；契约基线验证通过。|
|G1：自然语言 Workbench|未开始|待实现与验证。|
|G2：解析回归|未开始|待实现与验证。|
|G3：收口|未开始|待更新当前快照并运行完整检查。|

## 当前边界

保留 ADR-019 的三视图、QueryPlan 持久化、CPU window、范围/点数上限和本地 PromQL 注册表。此次只改变
Workbench 的意图输入方式并完善有限自然语言 cadence 解析。

## G0 验证证据

- `make validate-contracts`：三份 OpenAPI、24 份 JSON Schema 与 node_exporter fixture 通过。
- `git diff --check`：通过。
