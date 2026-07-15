# 当前骨架代码说明

> 本文以 2026-07-15 的实际代码为准，说明已可工作的同步 Agent 规划与有界 node_exporter 查询闭环；长期设计请参照
> [`code_skeleton_design.md`](code_skeleton_design.md)。

## 当前可演示的能力

> 已完成的 [`grouped_chart_canvas_execution_plan.md`](grouped_chart_canvas_execution_plan.md)
> 已取代 `chart_trio_ui_fit_plan.md` 留下的自由多列布局；历史文件仅保留原验证证据。
>
> 当前局部 UI 工作由
> [`fresh_conversation_workbench_execution_plan.md`](fresh_conversation_workbench_execution_plan.md)
> 跟踪；它不改变服务、契约、持久化或图表画布边界。

用户在 Grafana App Plugin 中只提交自然语言。AI Core 同步调用 Mock 或 Eino IntentPlanner，将注册 views 与可选 range/step 与 API hint/本地默认值合并，校验后冻结持久化 QueryPlan。后台仅执行这份计划的 `cpu|memory|load` views，PromQL 由 assistant-mcp 注册表编译，数值回复由实际查询结果在本地汇总。

```text
Grafana 浏览器前端
  -> Plugin Resource API
  -> Grafana Plugin Backend（认证上下文、受控代理）
  -> AI Core（QueryPlan、Session/Task 工作流、SQLite、SSE）
  -> assistant-mcp（view/window 注册表、grafana.* 只读工具）
  -> Mock fixture 或真实 Prometheus/node_exporter

SSE TaskEvent 按原路径回传到前端，前端恢复状态并渲染 Agent 所选的一至三张图。
```

## 模块与职责

