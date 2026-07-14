# 分层结果合理性诊断进度记录

> status: completed
> createdAt: 2026-07-14
> plan: [`layered_result_diagnostics_execution_plan.md`](layered_result_diagnostics_execution_plan.md)

## 执行状态

|阶段|状态|证据|
|-|-|-|
|G0：激活计划|完成|执行计划、进度记录和文档路由已新增。|
|P1：指标格式与语义分析|完成|24 个 Node 诊断测试、assistant-mcp 全量测试通过；真实 Prometheus/MCP 三视图通过标签、时间、有限值与区间校验，并输出 min/max/latest。|
|P2：Task/Event/Chart 结果分析|完成|34 个 Node 诊断测试通过；Mock、真实指标和真实 Agent E2E 均使用同一分析器验证连续事件、工具配对、Chart/Execution 关联及指标语义。|
|P3：测试文档与收口|完成|新增 code-agent 测试矩阵、预期返回形态和逐层故障定位；README、文档路由与当前代码快照已同步。|

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

## P2 验证证据

|入口|实际安全摘要|
|-|-|
|Mock E2E|30 events、7 tool calls、3 charts；CPU 2 series/14 samples、内存 2/14、load 2/14，终态 `task.completed`。|
|真实指标 E2E|30 events、7 tool calls、3 charts；CPU `97.22`、内存 `54.5429`、load `2.85`，每个 view 1 series/1 sample。|
|真实 Agent 概览|33 events、7 tool calls、3 charts；CPU `97.1296`、内存 `54.3617`、load `2.32`，终态 `task.completed`。|
|真实 Agent CPU 追问|13 events、1 tool call、1 chart；CPU `95.8749`，终态 `task.completed`。|

以上真实数值均为执行瞬间的 Docker Linux VM 指标，只用于证明本次 E2E 得到了可解码且语义合理的真实数据。
Task 分析器不固定这些数值，而是验证 sequence 从 1 连续递增、Task/Session 身份一致、唯一成功终态、
`tool.started`/`tool.completed` 一一配对、Chart/Execution 一一对应、规范 PromQL 与 view 匹配、
`seriesCount` 和实际 series 一致，以及唯一非空最终 Assistant Message。

自动 E2E 现默认使用 Grafana `13000`、AI Core `18080`、assistant-mcp `18081`，可分别通过
`GRAFANA_HOST_PORT`、`AI_CORE_HOST_PORT`、`ASSISTANT_MCP_HOST_PORT` 覆盖。手动 Compose 默认端口仍是
`3000`、`8080`、`8081`；本次测试未停止或修改用户正在运行的 manual 栈。

## P3 收口证据

- `make check` 通过：契约/生成物、全部 Go 测试、前端 7 files/18 tests、34 个诊断测试和依赖边界均通过。
- `make diagnose-real-metrics` 再次通过：原始 Prometheus 与 MCP 三视图均返回 1 series/1 sample；本次瞬时
  CPU 约 `97.08..97.55`、内存 `74.84`、load `0.84`。
- `make diagnose-deepseek` 通过：配置的 `deepseek-v4-flash` 在 584ms 内返回严格
  `{"status":"ok","answer":"pong"}`，日志未输出凭证。
- `git diff --check` 通过。
