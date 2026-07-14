# 自然语言单输入查询执行计划

> status: active
> createdAt: 2026-07-15
> implementationAuthorized: true
> decision: accepted by ADR-021, which supersedes ADR-020
> dependsOn: `bounded_node_exporter_query_parameters_execution_plan.md`、`current_codebase_overview.md`

## 1. 目标

Workbench 只提交自然语言和固定逻辑数据源。同步运行的 Agent 先从消息和有限历史中形成受限结构化意图；AI Core
本地补齐稳定默认值、验证并冻结有效 QueryPlan，再由确定性的注册视图执行器形成规范 PromQL 并查询数据。一个
Task 只使用这份持久化计划，后台不得再次让模型选择 view、时间或 step。

```text
自然语言消息 + 有限会话历史
  -> synchronous Agent IntentPlanner {views, rangeSeconds?, stepSeconds?}
  -> local merge/default/bounds validation + frozen absolute range
  -> durable Task.timeRange + QueryPlan.views/step/window
  -> deterministic registered-view executor
  -> assistant-mcp compiles canonical PromQL
  -> Prometheus query_range
  -> local factual formatter
```

## 2. 范围与非目标

本切片包含：

- 删除 Workbench 的默认时间范围与采样分辨率控件及其请求映射；
- 让 Workbench CreateTask 只携带 message 与 `prometheus-main`；
- 新增同步 `IntentPlanner` Port：Eino 为真实 Adapter，Mock 为确定性 Adapter；
- 将 Agent 选定的 views 作为 QueryPlan 的耐久字段，并在后台只执行该字段；
- 覆盖 `每隔 5s 采集` 等自然语言 cadence，以及模型/Mock 计划的边界失败；
- 更新跨进程契约、SQLite migration、浏览器与 API E2E、当前代码快照、文档路由和验证证据。

本切片不包含：任意 PromQL、额外指标/view、label 过滤、范围/点数上限调整、CPU rate window 策略调整、模型
读取原始时间序列、Dashboard 写入或异步未规划 Task 状态。

## 3. 已锁定的行为

### 3.1 Planner 输出与本地裁决

Planner 的严格 JSON 输出为：

```json
{"status":"planned","views":["cpu"],"rangeSeconds":60,"stepSeconds":5}
```

未明确出现的时间或间隔用 `null` 表示；不支持的请求使用 `status:"unsupported"` 与空 `views`。模型不得输出
PromQL、datasource、绝对时间、CPU window、label、reasoning 或用户可见数值。

AI Core 在 Planner 返回后，以注入 Clock 冻结时间，并按以下顺序解决参数：

1. API 的绝对 `from/to` 保持最高优先级；
2. Planner 的显式相对范围覆盖 API 的相对 hint；缺省为最近 30 分钟；
3. Planner 的显式 step 覆盖 API resolution；缺省采用现有最多约 300 点的 auto step；
4. 本地继续执行 30 秒至 6 小时、允许 step 与 1,000 点显式预算校验；
5. CPU window 继续按当前时长策略选择 `30|60|300` 秒，不由模型决定。

模型不可用、超时、输出非严格 JSON、重复/未知 view 或越界参数时，CreateTask 返回安全且可重试的错误，不写入
Message、Task 或 idempotency 记录。`unsupported` 不是错误：它创建一个 views 为空的 Task，由本地完成一条受限
说明，但不访问 Prometheus。

相同 idempotency key 的已完成请求在调用 Planner 前直接返回原 Task。两个并发首次请求允许重复进行规划，但只能
由现有事务幂等机制持久化一个 Task。

### 3.2 持久化与执行

`QueryPlan` 增加规范化且去重的 `views` 数组：

```json
{"views":["cpu","memory"],"stepSeconds":5,"cpuRateWindowSeconds":30}
```

- 新 Task 的 `views` 只允许 `cpu|memory|load`；空数组只表示 unsupported。
- SQLite 增加 `views_json`。migration 从已有 Chart PromQL 推断历史 Task 的视图；无法推断的历史失败/无图 Task
  保持空数组，不编造选择结果。
- Task Schema、TaskEvent snapshot、HTTP response、OpenAPI、JSON Schema 与生成的 Go/TypeScript 类型同时更新。
- CreateTask 请求的 `timeRange`、`resolution` 保持可选，兼容非 Workbench 调用方；Workbench 不发送它们。
- 工作流移除第二次模型选图。它以 QueryPlan.views 的规范顺序串行调用现有 QueryEngine 的 Validate/Execute，持久化
  tool、Chart、Execution 事件，并由本地 formatter 生成事实回复。
- 主分析链路不再为了固定流程强制执行 search/labels；这些 MCP 工具继续作为独立诊断能力保留。

