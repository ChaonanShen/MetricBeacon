# node_exporter 真实分析链路详细执行计划

> status: active
> createdAt: 2026-07-14
> implementationAuthorized: true
> dependsOn: `node_exporter_real_analysis_plan.md`、`code_skeleton_design.md`、
> `current_codebase_overview.md`

## 1. 目标、范围和锁定决策

本计划把现有确定性 Mock 闭环演进为以下三层能力，并保持 Mock 链路作为无模型、无实时依赖的回归基线：

```text
多轮持久化工作台
  -> Real Prometheus/node_exporter
  -> 最小 Eino + DeepSeek Agent
```

最终演示链路：

```text
Browser -> Grafana Plugin -> Plugin Backend -> AI Core
        -> Eino Agent -> assistant-mcp -> Prometheus -> node_exporter
        <- durable Session/Message/Task/Chart/TaskEvent + replay/SSE
```

第一版只支持 node_exporter CPU、内存可用率和系统负载三视图；不支持任意指标、任意 PromQL、知识库、
Grafana 写入、告警、Playbook、Multi-Agent 或 Resume。

实施顺序固定为：P1 多轮持久化 -> P2 前端工作台 -> P3 真实指标 -> P4 Agent -> P5 E2E 收口。

### 1.1 已锁定的契约与安全决策

- Mock 与 Real 使用同一逻辑数据源 UID：`prometheus-main`。这是
  `code_skeleton_design.md` 中既有的中性标识；不再使用 `mock-prometheus`。
- 每个 Session 最多一个非终态 Task；数据库 partial unique index 是最终并发兜底。
- `Message` 增加必填 `taskId`，但保留 `Task.inputMessageId`。一个 Task 恰好有一条 User Message，
  至多一条 Assistant Message，三者 tenant/session 必须一致。
- 历史事件使用有限 JSON replay；仅活动 Task 使用 SSE follow。SSE 在终态 durable events 排空后关闭。
- Agent 只带入当前消息之前、按时间正序的最近 12 条持久化 User/Assistant Message；总计最多
  12,000 个 Unicode 字符，按完整消息从最旧开始丢弃。当前用户消息最大 4,000 个 Unicode 字符。
- Agent 工具串行执行，最多 6 个模型 iteration、12 次工具调用；每次调用使用稳定 source call ID。
- 完整 `QueryExecutionResult` 只保留在当前 Agent run accumulator；模型只得到不超过 16 KiB 的摘要。
  任何完整 points、真实 label values、内部地址、身份、token、secret 或 private reasoning 都不得进入模型。
- 默认模型为 `deepseek-v4-flash`，可显式切换 `deepseek-v4-pro`；强制 `thinking=disabled`。
  DeepSeek 当前公开支持这两个模型及 Tool Calls/JSON 输出。
- 真实 Agent 是显式 opt-in：默认 driver 保持 `mock`；Mock 与 real-metrics 均不读取 API key。

## 2. 契约、存储和运行时设计

### 2.1 API 与生成类型

在 AI Core OpenAPI、Plugin Resource OpenAPI、JSON Schema、MCP Tool Schema、examples 和生成 Go/TypeScript
类型中同步以下变化：

|接口/类型|固定设计|
|-|-|
|`Message`|新增必填 `taskId`；Assistant completion event 内嵌 Message 同样携带该字段。|
|Message 列表|`GET /v1/sessions/{sessionId}/messages?pageSize=50&pageToken=...`；默认 50，最大 100，`createdAt DESC,id DESC` keyset 分页。|
|Task 列表|`GET /v1/sessions/{sessionId}/tasks?pageSize=20&pageToken=...`；默认 20，最大 50，使用同一稳定排序。|
|页面响应|统一 `{items: [], nextPageToken: string|null}`。Token 为含版本、资源类型、Session ID 和 cursor 的 base64url JSON；非法、跨接口或跨 Session 使用返回 `invalid_argument`。|
|有限 replay|`GET /v1/tasks/{taskId}/events/replay?afterSequence=0&pageSize=200&pageToken=...`，响应 `{items, targetSequence, nextPageToken}`。|
|SSE|保留 `GET /v1/tasks/{taskId}/events?afterSequence=N`，只承担 durable 补齐和活动 Task follow。|

