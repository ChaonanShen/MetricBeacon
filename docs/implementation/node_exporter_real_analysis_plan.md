# node_exporter 真实分析与多轮会话路线图

> status: active
> createdAt: 2026-07-14
> lastReviewedAt: 2026-07-14
> implementationAuthorized: true
> scope: 在已完成的确定性 Mock 闭环上，建立持久化多轮对话，并跑通真实
> Prometheus/node_exporter 与最小 Eino Agent 的端到端演示。
> dependsOn: `code_skeleton_design.md`、`current_codebase_overview.md`、
> `basic_mock_skeleton_execution_plan.md`

## 0. 文档定位

本文是下一阶段的范围和 Gate 路线图，用来锁定能力边界、依赖顺序、风险控制与验收分层。
逐文件、逐提交的执行以已激活的
[`node_exporter_real_analysis_execution_plan.md`](node_exporter_real_analysis_execution_plan.md)
为准。

实施已满足以下前置条件：

1. G0 已在 ADR-018 锁定。
2. 已生成逐切片的详细执行计划。
3. 详细执行计划已标记为 `active` 并获得明确执行指令。

详细执行计划应把每个 Gate 拆成可独立验证的小提交，并列出具体契约、文件、migration、测试、
回滚点和演进文档更新项。

## 1. 完成目标与可演示语义

用户在 Grafana 工作台内创建或恢复一个 Session，连续发送多条自然语言消息；系统保存完整
对话和每轮分析 Task。Agent 读取一份静态 node_exporter Profile，通过真实 MCP 调用真实
Prometheus，生成所请求的 CPU 使用率、内存可用率和系统负载图表，并给出基于受限摘要的简短
结论。刷新页面后，消息、Task、图表和进行中 Task 的事件位置均可恢复。

目标链路：

```text
Browser -> Grafana Plugin -> Plugin Backend -> AI Core
        -> minimal Eino Agent -> assistant-mcp -> Prometheus -> node_exporter
        <- durable Session/Message/Task/Chart/TaskEvent + finite replay/SSE follow
```

确定性 Mock 链路必须继续保留，作为不依赖模型和实时指标的稳定回归基线。

### 1.1 第一版支持的自然语言范围

第一版不是任意 PromQL 生成器，只支持以下意图：

- node_exporter 概览：CPU、内存和负载三图。
- CPU、内存或负载中的一个或多个指定视图。
- 针对最近持久化轮次的简单追问，例如“只看 CPU”或“再看一下内存”。

模型可以生成 PromQL 候选，但候选必须经 Adapter 内的 PromQL AST 校验，并归一化为本 Profile
允许的查询语义。任意指标、任意函数、任意标签值拼接和任意数据源查询均不在第一版范围内。
超出范围时，Agent 返回用户可见的受限能力说明，不执行越界查询，也不伪造图表。

### 1.2 第一版 PromQL 注册表

|视图|允许的语义|规范 PromQL|图表单位|
|-|-|-|-|
|CPU 使用率|按 `instance` 计算非 idle CPU 百分比|`100 * (1 - avg by (instance) (rate(node_cpu_seconds_total{mode="idle"}[5m])))`|`percent`|
|内存可用率|按 `instance` 计算可用内存占总内存百分比|`100 * node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes`|`percent`|
|系统负载|展示每个 `instance` 的 1 分钟平均负载|`node_load1`|`short`|

允许的原始指标至少包括：

```text
node_cpu_seconds_total
node_memory_MemAvailable_bytes
node_memory_MemTotal_bytes
node_load1
```

`node_load1` 未结合 CPU 核数时只允许描述当前值和趋势，不能直接判断“过高”或“健康”。如果后续
要做归一化负载或阈值判断，必须显式扩展指标注册表和 Profile，不能只修改提示词。

## 2. 范围与非目标

本路线图包含：

- Session 内的持久化多轮对话、有限历史重放和当前轮 SSE follow。
- Session、Message、Task、Chart 与 TaskEvent 之间可稳定恢复的契约和存储调整。
- 真实 Prometheus HTTP Adapter、Prometheus 和 node_exporter 本地 Compose 拓扑。
- 一份只读的 node_exporter 静态 Agent Profile。
- 位于 AI Core outbound adapter 内的最小 Eino `ChatModelAgent` 集成。
- DeepSeek OpenAI-compatible API 的环境配置和显式启用模式。
- Mock、real-metrics 和 real-agent 三类分层验收路径。