|位置|职责|当前实现边界|
|-|-|-|
|`contracts/`|跨进程协议的单一来源：Plugin Resource API、AI Core API、SSE 事件、MCP Tool Schema、错误码和示例。|通过 OpenAPI/JSON Schema 校验，并据此生成 Go/TypeScript 类型；业务模块不应另写重复 DTO。|
|`packages/generated-clients`、`packages/generated-contracts`|契约生成物。前者是 AI Core HTTP Client，后者是 Grafana MCP 工具类型。|由 `scripts/generate-clients.sh` 生成，`make generated-client-diff` 检查生成结果可复现。|
|`packages/request-context-go`|跨服务传递的租户、组织、用户、角色、权限、请求与 Trace 上下文。|Plugin Backend 从 Grafana 请求上下文生成它；浏览器传入的身份头不会被信任。|
|`packages/testkit-go`|测试用的确定性时钟与 ID 生成器。|仅为测试可重复性服务。|
|`services/ai-core/internal/domain`|核心领域模型和状态规则：Session/Message、AnalysisTask/TaskEvent/ToolCall、Chart/Execution、绝对时间范围、有效 QueryPlan 与领域错误。|QueryPlan 只接受注册 step 和 CPU rate window；不依赖数据库、MCP、Grafana 或模型 SDK。|
|`services/ai-core/internal/application` 与 `internal/ports`|命令服务、有限查询意图解析和分析工作流；Port 定义存储、事件通知、Agent、工具、时钟和 ID 等外部能力。|Task 创建前使用注入时钟解析最近 30 秒至 6 小时、auto/显式 step 与 CPU window；工作流把同一 QueryPlan 传给 Agent/MCP，并先持久化状态/事件再通知 SSE。|
|`services/ai-core/internal/adapters`|将 Port 接到具体实现：SQLite、内存通知器、MCP 客户端、系统时钟/随机 ID、确定性 Mock Agent 与受限 Eino Agent。HTTP 入站 Adapter 暴露会话、任务、读取与 SSE 重放接口。|AI Core 是 Session、Task、Event、Chart 和 SQLite 数据的唯一所有者。它不直接读取 fixture，也不承载 Grafana 鉴权。|
|`services/ai-core/internal/bootstrap`|组装依赖：SQLite Store、Mock 或显式 opt-in Eino/DeepSeek Agent、MCP Gateway、工作流、HTTP API。|默认 `AI_CORE_AGENT_DRIVER=mock` 不读取 API key；`eino` 启动时必须有 Profile 与 key，固定模型/任务/MCP 限制，`/readyz` 只检查 SQLite 和 MCP 工具，不请求模型。|
|`services/assistant-mcp`|以 Streamable HTTP（`/mcp`）暴露只读的 `grafana.*` MCP 工具：`search_metrics`、`get_metric_labels`、`query_prometheus`。|查询工具只接受注册 view、可空 CPU window、范围和 step；工具先做权限和 Schema/点数预算校验，再调用 Prometheus Port。该服务不拥有 AI Core 的任务或数据库。|
|`services/assistant-mcp/internal/adapters/prometheus/mock`、`http`|Prometheus Port 的 Mock 与真实 HTTP 实现。|默认 Mock 是唯一允许读取 `data/mock-scenarios` 的代码，并将 fixture 确定性重采样到请求范围/step；opt-in HTTP Adapter 只执行本地注册表生成的 CPU `[30s]/[1m]/[5m]`、内存和 load PromQL，并执行响应、时间范围、step、点数和基数上限。|
|`data/agent-knowledge/node_exporter.md`、`services/ai-core/internal/adapters/outbound/agent/profile`、`agent/eino`、`agent/localresult`|只读 node_exporter Agent Profile、受限 Eino AgentRuntime 与本地结果 formatter。|Profile 限制为 UTF-8、64 KiB，并校验三视图、解释、无数据/错误、最终状态和禁止项。Eino 的 query Tool 只接受 `view`，datasource/range/step/window/PromQL 全由本地注入；模型提交 expression 会在查询前拒绝。有成功 query proposal 时，成功工具调用是权威视图选择，模型终态文本被忽略；零 proposal 的 unsupported 路径仍要求严格 JSON。完整时序留在本地，持久化事实回复由 Mock/Eino 共用 formatter 根据 QueryPlan 与本地结果生成。Bootstrap 仅在显式 `eino` driver 下构造 thinking-disabled 的 DeepSeek model，并从镜像内只读 Profile 路径加载。|
|`apps/grafana-plugin/frontend`|React/Grafana 三栏工作台：创建/恢复 Session、提交自然语言、分页读取 Message/Task、有限重放 SSE，并把执行结果映射为 Grafana DataFrame 与时序图。|左栏只有自然语言输入；中栏按 Task oldest-first 分组，宽画布最多两列、奇数尾图跨行，窄容器单列，并保护新 Task/恢复/加载更早记录的选择与滚动；右栏只读显示 QueryPlan 和图表详情。|
|`apps/grafana-plugin/backend`|Grafana Plugin SDK 的薄 Resource API 层。|从 Grafana 上下文提取身份、读取 `aiCoreEndpoint` 配置，代理 Session、Message/Task 历史、有限事件重放与 SSE 字节流，并映射错误；不持久化业务数据、不调用 MCP。|
|`data/mock-scenarios/node_exporter_overview`|确定性场景数据：指标搜索、标签、三条查询结果、期望事件。|只供 MCP 的 Mock Prometheus Adapter 使用，并受 Schema 校验。|
|`scripts/`、`Makefile`、`tests/e2e/`、`tests/diagnostics/`|工程门禁、代码生成、契约/边界检查、分层诊断与端到端验收入口。|`compose.mock-e2e.yaml` 启动 Mock 栈；`compose.real-metrics-e2e.yaml` 叠加 Prometheus 与 node_exporter。`make diagnose-real-metrics` 只启动指标侧三服务，先直接验证三条 Prometheus vector，再通过真实 MCP transport 验证搜索、标签和三条 matrix 查询；两阶段都检查 instance、递增时间、有限值、CPU/内存 0..100、load 非负和基数上限，并只输出 series/samples/min/max/latest。诊断默认使用宿主 `18081`，可与占用默认 `8081` 的手工栈共存。`make diagnose-deepseek` 则绕过业务链路验证配置模型和最小严格 JSON 回复。指标诊断与两个真实 E2E 共用 target/two-scrape 就绪判定。Real-metrics 浏览器验证图表渲染和恢复，不复用 Mock fixture 的实例标签断言；真实 series 由 API E2E 覆盖。`make e2e-real-agent` 再叠加 opt-in Eino/DeepSeek，要求显式 key，并检查概览/CPU、重放恢复、工具配对和 API/日志/SQLite 的泄漏标记。|