首次 replay 在一个读事务中固定 `targetSequence=task.latestSequence`；后续 page token 固定携带该值和
最后 sequence，因此活动 Task 新事件不会让历史读取无限延长。`afterSequence` 与 `pageToken` 互斥。
replay 单页最多 200 个事件，且最多约 3 MiB 序列化内容；Plugin unary 响应上限调为 4 MiB，触及任一
限制则返回 next token。

Plugin Resource API 镜像上述三个读取接口。所有读取先 tenant-scope 验证 Session/Task；跨租户统一返回 404。

内部 DTO/Port 调整：

- `AgentRunRequest` 增加按时间正序的 `History []ConversationMessage`。
- `QueryValidationResult` 增加 `CanonicalExpression`。
- `AgentEvent` 与 `ToolCallRecord` 增加 `SourceCallID`；它仅用于内部审计和 started/completed 配对，
  对外 TaskEvent 继续只暴露系统生成的 `toolCallId`。
- Eino、Prometheus parser、HTTP SDK 类型不得穿过 Domain、Application、HTTP 或 MCP 契约。

### 2.2 SQLite migration 与事务语义

新增 `0002_multi_turn_and_tool_correlation.sql`。migration runner 在同一事务中执行 Go 前置校验、SQL、
Go 后置校验，全部成功后才写入 schema version。

1. 前置校验：每个 Task 的 `input_message_id` 唯一且指向同 tenant/session 的 User Message；每个旧 User
   Message 恰被一个 Task 引用；每个旧 Assistant Message 恰能由 `assistant.message.completed` 的
   `payload.message.id` 关联到 Task；每个 tenant/session 最多一个非终态 Task。发现孤立、歧义或重复数据
   即终止迁移，不按时间猜测关系。
2. 增加延迟外键：

   ```sql
   ALTER TABLE messages
   ADD COLUMN task_id TEXT
     REFERENCES tasks(id) DEFERRABLE INITIALLY DEFERRED;
   ```

3. Backfill User Message 的 `task_id`；从完成事件 backfill Assistant Message，并补写旧事件内
   `payload.message.taskId`。`tool_calls` 新增 `source_call_id`，旧行写为 `legacy:<toolCallId>`。
4. 新增：

   ```text
   UNIQUE tasks(tenant_id, input_message_id)
   UNIQUE messages(tenant_id, task_id, role)
   UNIQUE tool_calls(tenant_id, task_id, source_call_id)
   UNIQUE tasks(tenant_id, session_id) WHERE status NOT IN ('completed', 'failed')
   ```

   并补齐 `(tenant_id,session_id,created_at,id)` 的 Message/Task 分页索引。
5. 触发器强制非空 `task_id`；Task insert/update 验证它的 input Message 为同 tenant/session 的 User Message
   且双向关联一致；Assistant Message 必须关联已存在的同 tenant/session Task。延迟 FK 允许先写携带未来
   task ID 的 User Message，再写 Task。
6. 后置校验所有 Message 的 Task 关联、Task/User Message 双向一致性和 `PRAGMA foreign_key_check`。

创建 Task 时在同一事务内依次完成：幂等 reservation、生成并写入携带未来 Task ID 的 User Message、写入 Task、
写入 `task.created`、完成幂等记录。相同 key/请求返回原 Task；不同请求返回 `idempotency_conflict`。

Task 完成或失败时，终态状态、`task.status_changed` 和 terminal event 必须在同一事务中持久化，再通知 SSE。
创建时先检查已活动 Task 以返回 `resource_conflict` 与 `activeTaskId`；partial unique index 仍处理竞争窗口。

### 2.3 前端恢复和上下文

前端改为 Session 级状态：

```text
session
messagesById
tasksById
taskOrder
runtimeByTaskId: latestAppliedSequence, assistantDraft, chartsById, error
activeTaskId
messageNextPageToken
taskNextPageToken
```

恢复算法固定为：

1. 使用 URL 的 `sessionId` 恢复；`taskId` 仅定位/滚动某轮。
2. 读取 Session，再并行读取 MessagePage 和 TaskPage。
3. 对每个已加载 Task 做有限 JSON replay，直到达到其固定 `targetSequence`。
4. 存在活动 Task 时，从最后连续 sequence 创建唯一 SSE；replay 与订阅之间的事件由 SSE durable replay 补齐。
5. terminal event 到达后客户端主动关闭 SSE 并刷新 Message/Task 页面。
6. 加载更早记录时合并去重，只 replay 新出现的 Task。

