# 新建对话与对话栏微调进度记录

> status: active
> createdAt: 2026-07-15
> plan: [`fresh_conversation_workbench_execution_plan.md`](fresh_conversation_workbench_execution_plan.md)

## 执行状态

|阶段|状态|证据|
|-|-|-|
|G0：计划激活|已完成|用户已确认懒创建 Session，并允许从运行中的 Task 立即切换；执行计划和文档路由已提交。|
|G1：新建对话|已完成|Conversation header 已增加入口；Workbench 清理 Session/UI/route/ref/mutation 状态，旧 Task 迟到事件由 reducer 忽略；Mock browser E2E 验证新旧 Session 隔离。|
|G2：对话栏布局|未开始|待实现约 1:3 宽度与独立消息滚动。|
|G3：全量验收|未开始|待运行 Mock、真实指标、真实 Agent 与全量门禁。|

## 当前边界

本计划只修改 Grafana Plugin Frontend 和对应测试/文档。后端、契约、持久化模型、查询语义和已固定的图表画布保持不变。

## G1 验证目标

- reducer 覆盖完整 Session 清理以及清理后旧 TaskEvent no-op。
- route 继续覆盖删除 Workbench ID 且保留其他 Grafana 查询参数。
- browser E2E 覆盖清空、不同新 Session、旧 Session 仍可读取和新界面不混入旧内容。

## G1 验证证据

- `make test-frontend`：9 个测试文件、26 个用例和 TypeScript typecheck 通过；新增 late TaskEvent after clear 用例。
- `make e2e-mock`：有界 API E2E、frontend build 和 Playwright 全部通过；新建对话清空工作台，新 Session ID 与旧 ID 不同，旧 Session Resource 仍可读取，新界面只有新请求的一组图表。
