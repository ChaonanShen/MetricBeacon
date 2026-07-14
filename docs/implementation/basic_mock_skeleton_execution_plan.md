# Grafana 自然语言指标分析工作台：基本 Mock 闭环执行计划

> 文档状态：Completed / Historical record
> 版本：v1.2
> 适用范围：首个基本骨架实现；完成确定性 Mock 纵向闭环
> 依赖设计：[`code_skeleton_design.md`](code_skeleton_design.md) v1.3
> 最后更新：2026-07-13

> 2026-07-14：G0–G8 与后续 remediation 均已完成并复核；验证证据见
> [`basic_mock_progress.md`](basic_mock_progress.md)。本文件保留为
> 历史实现记录，不再是新工作的执行入口。当前文档路由和下一阶段状态见
> [`../CLAUDE.md`](../CLAUDE.md)。

---

## 0. 历史执行入口（请勿作为新工作指令）

本节保留原始执行指令，用于解释已完成的 G0–G8 验收证据；它不适用于当前或后续开发。

计划编写时，本节是新会话的唯一必读入口。执行者应完整读取本文件，然后从 Gate G0 开始实施；除非遇到本节定义的停止条件，不需要重新询问产品范围、模块划分、Mock 场景、命名或验收口径。

### 0.1 原始可复制的新 Session 指令

```text
在 /Users/a1111/proj/mini-torchbearing 中，严格按照
docs/implementation/basic_mock_skeleton_execution_plan.md v1.2 实现基本 Mock 闭环。

先读取 CLAUDE.md 和该执行计划全文，检查工作区并保留已有修改；然后从 G0 开始，按
G0 → G1 → ... → G7 → G8 的顺序持续实施和验证。不要扩展到真实 Eino/LLM、真实
Prometheus/node_exporter、Grafana Dashboard 写入、知识库、Playbook 或 Alert。

接口和契约优先于功能：Frontend → Plugin Backend、Plugin Backend → AI Core、
AI Core → assistant-mcp 必须走正式 OpenAPI/MCP Schema 和生成类型。Mock Agent 不得直接
读取 fixture 或返回最终图表，必须通过类型化 Port 和真实 MCP transport 调用
MockPrometheusAdapter。

每完成一个 Gate，运行该 Gate 的验证命令，并更新
docs/implementation/basic_mock_progress.md。安全且不改变边界的实现细节使用计划中的默认值自行
决定，不要停下来询问。只有缺少下载/安装权限、发现会改变模块边界的设计冲突，或现有用户
修改与计划直接冲突时才暂停并报告。未经额外明确授权不要创建 git commit。
```

### 0.2 原始新 Session 只需读取的资料

必读：

1. `CLAUDE.md`：仓库级约束和文档优先级。
2. 本文件全文：当前执行范围、默认决策、文件清单、步骤和验收。

按需读取：

- `docs/implementation/code_skeleton_design.md`：只有本文件引用的 Port/领域字段仍需展开，或发现接口缺口时读取相应章节。
- `docs/adr/ADR-017-grafana-delegation-grant.md`：本阶段不做 Grafana 写入，通常不需要读取。

默认不需要重新阅读 `design/original_task.md`、`design/product_design.md`、`design/arch_design_draft.md` 和完整 `design/arch_design_detail.md`。本文件已经把当前 Mock 切片需要的决策收敛在一起；不得从长期文档中捞取后续功能扩大范围。

### 0.3 已知仓库起点

计划编写时的仓库状态如下；新 Session 必须复核，不能盲目假设状态未变化：

- 工作目录：`/Users/a1111/proj/mini-torchbearing`。
- 当前分支：`main`。
- 基线提交：`f565739`（补充本地 Docker 混合 E2E 设计）。
- 仓库当前只有文档和 ADR，没有实际应用代码、Go module、Frontend package、Compose 或测试。
- 当前没有配置 Git remote；Go module path 使用第 0.5 节默认规则。
- 计划编写时宿主机没有 `go` 命令。
- 计划编写时有 Node `v22.23.1`、npm `10.9.8`、Docker CLI `29.6.1`；Docker daemon 和本地镜像仍需在执行 Session 中检查。
- 本文件和 `implementation/code_skeleton_design.md` v1.3 可能是尚未提交的用户工作区修改，必须保留，禁止 reset/checkout 覆盖。

### 0.4 第一轮预检命令

新 Session 开始后依次执行，保持输出可读，不把命令拼成一个长脚本：

```text
pwd
git status --short
git log -3 --oneline
rg --files -g 'AGENTS.md' -g 'CLAUDE.md' -g '!**/.git/**'
rg --files -g '!**/.git/**'
go version
node --version
npm --version
docker --version
docker image ls
```

处理规则：

- 如果存在 `AGENTS.md`，先完整读取并服从其作用域指令。
- 工作区有修改时逐个判断来源；用户修改一律保留，不能用 reset/checkout 清理。
- 若宿主机仍无 Go，先检查本地是否有可用 `golang` 构建镜像。存在则优先使用容器化 Go 工具链；不存在时再请求下载镜像或安装 Go 的权限。
- 若 Docker daemon 因沙箱权限不可访问，按执行环境规则请求相应权限，不绕过审批。
- 若依赖下载被网络沙箱阻止，先完成不依赖下载的契约、目录和文档工作，再对必要下载请求权限。
- 发现代码已经部分实现时，不重建或覆盖；对照 Gate 逐项审计并从首个未满足 Gate 继续。

### 0.5 已锁定的默认决策

|项目|默认值|执行规则|
|-|-|-|
|Grafana Plugin ID|`mini-torchbearing-app`|plugin.json、目录、Grafana allowlist、API path 全部一致|
|Frontend package|`@mini-torchbearing/grafana-plugin`|使用 npm 和 package-lock.json|
|Go module base|有 Git remote 时使用其 canonical import path；无 remote 时用 `mini-torchbearing.local`|当前默认 `mini-torchbearing.local/services/ai-core`、`mini-torchbearing.local/services/assistant-mcp`、`mini-torchbearing.local/apps/grafana-plugin/backend`|
|Grafana 地址|宿主机 `http://localhost:3000`|容器内不能用 localhost 访问其他服务|
|AI Core 地址|宿主机 `http://localhost:8080`；Compose `http://ai-core:8080`|HTTP `/healthz`、`/readyz`|
|assistant-mcp 地址|宿主机 `http://localhost:8081`；Compose `http://assistant-mcp:8081/mcp`|健康端点不放在 `/mcp`|
|Compose project|`mini-torchbearing-mock`|CI 可以追加 run ID，避免并发冲突|
|Grafana 测试用户|`admin`|密码只从测试 env 读取，默认示例值仅用于隔离的本地 Compose|
|存储|AI Core SQLite|`/var/lib/ai-core/ai-core.db`；assistant-mcp 本阶段不建库|
|Mock datasource UID|`mock-prometheus`|不解析 UID，不映射真实 URL|
|Mock scenario|`node_exporter_overview`|任意非空输入都选择同一场景|
|默认时间范围|最近 30 分钟|Create Task 时冻结为 UTC from/to|
|幂等记录 TTL|24 小时|同 tenant + `create_task` scope + key 唯一|
|SSE Replay batch|每次最多 200 条|读完立即继续；heartbeat 15 秒，不占 sequence|
|Task 执行|单进程、每 Task goroutine、并发上限 4|先提交 Task/Event 再启动；不引入外部 Queue|
|Workflow deadline|60 秒|MCP 单次调用 5 秒；超时统一收敛为 failed|
|Agent|`DeterministicMockAgentRuntime`|不引入 Eino SDK，不调用 ModelPort|
|MCP|mcp-go Streamable HTTP|必须真实跨进程调用，不允许 in-process shortcut|
|HTTP Router|Go 标准库优先|若标准库无法满足再引入轻量 Router，不改变 Handler Port|
|SQLite 时间格式|UTC RFC3339Nano `TEXT`|所有表和 JSON 一致；禁止同库混用 epoch|
|运行时 ID|UUIDv7 字符串|调用方不得解析；测试使用固定 ID Generator|
|依赖版本|在首次引入时固定到 go.mod/go.sum/package-lock/Docker tag|不使用浮动 `latest` 作为验收基线|
|自动提交|关闭|只有用户明确要求时才 git commit|

### 0.6 默认自主处理与必须停止条件

执行者可以自行处理：

- 文件拆分、私有函数命名、测试文件组织、日志字段顺序。
- 不改变 Wire/Port 语义的库选择和小范围实现调整。
- 编译错误、测试失败、lint、格式和容器启动问题。
- 计划已有明确 fallback 的环境差异。

只有以下情况必须暂停并请求用户方向：

- 需要改变 Frontend → Plugin Backend → AI Core → MCP → assistant-mcp 的调用边界。
- 需要让多个服务共享数据库、让 Frontend 直连 AI Core/MCP、或让 AI Core 直接读取 fixture。
- 现有用户代码与本计划的同一文件/接口发生无法安全合并的冲突。
- 完成任务必须新增真实外部凭证、生产系统写权限或其他未授权外部副作用。
- 必要依赖既不在本地，也未获准下载/安装，且已穷尽不依赖该依赖的工作。

其他问题使用本文件默认值继续推进，不重新向用户征求已锁定的命名、端口、Mock 内容或测试范围。

### 0.7 工具链和依赖锁定协议

G0 必须创建 `build/toolchain.lock`，记录实际使用的精确版本或镜像 digest。选择过程属于执行动作，不需要向用户询问技术偏好：

1. Node/npm 先固定为当前已验证的 Node `22.23.1`、npm `10.9.8`；若 Grafana 官方插件工具明确不兼容，只允许切换到其声明支持的 LTS，并在 progress 记录证据。
2. Go 若宿主机可用，固定其完整版本；若不可用，从本地 `golang` 镜像中选最高的稳定数字 tag。都不可用时，请求拉取 `golang:1.24-bookworm` 的权限，并把最终 digest 写入 lock。
3. Grafana 从本地镜像中选最高的稳定语义版本；若只有浮动 tag，读取并记录 image ID/digest，Compose 使用 digest 或已解析出的固定 tag。不得让验收依赖会漂移的 `latest`。
4. 首次引入第三方依赖时查询其正式发布版本和兼容矩阵，随后只通过 `go.mod/go.sum`、`package-lock.json`、Docker digest 锁定；同一 Gate 内不做无关升级。

库选择固定如下，只有发生已证实的不兼容才可替换，并必须记录到 progress：

|用途|固定选择|边界|
|-|-|-|
|MCP Go SDK|`github.com/mark3labs/mcp-go`|只允许出现在 MCP transport/runtime adapter|
|Grafana Backend SDK|`github.com/grafana/grafana-plugin-sdk-go`|只允许出现在 Plugin Backend|
|SQLite driver|`modernc.org/sqlite`|只允许出现在 AI Core SQLite adapter；避免本地 CGO 依赖|
|Go OpenAPI|`github.com/oapi-codegen/oapi-codegen/v2`|生成 AI Core server interface 和 Go client|
|TypeScript OpenAPI|`openapi-typescript` + `openapi-fetch`|生成 types；Resource client 只包装 path/method/auth transport|
|JSON Schema|选择一个支持目标 draft 的 Go validator 并锁版|只在 contract/adapter boundary；domain 不依赖 validator SDK|
|Frontend data|`@tanstack/react-query`|管理 Session/Task snapshot，不保存 SSE 派生图表副本|
|Browser E2E|Playwright|只在 `tests/e2e/mock`|

`build/toolchain.lock` 至少记录：Go、Node、npm、Grafana image、Go build image、OpenAPI generator、Grafana SDK、mcp-go 和 Playwright。lock 文件不记录本机绝对路径、registry token 或密码。

---

## 1. 计划目标