sequence 按 Task 独立维护；重复事件忽略，gap 不应用并从最后连续 sequence 重新读取。Assistant Message
按 Message ID upsert，防止历史接口与 completed event 重复显示。提交新消息时不 reset 历史；活动 Task 存在时
禁用提交；网络结果不确定时重试复用同一 idempotency key 与原请求。

### 2.4 Prometheus 注册表与 Real Adapter

固定第一版查询注册表：

|view|canonical PromQL|unit|
|-|-|-|
|`cpu`|`100 * (1 - avg by (instance) (rate(node_cpu_seconds_total{mode="idle"}[5m])))`|`percent`|
|`memory`|`100 * node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes`|`percent`|
|`load`|`node_load1`|`short`|

允许的原始指标仅为 `node_cpu_seconds_total`、`node_memory_MemAvailable_bytes`、
`node_memory_MemTotal_bytes`、`node_load1`。`node_load1` 未结合 CPU core 数时只描述数值与趋势，不判断
“健康”或“过高”。

契约修改：所有 datasource UID 改为 `enum:["prometheus-main"]`；labels schema 补
`node_memory_MemTotal_bytes`；Query validation 输出必填 `canonicalExpression`；Search source 只允许：

```text
mock_fixture/node_exporter_overview
prometheus_metadata/<registered-metric>
```

新增 `0003_datasource_uid.sql`，将旧 Task 与 Chart query JSON 的 `mock-prometheus` 改为
`prometheus-main`；不重算旧幂等 hash，也不改写历史审计事件。

Real Adapter 保持现有 `PrometheusPort`，由 Bootstrap 选择 `mock|http`，业务层不感知 driver。使用
`net/http` 和 `github.com/prometheus/prometheus v0.313.1` 的 parser：

- Search：对四个注册指标调用 `/api/v1/metadata?metric=<name>`，仅真实存在的指标进入本地确定性匹配/排序。
- Labels：POST `/api/v1/series`，固定最近 15 分钟，删除 `__name__` 后排序去重。
- Validate：仅做本地 AST 校验，不访问 Prometheus。
- Execute：再次校验，只 POST `canonicalExpression` 到 `/api/v1/query_range`，仅接受 matrix。

PromQL 最大 2,048 字符、AST 最多 32 nodes/2 selectors，最终必须等价于三条 canonical expression 之一。
仅允许空白和冗余括号差异；额外 matcher、aggregation、offset、`@`、subquery、regex、函数或指标一律拒绝。

固定上限：查询范围 6 小时、Prometheus HTTP 10 秒、AI Core MCP 外层 12 秒、20 series、5,000 原始 samples、
2 MiB 解压响应体；Labels 最多 20 series、每 label 20 个 sample values。禁用 redirect。NaN/Inf 过滤并生成本地
warning；超限直接失败。Prometheus API 映射遵循官方 metadata、series、query_range 契约。

错误映射固定为：输入/AST/limit 为 `schema_validation_failed`；指标不存在为 `resource_not_found`；
Prometheus/MCP timeout 为 `tool_timeout`；网络、429、5xx、上游畸形 JSON 或非 matrix 为
`dependency_unavailable`；401/403 为 `adapter_not_configured`。任何上游 URL、body、身份或内部错误不得透传。

### 2.5 Eino/DeepSeek Agent

依赖固定为：

```text
github.com/cloudwego/eino v0.7.13
github.com/cloudwego/eino-ext/components/model/deepseek v0.1.6
github.com/bytedance/sonic v1.15.0
```

使用 `adk.NewChatModelAgent`，`MaxIterations=6`、`ExecuteSequentially=true`、总工具调用上限 12，
Runner 非 streaming。生产 Bootstrap 构造 DeepSeek model；Adapter 构造器只接收 Eino
`ToolCallingChatModel`、现有 MetricCatalog/QueryEngine Ports、Profile 与 limits，测试注入 scripted fake。

DeepSeek 固定：`MaxTokens=2048`、`Temperature=0.1`、thinking disabled、单次模型 30 秒、
Task 总 60 秒。Profile 指令要求 JSON 对象，并接受被 Markdown fence 包裹的等价 JSON；因模型端不保证
`response_format` 兼容性，只有已存在成功的本地查询结果时才可把非 JSON 的最终文本安全映射为该结果。
模型调用失败、无本地结果的最终文本或无法验证的最终 JSON 映射为 `dependency_unavailable`；iteration/
总时限用 `execution_interrupted`。