## 关键数据与依赖边界

- AI Core 独占业务持久化。SQLite 迁移在 `services/ai-core/migrations/sqlite/`，Plugin 和 MCP 都不能直接读写它。`0004` 为 Task 回填并持久化有效 step/CPU rate window，为历史 Chart query 回填 step，并为 Execution 增加可空的实际样本范围。每条 Message 已持久化关联其 Task：User Message 与 `Task.inputMessageId` 双向一致，Assistant Message 也归属产生它的 Task；迁移会拒绝无法无歧义关联的旧数据。
- 同一 tenant/Session 最多允许一个非终态 Task，SQLite partial unique index 是并发竞争的最终约束；进程内顶层写事务串行进入 SQLite，避免锁竞争掩盖该业务冲突。工具审计以内部稳定 source call ID 关联 start/completed/failed 记录，Mock Runtime 使用可重复的 source call ID。
- AI Core 与 Plugin Resource API 都提供按 `createdAt DESC,id DESC` 的 Session Message/Task keyset 分页，以及固定 `targetSequence` 的有限 JSON TaskEvent replay。page token 绑定资源类型及 Session/Task，不能跨接口或跨资源复用；Plugin 由 Grafana 身份上下文覆盖浏览器伪造的身份头。
- 前端以 Session 级 Message/Task/runtimes 状态恢复工作台，历史事件重放至固定目标序列；只有非终态 Task 建立一个 SSE，收到终态事件后关闭。重复事件会忽略，sequence 间隙会从最后连续序列重连。若 URL Session 在当前独立 AI Core volume 中明确不存在，前端会清除两个 workbench 路由 ID 和旧 reducer/replay/幂等状态；网络、权限与依赖错误不会触发该恢复，而会显示安全分类。
- 每轮 AgentRuntime 接收当前 User Message 之外、按时间正序的最近至多 12 条持久化 User/Assistant 消息，并按完整消息边界限制在 12,000 个 Unicode 字符内；当前消息超过 4,000 个 Unicode 字符会被拒绝。SSE 已在终态 Task 的 durable events 排空后主动关闭。
- Mock 只位于 Adapter 层：Mock Agent 在 AI Core 的出站 Adapter，Mock Prometheus 在 MCP 的出站 Adapter；领域和工作流中没有 `mockMode` 分支。
- Mock 与真实 Adapter 共用逻辑数据源 UID `prometheus-main`。Task、Chart 和 MCP Tool 契约均限制为该 UID；SQLite `0003` 会前移迁移历史 Task 及 Chart query JSON 中的旧 UID。Agent 只提交 view，查询验证结果返回 assistant-mcp 生成的规范 PromQL，AI Core 只将该返回值持久化到 Chart。
- Prometheus Adapter 层的 node_exporter 注册表是 CPU、内存和负载规范 PromQL 的唯一来源。跨服务输入不再携带 expression：CPU view 必须携带 `30|60|300` 秒 window，memory/load 必须为 `null`；注册表渲染表达式并用 Prometheus AST 检查节点/selector 上限。Mock 和 opt-in HTTP Adapter 共享 view/window、注册 step、6 小时范围和 1,000 理论点上限，越界参数在访问数据前返回 `schema_validation_failed`。HTTP Adapter 只从服务环境取得 endpoint，禁用 redirect，限制请求为 10 秒、最多 20 series/5,000 samples/2 MiB 解压响应；`/healthz` 不访问依赖，HTTP 模式 `/readyz` 短探测 `/-/ready`。
- SSE 事件带有 Task、Session 和单调递增 sequence。事件先写入 durable store，客户端可通过 `afterSequence` 或 `Last-Event-ID` 获取断线后的后缀。
- 前端只能访问 Grafana Plugin Resource API；它不直连 AI Core、MCP 或 Prometheus。
- `scripts/check-boundaries.sh` 会阻止 AI Core 的 domain/application/ports import 外部 SDK、Adapter 或 Mock fixture。

## 尚未实现的范围