本计划只实现一个可运行、可重复、可验证的基本 Mock 闭环：用户在 Grafana App Plugin 中输入一句自然语言，系统经过 Plugin Backend、AI Core、Agent Runtime Port、MCP Tool、Mock Prometheus Adapter，返回固定的 `node_exporter` 指标结果，并在工作台中形成三张时间序列图表。

本阶段不追求真实智能。最重要的交付不是 Mock 数据本身，而是稳定的模块边界和可替换接口：后续接入 Eino、真实模型、Prometheus 或 Grafana Write Adapter 时，只替换 Adapter，不修改领域对象、应用流程和跨进程契约。

### 1.1 一句话完成标准

在 Grafana 插件中输入任意非空文本，系统返回固定说明文本，并展示 CPU 使用率、内存可用率、系统负载三张 `node_exporter` Mock 图表；刷新或 SSE 重连后可以按持久化事件恢复当前任务结果。

### 1.2 首个演示输入

```text
帮我看看 node_exporter 最近 30 分钟的 CPU、内存和系统负载
```

### 1.3 首个演示输出

- 固定助手回复：已生成 node_exporter 的 CPU、内存和系统负载视图。
- 三张 `ChartDraft`：CPU 使用率、内存可用率、系统负载。
- 每张图包含标题、PromQL、时间范围、状态、图例和固定时间序列数据。
- Task 状态最终为 `completed`。
- 全部 TaskEvent 已先写入 SQLite，再通过 SSE 推送。

---

## 2. 范围控制

### 2.1 本阶段必须实现

- Monorepo 基础目录、Go workspace、Frontend package、Makefile 和构建入口。
- Grafana App Plugin Frontend 与 Plugin Backend 的最小可运行骨架。
- AI Core HTTP 服务、最小 Workflow、领域对象、Port 和 Bootstrap。
- `assistant-mcp` Streamable HTTP 服务及 `grafana` namespace 的三个只读工具。
- Frontend → Plugin Backend、Plugin Backend → AI Core 的 OpenAPI 契约。
- AI Core → `assistant-mcp` 的 MCP Tool Schema 和真实 MCP transport。
- RequestContext、Session、Message、Task、TaskEvent、ToolCall、Query、Chart 的 Schema。
- Deterministic Mock Agent Runtime；不调用真实模型。
- Mock Prometheus Adapter 与固定 `node_exporter` fixture。
- AI Core SQLite migration、最小 Repository 和 durable TaskEventStore。
- SSE replay、断线重连和三张图表渲染。
- 单元测试、Contract Test、Adapter Contract Test、Component Smoke 和 Mock E2E。
- Docker Compose Mock 启动配置：Grafana、AI Core、`assistant-mcp` 三个常驻容器；Plugin build 为一次性任务。

### 2.2 只建立接口或占位，不实现真实能力

- `ModelPort` 和 Eino `AgentRuntime` Adapter 目录/构造器；启用时返回 `not_implemented`。
- Prometheus Real Adapter 构造器和配置键；本阶段不能被默认选择。
- DashboardRead/Write、PanelCompiler、Approval Port；不进入本次可执行流程。
- Knowledge、Playbook、Skill、Alert namespace/package；未注册的工具返回 `tool_not_supported`，不得返回空成功。
- PostgreSQL、Redis、向量库、对象存储配置插槽。

### 2.3 本阶段明确不做

- 不接真实 LLM、Model Gateway 或 Eino Agent Loop。
- 不启动真实 Prometheus 或 `node_exporter` 容器。
- 不查询 Grafana datasource，不读取或写入真实 Dashboard。
- 不实现图表编辑、保存 Dashboard、审批 UI、版本冲突处理。
- 不实现会话搜索、分享、Fork、模板和 Playbook。
- 不实现知识检索、Alert Webhook、Golden Query 质量评分。
- 不实现生产级 RBAC、delegation grant、限流、HA 和多实例恢复。
- 不为了演示在 Frontend、Application 或 Domain 中读取 fixture 或判断 `mockMode`。

---

## 3. 运行拓扑与纵向流程

### 3.1 Mock 运行拓扑

```text
Browser
  → Grafana container
      → App Plugin Frontend
      → Plugin Backend Resource Handler
          → generated AI Core Go client
              → AI Core container
                  → SQLite ApplicationStore / TaskEventStore
                  → DeterministicMockAgentRuntime
                      → MetricCatalogPort / QueryEnginePort
                          → MCP typed adapter
                              → assistant-mcp container
                                  → grafana.* Tool Handler
                                      → Grafana Namespace Service
                                          → PrometheusPort
                                              → MockPrometheusAdapter
                  → ChartDraft + ChartExecution
                  → durable TaskEvent → in-process notifier
          ← SSE proxy
      ← generated TypeScript DTO + SSE event
  ← Grafana TimeSeries chart
```

本阶段只有三个常驻 Runtime container；Grafana Plugin Backend 作为 Grafana 插件进程由 Grafana container 拉起，不单独计为服务：

|Compose service|宿主机绑定|持久化/挂载|readiness|
|-|-|-|-|
|`grafana`|`127.0.0.1:3000:3000`|构建后的插件目录只读挂载；Grafana provisioning|HTTP API healthy 且插件 backend registered|
|`ai-core`|`127.0.0.1:8080:8080`|named volume → `/var/lib/ai-core`|`GET /readyz`：migration 完成、SQLite 可写、assistant-mcp 可连接|
|`assistant-mcp`|`127.0.0.1:8081:8081`|fixture 目录只读挂载|`GET /readyz`：Schema/fixture 已加载、三个 Tool 已注册|

Compose network 内只能使用 service DNS：Grafana Plugin Backend → `http://ai-core:8080`，AI Core → `http://assistant-mcp:8081/mcp`。本轮不启动 Prometheus/node_exporter container；它们属于下一阶段把 `MockPrometheusAdapter` 替换为 Real Adapter 时的混合 E2E。

### 3.2 Golden Path

1. Frontend 调 Plugin Backend 创建 Session。
2. 用户提交任意非空消息；Frontend 调 Plugin Backend 创建 Task。
3. Plugin Backend 从 Grafana Plugin Context 构建只读 `RequestContext`，生成 request/trace ID，并调用生成的 AI Core Client。
4. AI Core 在同一事务中保存 Message、Task 和 `task.created` 事件，然后返回 `202 Accepted`。
5. Frontend 连接 Task SSE，初始使用 `afterSequence=0`。
6. AI Core Workflow 将 Task 推进到 `planning`，调用 `DeterministicMockAgentRuntime`。
7. Mock Agent 选择固定、结构化的 node_exporter 分析计划；它不能读取 fixture，也不能直接构造最终 `ChartDraft`。
8. Mock Agent 按固定计划通过类型化 `MetricCatalogPort`、`QueryEnginePort` 调用 MCP Adapter，结束后返回 `AgentRunResult`。
9. MCP Adapter 真实连接 `assistant-mcp`，调用一次 `grafana.search_metrics`、针对三个指标各调用一次 `grafana.get_metric_labels`，再执行三次 `grafana.query_prometheus`；共七次 ToolCall。
10. `assistant-mcp` 对 Tool 输入做 Schema 校验，经 namespace service 调用 `MockPrometheusAdapter`。
11. Mock Prometheus Adapter 从固定场景加载指标元数据、标签和时间序列，并返回领域结果。
12. AI Core 将查询结果转换为三张 `ChartDraft` 和 `ChartExecution`，持久化后追加 `chart.created`、`chart.execution_completed` 事件。
13. AI Core 追加固定助手回复和 `task.completed`，Task 进入 `completed`。
14. Plugin Backend 不解析业务事件，只以无缓冲方式转发 SSE。
15. Frontend 通过生成 DTO 解码事件，将 wire series 转为 Grafana DataFrame，展示三张 TimeSeries 图。
16. 浏览器断线或刷新后，从已知 Task ID 重新请求 `afterSequence`；AI Core 从 SQLite 重放未消费事件，前端重建相同结果。

Workbench 路由固定为 `/a/mini-torchbearing-app/workbench?sessionId=<id>&taskId=<id>`。首次打开没有 ID 时，在第一次合法提交时先创建标题为 `Node exporter mock analysis` 的 Session，再创建 Task，并用 URL replace 写入两个 ID；不使用 localStorage 作为事实来源。刷新时先 Get Session/Get Task，再从 `afterSequence=0` 重放 Task 全量事件。

### 3.3 最小状态路径

```text
created → planning → running_tools → validating → completed
```

错误路径只实现：

```text
任意非终态 → failed
```

所有状态迁移必须调用领域方法；Repository 和 Handler 不得直接赋值状态。

允许迁移表：

|当前状态|允许下一状态|
|-|-|
|created|planning、failed|
|planning|running_tools、failed|
|running_tools|validating、failed|
|validating|completed、failed|
|completed|无|
|failed|无|

Chart 状态只实现 `proposed → ready`；查询失败时不创建伪 Chart，而是让 Task 进入 failed。`ChartExecution.status` 本阶段只有 `success`，失败信息由 ToolCall/Task Error 表达；接口仍预留未来的 `failed` 枚举值。

---

## 4. 契约先行原则

实现顺序必须是：Schema/OpenAPI → 生成类型 → Domain/Port → Adapter → Handler/UI。契约 Gate 未通过前，不开始跨模块 Handler 联调。

### 4.1 单一来源

|契约|单一来源|消费者|
|-|-|-|
|Frontend → Plugin Backend|`contracts/openapi/plugin-resource.yaml`|Frontend TS Client、Plugin Backend Handler Test|
|Plugin Backend → AI Core|`contracts/openapi/plugin-ai-core.yaml`|Plugin Backend Go Client、AI Core Go Server|
|共享业务 DTO|`contracts/schemas/*.schema.json`|两份 OpenAPI、SSE、Fixture、生成类型|
|Task SSE|`contracts/events/task-events.schema.json`|AI Core、Plugin Backend、Frontend|
|MCP Tool|`contracts/tools/grafana/*.schema.json`|AI Core MCP Adapter、assistant-mcp Handler|
|错误码|`contracts/errors/error-codes.yaml`|所有服务和 Frontend|

两份 OpenAPI 必须 `$ref` 同一份共享 Schema 或由同一生成源产生，禁止复制并手工维护 Session、Task、Chart 等 DTO。

### 4.2 Wire 规则

- JSON 字段使用 `lowerCamelCase`。
- 枚举和错误码使用 `lower_snake_case`。
- 时间使用 UTC RFC3339。
- ID 为不透明字符串。
- 所有跨进程对象包含明确的 required/optional 定义。
- 可选字段不得依靠 Go/TypeScript 零值猜测。
- 所有错误使用统一 Error Envelope，不返回裸字符串。
- 外部 SDK 类型、Grafana DataFrame、Eino Message、MCP SDK Result 不得进入共享 DTO。

### 4.3 兼容性规则

- 首版契约版本为 `v1`。
- 新增 optional 字段允许向后兼容；删除/重命名字段必须升级契约版本。
- Tool 名和字段一经进入 Mock E2E 不随实现包名变化。
- 生成代码必须可重复；`make generated-client-diff` 证明生成目录执行前后内容 hash 不变。
- Fixture 必须在服务启动和测试阶段通过相应 JSON Schema 校验。

---

## 5. 接口完成矩阵

“接口完成”表示：类型、方法、上下文、错误、超时、幂等、Schema、Mock/Real Adapter 插槽和 Contract Test 都已定义；不表示本阶段已经有真实外部实现。

