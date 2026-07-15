# Grafana 自然语言指标分析工作台：可执行骨架代码设计

> 文档状态：Implementation Blueprint
> 版本：v1.3
> 适用范围：MS1–MS4；首个实现目标为“完整契约 + 可替换 Adapter + 可独立验证的模块骨架”
> 目标读者：架构师、前端工程师、Grafana 插件工程师、后端工程师、AI/Agent 工程师、测试工程师、SRE
> 最后更新：2026-07-13

---

## 1. 文档目的与权威性

本文件是后续生成骨架代码的直接输入。代码目录、模块依赖、Port、Adapter、领域对象、状态机、传输协议和验收分级均以本文为准；实现过程中发现设计缺口时，应先修改本文或新增 ADR，再修改代码。

这里的“骨架代码”不是一次性 Demo，也不要求首个提交就让所有服务端到端运行。它必须做到：

1. 模块边界完整，依赖方向明确。
2. 跨模块契约可校验，并可生成客户端或类型。
3. 核心业务只依赖 Port，不绑定数据库、Grafana、Prometheus、LLM、Eino 或 MCP 的具体实现。
4. 每个外部依赖至少有 Adapter 插槽；当前要验证的小模块可提供确定性 Mock Adapter。
5. 真实 Adapter 可以逐个替换 Mock，而不修改领域层和应用层。
6. 模块可以单独编译、执行单元测试或启动最小进程；全系统联调属于更高一级验收，不是首个骨架提交的硬门槛。
7. 权限、审批、审计、数据出域和可观测性从接口层进入设计。

### 1.1 与其他文档的关系

- `../design/original_task.md` 定义项目初心。
- `../design/product_design_final.md` 决定当前产品范围、功能边界和验收方向；具体增量仍须由 active execution plan 授权，不得因为本文预留了接口就提前扩大实现范围。`../design/product_design.md` 仅保留历史 MS1-MS4 阶段证据。
- `../design/arch_design_draft.md` 定义长期总体分层。
- `../design/arch_design_detail.md` 中标注“拍板”的模块决策优先于本文中的通用建议。
- `basic_mock_skeleton_execution_plan.md` 将本文收敛为首个可执行切片：只跑确定性 Mock Agent、Mock Prometheus 和三图纵向闭环，不改变本文的长期接口与替换边界。
- 本文负责把上述决策翻译成可直接建立的代码结构和稳定契约。

如果实现需要偏离本文，必须先记录 ADR。涉及产品边界、权限模型、数据所有权或不可逆存储结构的偏离，需要先向项目负责人确认。

### 1.2 规范用语

- **必须**：骨架代码和后续实现都不可违反。
- **应该**：默认实现方式；偏离时需要 ADR。
- **可以**：可选扩展，不得成为首版核心依赖。

---

## 2. 设计原则

### 2.1 逻辑分层，部署收敛

逻辑上分为：Grafana App Plugin Frontend、Grafana Plugin Backend、AI Core、MCP 工具层、Model Provider 层、数据与治理层。

初期部署单元收敛为：

- Grafana（承载 Plugin Frontend 与 Plugin Backend）
- AI Core
- `assistant-mcp`（一个进程、四个 namespace）
- SQLite 文件（分别由数据所有者独占）
- 可选 Prometheus（Mock Adapter 或真实 Prometheus 容器）

本地真实基础设施混合 E2E 额外运行 Prometheus 与 `node_exporter`。此时共有五个常驻运行容器：Grafana、AI Core、`assistant-mcp`、Prometheus、`node_exporter`；`plugin-build` 只能作为一次性构建任务，不计入常驻部署单元。模型仍默认使用 Deterministic Mock，以保证自然语言到工具计划的输出可重复；真实查询结果、插件通信、会话持久化和 Dashboard 写入走真实链路。

PostgreSQL、Redis、向量库和对象存储均不是首个骨架运行的前置条件。

### 2.2 契约先行

跨进程通信必须先定义 OpenAPI、SSE Event Schema、MCP Tool Schema、错误码、权限、幂等和超时。跨包接口必须先定义 Go Port 和领域类型。

契约的单一来源位于 `contracts/`。生成代码只能从契约产生，不允许前端、Plugin Backend、AI Core 各自手写同一 DTO。

### 2.3 Port / Adapter 隔离

领域层和应用层不得 import 以下实现依赖：

- Grafana Plugin SDK
- Prometheus/Grafana HTTP Client
- Eino、mcp-go 或具体模型 SDK
- SQLite、PostgreSQL 驱动
- Redis、向量库或对象存储 SDK

这些依赖只能出现在 `adapters/` 或 `infrastructure/`。业务层依赖 `MetricCatalogPort`、`QueryEnginePort`、`AgentRuntimePort`、`ToolGatewayPort`、`DashboardReadPort`、`DashboardWritePort`、Repository 和 Event Store 等接口。

### 2.4 当前实现与替换边界

|能力|首个 Adapter|后续 Adapter|业务层是否感知|
|-|-|-|-|
|应用持久化|SQLite|PostgreSQL|否|
|任务事件|SQLite durable event store|PostgreSQL + 可选 Redis notification|否|
|Agent Runtime|Eino|其他 runtime（如未来需要）|否|
|模型调用|Eino ChatModel/Model Gateway adapter|其他模型供应商|否|
|工具协议|mcp-go Streamable HTTP|其他兼容 MCP transport|否|
|指标目录/查询|Deterministic Mock|Prometheus|否|
|Grafana 读写|Mock|Plugin Backend 受控代理|否|
|知识检索|关键词/Mock|Embedding/Vector Search|否|
|文件资产|本地文件系统|对象存储|否|

“可替换”不表示首版必须实现全部 Adapter；但 Port、配置键、错误语义和 Contract Test 插槽必须存在。

### 2.5 Mock 只能替换 Adapter

禁止在业务代码中判断 `mockMode`。Mock 必须实现与 Real Adapter 相同的 Port，并通过同一套 Adapter Contract Test。

```text
MetricCatalogPort
├── MockMetricCatalogAdapter
└── PrometheusMetricCatalogAdapter
```

Mock 返回必须确定、可复现、可通过场景 ID 选择，不能返回随机或无 Schema 的结果。

### 2.6 只读和写操作分离

所有 Grafana 只读与写入能力使用不同 Port。写操作必须经过：

```text
Prepare → Draft/Diff → SaveIntent → Approval → Execute → Audit
```

写入时必须携带当前用户上下文、审批证据、幂等键和目标版本。删除/覆盖 Dashboard 或 Panel 在 v1 为 forbidden。

这里的 Approval 适用于 AI 发起、MCP write tool、Grafana/外部系统副作用和 Skill/Playbook 晋升。用户在 Web UI 中直接提交普通知识条目的 CRUD 表单时，该提交本身可视为显式确认，不额外进入 Eino Interrupt；但仍必须做 Folder 权限、乐观锁、幂等和审计。不同确认方式统一映射为 `ApprovalEvidence`/`UserConfirmationEvidence`，由 Policy 判断所需等级。

### 2.7 会话是一级对象

会话必须结构化保存消息、Grafana/Folder 上下文、可展示计划、工具调用摘要、图表、PromQL 修订、执行结果、备注、保存意图、审批、写入结果、分享和 Fork 关系。只保存聊天文本不合格。

### 2.8 ChartDraft 与 PanelDraft 分离

```text
ChartDraft（产品领域对象）
  ↓ PanelCompilerPort
PanelDraft（特定 Grafana 版本）
  ↓ Approval + DashboardWritePort
Grafana Panel
```

领域层不得保存或操作完整 Grafana Panel JSON。

### 2.9 任务和事件必须持久化

任务状态、步骤和事件必须先写入 durable store，再向 SSE 客户端推送。每个 Task 的 `sequence` 单调递增；刷新、断线或进程重启后可以从 `afterSequence` 重放。

Redis 后续只能作为通知/缓存 Adapter，不能成为事件事实来源。

### 2.10 不保存模型私有推理过程

只保存用户可见回复、计划摘要、工具调用、验证结果、风险和决策依据摘要；不保存模型私有思维链。

### 2.11 依赖方向

```text
inbound adapter → application → domain
outbound adapter ───────────→ ports ← application
bootstrap 负责组装 adapter，domain/application 不反向依赖 bootstrap
```

任何外部框架类型不得穿过 Port。Port 参数和返回值只能使用标准库类型或项目领域类型。

---

## 3. 目标系统边界

### 3.1 全周期骨架需要承载

- 自然语言创建指标分析任务
- Prometheus 指标/标签检索、PromQL 生成、校验和执行
- 多图表创建、编辑、替换、关闭、Pin
- 会话保存、恢复、搜索、分享和 Fork
- 模板执行、Skill、Candidate Playbook 和 Playbook
- 图表保存到 Dashboard、审批和版本冲突
- Grafana Alert Webhook 与异步分析
- Knowledge 检索、模型降级、评测、审计和成本统计

“承载”表示领域对象、接口和扩展点存在，不表示当前 Milestone 已有真实实现。

### 3.2 首个骨架允许只提供接口或 Mock

- Skill/Playbook 生成与执行
- Alert Webhook 完整链路
- Loki、Tempo、SQL 等非 Prometheus 数据源
- 截图、报告、对象存储
- 模型自动路由
- Dashboard 回滚
- 向量检索
- 智能推荐和自动 RCA

未实现能力必须返回结构化 `not_implemented`/`tool_not_supported`，不得 panic、返回空对象或伪装成功。

### 3.3 不进入骨架核心

- 自动修复线上系统
- 自动删除或无确认覆盖 Dashboard/Panel
- 完整 Grafana Panel Editor
- 全量日志/Trace 上传到外部模型
- 多人实时协作编辑
- 多租户计费和 License

---

## 4. 固定技术基线

本节是首个骨架的具体选择。实现仍需通过 Port/Adapter 隔离，不能把选择泄漏到业务层。

### 4.1 Grafana Plugin Frontend

- React + TypeScript
- Grafana UI / Grafana Runtime
- TanStack Query 管理服务端状态
- 轻量本地状态可用 Zustand；不得复制服务端事实状态
- SSE 客户端支持 `afterSequence` 重连

### 4.2 Grafana Plugin Backend

- Go
- Grafana Plugin SDK + Resource Handler
- OpenTelemetry
- 从 `contracts/openapi/plugin-ai-core.yaml` 生成 AI Core Client

### 4.3 AI Core

- Go
- HTTP 框架只存在于 inbound adapter，可选择标准库路由器或轻量路由器
- Eino 作为首个 `AgentRuntimePort` Adapter
- Eino Interrupt/CheckPoint 实现 HITL，ChatModel Failover 实现主备模型
- Eino 类型不得进入 domain、application 或公共 HTTP DTO

### 4.4 assistant-mcp

- Go + mcp-go
- Streamable HTTP
- v1 一个进程，四个 namespace：`grafana.*`、`knowledge.*`、`playbook.*`、`skills.*`
- namespace 在代码中独立包、独立 Port、独立 Contract Test；以后可拆进程但工具名和 Schema 不变

### 4.5 数据层

- SQLite 是首个真实 Adapter，适合本地和单实例骨架
- PostgreSQL 是后续 Adapter，Repository 接口和领域对象不得变化
- AI Core 与 assistant-mcp 各自拥有存储，不允许跨服务直接查表或共享 SQLite 文件
- Redis 仅作为未来锁、缓存或通知 Adapter
- 向量库、对象存储均后置

建议驱动边界：SQLite 与 PostgreSQL 驱动只出现在各自 Adapter 包；业务层不接触 `*sql.DB`、事务对象、SQL 错误码或数据库 JSON 类型。

### 4.6 契约与代码生成

- OpenAPI 3.1：HTTP API
- JSON Schema：领域快照、SSE Event、MCP Tool 输入输出
- TypeScript Client：Plugin Frontend
- Go Client：Plugin Backend 与服务间调用
- Go 类型为运行时代码，Schema 是跨进程契约单一来源

### 4.7 Wire 格式约定

- JSON 字段统一 `lowerCamelCase`
- 枚举值和错误码统一 `lower_snake_case`
- 时间统一 UTC RFC3339
- ID 为不透明字符串；调用方不得解析 ID 结构
- 金额、token 数量、耗时等需要明确单位；耗时字段使用 `durationMs`
- 可选字段必须在 Schema 中显式 nullable/optional，不用零值猜语义
- 所有列表接口预留 `pageSize` 和 `pageToken`

---

## 5. Monorepo 目录结构

首个骨架必须按以下结构创建；可以暂缺后续模块的实现文件，但目录、README、契约或占位 Port 必须可追踪。

```text
mini-torchbearing/
├── apps/
│   └── grafana-plugin/
│       ├── frontend/
│       │   ├── src/{app,pages,features,components,api,hooks,types}/
│       │   ├── package.json
│       │   └── tsconfig.json
│       ├── backend/
│       │   ├── cmd/plugin/
│       │   ├── internal/{handlers,context,auth,permissions,proxy,aicore,audit,config,errors}/
│       │   ├── go.mod
│       │   └── Magefile.go
│       ├── plugin.json
│       └── README.md
│
├── services/
│   ├── ai-core/
│   │   ├── cmd/server/main.go
│   │   ├── internal/
│   │   │   ├── domain/{session,task,chart,query,approval,template,playbook,alert,common}/
│   │   │   ├── application/{commands,queries,workflows,policies,services,dto}/
│   │   │   ├── ports/{repositories,agent,tools,grafana,model,knowledge,events,storage}/
│   │   │   ├── adapters/
│   │   │   │   ├── inbound/http/
│   │   │   │   └── outbound/{agent/eino,tools/mcp,model,storage/sqlite,storage/postgres,events,mock}/
│   │   │   └── bootstrap/
│   │   ├── migrations/{sqlite,postgres}/
│   │   ├── tests/{contract,integration}/
│   │   ├── go.mod
│   │   └── Dockerfile
│   │
│   ├── assistant-mcp/
│   │   ├── cmd/server/main.go
│   │   ├── internal/
│   │   │   ├── runtime/{registry,permissions,audit,errors}/
│   │   │   ├── namespaces/{grafana,knowledge,playbook,skills}/
│   │   │   ├── ports/{prometheus,grafana,knowledge,playbook,skills,storage}/
│   │   │   ├── adapters/{prometheus,grafana,storage/sqlite,storage/postgres,filesystem,mock}/
│   │   │   └── bootstrap/
│   │   ├── migrations/{sqlite,postgres}/
│   │   ├── tests/{contract,integration}/
│   │   ├── go.mod
│   │   └── Dockerfile
│   │
│   └── alert-receiver/                 # 后续模块；首版可仅有 README/contract
│       ├── cmd/server/
│       ├── internal/
│       └── go.mod
│
├── contracts/
│   ├── openapi/{plugin-resource.yaml,plugin-ai-core.yaml,ai-core-public.yaml,alert-receiver.yaml}
│   ├── schemas/{session,task,chart,approval,playbook,alert}.schema.json
│   ├── events/task-events.schema.json
│   ├── tools/{grafana,knowledge,playbook,skills}/
│   └── errors/error-codes.yaml
│
├── packages/
│   ├── generated-clients/{typescript,go}/
│   ├── generated-contracts/{typescript,go}/
│   ├── observability-go/
│   ├── request-context-go/
│   ├── testkit-go/
│   └── fixtures/
│
├── data/{skills,playbooks,mock-scenarios}/
├── tests/{contract,integration,e2e/{mock,local-real},golden-queries}/
├── deploy/
│   ├── docker-compose/{compose.mock-e2e.yaml,compose.local-real.yaml}
│   ├── prometheus/prometheus.yml
│   ├── grafana/provisioning/{datasources,dashboards}/
│   └── {kubernetes,helm}/
├── docs/{adr,api,development,runbooks}/
├── scripts/{generate-clients.sh,validate-contracts.sh,seed-mock-data.sh,run-e2e.sh}
├── go.work
├── docker-compose.yml
├── Makefile
├── README.md
└── CODEOWNERS
```