### 3.3 Agent Adapter

- Eino Adapter 改为无 Tool 的同步 Planner，输入为当前消息、最多 12 条/12,000 字的历史和受限 view 说明；只接受
  上述 JSON，不持久化模型原文。
- Mock Adapter 使用同一 Planner Port，确定性解析受支持中文/英文时间、`每`/`每个`/`每隔` cadence 和 view 关键词。
- 对话后续的代词/省略可参考历史；当前消息的明确范围或间隔始终优先。
- 现有 Eino `query_prometheus` Tool、模型终态文本和二次 view 选择逻辑删除或由确定性执行器替代。

### 3.4 CPU window 结论

“每隔 5 秒”是 query_range 的求值 step，不等于 CPU `rate()` 的 lookback。对于 5 秒 scrape，保留
`rate(...[30s])`：每五秒产生一个过去约 30 秒的 CPU 平均速率点，Mock 的 1 分钟闭区间因此是 13 个点。Prometheus
`rate()` 需要范围内至少两个样本，Grafana 的 rate interval 建议至少覆盖约四个 scrape interval；因此本切片不把
window 改成 5 秒。[Prometheus rate](https://prometheus.io/docs/prometheus/latest/querying/functions/#rate)
与 [Grafana rate interval](https://grafana.com/docs/grafana/latest/datasources/prometheus/template-variables/#use-__rate_interval)。

## 4. 分阶段执行

### G0：修订决策与计划

- 由于已提交的 ADR-020 记录的是“本地解析、模型只选 view”，它与本计划冲突；不改写历史，新增 ADR-021 将其
  supersede，并更新 ADR 索引、文档路由和结构蓝图。
- 将本计划从 `draft-review` 改为 `active` 后才开始实现。
- 验证：`make validate-contracts`、`git diff --check`。

### G1：契约与持久化先行

- QueryPlan 增加 views，更新 Task/事件/OpenAPI/Schema/examples，并重新生成客户端类型。
- 新增 `views_json` migration、历史 Chart 推断和 repository consistency coverage。
- 验证：`make generate generated-client-diff validate-contracts`，AI Core domain/SQLite/HTTP 定向测试。

### G2：同步 Agent 规划与冻结执行

- 增加 Planner Port、Mock/Eino Adapter、严格 JSON 协议和同步 CreateTask 集成。
- 在模型返回后合并默认值、冻结范围、持久化 views/QueryPlan；保留 idempotency 与安全错误语义。
- 用确定性注册视图执行器替代后台 Eino 二次选图，并保留 durable Tool/Chart/Execution/SSE 行为。
- 验证：Planner/commands/workflow/agent/MCP 定向测试，Mock 与 Eino Adapter 一致性覆盖。

### G3：自然语言 Workbench 与端到端收口

- 删除范围/resolution state、props、select、选项映射和请求字段；保留完成后的只读有效参数。
- 更新浏览器/API/真实 Agent E2E、README、current overview/tree、runbook 和本进度记录。
- 验证：`make test-frontend`、`make check`、`make e2e-mock`、`make e2e-real-metrics`；有凭证时
  `make e2e-real-agent`。

## 5. 验收标准

- Workbench 不渲染“默认时间范围”和“采样分辨率”控件。
- Workbench CreateTask request 不含 `timeRange` 或 `resolution`。
- `查看最近1分钟cpu的使用率变化，每隔5s采集个数据` 得到 `views:[cpu]`、60 秒范围、step 5 秒、30 秒 CPU
  window、Mock 13 个点和包含 `[30s]` 的规范 PromQL。
- `最近30分钟 CPU，每隔30s采集一个点` 得到 step 30 秒而不是 auto 的 10 秒，证明 `每隔` 被显式解析。
- 省略范围/step 时得到最近 30 分钟、auto step 10 秒、CPU window 60 秒。
- 单视图与三视图请求只执行持久化 views；unsupported 不建 Chart；模型失败不产生半成品 Task。
- Task、Chart、Execution、SSE replay 与 Assistant 回复始终反映同一个有效 QueryPlan，不存在后台二次选图。
- CPU 30 秒 rate window 保持为计算窗口，不被误改为五秒 query step。

## 6. 预期提交顺序

1. `docs: adopt synchronous agent query planning`
2. `feat(contracts): persist agent-selected query views`
3. `feat(ai-core): persist planned query intent`
4. `refactor(agent): plan and execute bounded views`
5. `refactor(frontend): submit natural-language query intent`
6. `test(e2e): verify agent-planned natural-language queries`

每一提交只含一个可验证切片；不 push、不建 PR、不 amend 或改写现有历史。
