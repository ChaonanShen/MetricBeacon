# 可控订单业务系统事件处置闭环进度记录

> status: active
> createdAt: 2026-07-16
> plan: [`order_service_incident_remediation_execution_plan.md`](order_service_incident_remediation_execution_plan.md)

## 执行状态

|Gate|状态|证据|
|-|-|-|
|G0：计划、ADR、文档路由|已完成|ADR-023/024/025 固定所有权、资产能力与类型化修复边界；新计划和文档路由建立；`git diff --check` 与 `make validate-contracts` 通过。|
|G1：Contracts-first|进行中|Business/Operational 与独立 Fault OpenAPI 已定义；Go client/server 类型已生成；`make validate-contracts`、`make generated-client-diff`、order-demo generated package tests 和 `git diff --check` 通过。Incident/Approval/asset/tool 合同将在对应实现前继续补齐。|
|G2：order-demo|进行中|纯领域订单状态机、worker 配置策略、bounded queue、动态 worker、三种真实处理故障、幂等订单、受限 `0 -> 2` CAS、operation reconcile 和真实队列 probe 已实现；`go test -race ./internal/domain/... ./internal/application/...` 通过。HTTP、metrics 和容器 Adapter 待完成。|
|G3：可观测性与隔离拓扑|未开始|—|
|G4：assistant-mcp 资产与能力|未开始|—|
|G5：AI Core 只读 Incident|未开始|—|
|G6：审批、执行、验证与审计|未开始|—|
|G7：Plugin 与 Workbench|未开始|—|
|G8：E2E 与收口|未开始|—|

## 当前边界

G0 已完成。代码、合同、数据库和运行拓扑尚未改变，下一步从跨进程合同开始。