### 5.1 包依赖守卫

CI 必须检查：

- `domain/` 只依赖 Go 标准库和同服务 domain/common。
- `application/` 只依赖 domain 与 ports。
- `ports/` 只依赖 domain/common，不依赖 adapters。
- `adapters/` 可以依赖外部 SDK，但不得被 domain/application import。
- 服务之间只通过生成 Client/MCP 契约通信，不 import 对方的 `internal/`。

---

## 6. 初期拓扑、数据所有权与启动模式

### 6.1 本地开发拓扑

```text
Browser
  → Grafana App Plugin Frontend
  → Grafana Plugin Backend
      → AI Core
          → Application Store Port → SQLite / PostgreSQL Adapter
          → AgentRuntimePort → Eino Adapter
          → ToolGatewayPort → MCP Client Adapter
              → assistant-mcp (4 namespaces)
                  → PrometheusPort → Mock / Prometheus Adapter
                  → GrafanaAccessPort → Mock / Plugin Backend controlled proxy
                  → Catalog Stores → SQLite / PostgreSQL / Filesystem Adapter
```

本地真实基础设施混合 E2E 使用以下具体拓扑：

```text
Browser
  → Grafana container（Plugin Frontend + Plugin Backend）
      → AI Core container
          → AI Core SQLite volume
          → Deterministic Mock Model
          → assistant-mcp container
              → Prometheus Adapter → Prometheus container
                                           ↑ scrape
                                    node_exporter container
              → Grafana Adapter → Plugin Backend 受控代理 → Grafana API
              → assistant-mcp SQLite volume / deterministic fixtures
```

该拓扑是“真实基础设施 + 真实应用链路 + 确定性 Mock 智能层”，不是生产拓扑。它必须真实覆盖 HTTP、SSE、MCP、Prometheus API、SQLite migration/恢复、Grafana 插件加载和 Dashboard 写入；不要求使用真实外部模型，也不以 `node_exporter` 的具体数值作为稳定测试夹具。

### 6.2 数据所有权

|数据|唯一所有者|首个存储|约束|
|-|-|-|-|
|Session、Message、Canvas、Task、TaskEvent、Chart、Revision、Approval、SaveIntent|AI Core|AI Core SQLite|其他服务只能通过 AI Core API 访问|
|Knowledge、Skill、Playbook 及其索引|assistant-mcp 对应 namespace|assistant-mcp SQLite + 文件资产|AI Core 通过 MCP Tool 访问，不直接查表|
|Grafana Dashboard/Folder/Permission|Grafana|Grafana|本项目只通过受控 Adapter 访问|
|审计事件|产生事件的服务，统一 Schema|JSONL Adapter；未来可接 Loki/对象存储|审计失败不可泄漏敏感数据|

SQLite 文件不得被多个进程共享写入。切换 PostgreSQL 后也应保持逻辑所有权和 schema 隔离，禁止跨服务 join。

### 6.3 启动模式

每个服务支持独立 Adapter 配置，例如：

```yaml
storage:
  driver: sqlite          # sqlite | postgres
  sqlite:
    path: ./var/ai-core.db
  postgres:
    dsnEnv: AI_CORE_POSTGRES_DSN

adapters:
  agentRuntime: eino
  model: deterministic_mock
  toolGateway: mcp
  taskEvents: database
```

只允许 `bootstrap` 根据配置选择 Adapter。业务代码不得读取 `storage.driver` 或 `adapters.*`。

### 6.4 docker-compose 分级

首个骨架提交可以只提供可校验的 Compose 文件，不要求所有服务已有真实实现。

```yaml
services:
  grafana:
  ai-core:
  assistant-mcp:
  plugin-build:      # one-shot，可由多阶段镜像构建替代
  prometheus:        # 由 local-real override/profile 启用
  node-exporter:     # 由 local-real override/profile 启用
  postgres:          # profile=postgres；默认关闭
```

- `mock-e2e`/`local-real` 是测试启动配置名，推荐由基础 `docker-compose.yml` 加 `deploy/docker-compose/compose.<profile>.yaml` override 和对应 env 文件实现；不能依赖 Compose profile 去隐式修改同一个服务的 Adapter 环境变量。
- 基础服务为 Grafana、AI Core 和 `assistant-mcp`，默认使用两个服务各自独占的 SQLite volume，不启动 PostgreSQL。
- `mock-e2e` profile 使用 Deterministic Mock Model、Mock Prometheus、Mock Grafana Read/Write 和固定 fixture，负责稳定回归及错误场景注入。
- `local-real` profile 额外启动 Prometheus 与 `node_exporter`，启用真实 Prometheus Adapter、真实 Grafana Read/Write Adapter；Model、Knowledge、Playbook 仍可使用确定性 Mock。
- Prometheus 必须以 `node-exporter:9100` 为 scrape target；容器间访问使用 Compose service name，不得使用 `localhost`。
- Grafana 必须 provisioning 一个指向 `http://prometheus:9090` 的 Prometheus datasource，并为测试创建独立 Folder/Dashboard。测试种子可以用测试管理员凭证初始化，但应用运行链路不得持有或使用该凭证。
- Grafana、Prometheus、`node_exporter` 镜像 tag/digest 必须在测试配置中固定，并通过 `.env.example` 暴露可替换变量；不得以浮动 `latest` 作为验收基线。
- 本地未签名插件必须显式加入 Grafana 开发环境 allowlist；Plugin Backend 必须构建为 Grafana 容器可执行的 Linux/目标架构二进制，不能直接挂载宿主机 macOS 二进制。
- `node_exporter` 在 Docker Desktop/Colima 中通常观测 Linux VM 或容器宿主环境；本测试只要求存在真实、持续变化的时间序列，不声称代表 macOS 宿主机全部指标。
- Redis 不进入首个 Compose；需要时以独立 Adapter 和 profile 增加。
- 全系统 `docker compose up` 可运行是集成阶段目标，不是“接口骨架完成”的唯一判断标准。

建议提供以下统一入口：

```text
make e2e-mock          # 确定性 Mock 纵向链路
make e2e-local         # Grafana + Prometheus + node_exporter 混合 E2E
make e2e-local-down    # 停止环境；保留或清理 volume 由显式参数决定
```

`e2e-local` 启动前必须检查镜像存在性、端口占用和 Docker daemon；启动后必须等待各服务 readiness，而不是依赖固定 sleep。

建议 readiness 至少检查 Grafana `/api/health`、Prometheus `/-/ready`、`node_exporter` `/metrics`、AI Core `/readyz` 和 `assistant-mcp` `/readyz`；最后再调用 Plugin Resource Handler 和 MCP `tools/list`，确认进程健康不等于业务链路可用。

---

## 7. Grafana Plugin Frontend 设计

当前已落地的有界切片使用常驻 `Workbench.tsx` controller 统一拥有 Session/Task 请求、SSE、reducer、route 与幂等状态；展示层由 `WorkbenchShell` 组合 `WorkbenchHeader`、真实 `ChartCanvas`、只读 `ContextPane` 和集成 Session history 的 `ChatPane`。宽屏为 `Canvas / Context / Chat`，中宽折叠 Context，窄屏按 `Chat / Context / Canvas` 纵排。所有样式限定在 Plugin 根容器，并由 Grafana theme 映射 CSS variables；浏览器仍只调用 Plugin Resource API。下列职责和目录是长期蓝图，其中编辑、保存、分享、模板、告警和手动模式尚未实现，不能从当前界面推断为已交付能力。

### 7.1 前端职责

前端负责：

- 展示会话列表
- 新建会话
- 展示对话
- 提交自然语言请求
- 展示工具执行状态
- 展示指标候选
- 展示临时图表
- 编辑图表
- 展示 PromQL
- 重新执行查询
- 展示保存确认
- 展示 Dashboard Diff
- 会话分享和 Fork
- 模板选择
- 告警会话入口
- 错误恢复
- AI 不可用时进入手动模式

前端不负责：

- 直接调用外部模型
- 直接持有 Prometheus 凭证
- 直接写 Grafana Dashboard
- 自行判断用户权限
- 在浏览器中实现 Agent
- 在浏览器中拼装最终 Panel JSON

### 7.2 前端目录建议

```text
frontend/src/
├── app/
│   ├── routes.tsx
│   ├── providers.tsx
│   └── store.ts
├── pages/
│   ├── WorkbenchPage/
│   ├── SessionDetailPage/
│   ├── SharedSessionPage/
│   └── AlertAnalysisPage/
├── features/
│   ├── sessions/
│   ├── conversation/
│   ├── task-progress/
│   ├── context-bar/
│   ├── metric-candidates/
│   ├── chart-workspace/
│   ├── chart-editor/
│   ├── dashboard-save/
│   ├── share-fork/
│   ├── templates/
│   └── playbooks/
├── components/
├── api/
│   ├── client.ts
│   ├── event-stream.ts
│   └── generated/
├── hooks/
├── utils/
└── types/
```

### 7.3 前端状态分层

建议分成三类状态：

#### 服务端状态

由 API 提供：

- Session
- Task
- ChartDraft
- Approval
- Template
- Playbook

使用 TanStack Query 或等效方案管理。

#### 流式事件状态

由 SSE 追加：

- AI 文本增量
- Tool 状态
- Chart 创建
- Chart 更新
- Approval Required
- Task Complete

#### 本地 UI 状态

仅存在于浏览器：

- 当前选中图表
- 工作台网格布局
- PromQL 编辑器是否展开
- 侧边栏宽度
- 是否显示高级信息

不要把服务端真实状态只保存在前端 Store 中。

### 7.4 前端消费的核心事件

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
chart.updated
chart.execution_started
chart.execution_completed
chart.execution_failed

approval.required
approval.approved
approval.rejected

dashboard.save_completed
dashboard.save_failed

task.completed
task.failed
```

### 7.5 图表工作区组件

每个图表卡片建议至少包含：

- 图表标题
- 状态标识
- 时间范围
- 数据源
- 图表类型
- PromQL 摘要
- 执行耗时
- 序列数
- 最近更新时间
- Pin
- 编辑
- 重新执行
- 复制 PromQL
- 关闭
- 保存到 Dashboard

### 7.6 指代关系

前端为图表提供稳定可见标识：

```text
Chart 1 · Checkout Error Rate
Chart 2 · Error Rate by Route
Chart 3 · Error Rate by Instance
```

用户输入“修改刚才那张图”时，前端把当前选中图表 ID 和最近交互图表 ID 一并传给后端。

不要仅依赖标题文本判断目标图表。

---

## 8. Grafana Plugin Backend 设计

### 8.1 职责

Plugin Backend 负责：

- Grafana Session 校验
- 获取 Plugin Context
- 获取用户、Org、角色
- 获取数据源和 Dashboard 上下文
- 将用户请求转发给 AI Core
- 转发 SSE
- 对 Grafana API 请求做白名单控制
- 对写操作再次校验权限
- 读取插件配置
- 限流
- 请求追踪
- 审计边界
- 统一错误映射
- 为跨进程 Grafana 调用签发短时、最小权限的 opaque delegation grant

### 8.2 不负责的内容

Plugin Backend 不应：

- 保存长期会话
- 实现复杂 Agent
- 直接拼 Prompt
- 直接调用多个模型
- 维护 Playbook 执行逻辑
- 处理复杂 RAG
- 持有跨租户全局管理员写凭证

### 8.3 Backend 目录建议

```text
backend/internal/
├── handlers/
│   ├── chat_handler.go
│   ├── session_handler.go
│   ├── task_handler.go
│   ├── approval_handler.go
│   └── grafana_proxy_handler.go
├── context/
│   ├── grafana_context.go
│   └── request_context.go
├── auth/
├── permissions/
├── proxy/
├── aicore/
│   └── generated_client/
├── audit/
├── rate_limit/
├── config/
└── errors/
```

### 8.4 核心接口

`AICoreClient` 必须由 OpenAPI 生成；下面的接口只作为 Plugin Backend 内部 Facade，避免 handler 直接依赖生成代码：

```go
type AICoreClient interface {
    CreateSession(ctx context.Context, req CreateSessionRequest) (*Session, error)
    CreateTask(ctx context.Context, req CreateTaskRequest) (*Task, error)
    AppendMessage(ctx context.Context, taskID string, req AppendMessageRequest) error
    StreamTaskEvents(ctx context.Context, taskID string, afterSequence int64) (<-chan TaskEvent, error)
    GetTask(ctx context.Context, taskID string) (*Task, error)
    Approve(ctx context.Context, approvalID string, req ApprovalRequest) (*Approval, error)
}
```

所有调用都必须携带 `RequestContext`。`GrafanaDelegationGrant` 是 Plugin Backend 签发的短时不透明字符串，只能用于白名单 Grafana 操作，不包含 Grafana Session、Cookie 或 Token：

```go
type RequestContext struct {
    TenantID        string
    OrgID           string
    UserID          string
    Roles           []string
    Permissions     []string
    RequestID       string
    TraceID         string
    ActiveFolderUID string
    GrafanaGrant    string // sensitive: 禁止日志、禁止持久化
}
```

Grant 只能进入内部 Plugin Backend ↔ AI Core 契约以及 MCP transport metadata（字段标记 `writeOnly`/`x-sensitive`），不能进入模型可见的 Tool Input Schema。`ToolGatewayPort` Adapter 从 RequestContext 注入，assistant-mcp runtime 再映射到 ToolContext。

- 创建只读任务时，Plugin Backend 根据当前登录态签发 read grant。
- 用户点击审批时，Plugin Backend 必须重新校验当前登录态和权限，并签发本次写操作专用 grant。
- AI Core 与 assistant-mcp 只能透传 grant；最终由 Plugin Backend 受控代理再次验证 scope、org、user、expiry、approval ID 和目标资源。
- 首个骨架可以用签名 Mock Grant 跑通校验接口，但不得退化为固定管理员 Token。

```go
type GrafanaContextProvider interface {
    BuildContext(
        ctx context.Context,
        pluginCtx backend.PluginContext,
        request *http.Request,
    ) (*GrafanaContext, error)
}
```

```go
type GrafanaReadProxy interface {
    GetDashboard(ctx RequestContext, uid string) (*DashboardSnapshot, error)
    GetPanel(ctx RequestContext, dashboardUID string, panelID int64) (*PanelSnapshot, error)
    SearchDashboards(ctx RequestContext, query string) ([]DashboardSummary, error)
    GetDatasource(ctx RequestContext, uid string) (*DatasourceSummary, error)
}
```

```go
type GrafanaWriteProxy interface {
    AddPanel(
        ctx RequestContext,
        command AddPanelCommand,
        approval ApprovalEvidence,
    ) (*DashboardSaveResult, error)
}
```

### 8.5 插件对外 Resource Handler

首个契约必须包含：

```text
POST /api/plugins/<PLUGIN_ID>/resources/sessions
GET  /api/plugins/<PLUGIN_ID>/resources/sessions
GET  /api/plugins/<PLUGIN_ID>/resources/sessions/{sessionId}