有界查询参数切片的契约、解析/持久化、MCP 动态执行、view-only Agent、本地可信回复、Workbench 控件及三模式端到端验收均已完成。以下仍是明确的非目标：任意 PromQL、图表编辑/重跑、Dashboard 写入与审批、真实 Grafana 写权限、知识库/Skill/Playbook、会话分享/Fork、告警和其他数据源。

## 上手使用

### 环境准备

仓库锁定的版本是 Go `1.26.5`、Node.js `22.23.1` 和 npm `10.9.8`。体验完整 UI 还需要可用的 Docker Engine 与 Docker Compose。首次使用先安装前端依赖并执行启动检查：

```sh
cd apps/grafana-plugin/frontend && npm ci
cd ../../..
make bootstrap-check
```

`make bootstrap-check` 会明确报告版本不一致、Go 依赖/编译问题、前端类型问题或 AI Core 分层违规。

这里的各命令并不启动业务服务：`npm ci` 严格按锁文件安装前端依赖，`bootstrap-check` 负责确认本机能编译和测试当前骨架。需要看到实际页面时，再执行下一节的 Compose 启动命令。

### 选择 Compose 运行模式

三种模式共用同一个 Grafana 工作台和持久化链路，通过 Compose overlay 逐层替换 Adapter：

|模式|Compose 文件|Agent|指标数据|额外要求|
|-|-|-|-|-|
|确定性 Mock|`compose.mock-e2e.yaml`|Mock Agent，固定生成 CPU、内存、负载三视图|fixture|无外部服务或模型凭证|
|真实指标|Mock 基础文件 + `compose.real-metrics-e2e.yaml`|Mock Agent，仍固定生成三视图|本地 Prometheus 抓取 node_exporter|Docker 可运行 Prometheus/node_exporter|
|真实 Agent|上述两个文件 + `compose.real-agent-e2e.yaml`|受限 Eino/DeepSeek Agent|本地 Prometheus 抓取 node_exporter|`.env` 中配置 `DEEPSEEK_API_KEY`|

三种模式都先构建 Plugin 前端：

```sh
(cd apps/grafana-plugin/frontend && npm run build)
```

#### 模式一：确定性 Mock

```sh
docker compose -p mini-torchbearing-mock-manual \
  -f compose.mock-e2e.yaml \
  up --build --wait
```

该模式启动 `grafana`、`ai-core`、`assistant-mcp` 三个容器。assistant-mcp 从 `node_exporter_overview` fixture 返回确定性结果，适合离线开发、页面调试和回归验证。

#### 模式二：真实 Prometheus/node_exporter

```sh
docker compose -p mini-torchbearing-real-metrics-manual \
  -f compose.mock-e2e.yaml \
  -f compose.real-metrics-e2e.yaml \
  up --build --wait
```

overlay 会新增 Prometheus 与 node_exporter，并把 assistant-mcp 的 Prometheus Adapter 切换为 HTTP。Agent 仍是确定性的 Mock，因此固定生成三视图，但每个视图都使用本轮 QueryPlan 的范围、step 和 CPU window；series 来自当前 Docker Linux VM/容器宿主，而不是 fixture 或 macOS 内核。Prometheus 每 5 秒抓取一次；首次分析前可等待至少两个 scrape。

#### 模式三：真实 Eino/DeepSeek Agent

先从 `.env.example` 创建本地 `.env` 并填写 `DEEPSEEK_API_KEY`；不要提交 `.env`。Docker Compose 会从仓库根目录的 `.env` 读取变量用于 overlay 插值：

```sh
docker compose -p mini-torchbearing-real-agent-manual \
  -f compose.mock-e2e.yaml \
  -f compose.real-metrics-e2e.yaml \
  -f compose.real-agent-e2e.yaml \
  up --build --wait
```

该模式同时使用真实 node_exporter series 与受限 Eino/DeepSeek Agent。可尝试“查看 node_exporter 概览”或“只看 CPU”；Agent 只能选择 CPU、内存、负载 view，不能提交 PromQL 或修改有效查询参数。模型只接收 Profile、受限会话上下文和本地统计摘要，不接收完整时序、身份、内部 URL 或密钥；用户可见数值回复由本地 formatter 生成。

