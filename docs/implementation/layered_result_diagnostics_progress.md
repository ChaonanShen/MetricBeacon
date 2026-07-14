# 分层结果合理性诊断进度记录

> status: active
> createdAt: 2026-07-14
> plan: [`layered_result_diagnostics_execution_plan.md`](layered_result_diagnostics_execution_plan.md)

## 执行状态

|阶段|状态|证据|
|-|-|-|
|G0：激活计划|完成|执行计划、进度记录和文档路由已新增。|
|P1：指标格式与语义分析|完成|24 个 Node 诊断测试、assistant-mcp 全量测试通过；真实 Prometheus/MCP 三视图通过标签、时间、有限值与区间校验，并输出 min/max/latest。|
|P2：Task/Event/Chart 结果分析|待开始|—|
|P3：测试文档与收口|待开始|—|

## 契约与决策评估

本切片只消费现有 Prometheus/MCP/TaskEvent/Chart wire 数据并增加测试断言，不修改跨进程字段、服务所有权、
权限或持久化结构，因此不更新契约、生成客户端、数据库迁移或 ADR。

## P1 验证证据

|层级|实际安全摘要|
|-|-|
|原始 Prometheus CPU|`vector`，1 series/1 sample，min=max=latest `98.6931`。|
|原始 Prometheus 内存可用率|`vector`，1 series/1 sample，min=max=latest `64.6373`。|
|原始 Prometheus load|`vector`，1 series/1 sample，min=max=latest `3.25`。|
|MCP CPU|`matrix`，1 series/1 sample，min=max=latest `98.1984`。|
|MCP 内存可用率|`matrix`，1 series/1 sample，min=max=latest `64.6372`。|
|MCP load|`matrix`，1 series/1 sample，min=max=latest `3.25`。|

上述数值只记录本次 Docker Linux VM 的验证证据，不是固定断言。稳定断言是 CPU/内存 0..100、load 非负、
值有限、instance 存在、时间严格递增以及 20 series/5,000 samples 上限。

首次执行时发现宿主 `8081` 已由用户的 `mini-torchbearing-real-metrics-manual` 栈占用。诊断入口现默认映射到
`18081`，并允许用 `MTB_DIAGNOSTIC_MCP_PORT` 覆盖；基础 Compose 的默认手工/E2E 端口仍为 `8081`。