POST /api/plugins/<PLUGIN_ID>/resources/tasks
GET  /api/plugins/<PLUGIN_ID>/resources/tasks/{taskId}
GET  /api/plugins/<PLUGIN_ID>/resources/tasks/{taskId}/events
POST /api/plugins/<PLUGIN_ID>/resources/tasks/{taskId}/messages

POST /api/plugins/<PLUGIN_ID>/resources/approvals/{approvalId}/approve
POST /api/plugins/<PLUGIN_ID>/resources/approvals/{approvalId}/reject

POST /api/plugins/<PLUGIN_ID>/resources/grafana/proxy
```

插件前端只调用 Plugin Backend，不直接调用 AI Core。

`grafana/proxy` 不是通用反向代理。路由、HTTP 方法、目标资源和最大响应体必须在 allowlist 中；生产模式禁止用户提交任意 Grafana URL。

本方案当前为暂定决策，具体约束、风险和复审条件见
[`../adr/ADR-017-grafana-delegation-grant.md`](../adr/ADR-017-grafana-delegation-grant.md)。真实 Grafana Write Adapter 不得在该 ADR 完成复审前进入生产。

---

## 9. AI Core 设计

### 9.1 定位

AI Core 是系统的业务编排中心，负责：

- 任务创建
- 意图识别
- 上下文构建
- Agent Workflow
- 工具调用
- PromQL Skill
- 图表 Skill
- 结果校验
- 自动修正
- 风险判断
- 保存确认
- 会话持久化
- 任务事件
- 质量记录
- 模型调用
- 知识检索
- 模板和 Playbook

### 9.2 代码结构

```text
services/ai-core/
├── cmd/server/main.go
├── internal/
│   ├── domain/
│   ├── session/
│   ├── task/
│   ├── chart/
│   ├── query/
│   ├── approval/
│   ├── template/
│   ├── playbook/
│   ├── alert/
│   │   └── common/
│   ├── application/
│   ├── commands/
│   ├── queries/
│   ├── workflows/
│   ├── policies/
│   ├── services/
│   │   └── dto/
│   ├── ports/
│   ├── repositories/
│   ├── tools/
│   ├── agent/
│   ├── model/
│   ├── knowledge/
│   ├── events/
│   ├── clocks/
│   │   └── ids/
│   ├── adapters/
│   ├── inbound/http/
│   └── outbound/
│       ├── agent/eino/
│       ├── tools/mcp/
│       ├── model/
│       ├── storage/{sqlite,postgres}/
│       ├── events/
│       └── mock/
│   ├── skills/
│   ├── generate_promql/
│   ├── edit_chart/
│   ├── explain_chart/
│   ├── explain_alert/
│   │   └── candidate_playbook/
│   └── bootstrap/{config.go,wire.go}
├── migrations/{sqlite,postgres}/
├── tests/{contract,integration}/
└── go.mod
```

### 9.3 核心应用用例

禁止实现一个包含全部用例的巨大 `AgentService`，按以下应用用例拆分：

拆分为：

```text
CreateAnalysisSession
CreateAnalysisTask
AppendUserMessage
BuildAnalysisContext
PlanMetricAnalysis
SearchMetricCandidates
GeneratePromQL
ValidateGeneratedQuery
ExecuteGeneratedQuery
CreateChartDraft
EditChartDraft
CreateQueryRevision
RetryFailedQuery
PrepareDashboardSave
ApproveDashboardSave
ExecuteDashboardSave
RestoreSession
SearchSessions
ShareSession
ForkSession
RunTemplate
GenerateCandidatePlaybook
RunPlaybook
HandleAlertEvent
```

### 9.4 Workflow Orchestrator

工作流负责按状态推进任务。

统一步骤接口使用领域类型，不暴露 Eino：

```go
type WorkflowStep interface {
    Name() string
    CanRun(ctx context.Context, state WorkflowState) (bool, error)
    Run(ctx context.Context, state WorkflowState) (StepResult, error)
}

type Workflow interface {
    Type() task.Type
    Start(ctx context.Context, command StartWorkflowCommand) (*task.AnalysisTask, error)
    Resume(ctx context.Context, command ResumeWorkflowCommand) (*task.AnalysisTask, error)
}
```

核心工作流：

```text
MetricAnalysisWorkflow
ChartEditWorkflow
DashboardSaveWorkflow
TemplateRunWorkflow
PlaybookRunWorkflow
AlertAnalysisWorkflow
```

Eino `ChatModelAgent`、`Interrupt`、`CheckPoint`、`Skill Middleware` 和 `ChatModel Failover` 全部封装在 outbound adapter。应用层只调用 `AgentRuntimePort.Run/Resume`。Eino CheckPoint 的持久化必须通过 `CheckpointRepository` 接入 Application Store，不能只保存在进程内。

### 9.5 Policy Guard

Policy Guard 应在工具调用之前执行，检查：

- 用户是否有权限
- 工具是否允许
- 是否只读
- 是否需要审批
- 是否包含敏感数据
- 是否可能跨租户
- 查询范围是否过大
- 是否超过成本预算
- 是否存在提示注入风险
- 是否允许向外部模型发送当前内容

---

## 10. assistant-mcp 设计

### 10.1 定位

`assistant-mcp` 是可执行能力边界。v1 只有一个进程，通过 namespace 保持模块隔离；达到拆分阈值后可以把 namespace 拆为独立进程，但工具名称、Schema 和错误语义不得改变。

AI Core 不直接访问 Prometheus、Grafana Dashboard API 或知识库，而是调用结构化 Tool。

### 10.2 目录结构

```text
services/assistant-mcp/
├── cmd/server/main.go
├── internal/
│   ├── runtime/{registry.go,tool_context.go,permissions.go,audit.go,errors.go}
│   ├── namespaces/
│   │   ├── grafana/{register.go,handlers.go,service.go}
│   │   ├── knowledge/{register.go,handlers.go,service.go}
│   │   ├── playbook/{register.go,handlers.go,service.go}
│   │   └── skills/{register.go,handlers.go,service.go}
│   ├── ports/{prometheus,grafana,knowledge,playbook,skills,storage}/
│   ├── adapters/{prometheus,grafana,storage/sqlite,storage/postgres,filesystem,mock}/
│   └── bootstrap/{config.go,wire.go}
├── migrations/{sqlite,postgres}/
├── tests/{contract,integration}/
└── go.mod
```

每个 Tool Handler 固定执行以下模板：

```text
decode schema
  → validate RequestContext
  → authorize folder/tenant/tool
  → verify approval evidence（write only）
  → call namespace service
  → call Port
  → validate output schema
  → audit sanitized summary
  → return typed result/error
```

Handler 不允许直接访问 HTTP Client、数据库或文件系统。

### 10.3 MCP Tool 元信息

每个工具应定义：

```json
{
  "name": "grafana.query_prometheus",
  "version": "v1",
  "description": "Validate a PromQL expression against a datasource.",
  "riskLevel": "read_only",
  "requiredPermissions": ["datasources:query"],
  "timeoutMs": 5000,
  "idempotent": true,
  "inputSchema": {"$ref": "contracts/tools/grafana/query-prometheus.input.schema.json"},
  "outputSchema": {"$ref": "contracts/tools/grafana/query-prometheus.output.schema.json"}
}
```

Tool Schema 的单一来源在 `contracts/tools/<namespace>/`。mcp-go 注册代码应由 Schema 生成或由 Contract Test 验证，禁止只在 handler 内定义匿名参数。

### 10.4 风险等级

统一风险等级：

```text
read_only
write_draft
write_requires_approval
destructive_requires_approval
forbidden
```

当前骨架中：

- Prometheus 查询：`read_only`
- 生成 Panel Draft：`write_draft`
- 新增 Dashboard Panel：`write_requires_approval`
- 删除 Panel：`forbidden`
- 覆盖 Dashboard：`forbidden`

### 10.5 v1 注册范围

四个 namespace 的完整工具名以 `../design/arch_design_detail.md` P4 为目标集合。首个骨架必须注册并提供 Mock 的最小子集：

```text
grafana.search_metrics
grafana.get_metric_labels
grafana.query_prometheus
grafana.list_dashboards
grafana.get_dashboard
grafana.prepare_add_panel
grafana.add_panel

knowledge.list_services
knowledge.get_service
knowledge.search_docs

playbook.list_playbooks
playbook.get_playbook
playbook.run_playbook

skills.list_skills
skills.get_skill
skills.load_skill_for_agent
```

其余目标工具可以只有 Schema 和 `tool_not_supported` 实现。未实现写工具不能注册成可成功执行的空 handler。

### 10.6 写工具的双重保护

AI Core 的 Eino approval Middleware 负责发起和恢复 HITL；assistant-mcp 不负责展示审批 UI，但必须验证 `ApprovalEvidence`，做到防御纵深。

- AI Core 调 write tool：必须携带已批准且未过期的 evidence。
- 外部 MCP Client 默认只能看到 read tool；若要调用 write tool，同样必须通过审批 API 获得 evidence。
- 缺少或 scope 不匹配时返回 `approval_required`/`tool_permission_denied`。
- MCP Server 不能因为“上游应该已经审批”而跳过校验。

---

## 11. 核心 Port 接口

本节接口是首个骨架必须创建的 Go 接口。示例中的请求/返回类型属于 `domain` 或 `application/dto`；Adapter 不得把 SDK 类型塞入这些对象。

### 11.1 通用上下文

```go
type RequestContext struct {
    TenantID        string
    OrgID           string
    UserID          string
    Roles           []string
    Permissions     []string
    RequestID       string
    TraceID         string
    ActiveFolderUID string
    GrafanaGrant    string // sensitive, ephemeral
}

type PageRequest struct {
    PageSize  int
    PageToken string
}

type Page[T any] struct {
    Items         []T
    NextPageToken string
}
```

所有访问外部数据或 Tool 的方法必须接收 `context.Context` 和业务 `RequestContext`；Repository 方法接收 `context.Context` 并显式携带/校验 tenant 标识，不接收含临时 grant 的完整 RequestContext。后台任务使用创建任务时保存的非敏感授权快照，并在写入前重新授权。

### 11.2 MetricCatalogPort

```go
type MetricCatalogPort interface {
    SearchMetrics(ctx context.Context, rc RequestContext, req SearchMetricsRequest) (SearchMetricsResult, error)
    GetMetricLabels(ctx context.Context, rc RequestContext, req GetMetricLabelsRequest) (MetricLabelsResult, error)
}
```

`SearchMetricsRequest`：

```json
{
  "datasourceUid": "prometheus-main",
  "query": "checkout 服务错误率",
  "service": "checkout",
  "environment": "prod",
  "metricType": "counter",
  "limit": 10
}
```

`SearchMetricsResult`：

```json
{
  "candidates": [
    {
      "metricName": "http_requests_total",
      "type": "counter",
      "description": "Total HTTP requests",
      "labels": ["service", "route", "status", "instance"],
      "score": 0.92,
      "sources": [
        {
          "type": "prometheus_metadata",
          "reference": "http_requests_total"
        },
        {
          "type": "dashboard_example",
          "reference": "dashboard:checkout-overview/panel:12"
        }
      ]
    }
  ]
}
```

### 11.3 QueryEnginePort

```go
type QueryEnginePort interface {
    Validate(ctx context.Context, rc RequestContext, req ValidateQueryRequest) (QueryValidationResult, error)
    Execute(ctx context.Context, rc RequestContext, req ExecuteQueryRequest) (QueryExecutionResult, error)
}
```

`QueryValidationResult`：

```json
{
  "valid": true,
  "status": "success",
  "errors": [],
  "warnings": [],
  "metricNames": ["http_requests_total"],
  "labelNames": ["service", "route", "status"],
  "estimatedSeriesCount": 24
}
```

`QueryExecutionResult`：

```json
{
  "status": "success",
  "seriesCount": 12,
  "durationMs": 286,
  "sampleRange": {
    "from": "2026-07-13T10:00:00Z",
    "to": "2026-07-13T10:30:00Z"
  },
  "warnings": [],
  "preview": {
    "type": "timeseries",
    "series": []
  }
}
```

### 11.4 ModelPort 与 AgentRuntimePort

```go
type ModelPort interface {
    CompleteStructured(ctx context.Context, rc ModelRequestContext, req StructuredCompletionRequest) (StructuredCompletionResult, error)
    StreamText(ctx context.Context, rc ModelRequestContext, req TextCompletionRequest, sink TextDeltaSink) error
}

type AgentRuntimePort interface {
    Run(ctx context.Context, rc RequestContext, req AgentRunRequest, sink AgentEventSink) (AgentRunResult, error)
    Resume(ctx context.Context, rc RequestContext, req AgentResumeRequest, sink AgentEventSink) (AgentRunResult, error)
}

