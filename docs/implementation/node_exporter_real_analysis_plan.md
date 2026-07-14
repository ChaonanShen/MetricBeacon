# node_exporter 真实分析与多轮会话执行计划

> status: active
> createdAt: 2026-07-14
> scope: 在已完成的确定性 Mock 闭环上，建立持久化多轮对话，并跑通真实
> Prometheus/node_exporter 与最小 Eino Agent 的端到端演示。
> dependsOn: `code_skeleton_design.md`、`current_codebase_overview.md`、
> `basic_mock_skeleton_execution_plan.md`

## 1. 完成目标

用户在 Grafana 工作台内创建或恢复一个 Session，连续发送多条自然语言消息；系统保存完整
对话和每轮分析任务。Agent 读取一份静态 node_exporter 领域说明，通过真实 MCP 调用真实
Prometheus，生成 CPU 使用率、内存可用率、系统负载三张图和简短分析结论。刷新页面后，消息、
任务和图表均可恢复。

目标链路：

```text
Browser -> Grafana Plugin -> Plugin Backend -> AI Core
        -> minimal Eino Agent -> assistant-mcp -> Prometheus -> node_exporter
        <- durable Session/Message/Task/Chart/TaskEvent + SSE replay
```

确定性 Mock 链路必须继续保留，作为不依赖模型和实时指标的稳定回归基线。

## 2. 范围与非目标

本计划包含：

- Session 内的持久化多轮对话、恢复和当前轮 SSE 展示。
- Session、Message、Task、Chart 之间可稳定关联的契约和存储调整。
- 真实 Prometheus HTTP Adapter、Prometheus 和 node_exporter 本地 Compose 拓扑。
- 一份只读的 node_exporter 静态 Agent Profile 文档。
- 位于 AI Core outbound adapter 内的最小 Eino `ChatModelAgent` 集成。
- Mock 与 local-real 两条 E2E 验收路径。

本计划不包含：

- Knowledge MCP、向量检索、Skill、Playbook、模板和会话分享/Fork。
- PromQL/图表编辑、Dashboard 写入、审批和 delegation grant。
- Alertmanager Webhook、告警去重、自动建告警或 Agent 自动处置。
- Multi-Agent、Checkpoint/Interrupt、长期 Memory、Skill Middleware 或自定义无限
  ReAct 循环。

阈值解释可以写入 Agent Profile，供分析结论使用；真正的告警系统另立计划。

## 3. 已锁定的实施决策

### 3.1 多轮模型

不新增复杂的 Conversation/Turn Aggregate：`Session` 是会话，`Task` 是一轮分析，
`Message` 是用户或助手消息，`Chart` 属于产生它的 Task。每次用户发送消息仍创建一个
Task，并在同一事务中保存 User Message、Task 和 `task.created`。

Message 增加稳定的 `taskId` 关联；Assistant Message 也关联其产生的 Task。前端以 Message
作为聊天历史事实来源，以 TaskEvent 作为执行过程事实来源。第一版限制每个 Session 同时只
运行一个非终态 Task，避免并发轮次和上下文歧义。

### 3.2 Agent 与上下文

保留现有 `AgentRuntime` Port；确定性 Mock Agent 不变。真实实现使用最小 Eino Adapter，
仅配置一个 ChatModel、三个只读工具和受限的工具调用轮数。Eino 类型不得进入 Domain、
Application、HTTP 或 MCP 契约。

每轮 Agent 输入仅包含：静态 node_exporter Profile、最近有限数量的持久化消息、当前用户
消息、数据源和时间范围。不得把完整时间序列、模型私有推理、Token 或 Grafana 身份发送给
模型。具体模型供应商和凭证不是本计划的默认决策；开始该 Gate 前需要提供可用的配置，Secret
只通过运行环境注入。

### 3.3 静态领域说明

新增 `data/agent-knowledge/node_exporter.md`，将其视为 Agent Profile 配置而非 Knowledge
产品能力。文档应说明三个可分析指标、固定 PromQL、标签/时间范围约束、解释口径、无数据
处理、输出格式和禁止能力。由 Eino Adapter 的只读配置加载；本轮不注册 `knowledge.*` MCP
工具，也不建设检索或索引。

### 3.4 真实指标链路

`assistant-mcp` 继续通过 `PrometheusPort` 访问指标。Mock 和 Real HTTP Adapter 都实现该
Port，且只允许 Bootstrap 根据配置选择；业务层不得感知 adapter 类型。Real 模式不能保留
`mock-prometheus` 的硬编码校验，仍需保留只读、时间范围、step、序列数量和 node_exporter
指标白名单约束。

