# PostgreSQL 阻塞事件最小处置闭环执行计划

> Status: superseded-before-implementation
> Last reviewed: 2026-07-15
> Superseded: 2026-07-16 by [`order_service_incident_remediation_execution_plan.md`](order_service_incident_remediation_execution_plan.md)

本计划未进入实现。其通用 Incident/Approval 边界由可控订单业务系统计划继承；PostgreSQL 专用 Adapter 和处置动作保留为后续真实外部系统验证候选。

## 1. 目标与架构边界

本阶段交付一条完整但受限的闭环：

```text
Grafana Alert
→ 创建组织级事件会话
→ 确定性匹配 Playbook
→ 加载静态 Knowledge / Skill
→ Prometheus 复核现象
→ PostgreSQL 实时诊断阻塞树
→ 准备终止会话 Intent
→ Grafana Admin 人工批准
→ 重新校验目标并终止
→ 验证数据库、业务探针和指标恢复
```

模块边界固定如下：

| 模块 | 负责 | 明确不负责 |
|---|---|---|
| Plugin Frontend | 展示组织事件会话、证据、步骤、审批和结果 | 不直连 AI Core、MCP、Prometheus、PostgreSQL |
| Plugin Backend | Grafana 身份映射、组织隔离、Admin 审批权限检查、HTTP/SSE 代理 | 不保存 Incident、Approval、Playbook 状态，不执行数据库操作 |
| AI Core | Session、Task、TaskEvent、RemediationIntent、Approval、Checkpoint 和暂停/恢复 | 不包含 `orders`、`order-worker`、故障用户等业务语义，不解释 Playbook YAML |
| assistant-mcp | 静态资产读取、Alert 映射、Playbook 解释执行、受控观测和 PostgreSQL Adapter | 不拥有 Session/Task/Approval，不与 AI Core 共享数据库 |
| 部署资产 | 业务拓扑、监控口径、Skill、Playbook、告警映射、允许处置范围 | 不授予权限，不绕过 Adapter 安全校验 |
| 技术 Adapter | 理解 Prometheus、PostgreSQL 等协议和安全能力 | 不硬编码具体业务系统语义 |

平台核心只认识通用的观测视图、证据、步骤、意图、审批和能力调用。同一种 PostgreSQL/Prometheus
接入新增业务系统时，只增加部署资产和配置；新增协议时才增加 Adapter。未来源码探索 MCP 通过能力
Port 扩展，本阶段不实现多 MCP 发现或路由。

## 2. 契约、资产与状态模型

### 2.1 静态 Knowledge / Skill / Playbook

assistant-mcp 只实现文件型只读资产，不建设知识库产品：

- 一个 Knowledge Markdown 文档，包含 node_exporter、PostgreSQL、订单演示系统的稳定事实、监控口径、角色含义、恢复标准和数据脱敏规则。
- 一个 PostgreSQL 锁诊断 Skill Markdown，只描述 Agent 如何理解状态、时间字段、阻塞关系、cancel/terminate 区别和验证思路；不得决定目标是否允许处置。
- 一个 YAML Playbook，固定引用 Knowledge 和 Skill，声明顺序步骤、参数、超时、审批暂停点和失败分支。
- 一个静态 Alert 映射：`tenant + alertName + 稳定 labels → playbookId + bounded parameters`。不做语义召回；未匹配返回 `playbook_not_mapped`，不猜测执行。
- 文件加载时计算 SHA-256 digest。Task 固定本轮 digest；等待审批期间资产变化则终止旧流程，不能用新内容继续执行。
- 不实现 CRUD、搜索、向量索引、Folder Review、private/shared 或数据库 CatalogStore。

Knowledge 只存事实，Skill 只存推理指导，Playbook 只存可执行顺序和分支；权限与允许目标放在
Adapter policy 配置中，避免三者重复或让自然语言承担安全判断。

### 2.2 跨进程接口

先在 `contracts/` 定义并生成以下类型：

- assistant-mcp：`knowledge.get_document`、`skills.get_skill`、`playbook.start_run`、`playbook.resume_run`。
- AI Core 内部告警入口：`POST /internal/v1/alerts/grafana`。使用按来源配置的服务凭证映射 tenant/org；只接受 `firing`，重复 fingerprint 返回已有 Task，`resolved` 不创建新 Task。
- Plugin Resource API：`GET /approvals/{approvalId}`、`POST /approvals/{approvalId}/approve`、`POST /approvals/{approvalId}/reject`。

`playbook.start_run` 返回步骤结果、脱敏证据、Agent 上下文、可持久化 checkpoint，以及可选
RemediationIntent；`resume_run` 在批准时执行，在服务恢复时只能进行只读 reconcile。