type AgentEventSink interface {
    Emit(ctx context.Context, event AgentEvent) error
}
```

`ModelPort` 不暴露模型供应商对象；`AgentRuntimePort` 不暴露 Eino Message、Graph、Checkpoint 或 Callback 类型。

### 11.5 ToolGatewayPort

```go
type ToolGatewayPort interface {
    ListTools(ctx context.Context, rc RequestContext, filter ToolFilter) ([]ToolDescriptor, error)
    CallTool(ctx context.Context, rc RequestContext, call ToolCall) (ToolResult, error)
}
```

`ToolCall.Input` 和 `ToolResult.Output` 必须经过对应 JSON Schema 校验。领域用例若需要稳定语义，应优先依赖下面的专用 Port，由 MCP Adapter 实现，而不是在应用层拼 tool name。

### 11.6 KnowledgeSearchPort

```go
type KnowledgeSearchPort interface {
    Search(ctx context.Context, rc RequestContext, req KnowledgeSearchRequest) (KnowledgeSearchResult, error)
}
```

### 11.7 DashboardReadPort、PanelCompilerPort、DashboardWritePort

```go
type DashboardReadPort interface {
    SearchDashboards(ctx context.Context, rc RequestContext, req SearchDashboardsRequest) (SearchDashboardsResult, error)
    GetDashboard(ctx context.Context, rc RequestContext, uid string) (DashboardSnapshot, error)
    GetPanel(ctx context.Context, rc RequestContext, dashboardUID string, panelID int64) (PanelSnapshot, error)
    GetAlertRule(ctx context.Context, rc RequestContext, uid string) (AlertRuleSnapshot, error)
}

type PanelCompilerPort interface {
    Compile(ctx context.Context, rc RequestContext, chart ChartDraft, target GrafanaTarget) (PanelDraft, error)
}

type DashboardWritePort interface {
    PrepareAddPanel(ctx context.Context, rc RequestContext, req PreparePanelSaveRequest) (DashboardSaveIntent, error)
    ExecuteAddPanel(ctx context.Context, rc RequestContext, intent DashboardSaveIntent, approval ApprovalEvidence) (DashboardSaveResult, error)
}
```

### 11.8 Repository Ports 与事务边界

```go
type SessionRepository interface {
    Create(ctx context.Context, session AnalysisSession) error
    Get(ctx context.Context, tenantID, sessionID string) (AnalysisSession, error)
    Search(ctx context.Context, filter SessionFilter, page PageRequest) (Page[AnalysisSession], error)
    Update(ctx context.Context, session AnalysisSession, expectedVersion int64) error
    Delete(ctx context.Context, tenantID, sessionID string, expectedVersion int64) error
}

type MessageRepository interface {
    Append(ctx context.Context, message SessionMessage) error
    ListBySession(ctx context.Context, tenantID, sessionID string, page PageRequest) (Page[SessionMessage], error)
}

type SessionNoteRepository interface {
    Append(ctx context.Context, note SessionNote) error
    ListBySession(ctx context.Context, tenantID, sessionID string, page PageRequest) (Page[SessionNote], error)
}

type SessionShareRepository interface {
    CreateSnapshot(ctx context.Context, snapshot SessionSnapshot, share SessionShare) error
    GetByTokenHash(ctx context.Context, tenantID, tokenHash string) (SessionShare, SessionSnapshot, error)
    Revoke(ctx context.Context, tenantID, shareID, actorID string, expectedVersion int64) error
}

type SessionRelationRepository interface {
    RecordFork(ctx context.Context, relation SessionForkRelation) error
    ListForks(ctx context.Context, tenantID, sourceSessionID string, page PageRequest) (Page[SessionForkRelation], error)
}

type TaskRepository interface {
    Create(ctx context.Context, task AnalysisTask) error
    Get(ctx context.Context, tenantID, taskID string) (AnalysisTask, error)
    Update(ctx context.Context, task AnalysisTask, expectedVersion int64) error
}

type ToolCallRepository interface {
    Create(ctx context.Context, call ToolCallRecord) error
    Update(ctx context.Context, call ToolCallRecord, expectedVersion int64) error
    ListByTask(ctx context.Context, tenantID, taskID string, page PageRequest) (Page[ToolCallRecord], error)
}

type ChartRepository interface {
    CreateDraft(ctx context.Context, chart ChartDraft) error
    GetDraft(ctx context.Context, tenantID, chartID string) (ChartDraft, error)
    UpdateDraft(ctx context.Context, chart ChartDraft, expectedVersion int64) error
    AppendRevision(ctx context.Context, revision ChartRevision) error
    AppendQueryRevision(ctx context.Context, revision QueryRevision) error
    AppendExecution(ctx context.Context, execution ChartExecution) error
    ListBySession(ctx context.Context, tenantID, sessionID string, page PageRequest) (Page[ChartDraft], error)
}

type CanvasRepository interface {
    GetBySession(ctx context.Context, tenantID, sessionID string) (Canvas, error)
    Put(ctx context.Context, canvas Canvas, expectedVersion int64) error
}

type ApprovalRepository interface {
    Create(ctx context.Context, approval Approval) error
    Get(ctx context.Context, tenantID, approvalID string) (Approval, error)
    Decide(ctx context.Context, decision ApprovalDecision, expectedVersion int64) error
}

type DashboardSaveRepository interface {
    PutPanelDraft(ctx context.Context, draft PanelDraft) error
    GetPanelDraft(ctx context.Context, tenantID, panelDraftID string) (PanelDraft, error)
    CreateIntent(ctx context.Context, intent DashboardSaveIntent) error
    GetIntent(ctx context.Context, tenantID, intentID string) (DashboardSaveIntent, error)
    UpdateIntent(ctx context.Context, intent DashboardSaveIntent, expectedVersion int64) error
    AppendResult(ctx context.Context, result DashboardSaveResult) error
}

type TemplateRepository interface {
    List(ctx context.Context, tenantID string, page PageRequest) (Page[Template], error)
    Get(ctx context.Context, tenantID, templateID string) (Template, error)
    AppendRun(ctx context.Context, run TemplateRun) error
}

type ObjectDraftRepository interface {
    Create(ctx context.Context, draft ObjectDraft) error
    Get(ctx context.Context, tenantID, draftID string) (ObjectDraft, error)
    Update(ctx context.Context, draft ObjectDraft, expectedVersion int64) error
}

type PromotionRequestRepository interface {
    Create(ctx context.Context, request PromotionRequest) error
    Get(ctx context.Context, tenantID, requestID string) (PromotionRequest, error)
    ListPending(ctx context.Context, filter PromotionFilter, page PageRequest) (Page[PromotionRequest], error)
    Decide(ctx context.Context, decision PromotionDecision, expectedVersion int64) error
}

type AlertRepository interface {
    UpsertEvent(ctx context.Context, event AlertEvent) error
    GetEvent(ctx context.Context, tenantID, eventID string) (AlertEvent, error)
    LinkSession(ctx context.Context, relation AlertSessionRelation) error
}

type EvaluationRepository interface {
    Create(ctx context.Context, evaluation EvaluationRecord) error
    ListByTask(ctx context.Context, tenantID, taskID string, page PageRequest) (Page[EvaluationRecord], error)
}

type IdempotencyRepository interface {
    Begin(ctx context.Context, key IdempotencyKey, requestHash string, expiresAt time.Time) (IdempotencyRecord, error)
    Complete(ctx context.Context, key IdempotencyKey, response IdempotencyResponse) error
    Get(ctx context.Context, key IdempotencyKey) (IdempotencyRecord, error)
}

type CheckpointRepository interface {
    Put(ctx context.Context, checkpoint AgentCheckpoint) error
    Get(ctx context.Context, tenantID, checkpointID string) (AgentCheckpoint, error)
    Delete(ctx context.Context, tenantID, checkpointID string) error
}

type ApplicationStore interface {
    Sessions() SessionRepository
    Messages() MessageRepository
    SessionNotes() SessionNoteRepository
    SessionShares() SessionShareRepository
    SessionRelations() SessionRelationRepository
    Tasks() TaskRepository
    ToolCalls() ToolCallRepository
    Charts() ChartRepository
    Canvases() CanvasRepository
    Approvals() ApprovalRepository
    DashboardSaves() DashboardSaveRepository
    Templates() TemplateRepository
    ObjectDrafts() ObjectDraftRepository
    PromotionRequests() PromotionRequestRepository
    Alerts() AlertRepository
    Evaluations() EvaluationRepository
    Idempotency() IdempotencyRepository
    Checkpoints() CheckpointRepository
    TaskEvents() TaskEventStore
    WithinTransaction(ctx context.Context, fn func(tx ApplicationStore) error) error
    Health(ctx context.Context) error
    Close() error
}
```

当前 node_exporter 私有会话 profile 先实现 `GetOwned` 和
`ListPageByOwner`：列表限定 tenant + creator、排除无 Task 的空 Session，
并按 `updatedAt DESC,id DESC` 使用专用游标分页。长期 `Search`、可见性、删除
和分享接口仍须等待对应产品切片，不能由浏览器本地列表替代。

Repository 方法只能返回领域错误，如 `ErrNotFound`、`ErrVersionConflict`、`ErrConstraintViolation`。SQLite/PostgreSQL 原始错误不得越过 Adapter。

### 11.9 Durable TaskEventStore 与可选通知器

```go
type TaskEventStore interface {
    Append(ctx context.Context, draft TaskEventDraft) (TaskEvent, error) // 在事务中分配 sequence
    Replay(ctx context.Context, tenantID, taskID string, afterSequence int64, limit int) ([]TaskEvent, error)
    LatestSequence(ctx context.Context, tenantID, taskID string) (int64, error)
}

type TaskEventNotifier interface {
    Notify(ctx context.Context, event TaskEvent) error
    Subscribe(ctx context.Context, tenantID, taskID string) (<-chan struct{}, error)
}
```

事实来源是 `TaskEventStore`。`TaskEventNotifier` 可以用进程内 channel 或未来 Redis 实现；通知丢失时 SSE handler 仍通过 Replay 恢复。

### 11.10 基础设施 Port

```go
type Clock interface { Now() time.Time }
type IDGenerator interface { NewID() string }

type AuditLogger interface {
    Log(ctx context.Context, event AuditEvent) error
}

type PermissionChecker interface {
    Check(ctx context.Context, rc RequestContext, requirement PermissionRequirement) (PermissionDecision, error)
}
```

### 11.11 assistant-mcp Provider Ports

Tool Handler 只调用 namespace service，namespace service 再依赖以下 Port；不得在 handler 中直接创建 SDK Client：

```go
type PrometheusPort interface {
    SearchMetrics(ctx context.Context, rc RequestContext, req SearchMetricsRequest) (SearchMetricsResult, error)
    GetMetricLabels(ctx context.Context, rc RequestContext, req GetMetricLabelsRequest) (MetricLabelsResult, error)
    Query(ctx context.Context, rc RequestContext, req PrometheusQueryRequest) (PrometheusQueryResult, error)
}

type GrafanaProxyPort interface {
    ListDashboards(ctx context.Context, rc RequestContext, req ListDashboardsRequest) (ListDashboardsResult, error)
    GetDashboard(ctx context.Context, rc RequestContext, uid string) (DashboardSnapshot, error)
    PrepareAddPanel(ctx context.Context, rc RequestContext, req PreparePanelSaveRequest) (PanelDraft, error)
    AddPanel(ctx context.Context, rc RequestContext, req AddPanelRequest, evidence ApprovalEvidence) (DashboardSaveResult, error)
}

type FolderPermissionPort interface {
    Check(ctx context.Context, rc RequestContext, folderUID string, required FolderPermission) (bool, error)
    List(ctx context.Context, rc RequestContext, minimum FolderPermission) ([]GrafanaFolder, error)
}

type ApprovalEvidenceVerifier interface {
    Verify(ctx context.Context, rc RequestContext, evidence ApprovalEvidence, expected ApprovalScope) error
}
```

### 11.12 assistant-mcp Catalog Store

Catalog Store 与 AI Core Application Store 分属不同服务，但使用相同的数据库替换原则：

```go
type ServiceEntryRepository interface {
    Create(ctx context.Context, entry ServiceEntry) error
    Get(ctx context.Context, tenantID, folderUID, id string) (ServiceEntry, error)
    List(ctx context.Context, filter ServiceEntryFilter, page PageRequest) (Page[ServiceEntry], error)
    Update(ctx context.Context, entry ServiceEntry, expectedVersion int64) error
    Delete(ctx context.Context, tenantID, folderUID, id string, expectedVersion int64) error
}

type RunbookRepository interface {
    Create(ctx context.Context, runbook Runbook) error
    Get(ctx context.Context, tenantID, folderUID, id string) (Runbook, error)
    List(ctx context.Context, filter RunbookFilter, page PageRequest) (Page[Runbook], error)
    Update(ctx context.Context, runbook Runbook, expectedVersion int64) error
    Delete(ctx context.Context, tenantID, folderUID, id string, expectedVersion int64) error
}

type DocumentRepository interface {
    Create(ctx context.Context, document Document) error
    Get(ctx context.Context, tenantID, folderUID, id string) (Document, error)
    Search(ctx context.Context, filter DocumentSearchFilter, page PageRequest) (Page[DocumentChunk], error)
    Update(ctx context.Context, document Document, expectedVersion int64) error
    Delete(ctx context.Context, tenantID, folderUID, id string, expectedVersion int64) error
}

type SkillRepository interface {
    Create(ctx context.Context, skill Skill) error
    Get(ctx context.Context, tenantID, id string) (Skill, error)
    List(ctx context.Context, filter SkillFilter, page PageRequest) (Page[Skill], error)
    Update(ctx context.Context, skill Skill, expectedVersion int64) error
    Delete(ctx context.Context, tenantID, id string, expectedVersion int64) error
}

type PlaybookRepository interface {
    Create(ctx context.Context, playbook Playbook) error
    Get(ctx context.Context, tenantID, id string) (Playbook, error)
    List(ctx context.Context, filter PlaybookFilter, page PageRequest) (Page[Playbook], error)
    Update(ctx context.Context, playbook Playbook, expectedVersion int64) error
    Delete(ctx context.Context, tenantID, id string, expectedVersion int64) error
    AppendRun(ctx context.Context, run PlaybookRun) error
}

type CatalogStore interface {
    Services() ServiceEntryRepository
    Runbooks() RunbookRepository
    Documents() DocumentRepository
    Skills() SkillRepository
    Playbooks() PlaybookRepository
    WithinTransaction(ctx context.Context, fn func(tx CatalogStore) error) error
    Health(ctx context.Context) error
    Close() error
}

type AssetStore interface {
    Put(ctx context.Context, key string, content io.Reader, metadata AssetMetadata) (AssetRef, error)
    Get(ctx context.Context, ref AssetRef) (io.ReadCloser, error)
    Delete(ctx context.Context, ref AssetRef) error
}
```

SQLite/PostgreSQL 分别实现 `CatalogStore`；本地文件系统/对象存储分别实现 `AssetStore`。Skill/Playbook 的 Markdown/YAML 物理内容不得由业务层用 `os.ReadFile` 直接读取。

### 11.13 核心 Aggregate 最小字段

后续代码应从 Schema 生成 wire DTO，再显式映射为领域对象。领域对象至少表达以下字段和不变量：

```go
type AnalysisSession struct {
    ID              string
    TenantID        string
    OrgID           string
    OwnerUserID     string
    Title           string
    Status          SessionStatus     // active | archived
    Visibility      SessionVisibility // private | team
    ActiveFolderUID string
    SourceTemplateID string
    SourceSessionID  string
    DatasourceUID   string
    DefaultTimeRange TimeRange
    Tags            []string
    Version         int64
    CreatedAt       time.Time
    UpdatedAt       time.Time
}

