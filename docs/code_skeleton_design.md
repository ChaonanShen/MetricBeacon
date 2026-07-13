# Grafana 自然语言指标分析工作台：全周期骨架代码设计文档

> 文档状态：Draft  
> 版本：v1.0  
> 适用范围：MS1–MS4 全周期  
> 目标读者：产品经理、架构师、前端工程师、Grafana 插件工程师、后端工程师、AI/Agent 工程师、数据平台工程师、测试工程师、SRE  
> 最后更新：2026-07-13

---

## 1. 文档目的

本文件用于定义“Grafana 自然语言指标分析工作台”的全周期骨架代码方案。

这里的“骨架代码”不是仅为 MS1 准备的临时代码，也不是为了演示而快速拼接的一次性工程，而是整个 MS1–MS4 周期持续演进的基础工程结构。其目标是：

1. 在功能尚未全部实现时，系统也可以通过 Mock 结果完整运行。
2. 先稳定模块边界、接口关系、领域模型和状态机，再由多人并行完成各自模块。
3. 允许真实实现逐步替换 Mock，而不影响其他模块。
4. 支持从自然语言分析、PromQL 生成、多图工作台、会话沉淀，逐步演进到分享/Fork、模板、Playbook、告警入口和更多数据源。
5. 保证权限、安全、审计、可观测性和接口兼容性从第一天进入设计，而不是后期补丁式接入。
6. 保证前端、Grafana Plugin Backend、AI Core、MCP 工具和持久化层之间通过稳定契约协作，避免多人开发过程中的字段漂移和隐式耦合。

本文主要回答以下问题：

- 整个代码仓库应如何组织。
- 初期需要运行哪些进程。
- 每个模块负责什么，不负责什么。
- 核心接口和数据对象如何定义。
- 模块之间如何通信。
- Mock 如何设计和替换。
- 任务和图表状态如何流转。
- 多人协作时如何拆分工作。
- 如何判断骨架代码已经“搭好”。

---

## 2. 设计原则

### 2.1 逻辑分层，部署收敛

系统在逻辑上按照完整目标架构拆分为：

1. Grafana App Plugin Frontend
2. Grafana Plugin Backend
3. AI Core
4. MCP 工具层
5. Model Gateway
6. 数据与治理层

但在早期部署上不应拆成过多微服务。建议初期只运行以下进程：

- Grafana
- Grafana App Plugin Backend
- AI Core
- MCP Host
- PostgreSQL
- 可选 Redis
- 可选 Mock Prometheus

即：

> 逻辑上完整分层，代码上模块化，部署上先收敛。

这样既能保留长期扩展空间，也能降低首期运维和联调成本。

### 2.2 契约先行

多人协作时，最重要的不是业务代码数量，而是稳定契约。

所有跨模块通信必须优先定义：

- HTTP OpenAPI
- SSE 事件 Schema
- MCP Tool 输入输出 Schema
- 核心领域对象 JSON Schema
- 数据库字段和状态枚举
- 错误码
- 权限要求
- 幂等规则
- 超时规则

任何模块的真实实现都只能在契约内部变化。

### 2.3 Port / Adapter 隔离

核心业务不直接依赖 Grafana SDK、Prometheus HTTP API、具体 LLM SDK 或某个数据库驱动。

业务代码只依赖抽象 Port，例如：

- `MetricCatalogPort`
- `QueryEnginePort`
- `LLMPort`
- `DashboardReadPort`
- `DashboardWritePort`
- `KnowledgeSearchPort`
- `SessionRepository`
- `TaskEventPublisher`

真实实现和 Mock 都作为 Adapter 注入。

### 2.4 Mock 只能替换 Adapter

Mock 不允许散落在业务代码中，不允许出现大量：

```text
if mockMode:
    return hardcodedResult
```

Mock 必须以 Adapter 的方式存在：

```text
MetricCatalogPort
├── PrometheusMetricCatalogAdapter
└── MockMetricCatalogAdapter
```

业务层不应知道当前运行的是 Mock 还是真实实现。

### 2.5 只读和写操作分离

所有 Grafana 只读操作与写操作必须通过不同接口暴露。

例如：

```text
GrafanaReadPort
GrafanaWritePort
```

写操作必须：

1. 先生成草稿。
2. 生成变更摘要或 Diff。
3. 生成保存意图。
4. 等待用户确认。
5. 再执行写入。
6. 记录审计和结果。