新增只读 Profile `data/agent-knowledge/node_exporter.md`：UTF-8、最大 64 KiB，必须包含三视图、规范 PromQL、
解释口径、无数据/错误处理、最终回复格式和禁止项。启动时校验 Profile；不创建 Knowledge MCP 或索引。

三个 Tool 显式实现严格 `InvokableTool`，用 `additionalProperties:false` 和 `DisallowUnknownFields`：

|Tool|模型输入|由 run closure 注入|模型输出|
|-|-|-|-|
|`search_metrics`|搜索词|datasource UID|metric/type/允许 label 名，不含 source URL 或原始 metadata 文本|
|`get_metric_labels`|四个注册指标之一|datasource UID|labelNames 与 sampleCounts，不含 label values|
|`query_prometheus`|`{view, expression}`|datasource、绝对时间范围、step=300|安全 ToolSummary|

Query Tool 先验证 view/expression 对应关系，再经既有 QueryEngine/MCP 执行。非法候选返回定长失败摘要，允许模型
在额度内纠正；依赖故障终止 Task。每次工具调用从 `compose.GetToolCallID(ctx)` 取 source call ID；空值或重复值失败。

每个 run 创建独立 accumulator：

```text
full QueryExecutionResult -> accumulator -> ChartProposal
bounded ToolSummary       -> Eino/DeepSeek
```

ToolSummary 最大 16 KiB，只包含 view、成功状态、series 数、匿名 alias 的 latest/min/max/mean/sampleCount
和最多五条本地 warning；不含 points、timestamps、真实 labels、source reference 或内部地址。超限先退化为
count-only，仍超限则失败。

最终模型输出必须严格为：

```json
{"status":"completed","views":["cpu","memory","load"],"answer":"用户可见结论"}
```

或：

```json
{"status":"unsupported","views":[],"answer":"第一版能力范围说明"}
```

unknown fields 拒绝；answer 为 1–4,096 个 Unicode 字符。`completed.views` 必须与 accumulator 成功执行的 views
完全一致且非空；`unsupported` 必须没有工具调用。Chart title、unit、refId、legend、canonical expression 全由
本地注册表生成，固定排序 `cpu -> memory -> load`，每个 view 仅能成功执行一次。任何 ReasoningContent 均丢弃，
不写数据库、事件或日志。

配置固定为：

```dotenv
AI_CORE_AGENT_DRIVER=mock
AI_CORE_AGENT_PROFILE_PATH=data/agent-knowledge/node_exporter.md
AI_CORE_AGENT_MAX_ITERATIONS=6
AI_CORE_AGENT_MAX_TOOL_CALLS=12
AI_CORE_AGENT_TIMEOUT=60s
AI_CORE_MODEL_TIMEOUT=30s
AI_CORE_MCP_TOOL_TIMEOUT=12s

ASSISTANT_MCP_PROMETHEUS_DRIVER=mock
ASSISTANT_MCP_PROMETHEUS_URL=http://prometheus:9090
ASSISTANT_MCP_PROMETHEUS_DATASOURCE_UID=prometheus-main
ASSISTANT_MCP_PROMETHEUS_TIMEOUT=10s

DEEPSEEK_API_KEY=
DEEPSEEK_BASE_URL=https://api.deepseek.com
DEEPSEEK_MODEL=deepseek-v4-flash
```

`.env` 保持 gitignore；用户从 `.env.example` 复制后填写 key。`.dockerignore` 增加 `.env`。eino 模式缺 key 或
Profile 时启动失败；Mock/real-metrics 不读取 key；readiness 不调用模型。

## 3. 分提交执行顺序

每个编号是一个独立 commit。每个完成切片在同一 commit 更新本计划的 progress、受影响的 current snapshot、
README 和必要 ADR；不得 push、amend 或覆盖无关工作区修改。

### G0：设计落盘

1. `docs: activate node exporter real analysis execution plan`

   - 在收到明确执行指令后把本计划标记 `active`，新建 progress 文档。
   - 新增 ADR，锁定有限 replay、Message/Task 循环关联、`prometheus-main`、完整数据隔离与 source call ID。
   - 更新 blueprint 与文档路由。
   - 验证：`make validate-contracts`。