type SessionMessage struct {
    ID              string
    TenantID        string
    SessionID       string
    TaskID          string
    Role            MessageRole // user | assistant | tool | system
    Content         string
    RelatedChartIDs []string
    ToolCallID      string
    CreatedAt       time.Time
}

type AnalysisTask struct {
    ID          string
    TenantID    string
    SessionID   string
    Type        TaskType
    Status      TaskStatus
    Intent      string
    InputMessageID string
    PlanSummary []PlanStepSummary
    ErrorCode   string
    Version     int64
    CreatedAt   time.Time
    StartedAt   *time.Time
    CompletedAt *time.Time
    UpdatedAt   time.Time
}
```

领域不变量：

- archived Session 只读；恢复分析需要 Fork 或显式 unarchive command。
- Session 的 `active_folder_uid` 可以为空和切换，但每次消费 Folder 资源前重新校验权限。
- Fork 只能从已发布且未撤销/未过期的 `SessionSnapshot` 创建；新 Session 深拷贝快照内容，不引用原对象的可变数据。
- 分享 Token 只向用户返回一次，数据库仅保存 hash；原始 Token 不写日志。
- 每个 Task 恰好关联一条同 tenant/session 的 User Message；Assistant Message 至多一条且也关联产生它的 Task。`AnalysisTask.InputMessageID` 与 User Message 的 `TaskID` 必须双向一致。
- Message 不保存模型私有推理；Tool Message 只保存脱敏摘要和 `tool_call_id`。
- Task 只能通过第 15 节状态机迁移，终态不可恢复为运行态。

---

## 12. PromQL 生成 Skill

PromQL 生成必须作为 AI Core 内部 Skill，而不是数据访问工具；指标检索和查询执行才是外部 Tool/Port。

### 12.1 输入

```json
{
  "intent": "分析 checkout 服务过去 30 分钟错误率",
  "datasourceUid": "prometheus-main",
  "timeRange": {
    "from": "now-30m",
    "to": "now"
  },
  "context": {
    "service": "checkout",
    "environment": "prod"
  },
  "metricCandidates": [],
  "labelCandidates": [],
  "existingCharts": []
}
```

### 12.2 输出

```json
{
  "queries": [
    {
      "expression": "sum(rate(http_requests_total{service=\"checkout\",status=~\"5..\"}[5m])) / sum(rate(http_requests_total{service=\"checkout\"}[5m]))",
      "title": "Checkout Error Rate",
      "legend": "error rate",
      "recommendedVisualization": "timeseries",
      "unit": "percentunit",
      "metricDependencies": ["http_requests_total"],
      "groupBy": [],
      "filters": {
        "service": "checkout",
        "status": "5xx"
      },
      "confidence": 0.87,
      "assumptions": [
        "HTTP 5xx responses are considered errors",
        "rate window uses 5 minutes"
      ]
    }
  ]
}
```

### 12.3 要求

PromQL Skill 输出必须包括：

- PromQL
- 图表标题
- 推荐图表类型
- 单位
- Legend
- 使用的指标
- 分组维度
- 过滤条件
- 假设
- 置信度

不能只返回字符串。

---

## 13. 图表领域模型

### 13.0 Canvas

Canvas 是 Session 内唯一的多图布局状态，由 AI Core 持久化，前端不能把它只放在本地 Store：

```json
{
  "id": "canvas_123",
  "tenantId": "tenant_1",
  "sessionId": "session_123",
  "layout": "grid",
  "items": [
    {"chartId": "chart_1", "x": 0, "y": 0, "w": 6, "h": 4},
    {"chartId": "chart_2", "x": 6, "y": 0, "w": 6, "h": 4}
  ],
  "version": 3,
  "updatedAt": "2026-07-13T10:00:00Z"
}
```

同一 Session 只能有一个 active Canvas；更新使用乐观锁。删除图表时，应用事务必须同时移除对应 CanvasItem，但保留 ChartRevision 审计历史。

### 13.1 ChartDraft

工作台中的临时图表。

首个契约必须包含以下字段；可以新增向后兼容字段，但不得删除 tenant、version 和来源信息：

```json
{
  "id": "chart_123",
  "tenantId": "tenant_1",
  "sessionId": "session_123",
  "taskId": "task_123",
  "title": "Checkout Error Rate",
  "query": {
    "language": "promql",
    "expression": "..."
  },
  "visualization": {
    "type": "timeseries",
    "unit": "percentunit",
    "legend": "{{route}}"
  },
  "timeRange": {
    "from": "now-30m",
    "to": "now"
  },
  "datasourceUid": "prometheus-main",
  "status": "ready",
  "revision": 3,
  "version": 3,
  "pinned": false,
  "createdBy": "ai",
  "createdAt": "...",
  "updatedAt": "..."
}
```

### 13.2 ChartRevision

记录图表配置变更。

```json
{
  "id": "chart_rev_3",
  "tenantId": "tenant_1",
  "chartId": "chart_123",
  "revision": 3,
  "source": "user_manual",
  "changes": {
    "query.expression": {
      "before": "...",
      "after": "..."
    }
  },
  "createdAt": "..."
}
```

`source` 固定枚举：

```text
ai_generated
ai_auto_fix
user_natural_language
user_manual
template
playbook
system_migration
```

### 13.3 QueryRevision

专门记录查询变化。

```json
{
  "id": "query_rev_3",
  "tenantId": "tenant_1",
  "chartId": "chart_123",
  "generatedQuery": "...",
  "editedQuery": "...",
  "finalQuery": "...",
  "editSource": "user_manual",
  "validationStatus": "valid"
}
```

### 13.4 ChartExecution

```json
{
  "id": "exec_123",
  "tenantId": "tenant_1",
  "chartId": "chart_123",
  "queryRevision": 3,
  "status": "success",
  "errorType": null,
  "seriesCount": 12,
  "durationMs": 286,
  "sampleRange": {},
  "executedAt": "..."
}
```

---

## 14. PanelDraft 与 Dashboard 保存

### 14.1 PanelDraft

PanelDraft 是针对特定 Grafana 版本构造的可写对象。

```json
{
  "id": "panel_draft_123",
  "tenantId": "tenant_1",
  "chartId": "chart_123",
  "grafanaVersion": "12.x",
  "panelJson": {},
  "validation": {
    "valid": true,
    "warnings": []
  },
  "diffSummary": [
    "Add a new timeseries panel",
    "Datasource: prometheus-main",
    "Query: checkout error rate"
  ],
  "createdAt": "2026-07-13T10:00:00Z",
  "expiresAt": "2026-07-13T10:30:00Z"
}
```

### 14.2 两阶段保存

```text
Prepare
  ↓
DashboardSaveIntent
  ↓
Approval
  ↓
Execute
  ↓
DashboardSaveResult
```

### 14.3 DashboardSaveIntent

```json
{
  "id": "save_intent_123",
  "tenantId": "tenant_1",
  "chartId": "chart_123",
  "panelDraftId": "panel_draft_123",
  "targetDashboardUid": "checkout-overview",
  "targetFolderUid": "production",
  "expectedDashboardVersion": 18,
  "panelTitle": "Checkout Error Rate",
  "requiredPermission": "dashboards:write",
  "riskLevel": "write_requires_approval",
  "status": "waiting_approval",
  "version": 1,
  "expiresAt": "..."
}
```

### 14.4 Approval

```json
{
  "id": "approval_123",
  "tenantId": "tenant_1",
  "intentId": "save_intent_123",
  "type": "dashboard_add_panel",
  "requestedBy": "user_123",
  "status": "pending",
  "version": 1,
  "approvedBy": null,
  "approvedAt": null,
  "scope": {
    "dashboardUid": "checkout-overview",
    "operation": "add_panel"
  }
}
```

### 14.5 DashboardSaveResult

```json
{
  "id": "save_result_123",
  "tenantId": "tenant_1",
  "intentId": "save_intent_123",
  "status": "success",
  "dashboardUid": "checkout-overview",
  "panelId": 27,
  "dashboardVersion": 19,
  "savedBy": "user_123",
  "savedAt": "..."
}
```

---

## 15. 任务状态机

### 15.1 AnalysisTask 状态

```text
created
  ↓
context_building
  ↓
planning
  ↓
running_tools
  ↓
validating
  ├── running_tools          # 最多一次自动修正
  ├── waiting_user_input
  ├── waiting_approval
  └── completed

waiting_user_input → context_building | cancelled
waiting_approval   → executing_write | cancelled | failed
executing_write    → completed | failed

任意非终态 → failed | cancelled
```

### 15.2 状态说明

|状态|说明|
|---|---|
|created|任务已创建但尚未执行|
|context_building|正在构建 Grafana、会话、知识和历史上下文|
|planning|正在生成可展示计划|
|running_tools|正在调用只读指标、查询、Grafana 或知识工具|
|validating|正在校验 PromQL、图表和权限|
|waiting_user_input|信息不足，需要用户选择或补充|
|waiting_approval|只读阶段完成，Eino CheckPoint 已持久化，等待写操作确认|
|executing_write|审批通过后执行已限定 scope 的写操作|
|completed|任务成功完成|
|failed|任务无法自动恢复地失败|
|cancelled|用户或系统取消任务|

状态变更必须通过 `task.Transition(to)` 领域方法；非法迁移返回 `invalid_state_transition`。Repository 不负责判断状态是否合法。

### 15.3 图表状态

```text
draft
  ↓
validating
  ├── ready → saved_to_dashboard
  ├── no_data
  ├── error
  └── timeout
```

### 15.4 工具调用状态

```text
pending
running
success
failed
timeout
cancelled
```

---

## 16. 流式事件协议

### 16.1 传输方式

任务事件使用 SSE。HTTP handler 必须从 durable `TaskEventStore` 重放历史，再订阅通知器获取新事件。

原因：

- 服务端单向推送足够
- 浏览器支持良好
- 实现成本低
- 可基于 sequence 恢复
- 比 WebSocket 更适合初期任务流

响应要求：

- `Content-Type: text/event-stream`
- `Cache-Control: no-cache`
- 支持查询参数 `afterSequence`，也可读取 `Last-Event-ID`
- 每 15 秒发送 heartbeat，heartbeat 不占业务 sequence
- 客户端较慢时可以断开，但不能丢失 durable event；重连后重放
- Plugin Backend 转发时禁止代理缓冲

### 16.2 事件结构

```json
{
  "eventId": "evt_123",
  "taskId": "task_123",
  "sessionId": "session_123",
  "sequence": 12,
  "type": "chart.created",
  "timestamp": "2026-07-13T10:00:00Z",
  "payload": {}
}
```

### 16.3 sequence

每个 Task 的事件 sequence 必须单调递增。

前端断线后重新连接：

```text
GET /v1/tasks/{taskId}/events?afterSequence=12
```

服务端从 13 开始重放。

### 16.4 核心事件

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
chart.updated
chart.execution_started
chart.execution_completed
chart.execution_failed

approval.required
approval.approved
approval.rejected

dashboard.save_completed
dashboard.save_failed

task.completed
task.failed
```

### 16.5 Tool 事件示例

```json
{
  "type": "tool.started",
  "payload": {
    "toolCallId": "tool_call_123",
    "toolName": "grafana.search_metrics",
    "displayName": "Searching metric catalog"
  }
}
```

### 16.6 Chart 事件示例

```json
{
  "type": "chart.created",
  "payload": {
    "chart": {
      "id": "chart_123",
      "title": "Checkout Error Rate",
      "status": "validating"
    }
  }
}
```

---

## 17. HTTP API 最小契约

本节路径必须进入 `contracts/openapi/`；handler 和生成客户端以 OpenAPI 为准。AI Core 公开业务 API 使用 `/v1`，运维端点不带版本。

### 17.0 运维端点

```text
GET /healthz    # 进程存活，不检查外部依赖
GET /readyz     # 配置已加载、migration 完成、必需 Adapter 可用
```

### 17.1 Session

```text
POST   /v1/sessions
GET    /v1/sessions
GET    /v1/sessions/{sessionId}
PATCH  /v1/sessions/{sessionId}
POST   /v1/sessions/{sessionId}/share
POST   /v1/sessions/{sessionId}/fork
GET    /v1/sessions/{sessionId}/messages
GET    /v1/sessions/{sessionId}/canvas
PATCH  /v1/sessions/{sessionId}/canvas
```

### 17.2 Task

```text
POST   /v1/tasks
GET    /v1/tasks/{taskId}
GET    /v1/tasks/{taskId}/events
POST   /v1/tasks/{taskId}/messages
POST   /v1/tasks/{taskId}/cancel
```

### 17.3 Chart

```text
GET    /v1/charts/{chartId}
PATCH  /v1/charts/{chartId}
POST   /v1/charts/{chartId}/execute
GET    /v1/charts/{chartId}/revisions
GET    /v1/charts/{chartId}/executions
POST   /v1/charts/{chartId}/save-intents
```

### 17.4 Approval

```text
GET    /v1/approvals/{approvalId}
POST   /v1/approvals/{approvalId}/approve
POST   /v1/approvals/{approvalId}/reject
```

### 17.5 Template

```text
GET    /v1/templates
GET    /v1/templates/{templateId}
POST   /v1/templates/{templateId}/runs
```

### 17.6 Playbook

```text
GET    /v1/playbooks
GET    /v1/playbooks/{playbookId}
POST   /v1/playbooks/{playbookId}/runs
```

这些是 AI Core 面向前端的 Facade；真实定义和执行由 assistant-mcp 拥有，AI Core 不复制 Playbook 数据表。

### 17.7 Alert

```text
POST   /v1/alerts/grafana
GET    /v1/alerts/{alertEventId}
POST   /v1/alerts/{alertEventId}/sessions
```

### 17.8 Evaluation

```text
POST   /v1/evaluations
GET    /v1/evaluations/tasks/{taskId}
```

### 17.9 通用请求头

所有请求必须传播：

```text
X-Request-ID
X-Trace-ID
```

创建任务、审批、保存等可能重试的命令必须携带：

```text
Idempotency-Key
```

HTTP 更新操作必须使用：

```text
If-Match
ETag
```

应用 Command 和 MCP Tool 在请求体显式携带：

```json
{
  "expectedVersion": 3
}
```

### 17.10 幂等语义