### 2.6 会话是一级对象

系统不以“聊天记录”作为唯一持久化对象。

一个完整分析会话应包括：

- 用户消息
- AI 可展示回复
- 当前 Grafana 上下文
- 任务计划摘要
- 工具调用
- 指标候选
- PromQL 生成版本
- PromQL 用户修改版本
- 图表草稿
- 图表执行结果
- 用户备注
- Dashboard 保存意图
- 审批记录
- 写入结果
- 分享和 Fork 信息

### 2.7 图表草稿和 Grafana Panel 分离

工作台中的临时图表不是 Grafana Panel。

必须区分：

```text
ChartDraft
  ↓ 转换
PanelDraft
  ↓ 用户确认
Grafana Panel
```

这样可以避免业务模型与具体 Grafana Panel JSON 版本强绑定。

### 2.8 任务状态必须持久化

AI 分析任务不是一次性 HTTP 请求，应当使用持久化任务模型。

任务可能处于：

- 正在构建上下文
- 正在规划
- 正在调用工具
- 等待用户补充
- 等待用户确认
- 完成
- 失败
- 取消

浏览器刷新、连接中断或服务重启后，系统应能恢复任务状态，而不是完全依赖内存。

### 2.9 不保存模型私有推理过程

系统保存的是可展示、可审计、可复现的信息：

- 任务计划摘要
- 工具调用
- 查询版本
- 验证结果
- 风险说明
- 决策依据摘要

不保存模型的私有思维链或不可解释的内部推理文本。

---

## 3. 目标系统边界

### 3.1 全周期需要支持的能力

骨架应能够承载以下能力，即使部分能力早期仅提供 Mock：

- 自然语言创建指标分析任务
- Prometheus 指标检索
- 标签检索
- PromQL 生成
- PromQL 校验和执行
- 自动修正一次
- 多图表工作区
- 图表新增、替换、编辑、关闭、Pin
- 会话保存、恢复、搜索
- 会话分享、Fork
- 模板执行
- 保存图表到 Dashboard
- 权限确认
- Dashboard 版本冲突处理
- Candidate Playbook
- 参数化 Playbook 执行
- Grafana Alert Webhook
- Knowledge 检索
- Skill Registry
- 模型路由和降级
- 质量评测
- 审计与成本统计

### 3.2 早期不要求真实实现的能力

以下能力可以先有接口、对象、Mock 和占位流程：

- 自动生成 Candidate Playbook
- 参数化 Playbook 执行
- Alert Webhook 完整处理
- Loki / Tempo / SQL 数据源
- 图表截图和报告生成
- 模型自动路由
- Dashboard 回滚
- 复杂知识库
- 模板智能推荐
- 历史会话主动推荐
- 自动 RCA

### 3.3 明确不进入骨架核心的能力

以下能力不应成为首个骨架版本的核心依赖：

- 自动修复线上系统
- 自动删除 Dashboard 或 Panel
- 无确认覆盖现有 Dashboard
- 全量日志和 Trace 上传给外部模型
- 完整 Grafana Panel Editor
- 多人实时协作编辑
- 多租户计费系统
- License 管理

---

## 4. 推荐技术栈

以下技术栈不是绝对限制，但骨架应围绕统一语言和生成工具设计。

### 4.1 Grafana Plugin Frontend

- React
- TypeScript
- Grafana UI
- Grafana Runtime
- TanStack Query 或等效请求状态管理
- Zustand / Redux Toolkit，可选
- SSE 客户端

### 4.2 Grafana Plugin Backend

- Go
- Grafana Plugin SDK
- Resource Handler
- OpenTelemetry
- OpenAPI 生成客户端

### 4.3 AI Core

建议选择团队最熟悉的一种：

- Python + FastAPI + Pydantic
- 或 Go + Gin/Fiber

若 AI 和数据处理人员主要使用 Python，推荐 AI Core 使用 Python。

### 4.4 MCP Host

- Python 或 Go
- 每个工具独立模块
- 统一 Tool Registry
- JSON Schema 输入输出
- 支持进程内调用和远程调用两种模式

### 4.5 数据层

- PostgreSQL：核心持久化
- Redis：任务锁、短期缓存、事件缓冲，可选
- Vector DB：后期启用
- Object Store：截图、预览、报告等，后期启用