### P1：多轮契约和持久化

2. `feat(ai-core): persist task-linked messages and source call ids`

   - 更新 Message/TaskEvent 契约、生成物、Domain/DTO、`0002` migration、双向关联和单活动 Task 约束。
   - Mock Runtime 生成稳定 `mock-01...` source call IDs，durable sink 按 source ID 配对。
   - 验证：`make generate generated-client-diff validate-contracts test-ai-core-domain test-sqlite`。

3. `feat(ai-core): add session history and finite event replay`

   - 增加 keyset repositories、历史 API、固定 target replay、终态 SSE 关闭和终态事务原子性。
   - 实现最近 12 条/12,000 字符上下文组装。
   - 验证：`make test-ai-core-domain test-sqlite test-ai-mcp`。

4. `feat(plugin): proxy history and terminal task streams`

   - Plugin Backend 增加 Message/Task/replay 只读代理，Grafana identity 覆盖伪造头。
   - unary client 保持 10 秒；SSE client 无全局 timeout，仅由 request context 控制。
   - 验证：`make test-plugin-backend`。

### P2：Session 工作台

5. `feat(frontend): support persistent multi-turn workbench`

   - 引入 Session reducer、URL 恢复、分页、有限 replay、唯一活动 SSE、terminal 自动关闭和 gap 恢复。
   - 新消息保留历史；pending retry 复用 idempotency key。
   - 验证：`make test-frontend`。

6. `test(e2e): cover multi-turn persistence and replay`

   - Mock API/Playwright 覆盖三轮、刷新、活动 Task 刷新、失败轮与历史流终止。
   - 三轮 deterministic Mock 断言 3 个 Task、6 条消息和 9 张图；故障只通过测试 Runtime 注入。
   - 验证：`make check && make e2e-mock`。

### P3：真实 Prometheus

7. `feat(contracts): adopt prometheus-main datasource`

   - UID、source、MemTotal labels、canonical expression 契约、调用方、fixture 和 `0003` migration。
   - Mock Adapter 与 Chart 使用 validation 返回的 canonical expression。
   - 验证：`make generate generated-client-diff validate-contracts test-sqlite test-assistant-mcp e2e-mock`。

8. `feat(mcp): enforce node exporter query registry`

   - 实现共享注册表、AST policy、Mock/Real 共用 Port contract suite。
   - 覆盖三条合法 query 和全部越界语法。
   - 验证：`make test-assistant-mcp test-ai-mcp`。

9. `feat(mcp): add real prometheus adapter`

   - HTTP Adapter、driver config、bootstrap、readiness、limits/error mapping、`httptest.Server` fixtures。
   - assistant-mcp `healthz` 只检查进程，http driver `readyz` 短超时探测 `/-/ready`。
   - 验证：`make test-assistant-mcp test-ai-mcp check`。

10. `test(e2e): add real prometheus topology`

    - 新增 real-metrics Compose overlay、Prometheus 配置、轮询等待脚本和 `make e2e-real-metrics`。
    - 固定镜像：

      ```text
      prom/prometheus:v3.13.1@sha256:3c42b892cf723fa54d2f262c37a0e1f80aa8c8ddb1da7b9b0df9455a35a7f893
      quay.io/prometheus/node-exporter:v1.12.0@sha256:9b0ade5e607f9dbedb0a8e11151b6011ae5bd79304c261804cfdd2cadf200a80
      ```

    - scrape interval 5 秒，target=`node-exporter:9100`；轮询 `up=1` 和 CPU idle 至少两个 scrape 样本，
      不使用固定 sleep。
    - README 说明 node_exporter 观测 Docker Linux VM/容器宿主视图，不声称是 macOS 内核。
    - 验证：`make check && make e2e-mock && make e2e-real-metrics`。

### P4：Eino/DeepSeek Agent

11. `feat(agent): add node exporter profile`

    - 新增 Profile、64 KiB loader、必需章节/UTF-8/禁止项测试和 Adapter-local view metadata。
    - 验证：Profile 定向测试与 `make check`。

12. `feat(agent): add constrained eino runtime`

    - 固定依赖，增加 fake model seam、三个严格 Tool、call counter、accumulator、最终 JSON 验证和 reasoning 丢弃。
    - 新增 `test-ai-agent` 并纳入 `make test`。
    - 验证：`make test-ai-agent test-ai-core-domain test-ai-mcp check`。