所有模式都会为 AI Core 挂载独立 named volume，所以容器运行期间 Session、Task、Event、Chart 和执行结果会保留；执行带 `-v` 的清理命令才会移除该模式的数据。示例命令使用不同 Compose project 名，因此从一种模式切换到另一种模式时，旧 URL 的 Session 不属于新 volume；Workbench 收到明确的 `resource_not_found` 后会移除旧 `sessionId`/`taskId` 并提示重新提交。前端没有单独的开发服务器，页面和 Plugin Backend 都由 Grafana 容器提供。

容器均就绪后：

1. 打开 `http://127.0.0.1:3000`，用 `admin` / `admin` 登录。
2. 访问 `http://127.0.0.1:3000/a/mini-torchbearing-app/workbench`。
3. 输入任意非空分析请求，例如“帮我看看 node_exporter 最近 30 分钟的 CPU、内存和系统负载”，点击“开始分析”。
4. Mock/真实指标模式应看到固定助手说明和三张图；真实 Agent 的“概览”应产生三张图，“只看 CPU”应只产生 CPU 图。刷新页面后，URL 中的 Session/Task 标识会使页面恢复结果。

Mock 和真实 Eino 模式都通过同一 IntentPlanner Port 生成受限计划；二者都由同一确定性执行器按持久化 views 顺序查询。

### Mock 模式点击“开始分析”后实际发生的事

下面是手动输入一段文本后的一次完整链路。理解这条链路基本就能把握当前骨架的运行方式。

|阶段|输入与去向|内部处理|可见/持久化输出|
|-|-|-|-|
|1. 创建会话|前端在没有 URL Session 时调用 `POST .../resources/sessions`，标题固定为 `Node exporter mock analysis`。|Plugin Backend 从 Grafana 登录态构造用户/组织/权限上下文，并代理到 AI Core。AI Core 在 SQLite 中创建 Session。|浏览器获得 `sessionId`。|
|2. 创建任务|前端只把用户输入、`datasourceUid: prometheus-main` 和新的 idempotency key 发送到 `POST .../resources/tasks`。|AI Core 在已完成幂等预检后同步规划 views/range/step，本地校验并冻结 QueryPlan；Planner 失败不写入 Message、Task 或幂等记录。|浏览器获得 `taskId`；右栏只读显示最终生效参数。|
|3. 启动工作流|Task 提交成功后，AI Core 在事务提交后异步运行工作流。|Task 依次进入 planning、running_tools、validating、completed；每次状态改变及后续事件都先写 SQLite，再通知 SSE 订阅者。|任务状态会实时变化，即使 SSE 断开也能从数据库重放。|
|4. 确定性执行|工作流读取 Task QueryPlan.views。|按持久化顺序为每个 view 各调用一次 Validate/Execute；不再让模型二次选图，unsupported 零查询完成。|最终回复由本地 formatter 按实际范围、step/window 和样本统计生成。|
|5. 真实 MCP 通信|AI Core 的 MCP Gateway 通过 HTTP 调用 assistant-mcp，并携带从 Grafana 派生的身份上下文。|每个 persisted view 调用 1 次 `grafana.query_prometheus`；search/labels 仅保留为独立诊断能力。|SSE 出现一一配对的 `tool.started` / `tool.completed`。|
|6. Mock 数据读取|MCP 的 Mock Prometheus Adapter 按注册 view 从 `data/mock-scenarios/node_exporter_overview` 读取基准数据。|查询不访问真实 Prometheus；fixture 会按本轮绝对范围和 step 确定性重采样，CPU 表达式由本轮 window 渲染。|每个查询返回具有精确首尾范围和 step 间隔的 matrix；未知 view/window 会在读取 fixture 前拒绝。|
|7. 图表与页面恢复|AI Core 为三项结果创建 Chart 和 Execution，并写入事件流；前端订阅 `.../events?afterSequence=N`。|前端 reducer 只接收连续 sequence，缺号或断线会从最后序号重连；mapper 将持久化 series 转成 Grafana DataFrame 并交给 `TimeSeries`。|出现三张图及 PromQL 折叠区。刷新页面时，前端从 URL 读取 ID，再从 sequence 0 重放事件，因此能恢复相同结果。|