- 幂等键作用域为 `tenantId + userId + operation + Idempotency-Key`。
- 同一键、相同 request hash：处理中返回当前状态；已完成返回已保存响应。
- 同一键、不同 request hash：返回 `idempotency_key_conflict`。
- 创建 Task、Approval decision、Dashboard save 和 Promotion decision 必须在业务事务中写 `idempotency_records`。
- Adapter 不得仅靠进程内 map 实现生产幂等；Mock 可以使用内存实现。

---

## 18. 错误模型

统一错误响应：

```json
{
  "error": {
    "code": "query_no_data",
    "message": "The query completed successfully but returned no data.",
    "retryable": true,
    "details": {},
    "requestId": "req_123",
    "traceId": "trace_123"
  }
}
```

### 18.1 主要错误码

```text
invalid_request
unauthorized
forbidden
tenant_mismatch
not_implemented
version_conflict
invalid_state_transition
idempotency_key_conflict

session_not_found
task_not_found
chart_not_found

intent_ambiguous
metric_not_found
label_not_found

promql_syntax_error
query_no_data
query_timeout
query_too_expensive
query_execution_failed

ai_unavailable
ai_rate_limited
ai_output_schema_invalid

tool_not_found
tool_not_supported
tool_permission_denied
tool_timeout
tool_execution_failed

dashboard_write_forbidden
dashboard_version_conflict
dashboard_save_failed

approval_required
approval_expired
approval_rejected

playbook_not_ready
playbook_validation_failed
```

同一错误码在 HTTP、SSE、MCP 和 Repository Adapter 中必须保持同一语义。HTTP status 与错误码的映射写入 `contracts/errors/error-codes.yaml`，不得散落在 handler 中。

---

## 19. 持久化设计与数据库替换

### 19.1 核心原则

- SQLite 是首个 Adapter，PostgreSQL 是后续 Adapter；二者实现同一组 Repository Contract Test。
- 领域对象、Repository、应用服务和 HTTP/MCP 契约不得出现 SQLite/PostgreSQL 专有类型。
- SQLite 和 PostgreSQL 使用独立 migration 目录，保持相同的逻辑 schema，不强求 SQL 文件逐字相同。
- JSON 字段在 SQLite 中用 TEXT 保存 canonical JSON，在 PostgreSQL 中可用 JSONB；Repository 返回强类型领域对象。
- 所有更新使用乐观锁：`WHERE id = ? AND tenant_id = ? AND version = expectedVersion`；影响行数为 0 时区分 not found 与 version conflict。
- 多对象写入使用 `ApplicationStore.WithinTransaction`。事务不得跨服务、不得跨外部 API。

### 19.2 AI Core Store 表

```text
analysis_sessions
session_messages
session_notes
session_shares
session_forks
session_canvases

analysis_tasks
task_steps
task_events
tool_calls
agent_checkpoints

chart_drafts
chart_revisions
query_revisions
chart_executions

panel_drafts
dashboard_save_intents
approvals
dashboard_save_results
rollback_points

templates
template_runs
object_drafts                 # Skill/Playbook 生成后的待确认草稿
promotion_requests            # private → shared 审批业务状态

alert_events
alert_event_sessions
evaluation_records
idempotency_records
```

### 19.3 assistant-mcp Store 表

```text
service_entries
runbooks
documents
document_chunks
skills
playbooks
playbook_versions
playbook_runs
resource_usage_counters
```

assistant-mcp 不保存 AI Core 的 Session、Task 或 Approval。两边只通过稳定 ID 和 API/MCP 契约关联。

### 19.4 必须独立建列的字段

下列字段不能只藏在 JSON 中：

```text
id, tenant_id, org_id, owner_user_id
session_id, task_id, chart_id, approval_id
folder_uid, datasource_uid, dashboard_uid, panel_id
status, visibility, version
created_at, updated_at
```

每张租户数据表的唯一键/索引必须包含 `tenant_id`，所有 Repository 查询必须显式带 tenant 条件。

### 19.5 关键表最小字段

`analysis_sessions`：

```text
id, tenant_id, org_id, owner_user_id, title, visibility, status
active_folder_uid, source_template_id, source_session_id, datasource_uid
default_time_range_json, tags_json, context_json
created_at, updated_at, version
```

`analysis_tasks`：

```text
id, tenant_id, session_id, task_type, status, intent
input_json, plan_summary_json, error_code
created_at, started_at, completed_at, updated_at, version
```

`session_canvases`：

```text
id, tenant_id, session_id, layout, items_json
created_at, updated_at, version
UNIQUE(tenant_id, session_id)
```

`task_events`：

```text
id, tenant_id, session_id, task_id, sequence, event_type
payload_json, created_at
UNIQUE(tenant_id, task_id, sequence)
```

`chart_drafts`：

```text
id, tenant_id, session_id, task_id, title
query_language, query_expression, visualization_json, time_range_json
datasource_uid, status, current_revision, pinned, created_by
created_at, updated_at, version
```

`approvals`：

```text
id, tenant_id, intent_id, approval_type, status
requested_by, decided_by, decision_reason, scope_json
checkpoint_id, expires_at, created_at, decided_at, version
```

`agent_checkpoints`：

```text
id, tenant_id, task_id, runtime_name, format_version
payload_bytes, created_at, expires_at
```

Checkpoint payload 不得包含 Grafana grant、Cookie、Token 或未脱敏 Secret。不同 runtime 可以不兼容旧 checkpoint，但必须通过 `runtime_name + format_version` 明确拒绝，而不是反序列化崩溃。

`tool_calls` 只保存脱敏摘要：

```text
id, tenant_id, task_id, tool_name, tool_version, risk_level
input_summary_json, output_summary_json, status, duration_ms
error_code, started_at, completed_at
```

### 19.6 SQLite Adapter 要求

- 每个服务使用自己的 SQLite 文件和连接池。
- 启动时启用 foreign keys、busy timeout；是否启用 WAL 由 Adapter 配置，默认启用。
- 写事务尽量短，不在事务中调用 LLM、MCP、Grafana 或 Prometheus。
- migration 执行完成后才开放 readiness。
- Repository Contract Test 使用临时数据库文件，不能依赖开发者本地固定路径。

### 19.7 PostgreSQL Adapter 要求

- PostgreSQL Adapter 可以后置，但目录、构造器、配置和 Contract Test target 必须预留。
- 切换仅修改 bootstrap 配置，不修改 command/workflow/domain。
- PostgreSQL migration 与 SQLite migration 共享逻辑版本号和变更说明。
- CI 在 PostgreSQL Adapter 开始实现后，对两种 Adapter 运行同一套 Repository Contract Test。

### 19.8 Migration 规则

```text
migrations/sqlite/0001_initial.sql
migrations/sqlite/0002_add_chart_reference.sql
migrations/postgres/0001_initial.sql
migrations/postgres/0002_add_chart_reference.sql
```

- 已合并的 migration 不得修改，只能追加。
- 同一逻辑变更在两个目录使用相同序号；暂未实现的 Adapter 可以保留说明文件，但启用该 Adapter 前必须补齐。
- migration 只负责 schema；种子和 Mock 数据放 `packages/fixtures`/`data/mock-scenarios`。
- 每次 migration 必须有 upgrade test；涉及数据转换时另加 fixture 回归测试。

---

## 20. Mock 设计

### 20.1 Mock 的目标

Mock 需要支持：

- 前端独立开发
- AI Core 独立开发
- Grafana Plugin 独立开发
- 数据层独立开发
- 稳定 E2E 测试
- 演示
- 错误场景测试

Mock 不是“返回随便一个结果”，而应返回可预测、可复现、带固定场景标识的结果。

### 20.2 Adapter 替换

```text
MetricCatalogPort
├── PrometheusMetricCatalogAdapter
└── MockMetricCatalogAdapter

QueryEnginePort
├── PrometheusQueryEngineAdapter
└── MockQueryEngineAdapter

ModelPort
├── ModelGatewayAdapter
└── DeterministicMockLLMAdapter

DashboardReadPort
├── GrafanaDashboardReadAdapter
└── MockGrafanaDashboardReadAdapter

DashboardWritePort
├── GrafanaDashboardWriteAdapter
└── MockGrafanaDashboardWriteAdapter

KnowledgeSearchPort
├── VectorKnowledgeAdapter
└── MockKnowledgeAdapter
```

### 20.3 配置示例

```yaml
adapters:
  model: deterministic_mock
  metricCatalog: mock
  queryEngine: mock
  grafanaRead: mock
  grafanaWrite: mock
  knowledge: mock
  playbook: mock
```

也允许混合模式：

```yaml
adapters:
  model: real
  metricCatalog: prometheus
  queryEngine: prometheus
  grafanaRead: plugin_proxy
  grafanaWrite: mock
  knowledge: mock
```

配置只在 bootstrap 读取并组装 Adapter；禁止把整个配置对象传入 domain/application。

### 20.4 固定场景

建议至少提供：

```text
checkout_error_rate
checkout_latency
checkout_full_analysis

query_no_data
query_timeout
query_invalid_syntax
query_invalid_label
query_too_expensive

dashboard_save_success
dashboard_permission_denied
dashboard_version_conflict

ai_unavailable
ai_schema_invalid

ambiguous_chart_reference
ambiguous_metric_candidate

session_restore
session_fork
template_run
candidate_playbook
alert_analysis
```

### 20.5 场景触发

开发/测试环境可通过请求头触发：

```text
X-Mock-Scenario: dashboard_version_conflict
```

生产环境必须禁用此能力。

启动时若 `environment=production` 且启用了 mock scenario header，服务必须 fail fast。每个 fixture 必须通过相应 JSON Schema 校验。

### 20.6 checkout_full_analysis 示例

输入：

```text
帮我排查 checkout 服务过去 30 分钟错误率升高
```

Mock 返回：

1. 服务整体错误率
2. 按 route 拆分错误率
3. 按 instance 拆分错误率
4. 固定 PromQL
5. 固定时间序列数据
6. 固定执行耗时
7. 固定 ToolCall 序列
8. 固定 Session 和 Task 事件

---

## 21. 模板和 Playbook 骨架

Template 的执行编排属于 AI Core；Skill/Playbook 的定义、CRUD 和执行能力属于 assistant-mcp。AI Core 只能通过 Port/MCP 契约访问 Skill/Playbook，不直接读取其数据库或 `data/` 文件。

### 21.1 Template

Template 是人工维护的标准入口。

最小契约：

```json
{
  "id": "template_service_error_rate",
  "title": "服务错误率分析",
  "scenario": "service_error_rate",
  "version": 1,
  "parameters": [
    {
      "name": "service",
      "type": "string",
      "required": true
    },
    {
      "name": "timeRange",
      "type": "time_range",
      "required": true
    }
  ],
  "steps": [],
  "defaultCharts": []
}
```

### 21.2 CandidatePlaybook

```json
{
  "id": "candidate_playbook_123",
  "tenantId": "tenant_1",
  "sourceSessionId": "session_123",
  "status": "draft",
  "visibility": "private",
  "ownerId": "user_123",
  "folderUid": null,
  "parameters": [
    "$service",
    "$environment",
    "$time_range"
  ],
  "steps": [],
  "evidence": {
    "sourceCharts": [],
    "successfulExecutions": 1
  }
}
```

### 21.3 PlaybookVersion

```json
{
  "id": "playbook_version_3",
  "tenantId": "tenant_1",
  "playbookId": "playbook_123",
  "version": 3,
  "status": "active",
  "inputSchema": {},
  "steps": [],
  "reviewedBy": "user_123"
}
```

### 21.4 Playbook 执行接口预留

即使当前不实现，也必须预留：

```text
PlaybookRepository
PlaybookExecutor
PlaybookValidator
PlaybookRunRepository
```

Mock 执行器可以返回固定任务和图表。

所有 AI 生成的 Skill/Playbook 先保存为 `object_drafts`，用户编辑并确认后才创建 private 对象；private → shared 必须创建 `promotion_request` 并由目标 Grafana Folder Admin 审批。首个骨架可以只实现对象和状态机，不要求执行引擎真实可用。

---

## 22. Alert Webhook 骨架

### 22.1 Alert Receiver 职责

- 接收 `POST /webhook/alert`
- 在解析 JSON 前校验 Grafana Webhook HMAC 和时间戳
- 解析 Grafana Alert Payload
- fingerprint / groupKey 去重
- 创建或更新 AlertEvent
- 触发 AlertAnalysisTask
- 关联 Session
- 生成摘要链接