|边界|接口|本阶段实现|完成要求|
|-|-|-|-|
|Frontend → Plugin Backend|Session/Task/Event Resource API|真实|OpenAPI、生成 TS DTO、Handler Contract、SSE 重连|
|Plugin Backend → AI Core|Session/Task/Event API|真实|OpenAPI、生成 Go Client/Server、错误映射、context 透传|
|AI Core → Agent Runtime|`AgentRuntimePort`|Deterministic Mock|固定计划；Eino 类型不越界；Real Adapter 插槽存在|
|AI Core → 指标目录|`MetricCatalogPort`|真实 MCP Adapter|类型化请求/结果；不得在 application 拼 Tool Name|
|AI Core → 查询执行|`QueryEnginePort`|真实 MCP Adapter|Validate/Execute 语义稳定；MCP 类型不越界|
|AI Core → MCP transport|`ToolGatewayPort`|真实 Streamable HTTP Adapter|Tool Schema 校验、超时、错误映射、trace 透传|
|assistant-mcp → 指标源|`PrometheusPort`|Mock Adapter|Mock/Real 构造器；同一 Adapter Contract Test|
|AI Core → Store|Repository/ApplicationStore Ports|SQLite|tenant 过滤、事务、领域错误、migration|
|AI Core → Event Store|`TaskEventStore`/Notifier|SQLite + in-process|sequence 原子分配、先存后通知、Replay|
|Plugin Backend → Grafana Context|`GrafanaContextProvider`|最小真实|读取当前用户/org/roles；只读流程不签发 write grant|
|Frontend → 图表组件|Wire Chart → Grafana DataFrame Mapper|真实|Mapper 独立测试；Wire DTO 不依赖 Grafana 类型|

### 5.1 本阶段 HTTP API

Plugin Resource API：

```text
POST /api/plugins/<PLUGIN_ID>/resources/sessions
GET  /api/plugins/<PLUGIN_ID>/resources/sessions/{sessionId}
POST /api/plugins/<PLUGIN_ID>/resources/tasks
GET  /api/plugins/<PLUGIN_ID>/resources/tasks/{taskId}
GET  /api/plugins/<PLUGIN_ID>/resources/tasks/{taskId}/events?afterSequence=<n>
```

AI Core API：

```text
GET  /healthz
GET  /readyz
POST /v1/sessions
GET  /v1/sessions/{sessionId}
POST /v1/tasks
GET  /v1/tasks/{taskId}
GET  /v1/tasks/{taskId}/events?afterSequence=<n>
```

`POST /tasks` 必须支持 `Idempotency-Key`，成功创建后立即返回 `202` 和 Task Snapshot；Workflow 异步推进。相同 key 与相同请求体返回同一 Task，不重复生成图表；相同 key 携带不同请求体返回 `idempotency_conflict`。

最小请求/响应语义：

|操作|请求|成功响应|
|-|-|-|
|Create Session|`title?`|`201` + Session Snapshot|
|Get Session|path `sessionId`|`200` + Session Snapshot|
|Create Task|`sessionId`、`message`、`analysisContext.datasourceUid`、`analysisContext.timeRange?`|`202` + Task Snapshot|
|Get Task|path `taskId`|`200` + Task Snapshot|
|Stream Events|path `taskId`、`afterSequence?` / `Last-Event-ID?`|`200 text/event-stream`|

Mock profile 使用不透明 `datasourceUid=mock-prometheus`；调用方不得从 UID 推导 URL。`timeRange` 未提供时默认最近 30 分钟，并在创建 Task 时解析为固定的 UTC `from/to` 保存，不在后续步骤反复读取当前时间。

HTTP/SSE Header 约定：

```text
Idempotency-Key     # Create Task 必需
X-Request-ID        # Plugin Backend 生成或校验后透传
X-Trace-ID          # 开发期关联标识
traceparent         # 标准 trace 传播
Last-Event-ID       # SSE 重连；值为已消费 sequence
```

Frontend 不能提交 user/org/roles/permissions。Plugin Backend 必须从 Grafana Plugin Context 构建这些字段；只有 request/trace/idempotency 和明确的分析上下文可以来自请求。

Plugin Backend → AI Core 和 AI Core → assistant-mcp 的内部身份 Header 固定为：

```text
X-MTB-Tenant-ID       # 本地 Mock 为 org:1
X-MTB-Org-ID          # 本地 Mock 为 1
X-MTB-User-ID         # 从 Grafana Plugin Context 获取
X-MTB-Roles           # 逗号分隔；Header 解析后 trim、去空、去重
X-MTB-Permissions     # 逗号分隔；本闭环至少 datasources:query
```

- 这些 Header 只能由 Plugin Backend 构建，不能透传同名浏览器 Header。
- AI Core 将其解析为 `RequestContext`；缺失 tenant/org/user 返回 `unauthenticated`，缺少 `datasources:query` 返回 `permission_denied`。
- AI Core 的 MCP transport 把同一组身份 Header 连同 request/trace Header 传给 assistant-mcp；Tool Input Schema 不重复携带身份字段。
- MCP client/HTTP transport 不得把 `RequestContext` 写入全局可变 Header；必须按调用从 `context.Context` 或 request clone 注入，避免并发 Task 串租户。
- 本地 Compose 中 AI Core 和 assistant-mcp 只暴露到内部网络；宿主机端口只用于开发/测试。该 Header 方案不是生产环境的服务身份认证替代品。

SSE Wire 格式：

```text
id: <sequence>
event: <task-event-type>
data: <TaskEvent JSON>
```

- `Content-Type: text/event-stream`、`Cache-Control: no-cache`。
- `afterSequence` 与 `Last-Event-ID` 同时存在时必须一致，否则返回 `invalid_argument`。
- 每 15 秒允许发送不占业务 sequence 的 heartbeat comment。
- Plugin Backend 必须禁用代理缓冲；Frontend 按 sequence 去重，发现 gap 时断开并从最后连续 sequence 重连。

### 5.2 本阶段 MCP Tool

```text
grafana.search_metrics
grafana.get_metric_labels
grafana.query_prometheus
```

每个 Tool 固定包含：

- `name`、`version=v1`、`riskLevel=read_only`。
- `requiredPermissions=[datasources:query]`。
- `timeoutMs`、`idempotent=true`。
- Input/Output JSON Schema。
- 统一 Error Envelope。
- Handler 输入输出 Schema 校验。
- Tool started/completed/failed 审计摘要；本阶段可以写结构化日志，不建立完整审计库。

最小 Tool 语义：

|Tool|Input|Output|
|-|-|-|
|`grafana.search_metrics`|datasourceUid、query、limit|candidates[]：metricName、type、description、labels、score、sources|
|`grafana.get_metric_labels`|datasourceUid、metricName|metricName、labelNames[]、sampleValues map|
|`grafana.query_prometheus`|datasourceUid、expression、start、end、stepSeconds、mode=`validate\|execute`|validation、status；execute 模式另含 resultType、series[]、durationMs、warnings[]|

`series[]` 使用领域 Wire 格式 `name + labels + points(timestamp,value)`，不得返回 Prometheus HTTP API 原始 matrix，也不得把 mcp-go Content 类型透出 Tool Adapter。

### 5.3 内部 Port 约束

- 每个 Port 方法首参数为 `context.Context`，外部数据访问显式接收业务 `RequestContext`。
- Repository 显式接收 tenant ID，不接收包含临时凭证的完整 `RequestContext`。
- Port 参数和返回值只使用标准库或项目 domain/application DTO。
- Mock 与未来 Real Adapter 使用相同构造接口和 Contract Test 工厂。
- Adapter 选择只在 Bootstrap；Application/Domain 不读取配置，不判断 `mockMode`。
- 未实现 Adapter 必须返回分类错误，不能 panic 或空成功。

### 5.4 本阶段 Port 方法集合

|Port|最小方法|
|-|-|
|AgentRuntimePort|`Run`、`Resume`|
|MetricCatalogPort|`SearchMetrics`、`GetMetricLabels`|
|QueryEnginePort|`Validate`、`Execute`|
|ToolGatewayPort|`ListTools`、`CallTool`|
|SessionRepository|`Create`、`Get`、`Update(expectedVersion)`|
|MessageRepository|`Append`、`ListBySession`|
|TaskRepository|`Create`、`Get`、`Update(expectedVersion)`|
|ToolCallRepository|`Create`、`Complete`、`ListByTask`|
|ChartRepository|`Create`、`Get`、`ListByTask`|
|ChartExecutionRepository|`Create`、`ListByChart`|
|IdempotencyRepository|`Reserve`、`GetResult`、`Complete`|
|TaskEventStore|`Append`、`Replay`、`LatestSequence`|
|TaskEventNotifier|`Notify`、`Subscribe`|
|ClockPort|`Now`|
|IDGenerator|按对象生成不透明 ID|

Repository 写方法必须接收 tenant ID；Update 必须显式携带 expectedVersion。`ApplicationStore.WithinTransaction` 暴露上述 Repository 的事务视图，但不得暴露 `*sql.Tx`。

### 5.5 当前切片的规范性 Port 签名

以下为实现本切片时的规范性 Go 形状。包名可以按第 8 节目录调整，但方法语义、输入输出和依赖方向不能改变。Wire DTO 仍由 Schema/OpenAPI 生成，下面的类型是显式映射后的 domain/application 类型。

```go
type RequestContext struct {
    TenantID   string
    OrgID      string
    UserID     string
    Roles      []string
    Permissions []string
    RequestID  string
    TraceID    string
}

type AbsoluteTimeRange struct {
    From time.Time
    To   time.Time
}

type AgentRunRequest struct {
    TaskID       string
    SessionID    string
    UserMessage  string
    DatasourceUID string
    TimeRange    AbsoluteTimeRange
}

type AgentRunResult struct {
    AssistantText string
    Proposals     []ChartProposal
}

type ChartProposal struct {
    Key           string // cpu | memory | load；仅用于确定性排序，不作为持久化 ID
    Title         string
    Visualization string // timeseries
    Unit          string
    Query         QuerySpec
    Execution     QueryExecutionResult
}

type AgentRuntimePort interface {
    Run(ctx context.Context, rc RequestContext, req AgentRunRequest, sink AgentEventSink) (AgentRunResult, error)
    Resume(ctx context.Context, rc RequestContext, req AgentResumeRequest, sink AgentEventSink) (AgentRunResult, error)
}

type AgentEventSink interface {
    Emit(ctx context.Context, event AgentEvent) error
}
```

`DeterministicMockAgentRuntime` 构造器必须显式注入 `MetricCatalogPort` 和 `QueryEnginePort`。`AgentEventSink` 由 Workflow 提供，负责把 Agent/Tool 事件映射为持久化 TaskEvent；Runtime 不持有 Repository。

```go
type SearchMetricsRequest struct {
    DatasourceUID string
    Query         string
    Limit         int
}

type GetMetricLabelsRequest struct {
    DatasourceUID string
    MetricName    string
}

type MetricCatalogPort interface {
    SearchMetrics(ctx context.Context, rc RequestContext, req SearchMetricsRequest) (SearchMetricsResult, error)
    GetMetricLabels(ctx context.Context, rc RequestContext, req GetMetricLabelsRequest) (MetricLabelsResult, error)
}

type ValidateQueryRequest struct {
    DatasourceUID string
    Expression    string
}

type ExecuteQueryRequest struct {
    DatasourceUID string
    Expression    string
    TimeRange     AbsoluteTimeRange
    StepSeconds   int
}

type QueryEnginePort interface {
    Validate(ctx context.Context, rc RequestContext, req ValidateQueryRequest) (QueryValidationResult, error)
    Execute(ctx context.Context, rc RequestContext, req ExecuteQueryRequest) (QueryExecutionResult, error)
}
```

Golden Path 对每张图只调用一次 `Execute`。MCP Adapter 将其映射为 `grafana.query_prometheus mode=execute`，assistant-mcp 在执行前完成 Schema/PromQL 校验，并在同一响应中返回 `validation` 和 `series`。`Validate` 方法及 `mode=validate` 必须实现并做 Contract Test，但不额外出现在 Golden Path，因此全程仍是 3 次 query ToolCall、7 次 ToolCall 总计。

