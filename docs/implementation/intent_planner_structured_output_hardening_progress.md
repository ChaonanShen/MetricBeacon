# IntentPlanner 结构化输出加固进度记录

> status: active
> createdAt: 2026-07-15
> plan: [`intent_planner_structured_output_hardening_execution_plan.md`](intent_planner_structured_output_hardening_execution_plan.md)

## 执行状态

|阶段|状态|证据|
|-|-|-|
|G0：计划与基线|已完成|63 次模型直连矩阵已定位多轮 Assistant 历史污染和 JSON-only 空 content 边界；契约基线通过。|
|G1：结构化 Planner 边界|已完成|JSON mode、专属 prompt、最多 6 个持久化结构化意图、严格四字段校验和一次契约重试已实现；Eino/Mock/commands/HTTP/bootstrap 定向测试通过。|
|G2：一致性与多轮 E2E|已完成|Mock cadence 已覆盖“每隔/间隔/一个数据点”及 `min` 完整单位；Mock E2E 新增 30 分钟/5 分钟采样输入；真实 Agent 在同一 Session 连续 8 次提交相同 10 分钟 CPU/内存请求，8 次均得到严格 `600/120` 双视图计划并完成 2 次工具调用和 2 张图。|
|G3：完整收口|待执行|待运行完整门禁并更新当前代码快照和 runbook。|

## 安全记录

基线只记录配置变体、历史轮数和 `expected|invalid_json|empty|wrong_semantics` 计数；未记录 API key、模型原文、
内部 URL 或完整时序。所有模型直连测试均未创建业务 Session、Message 或 Task。