## 4. 执行阶段

### P1：多轮会话契约和持久化

- 先更新 OpenAPI、JSON Schema、SSE/Event DTO 和生成客户端，再改 Domain、Port 与 SQLite
  migration。
- 增加会话 Message/Task 历史读取接口；`CreateTask` 保持原子追加 User Message 与 Task。
- 为 Message 建立 `taskId` 关联，并在 Assistant Message、Task、Chart 间保证可恢复关系。
- 限制一个 Session 的并行执行；失败轮保留用户消息和失败状态。

验收：相同幂等请求不重复创建 Message；会话隔离、排序、重放和数据库迁移测试通过。

### P2：前端多轮工作台

- 前端状态改为 Session 级：消息列表、按 Task 分组的轮次/状态/图表、当前活动 Task。
- 新消息不清空历史；只有当前 Task 建立 SSE 订阅，完成后保留历史。
- 以 `sessionId` 为主要 URL 恢复键；刷新后读取消息和 Task 列表，恢复历史与进行中的任务。
- 历史图表先通过 TaskEvent 重放恢复；在轮次变多前不引入额外的 Timeline 聚合 API。

验收：连续三轮提交、刷新恢复、断线重连、失败轮展示和前端 reducer/SSE 单元测试通过。

### P3：真实 Prometheus 与 node_exporter

- 实现 Prometheus HTTP Adapter 和与 Mock Adapter 对齐的 Contract Test。
- 增加 Prometheus/node_exporter Compose 服务、固定镜像版本和 datasource 配置；保留现有
  Mock Compose 作为默认稳定测试。
- 为 CPU 使用率、内存可用率、`node_load1` 建立真实 PromQL 和本地 real-mode 验证。
- local-real 测试只断言数据存在、时间范围合理和链路调用正确，不固定瞬时指标数值。

验收：Grafana -> Plugin -> AI Core -> MCP -> Prometheus -> node_exporter 返回三组非空真实序列。

### P4：静态 Profile 与最小 Eino Agent

- 编写并审阅 node_exporter Agent Profile；补充其读取与长度限制测试。
- 在 `outbound/agent/eino` 实现最小 Adapter，把 Eino 文本与工具事件转换为既有
  `AgentEventSink`，保持 TaskEvent、ToolCall 和 SSE 的持久化语义。
- 仅暴露指标搜索、标签读取和 PromQL 查询三项只读能力，并设置最大轮数、超时、查询范围与
  输出大小限制。
- 保留 DeterministicMockAgentRuntime，模型不可用时显式失败而不是悄悄回退为伪结果。

验收：Agent 读取 Profile 后能在真实指标环境完成三图分析；工具调用可审计且没有越界查询。

### P5：端到端验收和演进记录

- 增加 local-real Agent E2E：多轮输入、真实三图、持久化、刷新恢复、模型不可用与 MCP/查询
  失败恢复。
- 保持 `make e2e-mock` 通过；新增独立的 real-mode 验收入口，不让实时数据测试替代 Mock
  回归。
- 每个完成 Gate 同步更新本计划、进度记录、当前代码概览、代码树、README 和必要 ADR；本
  计划完成时标为 `completed`。

验收：Mock E2E、local-real 指标 E2E、真实 Agent E2E 和相应的 Contract/单元测试均通过。

## 5. 完成标准

1. 一个 Session 可以连续保存并展示至少三轮用户/助手消息及其独立 Task。
2. 刷新后，已完成的消息和图表恢复；进行中的 Task 从持久化 sequence 继续消费 SSE。
3. 真实 Prometheus/node_exporter 链路产出 CPU、内存和负载三张非空图。
4. Eino Agent 使用静态 Profile 和受限只读工具完成分析，不暴露私有推理或敏感上下文。
5. Mock 链路、Real Prometheus 链路和真实 Agent 链路分别有可重复的验收入口。
6. `current_codebase_overview.md`、`current_code_tree.md`、本计划及进度记录与实际代码一致。

## 6. 停止条件

以下情况必须在实现前确认或记录 ADR，而不是自行扩展：模型供应商/凭证与出域策略、改变
Session/Task 数据所有权、允许并发 Task、引入知识库/MCP namespace、Grafana 写入、或开始
真正的告警接收和自动化处置。