### 4.6 契约与代码生成

- OpenAPI 3.1
- JSON Schema
- TypeScript Client Generator
- Go Client Generator
- Python Pydantic Model Generator

---

## 5. Monorepo 目录结构

推荐仓库结构如下：

```text
grafana-ai-workbench/
├── apps/
│   ├── grafana-plugin/
│   │   ├── frontend/
│   │   │   ├── src/
│   │   │   ├── public/
│   │   │   ├── package.json
│   │   │   └── tsconfig.json
│   │   ├── backend/
│   │   │   ├── cmd/
│   │   │   ├── internal/
│   │   │   ├── go.mod
│   │   │   └── Magefile.go
│   │   ├── src/
│   │   ├── plugin.json
│   │   └── README.md
│   │
│   └── admin-console/
│       ├── frontend/
│       └── README.md
│
├── services/
│   ├── ai-core/
│   │   ├── src/
│   │   ├── tests/
│   │   ├── migrations/
│   │   ├── pyproject.toml
│   │   └── Dockerfile
│   │
│   ├── alert-receiver/
│   │   ├── src/
│   │   └── Dockerfile
│   │
│   └── mcp-host/
│       ├── src/
│       │   ├── runtime/
│       │   ├── tools/
│       │   │   ├── prometheus/
│       │   │   ├── grafana/
│       │   │   ├── knowledge/
│       │   │   ├── skills/
│       │   │   ├── playbooks/
│       │   │   └── render/
│       │   └── registry/
│       ├── tests/
│       ├── pyproject.toml
│       └── Dockerfile
│
├── contracts/
│   ├── openapi/
│   │   ├── plugin-ai-core.yaml
│   │   ├── ai-core-public.yaml
│   │   └── alert-receiver.yaml
│   ├── schemas/
│   │   ├── session.schema.json
│   │   ├── task.schema.json
│   │   ├── chart.schema.json
│   │   ├── approval.schema.json
│   │   ├── playbook.schema.json
│   │   └── alert.schema.json
│   ├── tools/
│   │   ├── prometheus/
│   │   ├── grafana/
│   │   ├── knowledge/
│   │   ├── skills/
│   │   └── playbooks/
│   ├── events/
│   │   └── task-events.schema.json
│   └── errors/
│       └── error-codes.yaml
│
├── packages/
│   ├── generated-clients/
│   │   ├── typescript/
│   │   ├── go/
│   │   └── python/
│   ├── domain-types/
│   ├── observability/
│   ├── auth-context/
│   ├── testkit/
│   └── fixtures/
│
├── migrations/
│   ├── 0001_sessions.sql
│   ├── 0002_tasks.sql
│   ├── 0003_charts.sql
│   ├── 0004_approvals.sql
│   ├── 0005_templates.sql
│   ├── 0006_playbooks.sql
│   └── 0007_alerts.sql
│
├── tests/
│   ├── contract/
│   ├── integration/
│   ├── e2e/
│   ├── golden-queries/
│   └── load/
│
├── deploy/
│   ├── docker-compose/
│   ├── grafana/
│   ├── kubernetes/
│   └── helm/
│
├── docs/
│   ├── architecture/
│   ├── api/
│   ├── development/
│   ├── runbooks/
│   └── adr/
│
├── scripts/
│   ├── generate-clients.sh
│   ├── validate-contracts.sh
│   ├── seed-mock-data.sh
│   └── run-e2e.sh
│
├── docker-compose.yml
├── Makefile
├── README.md
└── CODEOWNERS
```

---

## 6. 初期运行拓扑

### 6.1 本地开发拓扑

```text
Browser
  │
  ▼
Grafana
  ├── App Plugin Frontend
  └── Plugin Backend
          │
          ▼
       AI Core
          │
          ├── PostgreSQL
          ├── Model Gateway Adapter
          └── MCP Client
                 │
                 ▼
              MCP Host
                 ├── Mock/Real Prometheus Tool
                 ├── Mock/Real Grafana Tool
                 ├── Mock Knowledge Tool
                 ├── Mock Skill Tool
                 └── Mock Playbook Tool
```

### 6.2 推荐 docker-compose 服务

```yaml
services:
  grafana:
  plugin-build:
  ai-core:
  mcp-host:
  postgres:
  redis:
  mock-prometheus:
```

其中：