```go
type ToolGatewayPort interface {
    ListTools(ctx context.Context, rc RequestContext, filter ToolFilter) ([]ToolDescriptor, error)
    CallTool(ctx context.Context, rc RequestContext, call ToolCall) (ToolResult, error)
}

type PrometheusPort interface {
    SearchMetrics(ctx context.Context, rc RequestContext, req SearchMetricsRequest) (SearchMetricsResult, error)
    GetMetricLabels(ctx context.Context, rc RequestContext, req GetMetricLabelsRequest) (MetricLabelsResult, error)
    Query(ctx context.Context, rc RequestContext, req PrometheusQueryRequest) (PrometheusQueryResult, error)
}

type TaskEventStore interface {
    Append(ctx context.Context, draft TaskEventDraft) (TaskEvent, error)
    Replay(ctx context.Context, tenantID, taskID string, afterSequence int64, limit int) ([]TaskEvent, error)
    LatestSequence(ctx context.Context, tenantID, taskID string) (int64, error)
}

type TaskEventNotifier interface {
    Notify(ctx context.Context, event TaskEvent) error
    Subscribe(ctx context.Context, tenantID, taskID string) (<-chan struct{}, error)
}

type ClockPort interface { Now() time.Time }
type IDGenerator interface { NewID(kind string) string }
```

当前切片 Repository 采用以下最小签名；不能为了照搬长期设计而提前加入 Share、Approval、Canvas、Template 等 Repository：

```go
type SessionRepository interface {
    Create(ctx context.Context, session AnalysisSession) error
    Get(ctx context.Context, tenantID, sessionID string) (AnalysisSession, error)
    Update(ctx context.Context, session AnalysisSession, expectedVersion int64) error
}

type MessageRepository interface {
    Append(ctx context.Context, message Message) error
    ListBySession(ctx context.Context, tenantID, sessionID string) ([]Message, error)
}

type TaskRepository interface {
    Create(ctx context.Context, task AnalysisTask) error
    Get(ctx context.Context, tenantID, taskID string) (AnalysisTask, error)
    Update(ctx context.Context, task AnalysisTask, expectedVersion int64) error
}

type ToolCallRepository interface {
    Create(ctx context.Context, call ToolCallRecord) error
    Complete(ctx context.Context, call ToolCallRecord, expectedVersion int64) error
    ListByTask(ctx context.Context, tenantID, taskID string) ([]ToolCallRecord, error)
}

type ChartRepository interface {
    Create(ctx context.Context, chart ChartDraft) error
    Get(ctx context.Context, tenantID, chartID string) (ChartDraft, error)
    ListByTask(ctx context.Context, tenantID, taskID string) ([]ChartDraft, error)
}

type ChartExecutionRepository interface {
    Create(ctx context.Context, execution ChartExecution) error
    ListByChart(ctx context.Context, tenantID, chartID string) ([]ChartExecution, error)
}

type IdempotencyRepository interface {
    Reserve(ctx context.Context, key IdempotencyKey, requestHash string, expiresAt time.Time) (IdempotencyRecord, error)
    GetResult(ctx context.Context, key IdempotencyKey) (IdempotencyRecord, error)
    Complete(ctx context.Context, key IdempotencyKey, resourceID string, responseJSON []byte) error
}

type ApplicationStore interface {
    Sessions() SessionRepository
    Messages() MessageRepository
    Tasks() TaskRepository
    ToolCalls() ToolCallRepository
    Charts() ChartRepository
    ChartExecutions() ChartExecutionRepository
    Idempotency() IdempotencyRepository
    TaskEvents() TaskEventStore
    WithinTransaction(ctx context.Context, fn func(tx ApplicationStore) error) error
    Health(ctx context.Context) error
    Close() error
}
```

`WithinTransaction` 回调中的 Repository 与 `TaskEventStore` 必须绑定同一 SQLite transaction；回调结束后不得继续使用。Repository 只能返回领域错误，不能向上泄漏 `database/sql` 或驱动错误。

---

## 6. 最小领域与 Wire 对象

|对象|最小必需字段|所有者|
|-|-|-|
|RequestContext|tenantId、orgId、userId、roles、permissions、requestId、traceId|Plugin Backend 构建，沿调用链透传|
|AnalysisSession|id、tenantId、title、status、createdBy、createdAt、updatedAt、version|AI Core|
|Message|id、sessionId、role、content、createdAt|AI Core|
|AnalysisTask|id、sessionId、status、inputMessageId、datasourceUid、timeRange、latestSequence、error、createdAt、startedAt、completedAt、updatedAt、version|AI Core|
|TaskEvent|eventId、taskId、sessionId、sequence、type、timestamp、payload|AI Core|
|MetricCandidate|metricName、type、description、labels、score、sources|assistant-mcp 输出，AI Core 映射|
|QuerySpec|refId、expression、legend、datasourceUid、timeRange|AI Core Agent Result|
|QueryExecution|id、queryRefId、status、seriesCount、durationMs、sampleRange、series|AI Core|
|ChartDraft|id、sessionId、taskId、title、visualization、queries、status、latestExecutionId、createdAt、version|AI Core|
|Series|name、labels、points[]|跨进程 Chart Preview|
|Point|timestamp、value|跨进程 Chart Preview|
|Error|code、message、retryable、requestId、details|统一错误契约|

### 6.1 TaskEvent 最小事件集合

```text
task.created
task.status_changed
assistant.message.started
assistant.message.delta
assistant.message.completed
tool.started
tool.completed
tool.failed
metric.candidates_created
chart.created
chart.execution_completed
task.completed
task.failed
```

`payload` 必须按事件类型使用 `oneOf` Schema；不得只有无约束的自由 JSON 对象。

成功路径事件偏序必须固定：

```text
task.created
→ task.status_changed(planning)
→ assistant.message.started
→ assistant.message.delta("正在生成固定的 node_exporter 分析视图…")
→ task.status_changed(running_tools)
→ [tool.started → tool.completed] × 7
   └─ search_metrics 完成后紧接 metric.candidates_created
→ [chart.created → chart.execution_completed] × 3
→ task.status_changed(validating)
→ assistant.message.completed
→ task.status_changed(completed)
→ task.completed
```

七组 ToolCall 的固定顺序是 search → CPU labels → memory labels → load labels → CPU query → memory query → load query。三组 Chart 事件在 Runtime 返回 `AgentRunResult` 后由 Workflow 按 CPU → memory → load 顺序持久化。每个 `tool.started`/`tool.completed` 使用相同 `toolCallId`，并在 `tool_calls` 表存在对应记录。事件的绝对 sequence 由 Store 分配，测试断言上述偏序、连续性和一一对应关系，不在业务代码中硬编码 sequence 数值。

事件 payload 的最小字段固定如下；这些字段必须进入 `task-events.schema.json` 的 discriminator + `oneOf`：

|event type|payload 最小字段|
|-|-|
|task.created|`task` Snapshot|
|task.status_changed|`previousStatus`、`status`；失败时含 `error`|
|assistant.message.started|`messageId`、`role=assistant`|
|assistant.message.delta|`messageId`、`delta`、`ordinal`|
|assistant.message.completed|`message` Snapshot|
|tool.started|`toolCallId`、`toolName`、`toolVersion=v1`、`inputSummary`|
|tool.completed|`toolCallId`、`toolName`、`durationMs`、`outputSummary`|
|tool.failed|`toolCallId`、`toolName`、`durationMs`、`error`|
|metric.candidates_created|`candidates`|
|chart.created|`chart` Snapshot，status=`proposed`|
|chart.execution_completed|`chartId`、`execution`、`chartStatus=ready`|
|task.completed|`task` Snapshot，status=`completed`|
|task.failed|`task` Snapshot，status=`failed`、`error`|

`inputSummary` 只保留 datasourceUid、metricName 或 expression hash/长度；`outputSummary` 只保留 candidate/label/series/point 数量。完整 Query series 只存在于 `chart.execution_completed` 业务事件和 `chart_executions` 表，不能复制到 Tool 审计日志。

失败路径：已开始的 Tool 先产生 `tool.failed`，随后 `task.status_changed(failed)` 和 `task.failed`；已经持久化的候选或图表不得删除。

### 6.2 最小错误码

```text
invalid_argument
unauthenticated
permission_denied
resource_not_found
resource_conflict
invalid_state_transition
adapter_not_configured
dependency_unavailable
tool_not_supported
tool_timeout
schema_validation_failed
idempotency_conflict
execution_interrupted
internal_error
not_implemented
```

Plugin Backend 负责将 AI Core Error 映射为 Plugin Resource API Error；assistant-mcp/MCP Adapter 负责将 Tool Error 映射为同一领域语义。原始 SQLite、HTTP、MCP SDK 错误不得越过 Adapter。

最小 HTTP 状态映射：

|领域错误|HTTP|
|-|-|
|invalid_argument、schema_validation_failed|400|
|unauthenticated|401|
|permission_denied|403|
|resource_not_found|404|
|resource_conflict、invalid_state_transition、idempotency_conflict|409|
|dependency_unavailable、tool_timeout、execution_interrupted|503|
|not_implemented、tool_not_supported|501|
|internal_error|500|

### 6.3 最小 SQLite 表结构

本阶段只创建 AI Core 数据库。所有时间列统一保存 UTC RFC3339Nano `TEXT`，Go Adapter 读写时统一调用 UTC 转换；不得使用本地时区或在同库混入整数 epoch。JSON 列只保存经过 Schema 校验的领域快照。

|表|必要列|关键约束/索引|
|-|-|-|
|sessions|id、tenant_id、title、status、created_by、created_at、updated_at、version|PK(id)；index(tenant_id,updated_at)；version >= 1|
|messages|id、tenant_id、session_id、role、content、created_at|PK(id)；FK session；index(tenant_id,session_id,created_at)|
|tasks|id、tenant_id、session_id、status、input_message_id、datasource_uid、time_from、time_to、latest_sequence、error_code、error_message、created_at、started_at、completed_at、updated_at、version|PK(id)；FK session/message；index(tenant_id,session_id,created_at)；latest_sequence >= 0；time_from < time_to|
|task_events|event_id、tenant_id、task_id、session_id、sequence、type、timestamp、payload_json|PK(event_id)；UNIQUE(tenant_id,task_id,sequence)；index replay|
|tool_calls|id、tenant_id、task_id、tool_name、tool_version、status、input_summary_json、output_summary_json、error_code、started_at、completed_at、duration_ms、version|PK(id)；index(tenant_id,task_id,started_at)；version >= 1|
|charts|id、tenant_id、session_id、task_id、title、visualization、queries_json、status、latest_execution_id、created_at、updated_at、version|PK(id)；index(tenant_id,task_id)；version >= 1|
|chart_executions|id、tenant_id、chart_id、status、series_count、duration_ms、sample_from、sample_to、series_json、warnings_json、created_at|PK(id)；FK chart；index(tenant_id,chart_id,created_at)|
|idempotency_keys|tenant_id、scope、key、request_hash、status、resource_id、response_json、created_at、expires_at|PK(tenant_id,scope,key)；同 key 不同 hash 冲突|

SQLite migration：

```text
services/ai-core/migrations/sqlite/0001_initial.sql
```

`0001_initial.sql` 必须一次建立上述表、外键、唯一约束和索引。已提交 migration 后不得原地修改；后续变更追加 `0002_*.sql`。

SQLite 连接初始化必须启用 foreign keys 和 busy timeout。开发/Compose 使用 WAL；测试可以使用独立临时数据库文件，不能用会让多连接看到不同数据库的裸 `:memory:`。Migration 在 readiness 之前完成，失败时 readiness 保持失败。

