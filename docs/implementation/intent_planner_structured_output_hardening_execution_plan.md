# IntentPlanner 结构化输出加固执行计划

> status: completed
> createdAt: 2026-07-15
> completedAt: 2026-07-15
> implementationAuthorized: true
> decision: hardens ADR-021 without changing its boundary
> dependsOn: `natural_language_query_input_execution_plan.md`、`current_codebase_overview.md`

## 1. 目标与已复现故障

修复多轮会话累积后，Eino IntentPlanner 模仿历史 Assistant 事实回复并返回自然语言或空 content 的问题。保持
同步规划、本地边界裁决、持久化前失败和确定性执行语义不变。

前置模型直连矩阵固定使用同一句 `查看最近10min的cpu和内存变化图，每隔2min采集一个数据点`，对 0～6 轮
历史各重复 3 次，共 63 次调用：

- 当前实现：0～2 轮 9/9 正确；3～6 轮 12/12 返回非 JSON；
- 只启用 JSON mode：0～4 轮 15/15 正确；5～6 轮 6/6 返回空 content；
- JSON mode、专属 prompt 与结构化历史同时启用：0～6 轮 21/21 均返回正确的
  `cpu+memory / rangeSeconds=600 / stepSeconds=120`。

## 2. 锁定范围

本切片包含：

- DeepSeek JSON mode、可实际传输的近零 temperature 和受限输出长度；
- Planner 专属 prompt，不再把执行/事实回复 Profile 全文交给模型；
- 使用持久化用户消息与 QueryPlan 形成结构化历史，不发送 Assistant 事实回复；
- 严格四字段 JSON 校验，以及针对空/模型契约错误的一次有界重试；
- Mock cadence 一致性与相同请求连续多轮的真实 Agent 回归；
- 当前代码快照、测试矩阵和验证证据更新。

本切片不包含：模型升级、任意 PromQL、新 view、查询边界调整、API/Schema/SQLite 变更、Dashboard 写入或
将模型原始响应写入日志/数据库。

## 3. 实现决定

### 3.1 模型输出边界

- 保持 `deepseek-v4-flash` 和 non-thinking；启用 `json_object` response format。由于当前 Go SDK 会通过
  `omitempty` 省略精确零值，temperature 使用可实际传输的 0.01，max tokens 为 512。
- 输出必须恰好包含 `status`、`views`、`rangeSeconds`、`stepSeconds`。缺失/额外字段、尾随内容、未知或重复 view、
  不一致状态均是模型契约错误。
- 空 content 或模型契约错误进行一次新调用；重试不携带模型原文。网络错误和超时不重试。第二次失败沿用
  retryable `dependency_unavailable`。
- 不从 Markdown、自然语言或事实回复中提取计划。本地范围、step、点数预算和 CPU window 规则保持权威。

### 3.2 结构化历史

内部 Planner request 使用最多 6 个 `PreviousIntent`，每项只包含用户 message、持久化 views、Task range 秒数和
持久化 step。累计用户文本仍不超过 12,000 字，按时间正序传入。views 为空的 legacy/unsupported Task 不作为
历史意图；历史读取错误不得静默降级。

模型只接收一条 system prompt 和一条 JSON user envelope：

```json
{
  "previousIntents": [
    {"message":"查看最近10min……","views":["cpu","memory"],"rangeSeconds":600,"stepSeconds":120}
  ],
  "currentMessage":"查看最近10min……"
}
```

当前消息的显式 view/range/step 始终优先；历史只用于省略和指代。Planner prompt 只描述注册 views、字段协议、
允许范围与禁止项，不包含查询结果格式或事实数值。

### 3.3 兼容与文档

这是 AI Core 内部 Port 调整。OpenAPI、Task/QueryPlan Schema、SSE、MCP Tool Schema、SQLite 和生成客户端均不变，
无需 migration 或 codegen。ADR-021 的职责和安全边界不变，不新增 ADR。

## 4. 执行门

1. G0：激活本计划和进度记录。
2. G1：实现结构化历史、专属 prompt、JSON mode、严格校验、一次重试及定向测试。
3. G2：补齐 Mock cadence，并在真实 Agent E2E 中连续 8 次提交相同请求。
4. G3：运行完整门禁，更新 current overview/tree、runbook、文档路由和完成证据。

## 5. 验收标准

- Planner 输入中不出现 Assistant role、历史指标值、事实时间戳或本地 formatter 文本。
- 相同请求连续 8 次均形成 `views:[cpu,memory]`、600 秒 range、step 120 秒、CPU window 30 秒的 Task。
- 每个 Task 完成两次注册 view 查询、两张 Chart 和本地事实回复；失败重试仍发生在任何业务持久化之前。
- Mock 识别 `每隔2min`、`间隔2min`、`2min一个数据` 和 `2min一个数据点`。
- 定向测试、`make check`、`make e2e-mock`、`make e2e-real-metrics` 和有凭证的 `make e2e-real-agent` 通过。

## 6. 预期提交顺序

1. `docs: plan structured planner output hardening`
2. `fix(agent): harden structured query planning`
3. `test(e2e): repeat agent-planned conversations`
4. `docs: complete structured planner hardening`

每个提交保持独立可验证；不 push、不建 PR、不 amend 或改写历史。