- `redis` 可在第一版关闭。
- `mock-prometheus` 可以使用固定 API 响应，或直接由 MCP Mock Adapter 返回。
- `plugin-build` 可作为开发容器，也可以直接在宿主机运行。

---

## 7. Grafana Plugin Frontend 设计

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

建议：

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

### 9.2 推荐代码结构

```text
ai-core/src/
├── domain/
│   ├── session/
│   ├── task/
│   ├── chart/
│   ├── query/
│   ├── approval/
│   ├── template/
│   ├── playbook/
│   ├── alert/
│   └── common/
├── application/
│   ├── commands/
│   ├── queries/
│   ├── workflows/
│   ├── policies/
│   ├── services/
│   └── dto/
├── ports/
│   ├── repositories/
│   ├── tools/
│   ├── llm/
│   ├── knowledge/
│   ├── events/
│   ├── clocks/
│   └── ids/
├── adapters/
│   ├── inbound/http/
│   ├── outbound/postgres/
│   ├── outbound/mcp/
│   ├── outbound/llm/
│   ├── outbound/events/
│   └── mock/
├── skills/
│   ├── generate_promql/
│   ├── edit_chart/
│   ├── explain_chart/
│   ├── explain_alert/
│   └── candidate_playbook/
├── bootstrap/
│   ├── container.py
│   └── settings.py
└── main.py
```

### 9.3 核心应用用例

建议避免实现一个巨大的 `AgentService`。

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

建议定义统一步骤接口：

```python
class WorkflowStep(Protocol):
    name: str

    async def can_run(self, context: WorkflowContext) -> bool:
        ...

    async def run(self, context: WorkflowContext) -> StepResult:
        ...
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

## 10. MCP Host 设计

### 10.1 定位

MCP Host 是可执行能力边界。

AI Core 不直接访问 Prometheus、Grafana Dashboard API 或知识库，而是调用结构化 Tool。

### 10.2 目录结构

```text
mcp-host/src/
├── runtime/
│   ├── tool_context.py
│   ├── tool_result.py
│   ├── permissions.py
│   ├── timeout.py
│   └── audit.py
├── registry/
│   ├── registry.py
│   └── discovery.py
├── tools/
│   ├── prometheus/
│   │   ├── search_metrics.py
│   │   ├── get_metric_labels.py
│   │   ├── validate_query.py
│   │   └── execute_query.py
│   ├── grafana/
│   │   ├── search_dashboards.py
│   │   ├── get_dashboard.py
│   │   ├── get_panel.py
│   │   ├── get_alert_rule.py
│   │   ├── prepare_panel_patch.py
│   │   ├── apply_panel_patch.py
│   │   └── rollback_dashboard.py
│   ├── knowledge/
│   │   ├── search.py
│   │   └── get_document.py
│   ├── skills/
│   │   ├── list_skills.py
│   │   └── load_skill.py
│   ├── playbooks/
│   │   ├── list_playbooks.py
│   │   ├── get_playbook.py
│   │   └── execute_playbook.py
│   └── render/
│       ├── render_chart_preview.py
│       └── render_dashboard_preview.py
└── adapters/
    ├── real/
    └── mock/
```

### 10.3 MCP Tool 元信息

每个工具应定义：

```json
{
  "name": "prometheus.validate_query",
  "version": "v1",
  "description": "Validate a PromQL expression against a datasource.",
  "riskLevel": "read_only",
  "requiredPermissions": ["datasources:query"],
  "timeoutMs": 5000,
  "idempotent": true,
  "inputSchema": {},
  "outputSchema": {}
}
```

### 10.4 风险等级

建议：

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

---

## 11. 核心 Port 接口

### 11.1 MetricCatalogPort

```python
class MetricCatalogPort(Protocol):
    async def search_metrics(
        self,
        context: RequestContext,
        request: SearchMetricsRequest,
    ) -> SearchMetricsResult:
        ...

    async def get_metric_labels(
        self,
        context: RequestContext,
        request: GetMetricLabelsRequest,
    ) -> MetricLabelsResult:
        ...
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

### 11.2 QueryEnginePort