本路线图不包含：

- 任意 node_exporter 指标和开放式 PromQL 生成。
- Knowledge MCP、向量检索、Skill、Playbook、模板和会话分享/Fork。
- PromQL/图表编辑、Dashboard 写入、审批和 delegation grant。
- Alertmanager Webhook、告警去重、自动建告警或 Agent 自动处置。
- Multi-Agent、Checkpoint/Interrupt、长期 Memory、Skill Middleware 或自定义无限 ReAct
  循环。

阈值解释可以写入 Agent Profile，但只能覆盖已有数据足以支持的解释。真正的告警系统另立计划。

## 3. 已锁定的实施决策

### 3.1 多轮领域模型

不新增复杂的 Conversation/Turn Aggregate：`Session` 是会话，`Task` 是一轮分析，`Message`
是用户或助手消息，`Chart` 属于产生它的 Task。每次用户发送消息仍创建一个 Task，并在同一事务
中保存 User Message、Task 和 `task.created`。

Message 增加稳定的 `taskId` 关联；Assistant Message 也关联其产生的 Task。前端以 Message
作为聊天历史事实来源，以 TaskEvent 作为执行过程事实来源。第一版限制每个 Session 同时只运行
一个非终态 Task。

`Task.inputMessageId` 在第一版保留以避免无关的破坏性重构。详细计划必须明确它与
`Message.taskId` 的一致性约束和 migration/backfill 方案，不能引入无法插入或无法迁移的循环
外键。

### 3.2 持久化和并发约束

- 相同幂等键和相同规范请求返回原 Task，不重复创建 Message 或事件。
- 相同幂等键但不同请求返回 `idempotency_conflict`。
- 同一 Session 的非终态 Task 唯一性必须由数据库约束兜底，不能只依赖“查询后插入”。
- User Message、Task 和 `task.created` 必须原子提交。
- Assistant Message、Chart、Execution 和对应事件仍遵循“先持久化，后通知”。
- 失败 Task 保留 User Message、Task 状态、错误和已持久化事件；不得静默删除失败轮。

推荐使用 SQLite partial unique index 约束同一 tenant/session 的非终态 Task。详细计划需要验证
该索引与状态迁移、幂等重试、进程恢复之间的交互。

### 3.3 历史读取和事件恢复

新增 Session 下的 Message/Task 分页读取契约，排序键必须稳定，至少使用 `createdAt + id`。
列表接口显式提供 `pageSize`、`pageToken` 和 `nextPageToken`，避免无限加载完整 Session。

历史 TaskEvent 需要有限重放语义，不能为每个终态 Task 永久保持 SSE：

- 终态 Task 使用有限 replay，读取到其 `latestSequence` 后关闭。
- 只有当前非终态 Task 使用 SSE follow。
- 可以通过 `follow=false` 参数或独立的有限事件读取接口实现；G0 必须先锁定契约形态。
- 有限 replay 仍从 durable TaskEvent Store 读取，不引入新的前端事实来源。

### 3.4 Agent 与持久化上下文

保留现有 `AgentRuntime` Port；确定性 Mock Agent 不变。真实实现使用最小 Eino Adapter，仅配置
一个 ChatModel、三个只读工具和有限轮数。Eino 类型不得进入 Domain、Application、HTTP 或
MCP 契约。

每轮 Agent 输入只包含：

- 静态 node_exporter Profile。
- 最近有限数量的持久化用户/助手消息。
- 当前用户消息、逻辑数据源标识和时间范围。
- 已脱敏、定长的工具结果摘要。

第一版建议最多带入最近 12 条消息，并同时设置字符或 token 上限；超过上限时按完整消息边界从
旧到新丢弃。当前 User Message 不能同时出现在“历史”和“当前消息”中。具体上限在 G0 固化。

以下内容不得发送给外部模型：

- Grafana token、租户/组织/用户身份、内部 URL、Trace 或 Request ID。
- API key、Prometheus 地址、SQLite 路径或其他 Secret。
- 完整原始时间序列。
- 模型私有 reasoning、内部错误堆栈或未脱敏工具输出。

### 3.5 工具输出和图表数据隔离