上述表格完整描述 Mock 模式。在真实指标模式中，第 4 步仍使用固定计划，第 6 步改为 assistant-mcp 通过 HTTP 查询本地 Prometheus；在真实 Agent 模式中，第 4 步改为 Eino/DeepSeek 根据受限 Profile 选择视图，并且每个完成视图都必须有成功的本地 `query_prometheus` 调用。三种模式的 Session/Task 持久化、Plugin Resource API、事件顺序、有限 replay 和 SSE 恢复链路保持一致。

运行中可按启动时使用的 project 名查看日志。例如：

```sh
docker compose -p mini-torchbearing-mock-manual \
  -f compose.mock-e2e.yaml \
  logs -f
```

结束体验时必须使用相同的 project 名和 Compose 文件组合。以下分别清理三种模式的容器及 AI Core 数据：

```sh
docker compose -p mini-torchbearing-mock-manual \
  -f compose.mock-e2e.yaml \
  down -v --remove-orphans

docker compose -p mini-torchbearing-real-metrics-manual \
  -f compose.mock-e2e.yaml \
  -f compose.real-metrics-e2e.yaml \
  down -v --remove-orphans

docker compose -p mini-torchbearing-real-agent-manual \
  -f compose.mock-e2e.yaml \
  -f compose.real-metrics-e2e.yaml \
  -f compose.real-agent-e2e.yaml \
  down -v --remove-orphans
```

### 单独调试后端服务

不需要 Grafana UI 时，可以在两个终端分别运行 MCP 与 AI Core：

```sh
# 终端 1
cd services/assistant-mcp && go run ./cmd/server
```

```sh
# 终端 2
cd services/ai-core
AI_CORE_SQLITE_PATH=/tmp/mini-torchbearing-ai-core.sqlite \
ASSISTANT_MCP_ENDPOINT=http://127.0.0.1:8081/mcp \
go run ./cmd/server
```

随后可检查健康状态：

```sh
curl -i http://127.0.0.1:8081/readyz
curl -i http://127.0.0.1:8080/readyz
```

assistant-mcp 会从当前目录向上寻找 fixture 和 Tool Schema，并在 `/mcp` 暴露 Streamable HTTP；AI Core 启动时会迁移 `/tmp/mini-torchbearing-ai-core.sqlite`，且 `/readyz` 会通过 MCP 实际列出工具，所以它同时验证 SQLite 与服务间通信。若 MCP 尚未启动或三个工具未注册，AI Core 的就绪检查会失败。这条路径方便观察服务启动与依赖故障；若要通过浏览器发起任务，仍应使用前一节的 Compose 方式，因为 Plugin Backend 由 Grafana 承载。

## 测试与检查入口

按后端层级执行、判读安全摘要和定位失败时，使用
[`real_backend_test_matrix.md`](real_backend_test_matrix.md)；其中的真实数值示例仅是一次运行证据，不是固定断言。