```python
class QueryEnginePort(Protocol):
    async def validate(
        self,
        context: RequestContext,
        request: ValidateQueryRequest,
    ) -> QueryValidationResult:
        ...

    async def execute(
        self,
        context: RequestContext,
        request: ExecuteQueryRequest,
    ) -> QueryExecutionResult:
        ...
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

### 11.3 LLMPort

```python
class LLMPort(Protocol):
    async def complete_structured(
        self,
        context: ModelRequestContext,
        request: StructuredCompletionRequest,
    ) -> StructuredCompletionResult:
        ...

    async def stream_text(
        self,
        context: ModelRequestContext,
        request: TextCompletionRequest,
    ) -> AsyncIterator[TextDelta]:
        ...
```

LLMPort 不应直接暴露某家模型供应商的对象。

### 11.4 KnowledgeSearchPort

```python
class KnowledgeSearchPort(Protocol):
    async def search(
        self,
        context: RequestContext,
        request: KnowledgeSearchRequest,
    ) -> KnowledgeSearchResult:
        ...
```

### 11.5 DashboardReadPort

```python
class DashboardReadPort(Protocol):
    async def search_dashboards(...)
    async def get_dashboard(...)
    async def get_panel(...)
    async def get_alert_rule(...)
```

### 11.6 DashboardWritePort

```python
class DashboardWritePort(Protocol):
    async def prepare_add_panel(
        self,
        context: RequestContext,
        request: PreparePanelSaveRequest,
    ) -> DashboardSaveIntent:
        ...

    async def execute_add_panel(
        self,
        context: RequestContext,
        intent: DashboardSaveIntent,
        approval: ApprovalEvidence,
    ) -> DashboardSaveResult:
        ...
```

### 11.7 Repository Ports

```python
class SessionRepository(Protocol):
    async def create(...)
    async def get(...)
    async def search(...)
    async def update(...)
```

```python
class TaskRepository(Protocol):
    async def create(...)
    async def get(...)
    async def update_status(...)
```

```python
class ChartRepository(Protocol):
    async def create_draft(...)
    async def update_draft(...)
    async def append_revision(...)
    async def append_execution(...)
```

```python
class ApprovalRepository(Protocol):
    async def create(...)
    async def approve(...)
    async def reject(...)
```

### 11.8 EventPublisher

```python
class TaskEventPublisher(Protocol):
    async def publish(self, event: TaskEvent) -> None:
        ...

    async def replay(
        self,
        task_id: str,
        after_sequence: int,
    ) -> AsyncIterator[TaskEvent]:
        ...