Eino 的查询工具不能把 MCP 返回的完整 `series/points` 原样交回 ChatModel。真实 Agent Adapter
必须把一次查询拆成两个视图：

```text
full QueryExecutionResult -> 仅保存在本次 Agent run accumulator -> 生成 ChartProposal
bounded ToolSummary       -> 返回 ChatModel -> 生成用户可见分析结论
```

`ToolSummary` 第一版最多包含：查询键、成功/失败、序列数量、截断后的 series label、每序列最新值、
最小值、最大值、平均值、样本数量和 warning。不得包含完整 points 数组。

每次 Agent 工具调用必须有稳定的 source call ID，用它关联 `tool.started`、`tool.completed` 或
`tool.failed`。不能只按 `toolName` 关联同名调用。第一版默认禁止并行工具执行；即便如此，持久化
关联也应使用 call ID，为以后解除限制保留正确语义。

### 3.6 静态领域说明

新增 `data/agent-knowledge/node_exporter.md`，将其视为 Agent Profile 配置而非 Knowledge 产品
能力。Profile 至少说明：

- 支持的自然语言意图和规范 PromQL。
- 指标类型、标签要求、时间范围和 step 约束。
- CPU、内存和负载的解释口径。
- 无数据、部分序列、非有限数值和查询失败的处理方式。
- 结构化计划与最终用户回复格式。
- 禁止的指标、函数、写操作和数据源访问。
- 不得输出或要求模型 private reasoning。

Profile 由 Eino Adapter 只读加载，设置文件大小上限并在启动时校验。本轮不注册
`knowledge.*` MCP 工具，也不建设检索或索引。

### 3.7 真实指标链路

`assistant-mcp` 继续通过 `PrometheusPort` 访问指标。Mock 和 Real HTTP Adapter 都实现该
Port，且只允许 Bootstrap 根据配置选择；业务层不得感知 Adapter 类型。

`datasourceUid` 是受控的逻辑标识，不能是用户提供的 URL。Mock 和 Real 模式应使用同一个稳定
逻辑 UID，避免前端或业务状态出现 `mockMode` 分支。具体 UID 字符串在 G0 锁定，并同步更新
OpenAPI、Tool Schema、生成类型、fixture 和前端默认值。

Prometheus endpoint 只来自 assistant-mcp 环境配置。Real Adapter 不接受请求覆盖 endpoint、
认证信息或租户映射。

### 3.8 DeepSeek 与 Secret 注入