新增错误码至少包括：`alert_source_unauthorized`、`playbook_not_mapped`、`approval_required`、
`approval_expired`、`approval_rejected`、`asset_digest_mismatch`、`target_precondition_failed` 和
`recovery_not_verified`。

### 2.3 AI Core 领域模型

- Task 增加 `kind = metric_analysis | incident_remediation`，计划改为 `QueryPlan | IncidentPlan` 的互斥联合；不复制一套 IncidentTask/Event 基础设施。
- `IncidentPlan` 仅保存来源引用、Playbook ID/digest 和有界键值参数，不含 PostgreSQL 专用字段。
- 正常状态路径为 `created → planning → running_tools → waiting_approval → executing → validating → completed`；无需处置时从 `running_tools` 进入 `validating`，拒绝时从 `waiting_approval` 进入 `cancelled`，任一非终态可进入 `failed`。
- 新增通用 `RemediationIntent`、`Approval`、`TaskCheckpoint` 持久化；Approval 状态为 `pending | approved | rejected | expired`。
- Session 增加 `visibility = private | org_incident`，Message 增加 `trigger` 角色。原 private Session 规则保持不变；同 org 用户可读 `org_incident`，只有 Grafana Admin 获得 `incidents:approve` 权限。
- 等待审批的 Task 重启后保持可恢复；审批到期由 AI Core 周期性收口。执行中断时只做只读 reconcile，绝不自动重放写操作。
- 增加资产加载、Playbook 步骤、Intent 准备、审批请求/结果、动作结果和恢复验证等通用 TaskEvent；仍然先持久化再通知 SSE。
- 不增加 AuditEvent、审计存储、查询 API、保留策略或审计 UI；TaskEvent、Intent 和 Approval 仅作为闭环运行状态。

## 3. 执行、安全与真实拓扑

### 3.1 Playbook 固定步骤

1. 校验静态 Alert 映射并固定资产 digest。
2. 查询注册的 PostgreSQL 观测视图，复核持续 idle transaction 和 Lock waiter。
3. 通过只读 PostgreSQL Port 查询 `pg_stat_activity`、`pg_blocking_pids`，构建阻塞树并识别 root blocker。
4. 校验目标属于配置的 fault scope，形成由 `pid + backend_start + xact_start + database + role + application_name` 构成的会话指纹。
5. 将 Knowledge、Skill 和脱敏证据摘要交给 Agent。Agent 只生成业务解释；目标、动作和 Intent digest 由确定性代码产生。
6. 持久化 Intent、Checkpoint、Approval 和事件后进入 `waiting_approval`。
7. Admin 批准后，AI Core 生成最长 60 秒、绑定 tenant/org/task/approval/intent/target/action 的签名 ApprovalEvidence；不落盘、不记录、不进入模型。
8. assistant-mcp 重新读取实时会话并逐字段核对指纹、root blocker、允许角色和当前 runId；任一变化返回 `target_precondition_failed`，不执行。
9. 调用受限 PostgreSQL `SECURITY DEFINER` 函数终止目标会话。
10. 验证目标事务消失、Lock waiter 消失、业务成功探针恢复，并等待后续 scrape 使 Prometheus 条件恢复；否则以 `recovery_not_verified` 失败。
11. `already_resolved` 作为成功无操作结果；审批拒绝、过期、资产变化、目标变化或执行结果不确定时，不尝试其他 PID。

Agent 不接收 PID、SQL 文本、连接串、数据库账号、原始时序、ApprovalEvidence 或用户身份；它也不能
选择任意 PromQL、SQL、工具或处置目标。

### 3.2 assistant-mcp 内部能力

Playbook Engine 只调用启动时注册并通过 Schema 校验的能力：

- Prometheus：按部署控制的本地注册表查询 PostgreSQL idle transaction、Lock waiter 和业务成功探针；用户、模型和 Playbook 都不能提交原始 PromQL。
- PostgreSQL 只读 Port：`InspectBlockingTree`、`GetSession`、`VerifyRecovery`。
- PostgreSQL 写 Port：`TerminateSession`，只接受已准备 Intent、会话指纹和 ApprovalEvidence。
- YAML 不支持脚本、嵌套 Playbook、动态工具名或任意参数透传。

真实环境使用独立账号：exporter/诊断账号只获得监控所需权限，remediation 账号只能执行受限函数。
函数内部再次检查数据库、角色、`client backend`、`idle in transaction`、application name、时间指纹
和实际阻塞关系，并禁止维护角色、复制进程、后台进程和当前连接。`(tenant, instanceRef)` 到 DSN、
允许角色及当前 fault runId 的映射来自环境配置，不进入资产或模型。

### 3.3 确定性演示拓扑