```

---

## 12. PromQL 生成 Skill

PromQL 生成建议作为 AI Core 内部 Skill，而不是数据访问工具。

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

### 13.1 ChartDraft

工作台中的临时图表。

建议字段：

```json
{
  "id": "chart_123",
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

`source` 建议枚举：

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
  ]
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
  "chartId": "chart_123",
  "panelDraftId": "panel_draft_123",
  "targetDashboardUid": "checkout-overview",
  "targetFolderUid": "production",
  "expectedDashboardVersion": 18,
  "panelTitle": "Checkout Error Rate",
  "requiredPermission": "dashboards:write",
  "riskLevel": "write_requires_approval",
  "status": "waiting_approval",
  "expiresAt": "..."
}
```

### 14.4 Approval

```json
{
  "id": "approval_123",
  "intentId": "save_intent_123",
  "type": "dashboard_add_panel",
  "requestedBy": "user_123",
  "status": "pending",
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
CREATED
  ↓
CONTEXT_BUILDING
  ↓
PLANNING
  ↓
RUNNING_TOOLS
  ↓
VALIDATING
  ├── WAITING_USER_INPUT
  ├── WAITING_APPROVAL
  ├── COMPLETED
  ├── FAILED
  └── CANCELLED
```

### 15.2 状态说明

|状态|说明|
|---|---|
|CREATED|任务已创建但尚未执行|
|CONTEXT_BUILDING|正在构建 Grafana、会话、知识和历史上下文|
|PLANNING|正在生成可展示计划|
|RUNNING_TOOLS|正在调用指标、查询、Grafana 或知识工具|
|VALIDATING|正在校验 PromQL、图表和权限|
|WAITING_USER_INPUT|信息不足，需要用户选择或补充|
|WAITING_APPROVAL|只读阶段完成，等待写操作确认|
|COMPLETED|任务成功完成|
|FAILED|任务无法恢复地失败|
|CANCELLED|用户或系统取消任务|

### 15.3 图表状态

```text
DRAFT
  ↓
VALIDATING
  ├── READY
  ├── NO_DATA
  ├── ERROR
  └── TIMEOUT
       ↓
SAVED_TO_DASHBOARD
```

### 15.4 工具调用状态

```text
PENDING
RUNNING
SUCCESS
FAILED
TIMEOUT
CANCELLED
```

---

## 16. 流式事件协议

### 16.1 传输方式

建议使用 SSE。

原因：

- 服务端单向推送足够
- 浏览器支持良好
- 实现成本低
- 可基于 sequence 恢复
- 比 WebSocket 更适合初期任务流

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
    "toolName": "prometheus.search_metrics",
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

## 17. HTTP API 草案

### 17.1 Session

```text
POST   /v1/sessions
GET    /v1/sessions
GET    /v1/sessions/{sessionId}
PATCH  /v1/sessions/{sessionId}
POST   /v1/sessions/{sessionId}/share
POST   /v1/sessions/{sessionId}/fork
GET    /v1/sessions/{sessionId}/turns
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

写操作建议支持：

```text
Idempotency-Key
X-Request-ID
X-Trace-ID
```

更新操作使用：

```text
If-Match
ETag
```

或请求体显式携带：

```json
{
  "expectedVersion": 3
}
```

---

## 18. 错误模型

统一错误响应：

```json
{
  "error": {
    "code": "QUERY_NO_DATA",
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
INVALID_REQUEST
UNAUTHORIZED
FORBIDDEN
TENANT_MISMATCH

SESSION_NOT_FOUND
TASK_NOT_FOUND
CHART_NOT_FOUND

INTENT_AMBIGUOUS
METRIC_NOT_FOUND
LABEL_NOT_FOUND

PROMQL_SYNTAX_ERROR
QUERY_NO_DATA
QUERY_TIMEOUT
QUERY_TOO_EXPENSIVE
QUERY_EXECUTION_FAILED

AI_UNAVAILABLE
AI_RATE_LIMITED
AI_OUTPUT_SCHEMA_INVALID

TOOL_NOT_FOUND
TOOL_NOT_SUPPORTED
TOOL_PERMISSION_DENIED
TOOL_TIMEOUT
TOOL_EXECUTION_FAILED

DASHBOARD_WRITE_FORBIDDEN
DASHBOARD_VERSION_CONFLICT
DASHBOARD_SAVE_FAILED

APPROVAL_REQUIRED
APPROVAL_EXPIRED
APPROVAL_REJECTED

PLAYBOOK_NOT_READY
PLAYBOOK_VALIDATION_FAILED
```

---

## 19. 数据库设计

### 19.1 核心表

```text
analysis_sessions
session_turns
session_notes
session_shares
session_forks

analysis_tasks
task_steps
task_events
tool_calls

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

candidate_playbooks
playbook_versions
playbook_runs

alert_events
alert_event_sessions

audit_logs
evaluation_records
```

### 19.2 关键结构化字段

即使大量业务配置使用 JSONB，以下字段必须单独建列：

- tenant_id
- org_id
- user_id
- session_id
- task_id
- chart_id
- datasource_uid
- dashboard_uid
- panel_id
- status
- version
- created_at
- updated_at

### 19.3 analysis_sessions

建议字段：

```text
id
tenant_id
org_id
owner_user_id
title
visibility
source_template_id
source_session_id
datasource_uid
default_time_range_json
tags_json
context_json
status
created_at
updated_at
version
```

### 19.4 analysis_tasks

```text
id
session_id
tenant_id
task_type
status
intent
input_json
plan_summary_json
error_code
created_at
started_at
completed_at
version
```

### 19.5 chart_drafts

```text
id
session_id
task_id
tenant_id
title
query_language
query_expression
visualization_json
time_range_json
datasource_uid
status
current_revision
pinned
created_by
created_at
updated_at
version
```

### 19.6 tool_calls

```text
id
task_id
tenant_id
tool_name
tool_version
risk_level
input_summary_json
output_summary_json
status
duration_ms
error_code
started_at
completed_at
```

敏感原始输出不应全部进入 `tool_calls`。

### 19.7 audit_logs

```text
id
tenant_id
org_id
user_id
action
resource_type
resource_id
decision
request_id
trace_id
details_json
created_at
```

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

LLMPort
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
runtime:
  mode: mock

adapters:
  llm: deterministic_mock
  metric_catalog: mock
  query_engine: mock
  grafana_read: mock
  grafana_write: mock
  knowledge: mock
  playbook: mock
```

也允许混合模式：

```yaml
adapters:
  llm: real
  metric_catalog: real
  query_engine: real
  grafana_read: real
  grafana_write: mock
  knowledge: mock
```

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

开发环境可通过请求头或输入参数触发：

```text
X-Mock-Scenario: dashboard_version_conflict
```

生产环境必须禁用此能力。

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

### 21.1 Template

Template 是人工维护的标准入口。

建议结构：

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
  "sourceSessionId": "session_123",
  "status": "draft",
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
  "playbookId": "playbook_123",
  "version": 3,
  "status": "active",
  "inputSchema": {},
  "steps": [],
  "reviewedBy": "user_123"
}
```

### 21.4 Playbook 执行接口预留

即使当前不实现，仍建议预留：

```text
PlaybookRepository
PlaybookExecutor
PlaybookValidator
PlaybookRunRepository
```

Mock 执行器可以返回固定任务和图表。

---

## 22. Alert Webhook 骨架

### 22.1 Alert Receiver 职责

- 校验 Webhook
- 解析 Grafana Alert Payload
- fingerprint / groupKey 去重
- 创建或更新 AlertEvent
- 触发 AlertAnalysisTask
- 关联 Session
- 生成摘要链接

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

### 22.3 骨架要求

MS1 即使不真实处理告警，也应：

- 存在 AlertEvent Schema
- 存在 `/v1/alerts/grafana`
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
  "traceId": "trace_123"
}
```

### 23.2 权限原则

- 所有数据访问绑定 tenantId / orgId
- 会话搜索默认限制在当前租户
- 分享链接仍要经过权限检查
- Fork 后继承内容，不继承写权限
- AI Core 不持有全局管理员权限
- Dashboard 写入使用当前用户身份或受控代理
- ToolCall 必须记录权限判定

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

建议定义：

```python
class DataEgressPolicy(Protocol):
    async def evaluate(
        self,
        context: RequestContext,
        payload: ModelPayload,
    ) -> EgressDecision:
        ...