|命令|覆盖内容|本次结果（2026-07-14）|
|-|-|-|
|`make bootstrap-check`|固定 Go/Node/npm 版本；三个运行时 Go 模块全量编译测试；前端 typecheck；依赖边界。|通过。|
|`make test-ai-core-domain`|AI Core 领域、应用和 Port 的单元测试。|由 `make check` 通过。|
|`make test-sqlite`|SQLite Store 与内存事件通知器：CRUD、租户隔离、事务/幂等、sequence 与重放。|由 `make check` 通过。|
|`make test-assistant-mcp`|Mock Prometheus Adapter、MCP 接线和工具调用。|由 `make check` 通过。|
|`make test-ai-mcp`|AI Core MCP Gateway/查询 Adapter、HTTP API、分析工作流与 SSE。|由 `make check` 通过。|
|`make test-ai-agent`|受限 Eino Runtime：fake model、view-only Tool JSON、source-call 配对、expression 查询前拒绝、成功 proposal 权威性、本地 formatter 与模型输入摘要隔离。|通过。|
|`make e2e-real-agent`|有凭证的真实 Agent 验收：真实 CPU/内存/负载图、单 CPU 追问、durable tool 配对、有限 replay 与 API/日志/SQLite 泄漏检查。|通过：概览 21 events/3 query tool calls/3 charts，CPU 追问 13 events/1 query tool call/1 chart；调用进程从 `.env` 临时加载 key，未输出或持久化 key。|
|`make test-plugin-backend`|Grafana Resource API 代理、身份上下文、错误与 SSE 转发。|由 `make check` 通过。|
|`make test-frontend`|Vitest 工作台状态、SSE、路由、Resource 错误、查询控件、时间范围和 DataFrame mapper；随后 TypeScript typecheck。|通过：8 个测试文件、20 个用例。|
|`make test-diagnostics`|用 fake response/server 离线校验 Prometheus、指标语义、durable Task/Event/Chart 结果和 DeepSeek 探针的成功/失败分类，并检查真实指标诊断与 E2E Shell 语法。|通过：35 个 Node 测试。|
|`make test`|上述 Go 和前端测试的聚合入口。|由 `make check` 通过。|
|`make validate-contracts`|3 份 OpenAPI、24 份 JSON Schema 与 node_exporter fixture。|通过。|
|`make generated-client-diff`|重新生成 Client/类型后确认 Git 无差异。|通过。|
|`make lint`|Go 格式检查和前端 typecheck。|通过。|
|`make boundary-check`、`make secret-scan`|AI Core 依赖边界和常见私钥/AKIA 模式扫描。|通过。|
|`make check`|除容器 E2E 外的完整质量门禁：生成物、契约、lint、`make test`、边界与密钥扫描。|通过。|
|`make e2e-mock`|构建前端与三个容器；API E2E 覆盖 30 秒、1 分钟、30 分钟、显式 5 秒和默认三视图五种输入，校验 QueryPlan、精确点数/间隔/首尾、Chart/Execution、本地回复、有限 replay 与 SSE；Playwright 验证控件、连续提交、刷新及 stale-Session 恢复。|通过；五种输入分别生效为 `30s/5/30`、`1m/5/30`、`30m/10/60`、`5m/5/30` 和默认 `30m/10/60`。|
|`make e2e-real-metrics`|在同一应用栈叠加 Prometheus/node_exporter，等待真实 target 与 CPU idle 两次 scrape 后对相同五种输入执行 API/浏览器 E2E。|通过；每种输入均返回非空有限且按有效 step 对齐的真实 series，实际短历史由 Execution 如实记录；浮点秒边界仅在真实模式测试中允许 1 秒容差。|
|`make diagnose-real-metrics`|绕过 Grafana 与 AI Core，分阶段检查原始 Prometheus 与 assistant-mcp 的真实返回及指标语义。|通过：三条 vector/matrix 各 1 series/1 sample；CPU 约 98.2..98.7，内存约 64.64，load 3.25，均通过语义校验。|
|`make diagnose-deepseek`|绕过 Agent/MCP，验证配置 model 出现在 `/models` 并返回固定严格 JSON。|通过：`deepseek-v4-flash` 在 539 ms 返回 `{"status":"ok","answer":"pong"}`；未输出 key。|

日常开发先运行 `make check`。三种完整链路的自动验收入口分别是：

```sh
make e2e-mock
make e2e-real-metrics

set -a
. ./.env
set +a
make diagnose-deepseek
make e2e-real-agent
```

`make diagnose-deepseek` 和 `make e2e-real-agent` 都不会自动读取 `.env`，所以上例只把 `.env` 导出到当前 shell 后再执行；脚本不会输出 key。三个 E2E 入口都会自行构建前端、启动对应 Compose 组合、运行该模式的 API/浏览器或真实 Agent 验收，并在结束时删除它创建的容器与 volume，因此适合验收，不适合在命令结束后继续手动浏览。

若已按“选择 Compose 运行模式”手动启动 Mock 容器，可分阶段执行相同的 Mock E2E 用例：

```sh
tests/e2e/mock/api-e2e.sh
(cd apps/grafana-plugin/frontend && npm run test:e2e)
```

只调试某一层时，可使用表格中的 `make test-ai-mcp`、`make test-assistant-mcp`、`make test-plugin-backend` 或 `make test-frontend`；完整单元测试聚合入口为 `make test`。