### 6.4 事务与事件一致性

Create Task 必须满足：

1. Reserve idempotency key。
2. 校验 Session 属于同一 tenant。
3. 插入 user Message。
4. 插入 Task，`latest_sequence=0`。
5. Append `task.created`，原子得到 sequence 1，并更新 Task.latest_sequence。
6. 完成 idempotency record，记录 Task ID 和响应快照。
7. 提交事务。
8. 提交成功后 Notify，并启动异步 Workflow。

`requestHash` 固定为规范化 JSON `{"tenantId", "sessionId", "message", "analysisContext":{"datasourceUid","timeRange"}}` 的 SHA-256；对象 key 排序、UTC 时间规范化、字符串不做隐式 trim（输入校验阶段只拒绝全空白）。scope 固定为 `create_task`，TTL 为 24 小时。Frontend 在一次提交开始时用 `crypto.randomUUID()` 生成 key，并为该提交的网络重试复用；Plugin Backend 缺少 key 时返回 `invalid_argument`，存在时原样透传。

异步 Workflow 使用 AI Core 进程级 service context 派生 60 秒 deadline，不能继续使用会在 `202` 返回后取消的 HTTP request context。只复制非敏感 `RequestContext` 字段；本阶段没有 token/grant。每 Task goroutine 先取得容量为 4 的 semaphore，结束时释放；服务关闭会取消 service context，并尽力把在途 Task 收敛为 `execution_interrupted`。

Workflow 每次状态迁移、ToolCall 完成、Chart/Execution 创建与对应 TaskEvent 应在尽可能小的同一事务中完成。事务内禁止调用 MCP、HTTP 或未来 LLM；外部调用前记录 started，调用后开启新事务记录 result/event。

Runtime 返回后，Workflow 对每个 `ChartProposal` 各开一个事务：创建 status=`proposed` 的 Chart 并追加 `chart.created`，创建 success Execution，更新 Chart 为 `ready`/latestExecutionId，再追加 `chart.execution_completed`。三个 proposal 之间允许已有结果持久化；后续 proposal 失败时保留前面的 Chart。最后在一个事务中保存 assistant Message、追加 `assistant.message.completed`、完成 Task 并追加两个 completed 事件。

`TaskEventNotifier.Notify` 失败不能回滚已提交事件。SSE 收到通知后只把通知视为“可能有新事件”，必须调用 `Replay` 读取事实数据。

本阶段没有持久化 Queue/Checkpoint。AI Core 启动时扫描 `created/planning/running_tools/validating` Task，将其在事务中收敛为 `failed`，错误码为 `execution_interrupted`，并追加 `task.status_changed(failed)` 与 `task.failed`。不得静默重新执行导致重复 ToolCall/Chart；真正 Resume 留给后续 Adapter。

---

## 7. Deterministic Mock 设计

### 7.1 Mock Agent Runtime

`DeterministicMockAgentRuntime.Run` 对任意非空用户输入返回相同的 `node_exporter_overview` 计划：

1. 搜索 node exporter 指标。
2. 依次获取 `node_cpu_seconds_total`、`node_memory_MemAvailable_bytes`、`node_load1` 的标签。
3. 依次执行三条固定 PromQL；`mode=execute` 在服务端先校验再查询，不单独增加 Validate ToolCall。
4. 生成三张 timeseries Chart Proposal。
5. 返回固定助手说明。

规范性调用参数与顺序：

|顺序|Port 方法|关键参数|
|-|-|-|
|1|`SearchMetrics`|datasourceUid=`mock-prometheus`；query=`node exporter cpu memory load`；limit=`10`|
|2|`GetMetricLabels`|metricName=`node_cpu_seconds_total`|
|3|`GetMetricLabels`|metricName=`node_memory_MemAvailable_bytes`|
|4|`GetMetricLabels`|metricName=`node_load1`|
|5|`Execute`|CPU PromQL；stepSeconds=`300`；Task 冻结 timeRange|
|6|`Execute`|memory PromQL；stepSeconds=`300`；Task 冻结 timeRange|
|7|`Execute`|load PromQL；stepSeconds=`300`；Task 冻结 timeRange|

`AgentRunResult.AssistantText` 固定为 `已生成 node_exporter 的 CPU、内存和系统负载视图。`；`Proposals` 固定按 `cpu`、`memory`、`load` 排序。Runtime 必须校验 Search 结果包含固定 PromQL 所需指标、Labels 结果包含 `instance`，否则返回 `schema_validation_failed`，不能无视依赖结果继续硬编码成功。

Mock Agent 只能返回计划和结构化 Chart Proposal，并通过注入的 `MetricCatalogPort`、`QueryEnginePort` 获取数据。禁止直接 import fixture、直接构造查询结果或绕过 MCP Adapter。

`Resume` 本阶段返回 `not_implemented`；接口和测试必须存在。

### 7.2 固定 PromQL

```promql
100 * (1 - avg by (instance) (rate(node_cpu_seconds_total{mode="idle"}[5m])))
100 * node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes
node_load1
```

### 7.3 Fixture 目录

```text
data/mock-scenarios/node_exporter_overview/
├── manifest.yaml
├── search_metrics.json
├── metric_labels.json
├── query_cpu.json
├── query_memory.json
├── query_load.json
└── expected_task_events.json
```

Fixture 要求：

- `scenarioId=node_exporter_overview`，启动配置固定选择该场景。
- 每个文件通过相应 Tool/Domain Schema 校验。
- 时间点使用相对 offset，由 `ClockPort` 锚定到 Task 的 createdAt。
- 单元测试注入固定 Clock 和 ID Generator，禁止随机时间导致 snapshot 波动。
- 每张图至少两条 series、每条 series 至少六个 point，便于验证图例和折线。
- Fixture Loader 只存在于 Mock Adapter 包。
- Fixture 缺失或 Schema 不合法时 readiness 失败，不能运行到中途才返回空图。

Canonical Metric Fixture：

|metricName|type|description|labels|score|
|-|-|-|-|-|
|node_cpu_seconds_total|counter|Seconds the CPUs spent in each mode|cpu、instance、job、mode|1.00|
|node_memory_MemAvailable_bytes|gauge|Memory information field MemAvailable_bytes|instance、job|0.98|
|node_memory_MemTotal_bytes|gauge|Memory information field MemTotal_bytes|instance、job|0.96|
|node_load1|gauge|1m load average|instance、job|0.95|

所有 candidate 的 source 固定为：

```json
{"type":"mock_fixture","reference":"node_exporter_overview"}
```

Canonical Label Fixture：

|metricName|labelNames|sampleValues|
|-|-|-|
|node_cpu_seconds_total|cpu、instance、job、mode|cpu=`0,1`；instance=`node-a:9100,node-b:9100`；job=`node-exporter`；mode=`idle,user,system`|
|node_memory_MemAvailable_bytes|instance、job|instance=`node-a:9100,node-b:9100`；job=`node-exporter`|
|node_load1|instance、job|instance=`node-a:9100,node-b:9100`；job=`node-exporter`|

Canonical Chart Fixture：

|顺序|标题|PromQL|unit|legend|
|-|-|-|-|-|
|1|CPU 使用率|`100 * (1 - avg by (instance) (rate(node_cpu_seconds_total{mode="idle"}[5m])))`|percent|`{{instance}}`|
|2|内存可用率|`100 * node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes`|percent|`{{instance}}`|
|3|系统负载（1m）|`node_load1`|short|`{{instance}}`|

Canonical 时间序列以 Task.createdAt 为 `T0`，point offset 固定为 `[-30m,-25m,-20m,-15m,-10m,-5m,0m]`：

|图表|series|values|
|-|-|-|
|CPU 使用率|node-a:9100|22、24、31、35、29、27、26|
|CPU 使用率|node-b:9100|18、19、21、26、23、22、21|
|内存可用率|node-a:9100|68、67、65、64、66、65、65|
|内存可用率|node-b:9100|73、72、71、70、70、69、69|
|系统负载（1m）|node-a:9100|0.8、0.9、1.3、1.7、1.2、1.0、0.9|
|系统负载（1m）|node-b:9100|0.5、0.6、0.7、1.0、0.8、0.7、0.6|

每个 series labels 固定包含 `instance` 和 `job=node-exporter`。单元/Contract Test 固定 Clock 为 `2026-07-13T10:30:00Z`；浏览器演示使用真实 Task.createdAt，但 values 不变、timestamp 按 offset 平移。

### 7.4 最小启动配置

AI Core：

```yaml
environment: development
http:
  address: :8080
storage:
  driver: sqlite
  sqlitePath: /var/lib/ai-core/ai-core.db
adapters:
  agentRuntime: deterministic_mock
  metricCatalog: mcp
  queryEngine: mcp
  toolGateway: mcp
assistantMcp:
  endpoint: http://assistant-mcp:8081/mcp
  timeoutMs: 5000
```

assistant-mcp：

```yaml
environment: development
http:
  address: :8081
providers:
  prometheus: mock
mock:
  scenario: node_exporter_overview
  fixtureDir: /app/data/mock-scenarios
```

Plugin Backend：

```yaml
aiCore:
  endpoint: http://ai-core:8080
  timeoutMs: 10000
```

配置结构只在 Bootstrap/Config Adapter 中读取。Application、Domain、Tool Handler 不得接收整个配置对象。

---

## 8. 目标文件清单

以下是本计划完成时必须存在的最小文件结构。实现者可以在同一职责目录内增加私有辅助文件，但不得移动契约单一来源或把 Adapter 代码放入 domain/application。

