# 真实后端分层诊断进度记录

> status: active
> createdAt: 2026-07-14
> plan: [`real_backend_diagnostics_execution_plan.md`](real_backend_diagnostics_execution_plan.md)

## 执行状态

|阶段|状态|证据|
|-|-|-|
|G0：激活诊断记录|完成|执行计划、进度记录和文档路由已新增；`make validate-contracts` 与 `git diff --check` 通过。|
|P1：Prometheus 与 MCP 断点探针|待开始|—|
|P2：DeepSeek 直连探针|待开始|—|
|P3：模式切换恢复与收口|待开始|—|

## 基线复现

|检查|结果|说明|
|-|-|-|
|DeepSeek `/models` + 最小 Chat Completion|通过|配置模型为 `deepseek-v4-flash`；返回严格 `{"status":"ok","answer":"pong"}`。|
|`make e2e-real-metrics` API 阶段|通过|三条真实 Prometheus 查询均返回非空 series；浏览器阶段曾被三栏重复标题的旧选择器误报，已在前序提交修复。|
|`make e2e-real-agent`|通过|真实模型完成概览三图和 CPU 单图，并通过 durable tool 配对、恢复与泄漏检查。|

## 边界确认

诊断只调用现有只读接口，不新增业务真相源，不修改契约、SQLite、权限或模型出域范围。