```

输出：

```json
{
  "allowed": true,
  "redactions": [],
  "reason": "policy_allow_minimal_context"
}
```

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

### 26.2 Contract Test

验证：

- OpenAPI
- JSON Schema
- Tool Schema
- SSE Event Schema
- 生成客户端一致性

### 26.3 Adapter Contract Test

同一个 Port 的 Mock 和 Real Adapter 必须满足相同测试集。

例如：

```text
QueryEngineContractTests
  ├── MockQueryEngineAdapter
  └── PrometheusQueryEngineAdapter
```

### 26.4 Integration Test

覆盖：

- AI Core + PostgreSQL
- AI Core + MCP Host
- Plugin Backend + AI Core
- Grafana Write Adapter + 测试 Grafana

### 26.5 E2E

最少覆盖：

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

### 26.6 Golden Query

`tests/golden-queries/` 建议：

```text
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
security-scan
mock-e2e
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
0008_add_chart_reference.sql
```

---

## 28. 多人协作拆分

### 28.1 推荐模块负责人

|模块|主要职责|
|---|---|
|Plugin Frontend|工作台、会话、对话、多图、图表编辑、审批 UI|
|Plugin Backend|Grafana Context、RBAC、Resource Handler、API Proxy|
|AI Core|任务状态机、Workflow、Intent、Context、Policy|
|Prometheus / Metric Catalog|指标同步、指标搜索、标签、查询执行|
|Grafana MCP|Dashboard / Panel 读取、Patch、写入、版本冲突|
|Persistence|Session、Task、Chart、Approval、Audit|
|LLM / Skill|Model Gateway、PromQL Skill、Chart Skill|
|Knowledge / Playbook|Knowledge、Template、Candidate Playbook|
|Alert|Webhook、AlertEvent、告警关联会话|
|Evaluation / Observability|Golden Query、埋点、Trace、成本|

### 28.2 CODEOWNERS

建议：

```text
/apps/grafana-plugin/frontend/ @frontend-team
/apps/grafana-plugin/backend/ @grafana-team
/services/ai-core/ @ai-core-team
/services/mcp-host/src/tools/prometheus/ @observability-team
/services/mcp-host/src/tools/grafana/ @grafana-team
/contracts/ @architecture-team
/migrations/ @backend-team @architecture-team
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