```text
mini-torchbearing/
├── apps/
│   └── grafana-plugin/
│       ├── plugin.json
│       ├── README.md
│       ├── frontend/
│       │   ├── package.json
│       │   ├── package-lock.json
│       │   ├── tsconfig.json
│       │   └── src/
│       │       ├── module.tsx
│       │       ├── app/{routes.tsx,providers.tsx}
│       │       ├── pages/WorkbenchPage/WorkbenchPage.tsx
│       │       ├── features/conversation/{MessageList.tsx,PromptInput.tsx}
│       │       ├── features/task-progress/TaskProgress.tsx
│       │       ├── features/chart-workspace/{ChartGrid.tsx,ChartCard.tsx}
│       │       ├── api/{client.ts,event-stream.ts}
│       │       ├── api/generated/                 # [生成] plugin-resource 类型
│       │       ├── mappers/chart-data-frame.ts
│       │       └── state/task-event-reducer.ts
│       └── backend/
│           ├── go.mod
│           ├── go.sum
│           ├── Magefile.go
│           ├── cmd/plugin/main.go
│           └── internal/
│               ├── handlers/{session.go,task.go,events.go}
│               ├── context/grafana_context.go
│               ├── aicore/{client.go,generated/}   # generated/ 不手改
│               ├── errors/mapper.go
│               ├── config/config.go
│               └── bootstrap/wire.go
│
├── services/
│   ├── ai-core/
│   │   ├── go.mod
│   │   ├── go.sum
│   │   ├── Dockerfile
│   │   ├── README.md
│   │   ├── cmd/server/main.go
│   │   ├── internal/
│   │   │   ├── domain/
│   │   │   │   ├── common/{errors.go,ids.go,time_range.go}
│   │   │   │   ├── session/{session.go,message.go}
│   │   │   │   ├── task/{task.go,event.go,tool_call.go}
│   │   │   │   └── chart/{chart.go,query.go,execution.go,series.go}
│   │   │   ├── application/
│   │   │   │   ├── commands/{create_session.go,create_task.go}
│   │   │   │   ├── queries/{get_session.go,get_task.go}
│   │   │   │   ├── workflows/run_analysis.go
│   │   │   │   └── dto/{agent.go,metrics.go,query.go}
│   │   │   ├── ports/
│   │   │   │   ├── agent/runtime.go
│   │   │   │   ├── tools/{gateway.go,metric_catalog.go,query_engine.go}
│   │   │   │   ├── repositories/store.go
│   │   │   │   ├── events/{store.go,notifier.go}
│   │   │   │   ├── clocks/clock.go
│   │   │   │   ├── ids/generator.go
│   │   │   │   ├── model/model.go                 # [stub]
│   │   │   │   └── grafana/{read.go,write.go}     # [stub]
│   │   │   ├── adapters/
│   │   │   │   ├── inbound/http/
│   │   │   │   │   ├── server.go
│   │   │   │   │   ├── health.go
│   │   │   │   │   ├── sessions.go
│   │   │   │   │   ├── tasks.go
│   │   │   │   │   └── events.go
│   │   │   │   └── outbound/
│   │   │   │       ├── agent/mock/runtime.go
│   │   │   │       ├── agent/eino/stub.go
│   │   │   │       ├── tools/mcp/{gateway.go,metrics.go,query.go}
│   │   │   │       ├── storage/sqlite/{db.go,migrate.go,store.go}
│   │   │   │       ├── events/inmemory/notifier.go
│   │   │   │       ├── clocks/system.go
│   │   │   │       └── ids/random.go
│   │   │   └── bootstrap/{config.go,wire.go}
│   │   ├── migrations/sqlite/0001_initial.sql
│   │   └── tests/{contract,integration}/
│   │
│   └── assistant-mcp/
│       ├── go.mod
│       ├── go.sum
│       ├── Dockerfile
│       ├── README.md
│       ├── cmd/server/main.go
│       ├── internal/
│       │   ├── runtime/{registry.go,tool_context.go,permissions.go,errors.go}
│       │   ├── namespaces/grafana/{register.go,handlers.go,service.go}
│       │   ├── ports/prometheus/{port.go,types.go}
│       │   ├── adapters/prometheus/mock/{adapter.go,fixture_loader.go}
│       │   ├── adapters/prometheus/http/stub.go
│       │   └── bootstrap/{config.go,wire.go}
│       └── tests/{contract,integration}/
│
├── contracts/
│   ├── openapi/{plugin-resource.yaml,plugin-ai-core.yaml}
│   ├── schemas/
│   │   ├── common/{request-context.schema.json,error.schema.json,time-range.schema.json}
│   │   ├── session/{session.schema.json,message.schema.json}
│   │   ├── task/{task.schema.json,tool-call.schema.json}
│   │   └── chart/{chart.schema.json,execution.schema.json,series.schema.json}
│   ├── events/task-events.schema.json
│   ├── tools/grafana/
│   │   ├── search-metrics.{input,output}.schema.json
│   │   ├── get-metric-labels.{input,output}.schema.json
│   │   └── query-prometheus.{input,output}.schema.json
│   └── errors/error-codes.yaml
│
├── packages/
│   ├── generated-clients/{go,typescript}/         # [生成]
│   ├── generated-contracts/{go,typescript}/       # [生成]
│   ├── request-context-go/
│   └── testkit-go/{clock.go,ids.go,contracts.go}
│
├── data/mock-scenarios/node_exporter_overview/
│   ├── manifest.yaml
│   ├── search_metrics.json
│   ├── metric_labels.json
│   ├── query_cpu.json
│   ├── query_memory.json
│   ├── query_load.json
│   └── expected_task_events.json
│
├── tests/
│   ├── contract/
│   ├── integration/
│   └── e2e/mock/{api_test.go,workbench.spec.ts}
│
├── deploy/
│   ├── docker-compose/compose.mock-e2e.yaml
│   └── grafana/provisioning/plugins/
│
├── docs/implementation/basic_mock_progress.md
├── build/toolchain.lock
├── scripts/{generate-clients.sh,validate-contracts.sh,check-boundaries.sh,run-e2e.sh}
├── .env.example
├── .gitignore
├── docker-compose.yml
├── go.work
├── Makefile
└── README.md
```

### 8.1 关键文件职责

|文件|唯一职责|禁止内容|
|-|-|-|
|contracts/openapi/plugin-resource.yaml|Frontend 可见 Resource API|AI Core 内部 URL、数据库字段|
|contracts/openapi/plugin-ai-core.yaml|Plugin Backend 调 AI Core|Grafana SDK 类型|
|run_analysis.go|Task 状态推进、调用 Agent、持久化结果|MCP SDK、fixture、SQL|
|agent/mock/runtime.go|固定计划，调用类型化 Metric/Query Port|fixture 文件读取、最终 ChartDraft 持久化|
|tools/mcp/metrics.go、query.go|领域 DTO 与 MCP Tool DTO 映射|业务状态机|
|assistant-mcp handlers.go|Schema/context/permission/错误边界|HTTP Client、文件读取、SQL|
|assistant-mcp service.go|调用 PrometheusPort|mcp-go transport 类型|
|MockPrometheusAdapter|加载 fixture、实现 PrometheusPort|Task/Chart 业务逻辑|
|sqlite/store.go|Repository 和事务实现|HTTP/MCP/Agent 调用|
|Plugin Backend handlers|Grafana context、薄代理、错误映射|Session/Task 持久化、Agent 逻辑|
|event-stream.ts|SSE 连接、sequence、重连|图表业务状态|
|task-event-reducer.ts|幂等消费 TaskEvent|网络请求、Grafana SDK 调用|
|chart-data-frame.ts|Wire series → Grafana DataFrame|API 请求、fixture|
|bootstrap/wire.go|读取配置并组装 Adapter|业务分支|

### 8.2 文件创建顺序

1. Root/README/Makefile/go.work 和空模块。
2. `contracts/` 与 Fixture。
3. `packages/generated-*` 和 testkit。
4. AI Core domain/ports/store。
5. assistant-mcp runtime/namespace/Mock Adapter。
6. AI Core MCP Adapter/Mock Agent/Workflow/HTTP。
7. Plugin Backend。
8. Frontend。
9. Compose、E2E、运行文档。

任何阶段不得先创建调用方的手写 DTO 等待“以后再生成”。

---

## 9. 分阶段执行计划

以下阶段按顺序推进。每一阶段都有独立 Gate；上一 Gate 未通过时，不开始依赖其契约的下游实现。

### 阶段 0：范围冻结与仓库脚手架

任务：

- 创建计划要求的最小目录、Go modules、`go.work`、Frontend package。
- 建立各模块 README：职责、非职责、入口、Port、Adapter、数据所有权。
- 建立 `Makefile` 的占位 target：`generate`、`validate-contracts`、`test`、`smoke`、`e2e-mock`。
- 增加依赖边界检查脚本，禁止 domain/application import 外部 SDK。
- 固定 Go、Node、Grafana Plugin 工具链版本。

Gate G0：

- 所有模块可以执行最小 build/typecheck。
- 目录和包依赖方向通过检查。
- 本计划的范围内/范围外清单经确认，不在实现中扩张真实 Agent、Prometheus 或 Dashboard Write。

### 阶段 1：契约与生成代码

任务：

- 建立第 4、5、6 节列出的 OpenAPI、JSON Schema、SSE Schema、Tool Schema 和错误码。
- 建立 `plugin-resource.yaml` 与 `plugin-ai-core.yaml`，共享业务 DTO 只定义一次。
- 为 OpenAPI 生成 Go Server、Go Client 和 TypeScript Client/DTO。
- 为 MCP Tool Schema 生成或映射 Go/TypeScript 类型。
- 建立 Schema 校验脚本和 generated diff 检查。
- 创建 node_exporter fixture，并让 fixture validation 先通过。
- 编写契约示例：创建 Session、创建 Task、SSE Event、三个 MCP Tool 调用。

Gate G1（接口冻结 Gate）：

- OpenAPI/Schema 全部可校验。
- 生成代码可重复且无未提交 diff。
- Frontend、Plugin Backend、AI Core、assistant-mcp 不再手写跨进程 DTO。
- 所有接口明确错误码、required/optional、超时、幂等和 RequestContext 传播方式。
- Fixture 全部通过 Schema。

### 阶段 2：AI Core Domain、Port 与 SQLite

任务：

- 实现 Session、Message、Task、Chart、QueryExecution、TaskEvent 领域类型。
- 实现 Task/Chart 状态机与领域错误。
- 定义 AgentRuntime、MetricCatalog、QueryEngine、ToolGateway、Repository、Clock、ID、TaskEvent Store/Notifier Port。
- 建立 SQLite migration：sessions、messages、tasks、task_events、tool_calls、charts、chart_executions、idempotency_keys。
- 实现最小 SQLite ApplicationStore 和事务边界。
- 在事务中原子分配每 Task 的 sequence；事件提交后再通知。
- 实现内存 Notifier；SSE 总是以 Store Replay 为事实来源。
- 建立 Repository/TaskEvent Contract Test 套件。

Gate G2：

- Domain/Port 只依赖标准库和项目领域包。
- Task 合法/非法状态迁移单元测试通过。
- SQLite CRUD、tenant 隔离、幂等、事务回滚、sequence、重放测试通过。
- SQLite 错误已映射为领域错误。

### 阶段 3：assistant-mcp 与 Mock Prometheus Adapter

任务：

- 启动 mcp-go Streamable HTTP Server，提供 health/readiness 和 `tools/list`。
- 建立 `grafana` namespace registry、handler、service、ports、adapters 分层。
- 注册三个只读 Tool，并加载 Tool Schema。
- Handler 固定执行 decode → context validate → authorize → service → Port → output validate → sanitized log。
- 实现 `MockPrometheusAdapter` 和 fixture loader。
- 预留 `PrometheusHTTPAdapter` 构造器；启用返回 `not_implemented`。
- 建立 `PrometheusPort` Adapter Contract Test，Mock 实现必须通过。
- 建立 MCP 协议级 Contract/Smoke Test。

Gate G3：

- `assistant-mcp` 独立启动后 readiness 正常。
- `tools/list` 只暴露三个已实现只读 Tool。
- 三个 Tool 能通过真实 MCP transport 返回固定 node_exporter 结果。
- 空输入、未知 Tool、非法 Schema、无权限上下文返回分类错误。
- Handler/Service 不直接读取 fixture 或创建 SDK Client。

### 阶段 4：AI Core Mock Agent、MCP Adapter 与 Workflow

任务：

- 实现 MCP `ToolGatewayPort` Adapter，配置 assistant-mcp endpoint、timeout 和 trace headers。
- 实现类型化 `MetricCatalogPort`/`QueryEnginePort` MCP Adapter，Tool Name 只存在于该 Adapter。
- 实现 `DeterministicMockAgentRuntime`；依赖类型化指标/查询 Port，不直接依赖 MCP SDK。
- 实现最小 `RunAnalysisWorkflow`，按状态机推进 Task。
- 将 Agent Event 映射为 durable TaskEvent。
- 将 Query Result 映射为 Chart Proposal，再由 Application 创建 `ChartDraft`/`ChartExecution`。
- 实现失败收敛：Tool/MCP/Schema 错误 → `task.failed`，保留已写事件。
- 实现 AI Core health/readiness、Session/Task HTTP Handler 和 SSE Handler。
- Bootstrap 只在配置层组装 mock agent、MCP adapter、SQLite store。

Gate G4：

- AI Core + SQLite + assistant-mcp 集成测试可从 `POST /v1/tasks` 跑到三张 Chart。
- MCP 不可用时 Task 进入 failed，API 不返回假成功。
- SSE `afterSequence` 可重放，sequence 单调递增。
- Workflow/Application 无 `mockMode`、fixture、MCP SDK 或 Grafana SDK import。

### 阶段 5：Grafana Plugin Backend

任务：