真实 Agent 使用 DeepSeek OpenAI-compatible API。当前建议默认模型为
`deepseek-v4-flash`，允许通过环境变量显式切换到 `deepseek-v4-pro`；模型名称和 Base URL 以
[DeepSeek 官方 API 文档](https://api-docs.deepseek.com/)为准。

仓库根目录已有 `.env.example`，实际 `.env` 已被 `.gitignore` 忽略。实现时扩展示例但不写入
真实 key：

```dotenv
AI_CORE_AGENT_DRIVER=mock
AI_CORE_AGENT_PROFILE_PATH=/app/agent-knowledge/node_exporter.md
DEEPSEEK_API_KEY=
DEEPSEEK_BASE_URL=https://api.deepseek.com
DEEPSEEK_MODEL=deepseek-v4-flash
```

要求：

- `.dockerignore` 也必须排除 `.env`。
- Mock 和 real-metrics 模式不读取、不要求 DeepSeek key。
- real-agent Compose 只显式传递所需变量，不通过日志打印或 API 回显 Secret。
- 当 `AI_CORE_AGENT_DRIVER=eino` 时，缺少 key 或 Profile 应在启动时返回明确配置错误。
- 就绪检查只验证本地配置与依赖，不通过真实模型调用消耗额度。
- 运行中模型不可用时 Task 明确失败，不静默回退为 Mock 结果。

## 4. G0：详细计划前必须锁定的契约决策

G0 只做设计与契约决策，不实现业务代码。详细执行计划必须明确记录以下结果：

|决策|推荐默认|必须产出的记录|
|-|-|-|
|路线优先级|保持“多轮基础 -> 真实指标 -> 真实 Agent”|详细 Gate 顺序；若优先最快真实链路，可把 real-metrics 提前到前端多轮之前|
|逻辑 datasource UID|Mock/Real 共用一个中性 UID|OpenAPI、Tool Schema、fixture 与前端迁移范围|
|历史事件读取|有限 replay + 活动 Task SSE follow|HTTP/SSE 参数、终止条件和生成客户端影响|
|Message/Task 关联|保留 `inputMessageId`，新增 `Message.taskId`|SQLite migration、backfill、一致性和唯一性规则|
|Agent 上下文上限|最近 12 条消息并增加总长度上限|截断算法与单元测试|
|PromQL 能力|只允许 Profile 注册的三种视图|AST/注册表校验、越界错误语义|
|Tool 执行|稳定 call ID、第一版串行|AgentEvent DTO 与持久化关联方式|
|模型出域|仅发送 Profile、消息和摘要|字段白名单、日志/事件排除项|

如果上述任何决策会改变既有架构蓝图的接口、所有权或安全边界，必须先更新
`code_skeleton_design.md`，必要时增加 ADR，再开始实现。

## 5. 真实 Prometheus Adapter 细化要求

### 5.1 API 映射

Real Adapter 应通过 Prometheus HTTP API 实现现有 Port，建议映射如下：

|Port 操作|Prometheus 能力|第一版约束|
|-|-|-|
|SearchMetrics|指标名/metadata API|只返回注册表内且真实存在的指标；在 Adapter 内做确定性文本匹配和排序|
|GetMetricLabels|series/label API|只读取注册表指标；限制时间窗口、series 数和 sample value 数|
|Query validate|PromQL parser/AST|不执行查询；校验指标、函数、聚合、标签 matcher 和复杂度|
|Query execute|`query_range`|只允许校验通过的表达式；解析 matrix 并执行输出限制|

Real Adapter 的 Contract Test 使用本地 `httptest.Server` 模拟 Prometheus API，以便和 Mock
Adapter 共享可重复的 Port 契约，而不依赖实时指标数值。

### 5.2 第一版安全上限

以下数值是路线图默认值，G0 可以调整，但详细计划不能留空：

|限制|建议值|
|-|-|
|单次查询最大时间范围|6 小时|
|Prometheus HTTP 超时|10 秒|
|Agent 整体 Task 超时|60 秒|
|模型/工具轮数|最多 6 轮|
|总工具调用数|最多 12 次|
|单查询返回 series|最多 20 条|
|单查询总样本|最多 5,000 个|
|Prometheus 响应体|最多 2 MiB|
|返回模型的 ToolSummary|最多 16 KiB|

非有限 Prometheus 值必须被安全过滤并形成 warning，不能写入不合法 JSON。超过 series、样本、
响应体或时间限制时返回结构化错误，不能静默截断后伪装成完整结果。

### 5.3 local-real 拓扑

- 固定 Prometheus 和 node_exporter 镜像版本/摘要。
- Prometheus 以 Compose service name 抓取 node_exporter，不能使用容器内 `localhost`。
- node_exporter 的 host mount 和启动参数必须明确，使开发者知道采集对象是 Docker Linux VM/
  容器环境，而不是 macOS 本机内核。
- Prometheus 配置固定 scrape interval；real-metrics E2E 在执行 CPU `rate` 查询前至少等待两个
  scrape 样本。
- local-real 测试只断言数据存在、时间范围合理、查询和图表关联正确，不固定瞬时数值。

## 6. 执行 Gate

### P1：多轮会话契约和持久化

建议子切片：

1. 契约：Message `taskId`、Session Message/Task 分页、有限 TaskEvent replay。
2. 生成物：Go/TypeScript Client、HTTP server interface 和契约示例。
3. Domain/Port：关联和分页类型，不引入 HTTP/SQLite 类型。
4. SQLite migration：backfill、唯一性、并行 Task 数据库约束和迁移测试。
5. HTTP/Plugin Backend：租户隔离读取和有限重放代理。
6. Workflow：最近消息上下文组装和 Assistant Message 关联。

验收：

- 相同幂等请求不重复创建 Message。
- 会话隔离、稳定排序、分页、有限重放和数据库升级测试通过。
- 同一 Session 并发创建两个非终态 Task 时只有一个成功。
- 旧 Mock 数据库可以迁移，且不存在无法关联的 Message。

### P2：前端多轮工作台

建议子切片：

1. 状态改为 Session 级：消息、按 Task 分组的轮次、状态、图表和当前活动 Task。
2. 新消息不清空历史；只有当前非终态 Task 建立 SSE follow。
3. 以 `sessionId` 为主要 URL 恢复键；`taskId` 仅作为可选定位信息。
4. 刷新后读取 Message/Task 列表，对历史 Task 做有限事件重放。
5. 到达 Task `latestSequence` 后关闭历史 replay；活动 Task 从最后 sequence 继续 follow。
6. 失败轮显示用户消息、错误和已有进度，不污染其他轮次。

验收：连续三轮提交、刷新恢复、历史 replay 自动结束、活动 SSE 断线重连、失败轮展示，以及前端
reducer/route/SSE 单元测试通过。

### P3：真实 Prometheus 与 node_exporter

建议子切片：

1. 先修订 datasource、source metadata 和 PromQL 校验所需的 Tool Schema。
2. 实现 HTTP Client、Prometheus 响应 DTO、错误映射和安全上限。
3. 实现 SearchMetrics、GetMetricLabels、validate 和 query_range。
4. 让 Mock/Real Adapter 跑同一套 Port Contract Test。
5. 增加 Prometheus/node_exporter Compose、scrape 配置和独立 real-metrics 验收脚本。
6. 以 Deterministic Mock Agent 驱动真实三条查询，先隔离验证基础设施链路。

验收：

```text
Grafana -> Plugin -> AI Core -> Deterministic Mock Agent
        -> MCP -> Real Prometheus -> node_exporter
```

返回请求对应的非空真实序列；完整链路不依赖 DeepSeek key，`make e2e-mock` 继续通过。

### P4：静态 Profile 与最小 Eino/DeepSeek Agent

建议子切片：

1. 编写 Profile，补充读取、大小、必需章节和禁止项测试。
2. 定义 Eino Adapter 内部结构化 AnalysisPlan、ToolSummary 和 run accumulator。
3. 使用 fake ChatModel 验证 Eino 工具循环、call ID、串行限制、超限和 reasoning 丢弃。
4. 把三个工具包装到现有 MetricCatalog/QueryEngine Port；不得绕过 MCP Gateway。
5. 把完整 Execution 留在 accumulator，只把 ToolSummary 返回模型。
6. 接入 DeepSeek ChatModel 配置和显式 driver 选择。
7. 将 Eino 文本与工具事件映射为既有 AgentEventSink，保持事件先持久化后 SSE。

验收：

- fake ChatModel 的 Adapter Contract Test 可重复通过。
- 真实 DeepSeek 模式能根据概览请求完成三图，根据聚焦请求只产生对应图表。
- 越界 PromQL、额外数据源、并行/超量工具调用和完整 series 外发均被阻止。
- 模型不可用、MCP 不可用和 Prometheus 查询失败产生明确、可恢复展示的失败 Task。

### P5：端到端验收和演进记录

建立三个互不替代的入口：

|入口|Agent|Prometheus|API key|用途|
|-|-|-|-|-|
|`make e2e-mock`|Deterministic Mock|fixture Mock|不需要|稳定 CI 回归|
|`make e2e-real-metrics`|Deterministic Mock|真实 local Prometheus/node_exporter|不需要|基础设施与真实数据链路|
|`make e2e-real-agent`|Eino + DeepSeek|真实 local Prometheus/node_exporter|需要|显式启用的真实模型 smoke test|

real-agent 测试不能作为无凭证 CI 的必过门禁，也不能断言模型文本完全一致。它应断言结构化
AnalysisPlan、受限 PromQL、工具审计、图表数量/关联、数据非空、刷新恢复和敏感字段未出现。

每个完成 Gate 同步更新路线图/详细计划、进度记录、当前代码概览、代码树、README 和必要 ADR。
全部 Gate 通过后，详细执行计划和本路线图才能标记为 `completed`。

## 7. 配置与启动模式

### 7.1 assistant-mcp

建议配置键：

```dotenv
ASSISTANT_MCP_PROMETHEUS_DRIVER=mock
ASSISTANT_MCP_PROMETHEUS_URL=http://prometheus:9090
ASSISTANT_MCP_PROMETHEUS_DATASOURCE_UID=<G0-locked-logical-uid>
ASSISTANT_MCP_PROMETHEUS_TIMEOUT=10s
```

只有 Bootstrap 读取 driver 并选择 Adapter。Mock Adapter 忽略 endpoint；Real Adapter 缺少 endpoint
时启动失败。

### 7.2 AI Core

建议配置键：

```dotenv
AI_CORE_AGENT_DRIVER=mock
AI_CORE_AGENT_PROFILE_PATH=/app/agent-knowledge/node_exporter.md
AI_CORE_AGENT_MAX_ROUNDS=6
AI_CORE_AGENT_MAX_TOOL_CALLS=12
AI_CORE_AGENT_TIMEOUT=60s
DEEPSEEK_API_KEY=
DEEPSEEK_BASE_URL=https://api.deepseek.com
DEEPSEEK_MODEL=deepseek-v4-flash
```

Mock Compose 显式使用 `AI_CORE_AGENT_DRIVER=mock`；real-metrics 仍使用 Mock Agent；只有 real-agent
启用 `eino` 并注入 DeepSeek 配置。

## 8. 验证矩阵

|层级|必须覆盖|
|-|-|
|Contract|OpenAPI、JSON Schema、MCP Tool Schema、生成物 diff、Mock/Real fixture|
|Domain/Application|Message/Task 关联、并发状态、上下文截断、失败轮保留|
|SQLite|migration/backfill、partial unique index、租户隔离、分页、sequence|
|Plugin Backend|新历史接口、有限 replay/SSE follow、身份上下文和错误映射|
|Frontend|三轮状态、历史重放终止、活动重连、失败隔离、URL 恢复|
|Prometheus Adapter|HTTP mapping、AST 白名单、limits、NaN/Inf、错误响应|
|Eino Adapter|fake model、call ID、串行/轮数限制、摘要隔离、reasoning 丢弃|
|Mock E2E|原有确定性三图和刷新恢复|
|Real metrics E2E|真实非空序列、等待 scrape、无模型依赖|
|Real agent smoke|DeepSeek、受限计划/PromQL、真实图表、无敏感数据泄漏|

## 9. 完成标准

1. 一个 Session 可以连续保存并展示至少三轮用户/助手消息及其独立 Task。
2. 刷新后，已完成的消息和图表通过有限 replay 恢复；进行中 Task 从持久化 sequence 继续 SSE。
3. 真实 Prometheus/node_exporter 链路按请求产出 CPU、内存或负载非空图表；概览请求产出三图。
4. Eino Agent 使用静态 Profile 和受限只读工具完成分析，PromQL 不超出注册表。
5. 完整时序数据不进入外部模型上下文，private reasoning 不进入数据库、事件或日志。
6. Mock、real-metrics 和 real-agent 三类验收入口各自存在且职责清晰。
7. 缺少 key、模型不可用、MCP 失败、查询超限和无数据都有明确错误或用户可见说明。
8. `current_codebase_overview.md`、`current_code_tree.md`、详细计划、进度记录和 README 与代码一致。

## 10. 停止条件

以下情况必须停止实现并取得方向或记录 ADR，而不是自行扩展：

- 从受限三视图扩展为任意 PromQL 或任意 node_exporter 指标。
- 改变 Session/Task/Message 的数据所有权或移除既有公开关联字段。
- 允许同一 Session 并发非终态 Task 或并行 Agent 工具调用。
- 把完整时间序列、身份或内部地址发送给外部模型。
- 引入知识库/MCP namespace、Grafana 写入、告警接收或自动化处置。
- 把 real-agent 外部模型 smoke test 变成无凭证 CI 的稳定门禁。
- 新增不可逆存储结构，而 migration/backfill 和兼容策略尚未锁定。

## 11. 生成详细执行计划时的必备内容

后续详细计划至少要给出：

- 每个小提交的目标、文件清单、契约先后顺序和验证命令。
- G0 所有决策的最终值，不保留“实现时再决定”。
- SQLite migration SQL、旧数据 backfill、失败处理和测试夹具。
- 三个启动模式的 Compose 组合、环境变量和清理命令。
- Eino/DeepSeek 依赖版本、构造方式、fake model seam 和 Secret 处理。
- Prometheus HTTP API fixture、Port Contract Test 和 local-real 等待策略。
- 前端历史 replay 终止算法与活动 SSE 重连算法。
- 每个 Gate 需要同步更新的演进文档和是否需要 ADR。

只有上述详细计划经过确认后，才进入代码执行。
