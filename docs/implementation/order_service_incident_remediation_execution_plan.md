# 可控订单业务系统事件处置闭环执行计划

> Status: active
> Created: 2026-07-16
> Last reviewed: 2026-07-16

## 1. 交付目标

本阶段在 Docker 中实现一个带真实订单队列和异步 Worker 的可控业务系统，并完成以下闭环：

```text
订单真实积压 -> Prometheus/Grafana 告警 -> 组织 Incident Task
-> 精确召回 Knowledge/Skill/Playbook -> 有界只读诊断
-> 确定性 Intent/Diff -> Admin Approval -> 类型化 CAS 修复
-> 运行状态、指标、业务探针三重验证 -> 事件和审计持久化
```

Golden Scenario 将 worker concurrency 从健康值 2 真实修改为 0；订单仍能入队但不能完成。修复只允许在证据、资产摘要、实例 epoch、配置版本和旧值均匹配时执行 `0 -> 2`。

## 2. 固定边界

- `order-service` 提供 Business API、Operational API 和 `/metrics`；`fault-controller` 仅经 Unix socket 控制故障；`loadgen` 只访问 Business API。
- Business、Operational 和 Fault 契约分离。Fault 网络不连接 AI Core、assistant-mcp、Grafana 或宿主机端口。
- assistant-mcp 拥有静态 Knowledge、Skill、Playbook、Alert Mapping 及类型化业务 Adapter；AI Core 拥有 Incident 状态、Approval、Checkpoint 和权威审计。
- Agent 最多追加四次只读调用；它不能看到故障状态、原始时序、ApprovalEvidence 或任何写工具。
- QueryPlan 保持现状；Incident 使用独立 IncidentPlan 和 Playbook checkpoint。
- 所有跨进程 OpenAPI、JSON Schema、MCP Tool Schema、SSE 事件和错误码先于实现更新并生成。

## 3. 业务与运维契约

健康默认值为 concurrency 2、单订单处理 200ms、流量 2 orders/s、队列容量 100。Prometheus 每 5 秒抓取，当前 Grafana 镜像按其 10 秒 scheduler base interval 评估规则。业务 API 支持幂等创建和状态查询。Operational API 只提供运行时、队列、worker 配置、健康 policy、有限近期结果、类型化 worker CAS 更新、operation 查询和固定业务探针。

每次服务启动生成 `instanceEpoch`。写请求必须包含 `operationId, instanceEpoch, expectedVersion, expectedConcurrency, newConcurrency, intentDigest, approvalId`。第一阶段仅允许当前实例上 `0 -> 2`；重复相同请求返回原回执，operation ID 对应不同请求则冲突。

Fault 场景至少包含：

- `worker-stopped`：走真实配置路径修改为 0 并递增版本。
- `slow-processing`：worker 仍活跃但处理时间变为 2s。
- `dependency-errors`：真实处理路径出现有限错误、重试和失败。

指标必须源自业务状态，不允许注入器直接设置指标。指标包括接收/完成计数、queue depth/oldest age、configured/active/inflight worker、处理与端到端时延、有限原因的 retry/failure 以及业务探针结果；禁止高基数业务标识。

## 4. 资产、Agent 与 Playbook

- Knowledge 记录订单生命周期、指标语义、健康值和恢复标准。
- Skill 记录积压诊断方法，要求区分 worker stopped、slow processing、dependency errors 和证据不足。
- `order-queue-backlog@1` Playbook 固定 `load_assets -> observe -> needs_agent -> prepare -> needs_approval -> execute -> verify_runtime -> verify_metrics -> verify_business`。
- Alert Mapping 使用 `sourceId + alertName + requiredLabels` 精确匹配；零个或多个结果均失败。
- Agent 输出严格为主假设、证据引用、替代假设、置信度和候选动作。确定性 prepare policy 才能生成最终 Intent。

## 5. Incident、审批与恢复

Task 增加 `metric_analysis | incident_remediation` tagged union；Session 增加 `private | org_incident`；Message 增加 `trigger`。Incident 正常状态为 `created -> planning -> running_tools -> waiting_approval -> executing -> reconciling -> validating -> completed`，并支持 failed/cancelled。

AI Core 新增 AlertEvent、TaskCheckpoint、RemediationIntent、Approval、RemediationExecution 和 AuditRecord。所有状态和 TaskEvent 先持久化后 SSE。等待审批可在重启后恢复；执行响应不确定时只读取 operation receipt 和当前配置，不盲目重放写操作。

Approval 默认十分钟到期，使用 Idempotency-Key 和资源版本 CAS。ApprovalEvidence 最长 60 秒，绑定 tenant/org/task/approval/intent/capability/target/version/operation，不持久化且不进入模型或前端。

完成必须同时满足：配置和活跃 worker 恢复；连续指标显示完成速率恢复、积压下降且告警表达式恢复；一个固定探针订单在五秒内完成。

## 6. 实施 Gates

|Gate|内容|完成条件|
|-|-|-|
|G0|计划、ADR、文档路由|决策和 owner 无歧义；PostgreSQL 计划保留为被取代历史|
|G1|OpenAPI、Schema、事件、错误码、生成物|合同校验和生成物 diff 门禁通过；Fault 类型不进入产品客户端|
|G2|order-demo Domain/Application/Adapters|真实积压和恢复；Mock/Real Port 语义测试；指标状态一致|
|G3|Compose、Prometheus、Grafana Alert、故障隔离|告警 firing/resolved 稳定；Fault 网络不可达|
|G4|assistant-mcp 资产、Playbook、工具与审计|资产摘要固定；读写工具隔离；Mock/Real 合同测试|
|G5|AI Core Incident、告警、持久化和只读 Agent|告警幂等创建组织事件；checkpoint 可恢复；无写能力|
|G6|Intent、Approval、Execute、Verify、Audit|无批准或任一前置不符时零写入；三重验证完成|
|G7|Plugin Backend 与 Workbench|组织列表、时间线、证据、Diff 和 Admin 审批；跨 org 隔离|
|G8|E2E、真实 Agent smoke、回归与文档收口|Golden、相似故障、竞态、重启、安全和现有功能全部通过|

每个 Gate 独立提交；同一提交同步进度记录、当前代码快照、代码树和相关运行手册。

## 7. 强制测试矩阵

- Domain：队列、订单状态、配置 CAS、实例 epoch、幂等回执、Task/Approval 状态机。
- Contract：OpenAPI、JSON Schema、MCP schema、事件 fixture、生成物可复现。
- Adapter：Mock/Real 对 healthy、stopped、slow、dependency-error、already-resolved 和 conflict 返回一致领域语义。
- Security：跨 org、非 Admin、伪造/过期/重放 Evidence、任意查询/命令、Fault 工具发现和敏感数据泄漏全部拒绝。
- Recovery：waiting approval 重启恢复；executing 只读 reconcile；服务新 epoch 使旧 Intent 失效；重复 alert 不重复建 Task。
- Diagnosis：worker-stopped 才产生 `0 -> 2` Intent；slow、dependency-errors 和证据不足均 no-action。
- E2E：稳定流量、故障、告警、诊断、批准、CAS 修复、积压下降、探针成功、resolved、连续事件和三层审计。
- Regression：现有 Mock、real-metrics、real-agent、Session history、SSE replay、Plugin 和 Workbench 全部通过。

## 8. 明确不做

本阶段不实现任意 shell/SQL/PromQL/HTTP、通用 execute、自动批准、在线资产编辑、语义检索、多服务处置、生产级密钥管理或 PostgreSQL 接入。新增第二个写动作或改变服务所有权、权限、存储边界时必须先新增 ADR。