- 建立 App Plugin Backend Resource Handler。
- 使用生成的 AI Core Go Client，不手写请求/响应 DTO。
- 从 Grafana Plugin Context 构建 `RequestContext`；开发环境也不得伪造管理员写权限。
- 实现 Session/Task API 薄代理、统一错误映射、request/trace ID 传播。
- 实现 SSE 无缓冲代理，透传 `Last-Event-ID`/`afterSequence`，正确处理客户端取消。
- 配置 AI Core endpoint、超时和最大响应体。
- 使用 Mock AI Core Client 编写 Handler Component Test，再接真实 AI Core Integration Test。

Gate G5：

- Plugin Backend 不保存 Session/Task、不执行业务 Workflow、不读取 fixture。
- Resource Handler → AI Core 的 Session/Task/Event 流程可调用。
- SSE 代理不合并、丢弃或重编号业务事件。
- AI Core 分类错误能稳定映射到 Plugin Resource Error。

### 阶段 6：Grafana Plugin Frontend

任务：

- 建立最小 Workbench：输入框、提交按钮、助手消息区、Task 状态区、三图网格；每个 ChartCard 显示标题、状态、可展开 PromQL 和 TimeSeries。
- 使用生成 TypeScript Client/DTO，只调用 Plugin Backend Resource API。
- 使用 TanStack Query 管理 Session/Task Snapshot。
- 实现 SSE Client：记录 latest sequence、断线退避、`afterSequence` 重连、事件去重。
- 实现 `ChartWireToDataFrame` Mapper，将领域 series 转成 Grafana DataFrame。
- 使用 Grafana UI TimeSeries 组件渲染三图，不自行实现 Canvas 绘图库。
- 支持 loading、completed、failed 三种最小状态。
- 页面刷新后根据当前 Task ID 从 sequence 0 或已知 sequence 重放并恢复三图。
- 编写 Mapper、Reducer、SSE 去重和组件测试。

Gate G6：

- Frontend 不直连 AI Core、assistant-mcp 或 fixture。
- 输入消息后可看到固定回复、三张标题正确的图和非空折线。
- 重复 SSE Event 不产生重复图表。
- Tool/Task 失败时显示结构化错误且保留已收到内容。

### 阶段 7：Docker Compose 与 Mock E2E

任务：

- 为 AI Core、assistant-mcp 建立最小多阶段 Dockerfile。
- 在 Linux 目标架构构建 Grafana Plugin Backend，构建 Frontend dist。
- 配置 Grafana 允许加载本地未签名插件。
- 建立 `compose.mock-e2e.yaml`：Grafana、AI Core、assistant-mcp；SQLite 使用独立 volume。
- 所有服务使用 readiness/healthcheck，不使用固定 sleep。
- 建立 `make e2e-mock` 和 teardown；CI 使用独立 Compose project name 并 `down -v`。
- 建立 API E2E：创建 Session/Task、收集 SSE、校验事件顺序和三张 Chart。
- 建立浏览器 E2E：登录 Grafana、输入消息、断言固定回复和三张图。
- 增加一次刷新/重连验证。

Gate G7（本计划完成 Gate）：

- 一条命令可启动环境并完成 readiness。
- API E2E 和浏览器 E2E 稳定通过。
- 全链路真实经过 Plugin Backend、AI Core、MCP transport 和 Mock Prometheus Adapter。
- 三张图不是 Frontend fixture，也不是 AI Core 直接硬编码结果。
- `make generate && make check && make e2e-mock` 通过。

### 阶段 8：骨架收口

任务：

- 补齐各服务 README、配置项、端口、启动方式、错误码和 Mock 场景说明。
- 输出一份调用示例和事件序列示例。
- 运行 secret scan、dependency boundary check、generated diff。
- 标记所有未实现接口的结构化错误和后续 Adapter 接入点。
- 记录已知限制；不在本阶段补做真实 Agent/Prometheus/Grafana Write。

Gate G8：

- 新开发者只按 README 即可启动和演示 Mock 闭环。
- 所有未实现能力均可识别，不存在空 handler、panic 或伪成功。
- 工作区无生成代码 diff，测试无依赖本机固定路径或当前时间。

---

## 10. 测试计划

### 10.1 Contract Test

- OpenAPI 3.1 校验。
- Session/Task/Event/Chart/Error JSON Schema 校验。
- MCP Tool Input/Output Schema 校验。
- 生成 Go/TypeScript Client 与契约一致性。
- Fixture Schema 校验。
- Error code 引用完整性。

### 10.2 Domain/Unit Test

- Task 状态机合法与非法迁移。
- Chart 状态和 QueryExecution 映射。
- Mock Agent 对非空输入生成固定计划；空输入返回 `invalid_argument`。
- Clock/ID 注入保证结果可重复。
- Event payload 与事件类型匹配。
- Wire Chart → Grafana DataFrame Mapper。
- Frontend SSE 去重和 sequence gap 处理。

### 10.3 Adapter Contract Test

- SQLite Store：CRUD、tenant 隔离、幂等、事务回滚、唯一约束、sequence、Replay。
- PrometheusPort：SearchMetrics、GetLabels、Query 的成功/非法输入/未知 PromQL。
- Mock AgentRuntime：Run 成功、空输入、Resume not implemented、dependency failure。
- ToolGateway：Tool Schema、timeout、unknown tool、MCP error mapping。
- ToolGateway 并发：两个不同 RequestContext 的调用不会复用/串写身份 Header。

### 10.4 Component Smoke

- AI Core：health → readiness → migration → 创建 Session/Task。
- assistant-mcp：health → readiness → tools/list → query Mock CPU。
- Plugin Backend：Mock AI Core 下代理创建 Task 和一段 SSE。
- Plugin Backend：丢弃浏览器伪造的 `X-MTB-*`，只注入 Grafana Context 派生身份。
- Frontend：Mock Plugin API 下显示空态、运行态、三图、失败态。

### 10.5 Integration Test

- AI Core + SQLite。
- AI Core + assistant-mcp（真实 MCP transport）。
- Plugin Backend + AI Core（真实 HTTP/SSE）。
- Frontend reducer/mapper + 真实录制 Event Fixture。

### 10.6 Mock E2E 必测场景

主路径：

1. 创建 Session。
2. 输入任意消息并创建 Task。
3. 收到固定 assistant delta/completed。
4. 收到七组 Tool started/completed：1 次 search、3 次 labels、3 次 query；`tool_calls` 表与事件 toolCallId 一一对应。
5. 收到三张 Chart 和非空 series。
6. Task 完成，事件 sequence 无重复、无空洞。
7. 重连 `afterSequence=N` 只返回 N 之后的事件。
8. 重复 Idempotency-Key 不创建第二个 Task/Chart。
9. Plugin Backend、AI Core、assistant-mcp 日志中可用同一 requestId/traceId 串起调用链，日志不包含敏感字段或完整 series。

最小失败路径：

1. 空消息 → `invalid_argument`。
2. 未知 Session/Task → `resource_not_found`。
3. assistant-mcp 不可用 → Task `failed` + `dependency_unavailable`。
4. Fixture Schema 非法 → assistant-mcp readiness 失败。
5. SSE 重复投递 → Frontend 不重复建图。

---

## 11. Make/CI 入口

```text
make bootstrap-check        # 目录、工具链、模块和依赖边界基础检查
make generate             # 生成 Go/TS Client 和 Contract 类型
make generated-client-diff # 比较生成前后 hash；不受其他未提交文件影响
make validate-contracts   # OpenAPI/JSON Schema/Tool/Event/Fixture
make lint                 # Go/TS/格式
make test                 # Unit + Contract
make test-adapters        # SQLite/Prometheus Mock/ToolGateway Contract
make test-ai-core-domain  # AI Core domain/application unit
make test-sqlite          # SQLite Repository/Event Contract
make test-assistant-mcp   # MCP namespace/adapter/contract
make test-ai-mcp          # AI Core + assistant-mcp integration
make test-plugin-backend  # Resource Handler/component
make test-frontend        # TS mapper/reducer/component
make smoke                # 三个组件的独立 Smoke
make e2e-mock             # Compose + API E2E + Browser E2E
make check                # generated-client-diff + validate + lint + test + boundary + secret scan
```

`generated-client-diff` 必须在运行生成器前记录所有生成目录的内容 hash，生成后比较同一组路径；不得用整个工作区的 `git diff --exit-code`，否则已有用户文档/代码修改会造成假失败。虽然 target 沿用 `client` 名称，它同时覆盖 generated contracts。

PR 硬门槛：

```text
validate-contracts
generated-client-diff
lint
typecheck
unit-test
contract-test
dependency-boundary-check
```

当阶段 7 稳定后追加：

```text
component-smoke
mock-e2e
```

---

## 12. 推荐提交拆分

每个提交必须可编译或只包含可校验契约，避免一个提交同时引入全部模块。

1. `建立 monorepo 与模块脚手架`
2. `定义基本闭环 OpenAPI 与共享 Schema`
3. `生成 Go TypeScript 客户端并加入契约校验`
4. `实现 AI Core 领域对象 Port 与 SQLite Store`
5. `实现 assistant-mcp 和 node_exporter Mock Adapter`
6. `实现 Deterministic Mock Agent 与 AI Core Workflow`
7. `实现 Plugin Backend 薄代理与 SSE 转发`
8. `实现 Grafana 工作台和三图渲染`
9. `加入 Mock Compose 与纵向 E2E`
10. `补齐边界检查 README 与骨架验收`

---

## 13. 完成定义（Definition of Done）

只有同时满足以下条件，本计划才算完成：

- Grafana 插件页面可以输入消息并展示固定助手回复和三张 node_exporter 图。
- 数据真实经过 Plugin Backend → AI Core → MCP → assistant-mcp → Mock Prometheus Adapter，再返回前端。
- Mock Agent 不直接读取 fixture、不直接返回最终 ChartDraft。
- Frontend 不直连 AI Core/MCP，不包含时间序列 fixture。
- Application/Domain 无 `mockMode` 分支，无外部 SDK 类型。
- 跨进程 DTO 全部由契约生成或引用共享 Schema，无手写重复 DTO。
- Session、Task、Chart、TaskEvent 已持久化到 AI Core SQLite。
- TaskEvent 先持久化后通知，sequence 单调递增，SSE 支持 replay。
- Mock/Real Adapter 插槽、构造器、配置键和 Contract Test 入口存在。
- 未实现能力返回 `not_implemented`/`tool_not_supported`，不伪装成功。
- `make generate` 可重复，`make check` 和 `make e2e-mock` 通过。
- README 能指导新开发者从空环境启动并复现闭环。

以下结果不算完成：

- Frontend 直接 import 三张图 fixture。
- AI Core 根据 `mockMode` 直接返回固定 HTTP JSON。
- Mock Agent 跳过 MCP，直接拼三张 Chart。
- assistant-mcp Handler 直接读取文件，绕过 namespace service 和 PrometheusPort。
- SSE 只推内存事件，刷新后无法重放。
- DTO 在 Frontend、Plugin Backend、AI Core 各写一份。
- 未实现接口返回 `200 {}`。

---

## 14. 主要风险与控制

|风险|控制方式|
|-|-|
|为了尽快出图绕过模块边界|E2E 增加调用链证据；禁止 Frontend/Application 读取 fixture|
|契约先写但实现中漂移|生成 Client、generated diff、Handler Contract Test 作为硬门槛|
|Mock Agent 与未来 Agent 语义不一致|固定使用 `AgentRuntimePort`、类型化 Tool Port 和结构化 AgentRunResult；Eino 只替换 Adapter|
|MCP 返回自由 JSON|Input/Output 都有 JSON Schema；MCP Adapter 映射为领域类型|
|异步事件偶发丢失|SQLite TaskEventStore 为事实来源，Notifier 只做唤醒，SSE 必须 Replay|
|固定时间导致图表过期或测试波动|Fixture 使用相对 offset；ClockPort 在测试中固定|
|Grafana Plugin Backend 架构不匹配|只在 Linux 目标架构构建并在 Compose 中加载|
|范围膨胀到真实 Agent/Prometheus/写 Dashboard|以第 2 节为 Scope Gate；本计划完成后再开下一执行计划|

