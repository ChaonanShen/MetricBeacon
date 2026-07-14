# 有界 node_exporter 查询参数进度记录

> status: active
> createdAt: 2026-07-14
> plan: [`bounded_node_exporter_query_parameters_execution_plan.md`](bounded_node_exporter_query_parameters_execution_plan.md)

## 执行状态

|阶段|状态|证据|
|-|-|-|
|G0：决策与计划落盘|完成|ADR-019、active execution plan、结构蓝图和文档路由已落盘；契约基线验证通过。|
|G1：契约先行|完成|CreateTask resolution、Task QueryPlan、Chart step、Execution actual range 和 MCP view/window 输入已定义并生成。|
|G2：QueryPlan 与持久化|未开始|待实现解析、迁移、repository/workflow。|
|G3：MCP 注册视图查询|未开始|待实现 view/window registry 与 Mock/Real 一致性。|
|G4：极简 Agent 与可信回复|未开始|待缩减 Tool、使用 QueryPlan 和本地 formatter。|
|G5：Workbench 参数体验|未开始|待增加时间范围/resolution 控件和有效参数展示。|
|G6：端到端收口|未开始|待完成全量/E2E 验证和演进文档。|

## 当前边界

本切片保持 CPU、内存、load 三视图和 `prometheus-main` 不变。任何任意 PromQL、标签过滤、额外指标、
Dashboard 写入、Skill/Playbook 或模型读取原始 series 均不在本计划内。

## G0 验证证据

- `make validate-contracts`：两个 Plugin/AI Core OpenAPI、MCP OpenAPI、24 个 JSON Schema 与 Mock fixture 通过。
- `git diff --check`：通过。

## G1 验证证据

- `make generate`：Go/TypeScript AI Core、Plugin Resource 和 Grafana Tool 类型已重新生成。
- `make generated-client-diff`：生成物可复现。
- `make validate-contracts`：OpenAPI、24 个 JSON Schema、examples 与 fixture 通过。
