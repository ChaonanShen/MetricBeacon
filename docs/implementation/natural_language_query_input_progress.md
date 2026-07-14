# 自然语言单输入查询进度记录

> status: completed
> createdAt: 2026-07-15
> plan: [`natural_language_query_input_execution_plan.md`](natural_language_query_input_execution_plan.md)

## 执行状态

|阶段|状态|证据|
|-|-|-|
|G0：修订决策与计划|已完成|ADR-021 supersede ADR-020；计划已激活，蓝图与文档路由已更新。|
|G1：契约与持久化先行|已完成|Task QueryPlan 契约、领域、HTTP snapshot 与生成类型已增加 views；SQLite `0005` 持久化并从历史 Chart PromQL 回填。|
|G2：同步 Agent 规划与冻结执行|已完成|IntentPlanner Port 及 Mock/Eino Adapter 已实现；CreateTask 同步冻结计划，后台确定性执行 persisted views。|
|G3：自然语言 Workbench 与端到端收口|已完成|Workbench 仅提交 message/datasource；Mock 与真实指标 E2E、完整检查通过。|

## 当前边界

计划保留 ADR-019 的三视图、CPU window、范围/点数上限和本地 PromQL 注册表。它将 QueryPlan 扩展为
Agent 选择的 durable views，并把模型职责改为同步结构化意图规划；后台只执行冻结的注册视图，不再次让模型
选择 view、时间或 step。

## 草案基线证据

- `make validate-contracts`：三份 OpenAPI、24 份 JSON Schema 与 node_exporter fixture 通过。
- `git diff --check`：通过。

## G1 验证证据

- `make generate generated-client-diff validate-contracts`：通过；三份 OpenAPI、24 份 JSON Schema 与 fixture 有效。
- `make test-ai-core-domain test-sqlite test-ai-mcp test-plugin-backend test-frontend`：通过。
- 新增领域测试覆盖 view 顺序、重复/未知 view 拒绝和空 views；SQLite 测试覆盖 views 往返与历史 CPU Chart 回填。

## G2/G3 验证证据

- `make test-ai-core-domain test-sqlite test-ai-mcp test-ai-agent test-plugin-backend test-frontend`：通过。
- frontend `npm run build`：通过。
- `make e2e-mock`：API 与 Playwright 通过；60 秒/5 秒用例每序列 13 点，CPU PromQL 使用 `[30s]`。
- `make e2e-real-metrics`：API 与 Playwright 通过，五组自然语言意图均返回受限真实 series。
- `make check`：通过。`make e2e-real-agent` 需外部 DeepSeek 凭证，本次未运行。