- fault-injector 使用 `mtb-fault-injector/<runId>` 开启事务、更新 `fault_marker` 后保持连接。
- order-worker 更新同一行的独立 heartbeat 字段并形成稳定 Lock waiter。
- postgres_exporter 暴露 activity 指标；业务 worker 暴露低基数成功时间指标。
- Grafana Alert 使用固定名称 `PostgresIdleTransactionBlocking`，稳定 labels 仅含 `instance_ref`、`database_ref` 等；禁止 PID、SQL、用户名、application name 和 runId。
- 演示 scrape 间隔为 5 秒，告警 `for` 为 15 秒；故障与恢复均至少跨越两个 scrape。
- Alert Receiver 是 AI Core 的服务到服务入口，不经过浏览器；用户查看和审批仍只能通过 Plugin Resource API。

## 4. 实施顺序与文档同步

1. 先记录本切片 ADR，固定业务知识隔离、静态资产所有权、组织事件会话和人工批准的 PostgreSQL 写边界；为 ADR-022 增加 `org_incident` 例外，并把产品基线的告警边界明确为“无人确认部分只读，人工确认后才可执行受限动作”。
2. 先定义 Task/Session/Approval/TaskEvent、Alert HTTP、MCP Tool、错误码和静态资产 Schema，再生成 Go/TypeScript 类型。
3. 实现静态资产 Adapter、Playbook Engine、能力注册表和全部 Mock Adapter；将现有 node_exporter 业务说明迁入 assistant-mcp 拥有的单一 Knowledge 文档，AI Core 只保留通用 Agent policy，现有 CPU/内存/负载闭环不得回归。
4. 实现 AI Core 通用 Incident workflow、SQLite migration、审批恢复、组织事件访问控制，以及 Plugin Backend/Workbench 的事件和审批展示。
5. 实现 PostgreSQL/Prometheus Real Adapter、受限数据库函数、Grafana Alert provisioning 和故障注入拓扑。
6. 每个可独立验证的切片单独提交，并在同一提交更新执行计划、进度证据、当前代码快照、代码树、运行手册和 ADR 索引。

## 5. 测试与验收

- 契约测试：全部新增 OpenAPI、JSON Schema、MCP Schema、事件 payload、错误码和生成物可复现。
- 边界测试：AI Core Domain/Application、Plugin 和通用 Playbook Engine 中不得出现 `orders`、`order-worker`、`mtb-fault-injector` 等业务常量；这些只能出现在静态资产、部署配置和测试夹具。
- 资产测试：缺文件、非法 YAML、未知 capability、digest 变化、跨 tenant 访问全部拒绝；Knowledge/Skill 加载不会增加工具权限。
- Port 一致性测试：Mock/Real PostgreSQL Adapter 对相同会话指纹、阻塞树、already-resolved 和 precondition-failed 场景返回一致领域语义。
- 安全测试：非 Admin 审批、跨 org、过期或 scope 不匹配的 Evidence、PID 复用、runId 不符、受保护角色、重复 Evidence、任意 SQL/PromQL 均被拒绝。
- 恢复测试：重启后 `waiting_approval` 可继续；执行中断只读 reconcile，不重复终止。
- Mock E2E：确定性覆盖 Alert 去重、资产加载、诊断、等待审批、拒绝、批准、执行、恢复及完整连续 TaskEvent replay。
- Real E2E：故障稳定进入 `idle in transaction` 并产生 Lock waiter；一个告警只创建一个组织事件 Task；Viewer 可读不可批，Admin 批准后只终止当前 runId 的 fault-injector；未提交的 `fault_marker` 回滚，order-worker heartbeat 恢复；实时阻塞关系和 Prometheus 条件恢复；其他数据库会话不受影响。
- 泄漏测试：模型输入、日志、TaskEvent、Checkpoint 中不存在密码、DSN、完整 SQL、原始时序或 ApprovalEvidence。
- 回归门禁：现有 node_exporter Mock、real-metrics、real-agent、Session history、SSE replay 和前端测试继续通过。

## 6. 默认假设与明确不做

- 按完整最小闭环实施：告警自动诊断和准备 Intent，Grafana Admin 人工确认后自动执行与验证。
- 告警创建同 org 可读的事件会话；现有用户私有会话不改变。
- 仅支持单 PostgreSQL 实例、一个确定性订单演示和一种终止动作。
- 不做独立审计系统、知识/Skill/Playbook 管理、语义检索、通知推送、Grafana Dashboard 写入、告警规则编辑、自动预授权恢复、任意 SQL/PromQL/shell、通用 HTTP 探索或源码探索 MCP。
- TaskEvent、Approval、Intent 和 Checkpoint 是闭环必需的运行状态，不作为审计产品对外提供。