---

## 15. 后续替换点

本计划完成后，下一阶段只能按以下顺序替换 Adapter，不重写闭环：

1. `MockPrometheusAdapter → PrometheusHTTPAdapter`。
2. `DeterministicMockAgentRuntime → EinoAgentRuntimeAdapter`。
3. `Model disabled/mock → Model Gateway Adapter`。
4. `Mock Grafana Read/Write → Plugin Backend controlled proxy`。
5. 在已有 ChartDraft 之上增加 PanelDraft、Approval 和 Dashboard Save。

每次替换必须先让 Real Adapter 通过与 Mock 相同的 Contract Test，再进入纵向 E2E。

---

## 16. 新 Session 进度记录与恢复

执行开始时创建 `docs/implementation/basic_mock_progress.md`。该文件是跨 Session 的唯一进度事实，不用聊天历史作为完成依据。

### 16.1 进度文件模板

```markdown
# Basic Mock Skeleton Progress

lastUpdated: <UTC RFC3339>
currentGate: G0
status: in_progress  # pending | in_progress | passed | blocked
headCommit: <git rev-parse --short HEAD>
worktreeSummary: <git status --short 摘要>

## Passed Gates

- [ ] G0 Scaffold
- [ ] G1 Contracts
- [ ] G2 Domain/SQLite
- [ ] G3 assistant-mcp
- [ ] G4 AI Workflow
- [ ] G5 Plugin Backend
- [ ] G6 Frontend
- [ ] G7 Mock E2E
- [ ] G8 Closeout

## Last Verified Commands

|command|result|timestamp|notes|
|-|-|-|-|

## Files/Interfaces Completed

- <path or interface>: <status>

## Remaining Work For Current Gate

1. ...

## Known Failures

- command:
- relevant output summary:
- suspected cause:
- next safe action:

## Decisions Made Within Plan Defaults

- decision:
- reason:
- affected files:

## Blockers Requiring User/Approval

- none
```

禁止把 token、Cookie、密码、Grafana grant 或大段构建日志写入 progress 文件。只记录命令、结果和定位问题所需的摘要。

### 16.2 Gate 验证与证据

|Gate|最低验证命令|必须记录的证据|
|-|-|-|
|G0|`make bootstrap-check`|模块/工具链版本、目录边界通过；缺失依赖及处理方式|
|G1|`make validate-contracts`、`make generate`、`make generated-client-diff`|契约文件列表、生成器版本、生成前后零 diff|
|G2|`make test-ai-core-domain`、`make test-sqlite`|状态机、tenant、事务、sequence、Replay 测试通过|
|G3|`make test-assistant-mcp` + assistant-mcp smoke|tools/list 为 3；三个 fixture tool 可通过 MCP client 调用|
|G4|`make test-ai-mcp`|创建 Task 后得到 3 Chart；MCP down 进入 failed|
|G5|`make test-plugin-backend`|Resource API/错误/SSE proxy component test|
|G6|`make test-frontend`|Mapper、Reducer、重连去重、三图组件|
|G7|`make e2e-mock`|API + Browser E2E；7 ToolCall、3 Chart、Replay、幂等|
|G8|`make check`|生成零 diff、所有测试/边界/secret scan 通过|

如果某个 Make target 尚未建立，先把该 target 作为当前 Gate 的交付物实现；不能用一串个人临时命令长期替代标准入口。

### 16.3 新 Session 恢复算法

1. 读取 `CLAUDE.md`、本文件和 progress 文件。
2. 查看 `git status --short`、最近提交和用户未提交修改。
3. 不相信 progress 中单独写的“passed”；重新运行 currentGate 的最低验证命令。
4. 对照第 8 节目标文件清单，确认关键契约/生成文件/测试实际存在。
5. 如果验证通过，将 currentGate 标为 passed，进入下一个 Gate。
6. 如果验证失败，从 progress 记录的 known failure 继续定位，不重新生成已完成模块。
7. 如果 progress 缺失但代码已存在，从 G0 开始审计，找到首个未满足 Gate 后继续。
8. 未经用户明确授权，不因“阶段完成”自动 commit、push 或创建 PR。

### 16.4 中断前的最小交接动作

任何执行 Session 在结束前必须：

- 格式化本次修改。
- 运行与本次修改最相关的最小测试。
- 更新 progress 的 currentGate、已验证命令、remaining 和 known failure。
- 输出 `git status --short`，不清理用户修改。
- 在最终回复中说明已完成 Gate、验证结果和下一个具体动作。

---

## 17. 接口调用与验收样例

本节给出实现时必须支持的代表性 Wire 示例。正式 Schema 是单一来源；示例与 Schema 冲突时先修正文档/Schema，不能让各模块分别兼容两套格式。

### 17.1 Create Session

Frontend → Plugin Resource：

```http
POST /api/plugins/mini-torchbearing-app/resources/sessions
Content-Type: application/json
X-Request-ID: req-demo-001

{"title":"Node exporter mock analysis"}
```

Plugin Backend → AI Core：

```http
POST /v1/sessions
Content-Type: application/json
X-Request-ID: req-demo-001
X-Trace-ID: trace-demo-001
X-MTB-Tenant-ID: org:1
X-MTB-Org-ID: 1
X-MTB-User-ID: user:1
X-MTB-Roles: Admin
X-MTB-Permissions: datasources:query

{"title":"Node exporter mock analysis"}
```

代表性响应：

```json
{
  "id": "session_demo",
  "tenantId": "org:1",
  "title": "Node exporter mock analysis",
  "status": "active",
  "createdBy": "user:1",
  "createdAt": "2026-07-13T10:30:00Z",
  "updatedAt": "2026-07-13T10:30:00Z",
  "version": 1
}
```

### 17.2 Create Task

```http
POST /api/plugins/mini-torchbearing-app/resources/tasks
Content-Type: application/json
Idempotency-Key: demo-task-001
X-Request-ID: req-demo-002

{
  "sessionId": "session_demo",
  "message": "帮我看看 node_exporter 最近 30 分钟的 CPU、内存和系统负载",
  "analysisContext": {
    "datasourceUid": "mock-prometheus",
    "timeRange": {"relativeDuration": "30m"}
  }
}
```

代表性 `202` 响应：

```json
{
  "id": "task_demo",
  "sessionId": "session_demo",
  "status": "created",
  "inputMessageId": "message_demo",
  "datasourceUid": "mock-prometheus",
  "timeRange": {
    "from": "2026-07-13T10:00:00Z",
    "to": "2026-07-13T10:30:00Z"
  },
  "latestSequence": 1,
  "error": null,
  "createdAt": "2026-07-13T10:30:00Z",
  "startedAt": null,
  "completedAt": null,
  "updatedAt": "2026-07-13T10:30:00Z",
  "version": 1
}
```

### 17.3 MCP Tool 逻辑载荷

以下展示 `tools/call` 经 mcp-go 解码后的逻辑参数和结构化结果；JSON-RPC envelope、session ID 和协议版本由 SDK 处理，业务代码不能自行发明另一套 HTTP Tool API。

Search：

```json
{
  "name": "grafana.search_metrics",
  "arguments": {
    "datasourceUid": "mock-prometheus",
    "query": "node exporter cpu memory load",
    "limit": 10
  }
}
```

```json
{
  "candidates": [
    {
      "metricName": "node_cpu_seconds_total",
      "type": "counter",
      "description": "Seconds the CPUs spent in each mode",
      "labels": ["cpu", "instance", "job", "mode"],
      "score": 1,
      "sources": [{"type": "mock_fixture", "reference": "node_exporter_overview"}]
    }
  ]
}
```

Labels（另外两个调用只替换 `metricName`）：

```json
{
  "name": "grafana.get_metric_labels",
  "arguments": {
    "datasourceUid": "mock-prometheus",
    "metricName": "node_cpu_seconds_total"
  }
}
```

```json
{
  "metricName": "node_cpu_seconds_total",
  "labelNames": ["cpu", "instance", "job", "mode"],
  "sampleValues": {
    "cpu": ["0", "1"],
    "instance": ["node-a:9100", "node-b:9100"],
    "job": ["node-exporter"],
    "mode": ["idle", "user", "system"]
  }
}
```

Execute CPU Query（memory/load 调用替换 expression，其他字段相同）：

```json
{
  "name": "grafana.query_prometheus",
  "arguments": {
    "datasourceUid": "mock-prometheus",
    "expression": "100 * (1 - avg by (instance) (rate(node_cpu_seconds_total{mode=\"idle\"}[5m])))",
    "start": "2026-07-13T10:00:00Z",
    "end": "2026-07-13T10:30:00Z",
    "stepSeconds": 300,
    "mode": "execute"
  }
}
```

```json
{
  "validation": {
    "valid": true,
    "errors": [],
    "warnings": [],
    "metricNames": ["node_cpu_seconds_total"],
    "labelNames": ["instance", "mode"]
  },
  "status": "success",
  "resultType": "matrix",
  "series": [
    {
      "name": "node-a:9100",
      "labels": {"instance": "node-a:9100", "job": "node-exporter"},
      "points": [{"timestamp": "2026-07-13T10:00:00Z", "value": 22}]
    }
  ],
  "durationMs": 12,
  "warnings": []
}
```

正式 output 包含第 7.3 节全部 series/points。三个 Tool 的 schema 必须拒绝额外未知字段，除非字段在契约中显式声明为向后兼容扩展。

### 17.4 SSE TaskEvent

```text
id: 12
event: chart.execution_completed
data: {"eventId":"event_12","taskId":"task_demo","sessionId":"session_demo","sequence":12,"type":"chart.execution_completed","timestamp":"2026-07-13T10:30:00Z","payload":{"chartId":"chart_cpu","chartStatus":"ready","execution":{"id":"execution_cpu","status":"success","seriesCount":2,"durationMs":12,"sampleRange":{"from":"2026-07-13T10:00:00Z","to":"2026-07-13T10:30:00Z"},"series":[{"name":"node-a:9100","labels":{"instance":"node-a:9100","job":"node-exporter"},"points":[{"timestamp":"2026-07-13T10:00:00Z","value":22},{"timestamp":"2026-07-13T10:05:00Z","value":24}]}]}}}
```

正式 fixture 必须包含第 7.3 节的全部 7 个 point 和 2 条 series；此处为文档可读性只截取部分 point。

### 17.5 统一 Error Envelope

```json
{
  "error": {
    "code": "dependency_unavailable",
    "message": "assistant-mcp is unavailable",
    "retryable": true,
    "requestId": "req-demo-002",
    "details": {
      "dependency": "assistant-mcp"
    }
  }
}
```

`details` 只能包含已分类、可公开字段；不得放原始响应体、SQL、路径、凭证或 stack trace。

### 17.6 最终 UI 验收

页面必须稳定呈现：

```text
Node exporter mock analysis

User:
帮我看看 node_exporter 最近 30 分钟的 CPU、内存和系统负载

Assistant:
已生成 node_exporter 的 CPU、内存和系统负载视图。

Task: completed

[CPU 使用率]       [内存可用率]       [系统负载（1m）]
[2 series / %]     [2 series / %]     [2 series]
[PromQL 可展开]     [PromQL 可展开]     [PromQL 可展开]
```

验收不依赖像素级截图，但浏览器测试必须断言：三张标题、固定回复、completed 状态、每张图两条 series、PromQL 文本存在，以及刷新后结果仍在。
