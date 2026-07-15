# 可控订单业务系统事件处置闭环进度记录

> status: active
> createdAt: 2026-07-16
> plan: [`order_service_incident_remediation_execution_plan.md`](order_service_incident_remediation_execution_plan.md)

## 执行状态

|Gate|状态|证据|
|-|-|-|
|G0：计划、ADR、文档路由|已完成|ADR-023/024/025 固定所有权、资产能力与类型化修复边界；新计划和文档路由建立；`git diff --check` 与 `make validate-contracts` 通过。|
|G1：Contracts-first|已完成|除订单与 Incident MCP 合同外，现已定义 Grafana HMAC Alert ingress、`Task(metric_analysis \| incident_remediation)`、`Session(private \| org_incident)`、trigger Message、IncidentPlan、Approval decision、组织 Incident list、Plugin Resource 代理面和 13 类处置事件；Incident 分支被 Schema 明确禁止携带 QueryPlan，Webhook 禁止未知字段/ground truth。Go/TypeScript 生成物、AI Core 可构建端点占位和旧指标 Workbench tagged-union 兼容已同步。`make validate-contracts`（36 Schemas）、`make generated-client-diff`、AI Core HTTP tests、前端 typecheck/目标 Vitest 与 `git diff --check` 通过。|
|G2：order-demo|已完成|Domain/Application、分离的 Business/Operational HTTP、读写 token、Unix Socket Fault、Prometheus collector、loadgen 和三命令镜像已实现。全模块 race tests、重复应用 race tests、合同/生成门禁和 Docker 冷构建通过；真实双容器验证显示 Fault 容器 `network_mode=none`、Business 对 Ops 返回 404、积压 depth=1/configured=0/active=0，CAS `0 -> 2` 后 queue=0 且 probe 203ms 完成。|
|G3：可观测性与隔离拓扑|已完成|新增 incident Compose mode、三个 internal order network、无网络 fault-controller、5s Prometheus scrape、只读 Grafana datasource、10s 规则评估和 HMAC Webhook contact point。冷构建全栈通过；真实 E2E 验证 order target up=1、Business 对 Ops 404、Fault network mode=none、健康 Normal(NoData)、注入后 Pending→Alerting、reset 后恢复 Normal 且 queue=0。Grafana 文件 provisioning 格式以当前镜像实际导出 API 验证。|
|G4：assistant-mcp 资产与能力|只读切片已完成|文件资产与 order-demo Mock/HTTP 只读 Adapter、HMAC checkpoint 和确定性 prepare 已完成。默认 profile 工具仍精确为原三个；incident profile 新增 11 个 closed-world/read-only 工具并要求 `incidents:diagnose`，工具/Port 无 Fault、shell、任意 HTTP、通用 execute 或写入口。MCP transport 覆盖资产→映射→观测→诊断→版本绑定 Intent，以及权限、额外参数、零映射、missing operation、checkpoint 篡改；healthy/slow/dependency 三场景即使请求 restore 也完成为 no-action。race tests 重复 10 次、Docker 冷构建、三层 Compose config 和真实容器 Operational HTTP→MCP（注入 stopped 后固定真实 epoch/version）均通过。写工具与执行审计属于 G6。|
|G5：AI Core 只读 Incident|已完成|领域、SQLite `0007`、摘要 AlertEvent、opaque checkpoint 和 Incident Toolset 已落地；Grafana ingress 在解析前校验 source、原始字节 HMAC、常量时间签名和 ±5 分钟时间窗，并限制 64 KiB/10 alerts、严格拒绝未知字段。firing 告警按 tenant/org/source/fingerprint/startsAt 幂等创建 `org_incident` Session、trigger Message、Incident Task、固定资产和 checkpoint；同 fingerprint 并发 12 路只创建一个 Task，resolved 只追加 AlertEvent。可恢复工作流只接受 queue/worker/policy/recent 四个无重复只读证据，先持久化 ToolCall/TaskEvent 再通知，确定性写入 Diagnosis 后停在 prepare；分析恢复器与 Incident 恢复器不会串线，重复恢复不重复诊断。原始 webhook、annotations、values、URL、长 transport label、订单 ID 和 ground truth 均不进入 Incident 持久化或模型输入。AI Core 全测试、重点包 5 次 race、vet 与 HMAC 边界测试通过。|
|G6：审批、执行、验证与审计|代码闭环已完成，待真实 E2E|Incident MCP/ApprovalEvidence/assistant-mcp 修复边界、AI Core 闭集领域、SQLite `0008` 和 durable prepare 已就绪。Approval 决策必须 Admin、Idempotency-Key、Task/Approval 双版本和精确 Intent digest。AI Core 独立 Remediation Toolset 已接入 durable 工作流：先持久化稳定 operation ID、Execution started 与 accepted Audit，再签发不落盘的 60 秒 ApprovalEvidence 并执行唯一 `0 -> 2` 写；任何不确定响应或 started 重启只调用 `get_operation`，绝不重放写；精确回执后依次验证 runtime/worker、两次相邻固定 30 秒 recovery view 和稳定幂等 probe ID 的真实业务探针。runtime→validating 与 metrics→business checkpoint 均原子推进，恢复可从 waiting/executing/reconciling/validating 精确续跑。bootstrap 仅在 webhook、至少 32 字节 Evidence key 都有效时，按诊断恢复→已批准执行恢复顺序原子注入 Approval API；readiness 同时携带 diagnose/remediate 权限检查完整 profile，AI Core 镜像显式纳入 Evidence 模块。黄金路径、写后超时、started/unknown 重启、business checkpoint、回执篡改、receipt 缺失、指标失败、权限和 Evidence 不泄漏使用真实 SQLite 测试；workflow/MCP 重复 10 次、race 3 次、AI Core 全测试/vet、bootstrap 测试、Docker 构建及 Compose config 通过。|
|G7：Plugin 与 Workbench|进行中|Plugin Backend 已增加组织 Incident list、Approval GET/POST 受控代理；可信 Grafana identity 为所有角色增加 `incidents:read`，仅真实 Admin 增加 `incidents:approve`，并在代理边界提前拒绝 Viewer 写请求。tenant/org/role/permission 伪造头均被覆盖，page token 与 Idempotency-Key 原样受控转发；handler 重复 5 次、race 3 次和 vet 通过。AI Core Incident list 查询与 Workbench 尚待完成。|
|G8：E2E 与收口|未开始|—|

## 当前边界

G5 与 G6 的服务端代码闭环已完成，AI Core 已原子接通 Approval API 和恢复启动顺序。下一步完成 Plugin 受控代理与 Workbench 事件/审批交互，然后进行跨容器真实批准、执行、恢复与审计 E2E。
