# 新建对话与对话栏微调进度记录

> status: active
> createdAt: 2026-07-15
> plan: [`fresh_conversation_workbench_execution_plan.md`](fresh_conversation_workbench_execution_plan.md)

## 执行状态

|阶段|状态|证据|
|-|-|-|
|G0：计划激活|进行中|用户已确认懒创建 Session，并允许从运行中的 Task 立即切换。|
|G1：新建对话|未开始|待实现状态、URL、订阅与异步清理。|
|G2：对话栏布局|未开始|待实现约 1:3 宽度与独立消息滚动。|
|G3：全量验收|未开始|待运行 Mock、真实指标、真实 Agent 与全量门禁。|

## 当前边界

本计划只修改 Grafana Plugin Frontend 和对应测试/文档。后端、契约、持久化模型、查询语义和已固定的图表画布保持不变。