13. `feat(agent): wire deepseek configuration`

    - 接入 driver=`mock|eino`、DeepSeek model、Profile/key/limit 校验、Docker Profile copy 和 real-agent overlay。
    - 增量合并当前 `.env.example` 的用户修改，补完整变量和文件换行；绝不写真实 key。
    - 验证：bootstrap/config tests、Mock 无 key 启动、eino 缺 key/Profile 启动失败、`docker compose config`。

14. `fix(agent): harden real deepseek completion`

    - 把受限执行协议附加到 Profile，要求每个已完成视图先有成功的规范 `query_prometheus` 调用；兼容 JSON fence，
      仅在本地 accumulator 已有成功结果时接受普通文本终态。
    - SSE follow 仅在最后一个 durable event 已是 terminal event 后关闭，避免 Task 状态已终态但 terminal event
      尚未重放时截断客户端。
    - E2E SSE parser 只解析完整帧，不因读取到半帧而把成功流判为失败。
    - 验证：`go test ./internal/adapters/inbound/http ./internal/adapters/outbound/agent/eino ./internal/bootstrap`、
      `make e2e-real-agent`（加载用户提供的 key）。

### P5：端到端收口

15. `test(e2e): add real agent smoke`

    - `make e2e-real-agent` 在 key 为空时明确失败，不加入无凭证 `make check`。
    - 依次发送“概览”和“只看 CPU”，断言 3 图/1 图、注册表 PromQL、真实非空 series、工具 start/end 配对、
      刷新恢复和 SSE 终止。
    - 检查 API、SQLite export 和容器日志均不含 API key、Prometheus URL、private reasoning 或敏感 marker。
    - 最终标记 roadmap/本计划/progress completed，更新 overview、tree、README 和文档索引。
    - 验证：`make check && make e2e-mock && make e2e-real-metrics && make e2e-real-agent`。

## 4. 验证矩阵、回滚与完成标准

必须覆盖：

- Contract：OpenAPI、JSON Schema、MCP Tool Schema、examples、generated diff。
- Migration：正常 v1、失败 Task、孤立/歧义 Message、重复活动 Task、重复启动、FK 和整体回滚。
- 并发：两个不同 idempotency key 同时创建，仅一个成功；终态后允许下一轮。
- Pagination/Replay：时间戳 tie-break、跨页无遗漏、scope token、超过 200 events、固定 target、SSE 竞态、gap、EOF。
- Frontend：三轮、分页、刷新、活动重连、失败隔离和 Message/Event 去重。
- Prometheus：AST allowlist、所有 limits、非有限值、空 matrix、timeout、4xx/5xx 和错误脱敏。
- Eino：概览/单视图/unsupported、12 条上下文、严格 Tool JSON、重复 call ID、串行度 1、12 次调用、
  6 轮上限、final views/accumulator 一致性。
- 隔离：在 points、labels、reasoning 和上游错误中植入 marker，断言模型输入、ToolSummary、AgentEvent、
  Assistant Message 和日志均不包含 marker。

完成标准：

1. 同一 Session 能连续完成至少三轮，刷新后恢复 Message、Task、Chart 和失败信息。
2. 历史 replay 有限结束；活动 Task 从 durable sequence 继续 SSE。
3. Real Prometheus/node_exporter 按请求产生 CPU、内存或负载的非空数据；概览产生三图。
4. Eino/DeepSeek 只执行注册表内的 PromQL；越界请求不执行查询。
5. 聚焦请求只产生相应图表；unsupported 正常完成、零工具调用、零图表。
6. 完整时序、身份、内部 URL、secret 和 reasoning 不进入模型。
7. `make check`、`make e2e-mock`、`make e2e-real-metrics` 通过；有 key 时 `make e2e-real-agent` 通过。

回滚策略：每个 commit 是独立回滚点。新 driver 默认 Mock，P3/P4 可通过配置回退而不删除 Mock Adapter。
SQLite migration 为 forward-only：首次升级前备份 `data/ai-core.sqlite`；迁移失败自动整体回滚，成功升级后若要回退
旧二进制，必须同时恢复备份数据库。任何契约、迁移或数据隔离失败都停止在当前 Gate，不叠加后续功能。

本文件仅定义可执行规格；在 `implementationAuthorized: false` 时不得开始代码修改。