---

## 29. 分阶段替换策略

### 29.1 骨架初版

全部或大部分使用 Mock：

```text
Plugin Frontend: Real
Plugin Backend: Real
AI Core Workflow: Minimal Real
LLM: Mock
Prometheus: Mock
Grafana Read: Mock
Grafana Write: Mock
Persistence: Real
```

### 29.2 MS2

```text
LLM: Real
Prometheus: Real
Metric Catalog: Real
Grafana Read: Real
Grafana Write: Basic Real
Persistence: Real
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

## 30. 骨架代码首版必须跑通的纵向链路

首版骨架至少应实现：

1. `docker compose up` 启动 Grafana、Plugin、AI Core、MCP Host 和 PostgreSQL。
2. 用户进入 Grafana App Plugin 页面。
3. 用户创建新会话。
4. 用户输入：
   `帮我排查 checkout 服务过去 30 分钟错误率升高`
5. Plugin Backend 获取 Grafana 上下文。
6. Plugin Backend 创建 AI Core Task。
7. AI Core 保存 Session 和 Task。
8. AI Core 发布任务状态事件。
9. Mock Metric Catalog 返回候选指标。
10. Mock PromQL Skill 返回三个查询。
11. Mock Query Engine 返回固定执行结果。
12. AI Core 创建三张 ChartDraft。
13. 前端实时展示三张图。
14. 用户手动编辑其中一条 PromQL。
15. 系统创建 QueryRevision。
16. 用户重新执行。
17. 系统创建新的 ChartExecution。
18. 用户刷新页面。
19. 会话、对话、图表、查询和执行状态恢复。
20. 用户点击“保存到 Dashboard”。
21. AI Core 创建 PanelDraft 和 DashboardSaveIntent。
22. 前端展示确认界面。
23. 用户确认。
24. Mock Grafana Write 返回保存成功。
25. 系统记录 Approval、DashboardSaveResult 和 AuditLog。
26. 可以通过 Mock 场景重放无权限和版本冲突。

---

## 31. 骨架验收标准

### 31.1 代码层

- Monorepo 结构建立
- 所有核心服务有 README
- OpenAPI 可校验
- JSON Schema 可校验
- 客户端可自动生成
- 数据库可自动迁移
- Mock Fixture 可加载
- docker-compose 可启动
- CI 可运行

### 31.2 架构层

- Plugin Backend 与 AI Core 解耦
- AI Core 与工具实现解耦
- Mock 与 Real Adapter 可配置切换
- Grafana 读写接口分离
- ChartDraft 与 PanelDraft 分离
- 任务状态持久化
- 事件可重放
- 写操作必须 Approval
- tenantId / orgId 贯穿全链路

### 31.3 产品链路

- 自然语言输入可创建图表
- 多图可同屏
- 图表可编辑
- 会话可恢复
- 保存 Dashboard 有确认
- 错误场景可解释
- AI 不可用时已有内容不丢失

### 31.4 协作层

- 前端可只依赖 Mock 开发
- AI Core 可只依赖 Tool Mock 开发
- MCP 工程师可独立实现 Tool
- 数据库工程师可独立完成 Repository
- 测试可通过固定场景稳定复现
- 模块替换不要求修改调用方

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
```

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

---

## 34. 总结

该骨架代码的核心目标不是尽早堆出最多功能，而是确保整个项目从一开始就具备稳定演进能力。

最终骨架应满足：

- 前端可以不等 AI 实现完成。
- AI Core 可以不等 Prometheus 和 Grafana Tool 完成。
- Prometheus 和 Grafana 工具可以独立实现。
- 会话和持久化可以独立建设。
- Mock 可以支持完整演示和 E2E。
- 后续真实能力可以逐步替换 Mock。
- 替换过程中接口、状态机和领域对象保持稳定。
- 权限、安全、审批、审计和可观测性不会在后期被迫重构。

整个方案可以概括为：

> 接口和状态机先完整，能力实现允许为空；Mock 只能替换 Adapter，不能侵入领域逻辑；会话是分析过程的核心资产，图表草稿和 Grafana Panel 必须分离，所有写操作必须进入确认流。

这将使该项目具备多人并行开发、持续联调、阶段性交付和全周期演进的基础。