Grafana 官方 Webhook HMAC 的默认签名头是 `X-Grafana-Alerting-Signature`。配置 timestamp header 后，签名内容为 `timestamp + ":" + rawBody`，再做 HMAC-SHA256 并 hex 编码。接收端必须使用常量时间比较，并在验签前保留原始 body 字节。参考 [Grafana Webhook HMAC 文档](https://grafana.com/docs/grafana-cloud/alerting-and-irm/alerting/configure-notifications/manage-contact-points/integrations/webhook-notifier/#hmac-signature)。

接收模块依赖以下 Port，以便后续替换去重和投递实现：

```go
type AlertDeduplicator interface {
    FirstSeen(ctx context.Context, tenantID, fingerprint string, ttl time.Duration) (bool, error)
}

type AlertSinkPort interface {
    Deliver(ctx context.Context, event AlertEvent) error
}
```

首个 Mock 可使用内存 Deduplicator 与 channel Sink；真实实现可以切换 SQLite inbox / HTTP AI Core Adapter，而不修改 handler。

### 22.2 AlertEvent

```json
{
  "id": "alert_123",
  "tenantId": "tenant_1",
  "fingerprint": "abc",
  "groupKey": "group_1",
  "status": "firing",
  "labels": {},
  "annotations": {},
  "startsAt": "...",
  "endsAt": null,
  "dashboardUrl": "...",
  "panelUrl": "...",
  "rawPayloadRef": null
}
```

原始 payload 默认不进入 LLM，也不直接写审计日志；如需留存，使用 `rawPayloadRef` 指向受控存储。

### 22.3 骨架要求

MS1 即使不真实处理告警，也应：

- 存在 AlertEvent Schema
- 存在外部接收契约 `POST /webhook/alert`
- HMAC 计算覆盖 `timestamp + ":" + rawBody`，并有错误签名/过期时间戳测试
- 支持 Mock AlertEvent
- 支持从 AlertEvent 创建 Session
- 预留 `alert_event_sessions` 关系

---

## 23. 权限和租户模型

### 23.1 RequestContext

每个请求都应包含统一上下文：

```json
{
  "tenantId": "tenant_1",
  "orgId": "1",
  "user": {
    "id": "user_123",
    "login": "alice",
    "roles": ["Editor"]
  },
  "permissions": [
    "datasources:query",
    "dashboards:read",
    "dashboards:write"
  ],
  "requestId": "req_123",
  "traceId": "trace_123",
  "activeFolderUid": "payment"
}
```

Wire DTO 可包含嵌套 `user`；进入 application 前必须映射为第 11.1 节的 Go `RequestContext`。`GrafanaGrant` 只通过内部 OpenAPI 的 `writeOnly`/`x-sensitive` 字段或 transport metadata 传递，不进入公共业务 Schema、模型 Tool Schema、会话存储或日志。

### 23.2 权限原则

- 所有数据访问绑定 tenantId / orgId
- 会话搜索默认限制在当前租户
- 分享链接仍要经过权限检查
- Fork 后继承内容，不继承写权限
- AI Core 不持有全局管理员权限
- Dashboard 写入使用当前用户身份或受控代理
- ToolCall 必须记录权限判定
- Knowledge、shared Skill、shared Playbook 的读取要求 Folder Permission ≥ View，修改要求 ≥ Edit，晋升审批要求目标 Folder Admin
- private Skill/Playbook 仅 Owner 可访问；默认 list 聚合 active Folder + Shared Folder + 当前用户 private
- Session 可切换 `active_folder_uid`，但切换前后都要校验 View 权限；Session 本身不按 Folder 硬隔离
- 后台只读任务使用创建时的非敏感授权快照；写操作必须在用户确认时重新授权

### 23.3 工具权限

每个 Tool 都声明：

- 所需权限
- 风险等级
- 是否需要 Approval
- 是否允许特定租户启用
- 是否可访问外部网络

---

## 24. 数据安全与出域

### 24.1 可以发送到外部模型的内容

根据配置和脱敏策略，可发送：

- 用户自然语言问题
- PromQL
- 指标名
- 指标类型
- 标签名
- 图表类型
- 时间范围
- 脱敏后的服务名
- 脱敏后的环境名
- Dashboard 和 Panel 摘要

### 24.2 默认不发送

- 原始大量时间序列
- 完整日志
- 完整 Trace
- Token
- API Key
- 用户身份详情
- 内部 URL
- 未脱敏标签值
- Grafana Session
- 数据源 Secret

### 24.3 DataEgressPolicy

必须定义：

```go
type DataEgressPolicy interface {
    Evaluate(ctx context.Context, rc RequestContext, payload ModelPayload) (EgressDecision, error)
}
```

输出：

```json
{
  "allowed": true,
  "redactions": [],
  "reason": "policy_allow_minimal_context"
}
```

Policy 必须在调用 `ModelPort` 前执行，Model Adapter 也应拒绝没有 `EgressDecisionID` 的请求。脱敏规则和允许字段需要单元测试；Grafana grant、Session Cookie、Token 和数据源 Secret 在任何配置下都不可出域。

---

## 25. 可观测性

### 25.1 Trace

一次用户任务至少贯穿：

```text
Browser Request
Plugin Backend
AI Core
Workflow
LLM Call
MCP ToolCall
Prometheus/Grafana
Database
```

统一传播：

```text
traceparent
X-Trace-ID
X-Request-ID
```

### 25.2 Metrics

建议骨架中预置：

```text
task_created_total
task_completed_total
task_failed_total
task_duration_seconds

tool_call_total
tool_call_duration_seconds
tool_call_failed_total

promql_generated_total
promql_validation_success_total
promql_execution_success_total
promql_first_pass_usable_total

chart_created_total
chart_edit_total
chart_save_total

session_restore_total
session_restore_failed_total
session_fork_total

approval_requested_total
approval_approved_total
approval_rejected_total

model_request_total
model_token_total
model_cost_total
model_failure_total
```

### 25.3 日志

日志必须结构化，至少包含：

```text
timestamp
level
service
tenant_id
org_id
user_id
session_id
task_id
tool_call_id
request_id
trace_id
event
error_code
duration_ms
```

禁止把 Secret 和完整原始数据写入日志。

---

## 26. 测试策略

### 26.1 单元测试

每个领域模块测试：

- 状态机
- 权限策略
- Prompt 输入构造
- Schema 校验
- Revision 生成
- Approval 规则
- tenant/folder 过滤规则
- 不允许敏感字段进入 checkpoint、event 和 audit summary

### 26.2 Contract Test

验证：

- OpenAPI
- JSON Schema
- Tool Schema
- SSE Event Schema
- 生成客户端一致性

### 26.3 Adapter Contract Test

同一个 Port 的 Mock 和 Real Adapter 必须满足相同测试集。Contract Test 由 Port 所在模块提供，Adapter 只把构造器传入测试套件。

例如：

```text
QueryEngineContractTests
  ├── MockQueryEngineAdapter
  └── PrometheusQueryEngineAdapter

ApplicationStoreContractTests
  ├── SQLiteApplicationStore
  └── PostgreSQLApplicationStore（实现后启用）
```

Repository Contract Test 至少验证：CRUD、tenant 隔离、乐观锁、事务回滚、唯一约束、分页稳定性、时间精度和领域错误映射。

### 26.4 Integration Test

覆盖：

- AI Core + SQLite（首个真实集成测试）
- AI Core + PostgreSQL（PostgreSQL Adapter 实现后）
- AI Core + assistant-mcp
- Plugin Backend + AI Core
- Grafana Write Adapter + 测试 Grafana

集成测试按 profile 运行，不作为只改文档/契约 PR 的统一硬门槛。

### 26.5 Component Smoke Test

每个可启动模块至少提供一个 smoke target：

- AI Core：启动 → health/readiness → SQLite migration → 创建/读取 Mock Session。
- assistant-mcp：启动 → tools/list → 调一个 read-only Mock Tool。
- Plugin Backend：使用 Mock AI Core Client 处理一个 Resource Handler 请求。
- Frontend：Mock API 下渲染空态、三张图和审批态。

### 26.6 Mock E2E（集成阶段）

Mock E2E 运行完整应用调用链，但外部系统 Adapter 使用确定性 fixture，至少覆盖：

1. 创建会话
2. 输入自然语言
3. 生成三张图
4. 编辑 PromQL
5. 保存并恢复会话
6. 保存到 Dashboard
7. 权限失败
8. 版本冲突
9. AI 不可用
10. Fork 会话

Mock E2E 必须支持固定 ID/时间或可注入时钟，且可通过 `X-Mock-Scenario` 稳定触发权限失败、版本冲突、查询超时、无数据、AI 不可用和断线续传。它是 PR/CI 中优先自动化的纵向链路。

### 26.7 本地真实基础设施混合 E2E

本地混合 E2E 使用 Grafana、Prometheus 和 `node_exporter` 真实容器，加上项目的 AI Core 与 `assistant-mcp` 容器。它验证真实基础设施和服务集成，不取代 Mock E2E。

真实部分必须包括：

- Grafana 插件加载、Frontend → Plugin Backend Resource Handler。
- Plugin Backend → AI Core HTTP/SSE，以及事件断线后按 `sequence` 重放。
- AI Core 与 `assistant-mcp` 之间的 Streamable HTTP MCP 调用。
- Prometheus 对 `node-exporter:9100` 的真实抓取、metadata/label 查询和 PromQL range query。
- AI Core 与 `assistant-mcp` 各自独占的 SQLite migration、写入和重启恢复。
- `PanelDraft → SaveIntent → Approval → AddPanel → Audit`，并真实写入测试 Grafana Dashboard。

为保持结果可重复，Model 默认使用 Deterministic Mock，Knowledge/Playbook/Skill 可以使用 fixture；这些 Mock 只能通过 Adapter 注入。首个 Golden Path 输入为：

```text
查看 node-exporter 最近 30 分钟的 CPU 使用率、内存可用率和系统负载
```

Deterministic Mock Model 应产生固定计划，但指标检索和查询必须真实命中至少以下指标：

```text
node_cpu_seconds_total
node_memory_MemAvailable_bytes
node_load1
```

首版固定 PromQL 建议为：

```promql
100 * (1 - avg by (instance) (rate(node_cpu_seconds_total{mode="idle"}[5m])))
100 * node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes
node_load1
```

PromQL 是 Golden Path 的确定性输入，时间序列样本来自真实 Prometheus；测试可以校验表达式、返回类型和 series 数，但不能校验运行时样本的精确值。

ADR-019 在不增加视图、指标或标签能力的前提下，将该 Golden Path 扩展为有界参数查询。Task 必须持久化
绝对时间范围、有效 step 和 CPU rate window；允许范围为 30 秒至 6 小时，step 来自
`5/10/15/30/60/120/300` 秒，CPU window 只允许 30/60/300 秒。模型只选择 `cpu/memory/load` view，
不得提交 expression、时间或 step；assistant-mcp 注册表根据 view/window 渲染规范 PromQL 并执行 AST allowlist。
Chart Query 持久化 step，Execution 同时区分请求范围和实际返回数据范围。最终事实回复由 AI Core 使用本地
QueryPlan 与有界统计生成，不持久化未经校验的模型自由文本。详细字段、解析优先级和 Gate 见
[`bounded_node_exporter_query_parameters_execution_plan.md`](bounded_node_exporter_query_parameters_execution_plan.md)。

ADR-021 supersede ADR-020 的本地解析职责：Workbench 只提交消息和固定逻辑数据源；AI Core 在 Task 创建前
同步调用 `IntentPlanner` Port，由 Mock 或 Eino Adapter 返回严格的注册 views 与可选相对 range/step。应用层补默认值、
执行既有范围/点数上限、冻结绝对时间并持久化 `QueryPlan.views`。工作流只执行这份冻结计划，不再让模型二次选图；
assistant-mcp 仍只编译注册视图的规范 PromQL。Workbench 可在查询后只读展示有效 QueryPlan。详细 Gate 见
[`natural_language_query_input_execution_plan.md`](natural_language_query_input_execution_plan.md)。

IntentPlanner 的外部模型输入只包含当前消息和最近最多 6 个持久化 User Message + QueryPlan 形成的结构化意图，
不包含 Assistant 事实回复、指标值或本地 formatter 文本。Eino Adapter 使用 provider JSON mode、专属四字段协议和
严格本地校验；空 content 或模型契约错误只允许一次不携带原始响应的重试。模型 transport 错误不重试，所有失败仍
发生在 Message、Task 和幂等记录写入之前。该加固不改变 ADR-021 的 view、范围、step 或本地执行边界。

验收断言至少包括：

1. Prometheus target `up{job="node-exporter"}` 为 1。
2. 指标 metadata/labels 可检索，三条 PromQL 执行成功且每张图至少返回一个 series/sample。
3. 前端收到三张 `ChartDraft`，TaskEvent `sequence` 单调递增且刷新后可重放。
4. 编辑一条 PromQL 后产生新的 `QueryRevision` 和 `ChartExecution`，旧版本仍可追溯。
5. 重启 AI Core 后，会话、图表、任务事件和审批状态能从 SQLite 恢复。
6. 未审批的写入返回 `approval_required`；批准后测试 Dashboard 新增 Panel，Dashboard version 递增，Panel PromQL 与已批准草稿一致。
7. 审计记录包含 request/trace、用户、目标 Dashboard、approval 和结果摘要，但不包含 Grafana grant、Cookie、Token 或时间序列明细。
8. 使用相同 idempotency key 重试已批准保存时不产生重复 Panel；使用过期或 scope 不匹配的 ApprovalEvidence 必须失败。

真实指标值随时间和运行环境变化，测试不得断言 CPU、内存或 load 的精确数值；只断言 target 健康、返回类型、非空结果、时间范围和 Schema。无数据、超时、权限失败、版本冲突等需要精确复现的异常仍由 Mock E2E 覆盖。

测试环境必须与日常/生产 Grafana 隔离：使用独立 Compose project name、Grafana org/folder/dashboard、datasource UID 和命名 volume。测试断言使用本次 `runId`、保存前后 Dashboard version 与新 Panel ID，不能依赖“同名 Panel 已存在”。本地停止默认不隐式删除调试数据，CI teardown 必须显式 `down -v` 清理；任何脚本都不得接受生产 Grafana/Prometheus 地址作为默认值。

本地首版 Prometheus Adapter 可以通过 bootstrap 配置直连 `http://prometheus:9090`，但 Request/Domain 仍必须携带不透明 `datasourceUid`。直连模式不覆盖 Grafana datasource RBAC；若生产要求所有查询经过 Grafana datasource proxy，应新增对应 Adapter/ADR，并复用同一 `PrometheusPort` Contract Test。

所有 E2E 都是纵向链路目标，不是 Level 1 接口骨架的阻塞条件。`local-real` 可以作为手工、nightly 或合并前专项检查；当环境和时长稳定后再提升为常规 CI 门槛。

### 26.8 Golden Query

`tests/golden-queries/` 建议：

```text
node_exporter_analysis.yaml
service_error_rate.yaml
service_latency.yaml
instance_cpu.yaml
pod_restart.yaml
request_rate.yaml
queue_lag.yaml
database_connection.yaml
```

每个 Case 包括：

- 用户输入
- 上下文
- 期望指标
- 期望 PromQL 特征
- 期望图表数
- 允许的变体
- 评分规则

---

## 27. CI/CD

### 27.1 Pull Request 检查

每次 PR 必须运行：

```text
lint
typecheck
unit-test
contract-validation
generated-client-diff
migration-validation
dependency-boundary-check
secret-scan
```

按代码成熟度追加：

```text
component-smoke          # 对已有可启动模块
sqlite-contract-test     # SQLite Adapter 建立后必须
postgres-contract-test   # PostgreSQL Adapter 建立后必须
mock-e2e                 # Level 3 集成阶段
local-real-e2e           # Level 4，先手工/nightly，稳定后再进入常规 CI
security-scan            # 有可构建镜像后
```

### 27.2 契约变更规则

契约修改必须：

1. 修改 OpenAPI / Schema
2. 更新版本
3. 重新生成客户端
4. 更新 Mock Fixture
5. 更新 Contract Test
6. 说明兼容性

### 27.3 数据库 Migration

禁止修改已经合并并发布的 Migration 文件。

新增 Migration：

```text
migrations/sqlite/0008_add_chart_reference.sql
migrations/postgres/0008_add_chart_reference.sql
```

Migration validation 必须检查逻辑版本号成对、SQLite upgrade 可执行，以及已发布文件 checksum 未变化。PostgreSQL Adapter 尚未启用时，可以只校验占位 manifest；启用前必须补齐 SQL。

---

## 28. 多人协作拆分

### 28.1 推荐模块负责人

|模块|主要职责|
|---|---|
|Plugin Frontend|工作台、会话、对话、多图、图表编辑、审批 UI|
|Plugin Backend|Grafana Context、RBAC、Resource Handler、API Proxy|
|AI Core|任务状态机、Workflow、Intent、Context、Policy|
|assistant-mcp / Grafana namespace|指标、Dashboard/Panel 读取、Patch、写入、版本冲突|
|AI Core Persistence|Session、Task、Chart、Approval、Checkpoint、TaskEvent|
|assistant-mcp Catalog Persistence|Knowledge、Skill、Playbook|
|Agent / Model|Eino Adapter、Model Gateway、PromQL Skill、Chart Skill|
|Knowledge / Playbook / Skill|四 namespace 内相应服务和 Contract|
|Alert|Webhook、AlertEvent、告警关联会话|
|Evaluation / Observability|Golden Query、埋点、Trace、成本|

### 28.2 CODEOWNERS

建议：

```text
/apps/grafana-plugin/frontend/ @frontend-team
/apps/grafana-plugin/backend/ @grafana-team
/services/ai-core/ @ai-core-team
/services/assistant-mcp/internal/namespaces/grafana/ @observability-team @grafana-team
/services/assistant-mcp/internal/namespaces/knowledge/ @knowledge-team
/services/assistant-mcp/internal/namespaces/playbook/ @ai-core-team
/services/assistant-mcp/internal/namespaces/skills/ @ai-core-team
/contracts/ @architecture-team
/services/ai-core/migrations/ @backend-team @architecture-team
/services/assistant-mcp/migrations/ @backend-team @architecture-team
/tests/golden-queries/ @ai-quality-team
```

### 28.3 协作要求

每个模块负责人必须提供：

- Port 实现
- Mock 实现
- Contract Test
- README
- 示例调用
- 错误码
- Metrics
- 配置说明
- 数据所有权说明
- Adapter Contract Test 入口

---

## 29. 分阶段替换策略

### 29.1 骨架初版

接口、领域状态和真实 SQLite Store 优先；外部系统大部分使用 Mock：

```text
Plugin Frontend: Real
Plugin Backend: Real
AI Core Workflow: Minimal Real
Agent Runtime: Eino Adapter skeleton
Model: Mock
Prometheus: Mock
Grafana Read: Mock
Grafana Write: Mock
AI Core Persistence: SQLite Real
assistant-mcp Catalog Persistence: SQLite or deterministic fixture
```

骨架阶段可以并行实现一个最薄的真实 Prometheus Adapter 和测试 Grafana Write Adapter，用于 `local-real` profile 验证 Port/Adapter 可替换性。这是集成测试基础设施前置，不表示把 MS2 的真实模型、完整指标字典、生产鉴权或产品质量目标提前到 MS1；默认骨架验收仍以 Mock 可重复、模块可独立验证为准。

### 29.2 MS2

```text
Model: Real
Prometheus: Real
Metric Catalog: Real
Grafana Read: Real
Grafana Write: Basic Real
Persistence: SQLite Real；开始 PostgreSQL Adapter
```

### 29.3 MS3

```text
Session Search: Real
Share/Fork: Real
Template: Real
Candidate Playbook: Partial Real
Dashboard Conflict: Real
Audit: Real
```

### 29.4 MS4

```text
Alert Receiver: Real
Model Routing: Real
Playbook: Minimal Real
Knowledge: Expanded
Quality Evaluation: Real
Deployment: Production-ready
```

关键要求：

> 每个阶段只替换 Adapter，不重写上层业务流程。

---

## 30. 首个骨架实现清单与顺序

后续根据本文生成代码时按以下顺序提交。前一步未稳定时，不应越过接口直接堆业务实现。

### 30.1 第 0 步：仓库与决策记录

- 创建第 5 节目录、`go.work`、各 Go module、Frontend package、Makefile。
- 建立 ADR-001 至 ADR-018 的文件和状态。
- 为每个服务添加 README：职责、非职责、启动方式、Port、数据所有权。
- CI 先跑格式、编译、类型检查和依赖边界检查。

### 30.2 第 1 步：契约

- 建立 RequestContext、Error、Session、Task、TaskEvent、Chart、Approval Schema。
- 建立 Plugin Frontend ↔ Plugin Backend Resource OpenAPI，以及 Plugin Backend ↔ AI Core OpenAPI；共享业务 Schema 不得复制定义。
- 建立 assistant-mcp 最小工具集 Schema。
- 生成 TypeScript/Go client，并让 `generated-client-diff` 通过。
- 所有 fixture 先通过 Schema 校验。

### 30.3 第 2 步：Domain 与 Port

- 建立 Task/Chart/Approval 状态机和领域错误。
- 建立第 11 节全部 Port，方法可暂时没有真实 Adapter。
- 建立 `ApplicationStore`、事务边界、Repository Contract Test 套件。
- 建立 Policy：tenant、permission、data egress、approval。

### 30.4 第 3 步：可独立验证的 Adapter

- 实现 AI Core SQLite Store 与 migration，跑通 Repository Contract Test。
- 实现 Deterministic Mock Model、Metric Catalog、Query Engine、Grafana Read/Write。
- 实现 durable SQLite TaskEventStore + 进程内 Notifier。
- 建立 Eino AgentRuntime Adapter 构造器；暂未实现的 Run 可以返回 `not_implemented`，但不能伪装成功。
- 建立 assistant-mcp registry，最少让 tools/list 和一个 Mock read tool 可运行。

### 30.5 第 4 步：Inbound 与 Bootstrap

- AI Core 提供 health/readiness、Session/Task 最小 API 和 SSE replay handler。
- Plugin Backend 使用生成 client 代理最小 API。
- Frontend 建立工作台页面和 Mock 三图状态，不要求真实 Grafana 数据。
- 所有 Adapter 选择只发生在 bootstrap。

### 30.6 第 5 步：参考纵向链路（集成目标）

以下链路是后续 Mock E2E 目标，不是首个“接口骨架完成”的硬条件：

1. 默认 SQLite profile 启动 Grafana、Plugin、AI Core 和 assistant-mcp。
2. 用户创建会话并输入“帮我排查 checkout 服务过去 30 分钟错误率升高”。
3. Mock Metric Catalog、PromQL Skill、Query Engine 产生三张 ChartDraft。
4. 前端展示三张图；编辑 PromQL 后创建 QueryRevision 和新的 ChartExecution。
5. 刷新后从 SQLite 恢复会话、图表和任务事件。
6. 保存 Dashboard 时生成 PanelDraft、SaveIntent 和 Approval；用户确认后 Mock Write 返回成功。
7. 记录 DashboardSaveResult 和 AuditEvent。
8. 可重放无权限、版本冲突、AI 不可用和断线续传场景。

### 30.7 第 6 步：本地真实基础设施混合 E2E

Mock 纵向链路稳定后，按以下顺序替换真实基础设施 Adapter，不修改 application/domain：

1. 增加 Prometheus 和 `node_exporter` Compose 服务，配置 scrape、readiness 和持久化/临时数据目录。
2. Provision Grafana Prometheus datasource、测试 Folder 和 `AI E2E` Dashboard；应用路径不得使用测试初始化管理员凭证。
3. 为 Grafana 容器构建并加载 Linux 目标架构的 Plugin Frontend/Backend。
4. 实现最薄 Prometheus Adapter，使用 `node_*` 指标通过同一 MetricCatalog/QueryEngine Contract Test。
5. 保持 Deterministic Mock Model，执行 `node_exporter_analysis` Golden Path，真实生成三张图。
6. 验证 SSE 重放、PromQL 编辑、SQLite 重启恢复。
7. 实现/接入测试 Grafana 受控代理，验证审批前拒绝、审批后新增 Panel、版本递增与审计落盘。
8. 将 `make e2e-local` 接入手工/nightly 检查；稳定后再评估是否成为合并门槛。

---

## 31. 骨架验收标准

### 31.1 Level 1：接口骨架完成（首个硬门槛）

- Monorepo 和模块 README 建立，Go module/Frontend 可编译或 typecheck。
- OpenAPI、JSON Schema、MCP Tool Schema 和错误码可校验。
- 生成客户端可重复生成且无未提交 diff。
- Domain、Port、状态机和依赖边界存在并有单元测试。
- AI Core SQLite migration 与 Repository Contract Test 可运行。
- Mock Fixture 可加载并通过 Schema。
- 未实现能力明确返回 `not_implemented`/`tool_not_supported`。
- 不要求全系统 `docker compose up` 成功。

### 31.2 Level 2：模块可运行（能做的小模块优先达到）

- AI Core 可独立启动，health/readiness 正常，可用 SQLite 创建/恢复最小 Session/Task。
- assistant-mcp 可独立启动，tools/list 和至少一个 Mock read tool 可调用。
- Plugin Backend 可用 Mock AI Core Client 跑一个 Resource Handler。
- Frontend 可用 Mock API 展示空态、多图和审批态。
- TaskEvent 可持久化并按 sequence 重放。
- Mock/Real Adapter 可通过配置切换，application 代码无分支。

### 31.3 Level 3：Mock 纵向链路（集成目标）

- 完成第 30.6 节参考链路。
- 自然语言输入、多图、编辑、恢复和审批可串联。
- 权限失败、版本冲突、AI 不可用和断线重放可解释且不丢已有内容。
- Compose 默认使用 SQLite；PostgreSQL profile 不要求在此阶段完成。

### 31.4 Level 4：本地真实基础设施混合 E2E（集成增强目标）

- 五个常驻容器 Grafana、AI Core、`assistant-mcp`、Prometheus、`node_exporter` 可通过统一命令启动并通过 readiness。
- `node-exporter` target 可被 Prometheus 抓取，真实 metadata/label/PromQL 查询可生成三张非空图表。
- HTTP、SSE、MCP、SQLite 和 Grafana Dashboard 写入均走真实链路；Model 可以保持确定性 Mock。
- 会话恢复、QueryRevision、TaskEvent replay 和 Approval 状态在容器/进程重启后不丢失。
- 未审批写入失败，审批后只新增 Panel、不覆盖或删除现有 Panel；Grafana version 与审计记录可验证。
- 真实数据测试只断言健康、Schema、非空和状态迁移；确定性错误语义继续由 Level 3 覆盖。
- 该 Level 不反向阻塞 Level 1；是否成为日常 CI 门槛由运行时长和稳定性决定。

### 31.5 全阶段架构验收

- 前端可只依赖 Mock 开发
- AI Core 可只依赖 Tool Mock 开发
- MCP 工程师可独立实现 Tool
- SQLite/PostgreSQL 工程师可在不改应用层的前提下独立实现 Repository Adapter
- 测试可通过固定场景稳定复现
- 模块替换不要求修改调用方
- Plugin Backend 与 AI Core 解耦，AI Core 与 assistant-mcp 解耦
- Grafana 读写分离，ChartDraft/PanelDraft 分离
- tenant/org/user/folder 上下文贯穿，写操作必须审批和重新授权

---

## 32. 推荐的首批 ADR

建议在 `docs/adr/` 建立：

```text
ADR-001-use-monorepo.md
ADR-002-plugin-backend-is-thin-proxy.md
ADR-003-ai-core-is-independent-service.md
ADR-004-use-port-adapter-pattern.md
ADR-005-use-sse-for-task-events.md
ADR-006-chart-draft-separated-from-panel-draft.md
ADR-007-dashboard-write-requires-approval.md
ADR-008-session-is-primary-persistence-unit.md
ADR-009-mock-only-through-adapters.md
ADR-010-contract-first-development.md
ADR-011-prometheus-first.md
ADR-012-do-not-persist-private-model-reasoning.md
ADR-013-use-go-and-eino-for-ai-core.md
ADR-014-sqlite-first-postgresql-adapter-later.md
ADR-015-single-assistant-mcp-with-namespaces.md
ADR-016-service-data-ownership.md
ADR-017-grafana-delegation-grant.md
ADR-018-reuse-grafana-folder-permissions.md
ADR-019-local-docker-hybrid-e2e.md
```

其中 ADR-017 状态为 Provisional；ADR-019 记录 `mock-e2e` 与 `local-real` 双测试跑道、五容器拓扑、直连 Prometheus 的测试边界及升级 CI 门槛的条件。其他 ADR 文件可在建立骨架时按本文内容补齐。Provisional ADR 不妨碍创建接口和 Mock，但会阻止对应真实高风险 Adapter 进入生产。

---

## 33. 关键开发约束

1. 不允许前端直接调用 AI 模型。
2. 不允许 AI Core 绕过 Plugin Backend 任意写 Grafana。
3. 不允许业务代码判断 `mockMode`。
4. 不允许 Tool 返回没有 Schema 的任意对象。
5. 不允许只保存聊天文本。
6. 不允许 ChartDraft 直接等同于 Grafana Panel JSON。
7. 不允许写操作没有 Approval。
8. 不允许跨租户查询。
9. 不允许事件没有 taskId、sessionId 和 sequence。
10. 不允许接口字段在多个模块中手写多份。
11. 不允许把完整时间序列发送给外部模型作为默认行为。
12. 不允许把模型私有推理过程持久化。
13. 不允许无版本控制覆盖 Dashboard。
14. 不允许未分类错误直接返回 500 文本。
15. 不允许 domain/application import Eino、mcp-go、Grafana SDK 或数据库驱动。
16. 不允许 Repository 暴露 `*sql.DB`、SQL error、JSONB 或数据库事务类型。
17. 不允许两个服务共享写同一个 SQLite 文件或跨服务直查数据库。
18. 不允许 write MCP Tool 在缺少有效 ApprovalEvidence 时执行。
19. 不允许未实现能力返回空成功结果。
20. 不允许在生产启用 `X-Mock-Scenario`。
21. 不允许把 Grafana delegation grant 持久化或写入日志。
22. 不允许跨进程直接复用手写 DTO；必须使用契约生成类型或显式 mapping。

---

## 34. 总结

该骨架代码的核心目标不是尽早堆出最多功能，而是确保整个项目从一开始就具备稳定演进能力。

最终骨架应满足：

- 前端可以不等 AI 实现完成。
- AI Core 可以不等 Prometheus 和 Grafana Tool 完成。
- Prometheus 和 Grafana 工具可以独立实现。
- 会话和持久化可以独立建设。
- Mock 可以逐步支持完整演示和 E2E，但首个接口骨架不以全系统可运行为完成条件。
- 本地可以用 Grafana、Prometheus、`node_exporter` 加项目服务运行真实基础设施混合 E2E，验证 Adapter 替换和完整协议链路。
- 确定性 Mock E2E 与真实基础设施 E2E 长期并存：前者负责精确回归和错误注入，后者负责发现网络、镜像、插件、真实 API 与权限回程问题。
- 后续真实能力可以逐步替换 Mock。
- 替换过程中接口、状态机和领域对象保持稳定。
- 权限、安全、审批、审计和可观测性不会在后期被迫重构。

整个方案可以概括为：

> 接口、领域模型和状态机先完整；当前技术选择必须封装为 Adapter；未实现能力明确报错，能独立验证的小模块优先使用确定性 Mock；会话是分析过程的核心资产，图表草稿和 Grafana Panel 必须分离，所有写操作必须进入确认流。

这将使该项目具备多人并行开发、持续联调、阶段性交付和全周期演进的基础。
