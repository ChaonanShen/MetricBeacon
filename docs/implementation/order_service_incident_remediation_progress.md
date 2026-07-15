# 可控订单业务系统事件处置闭环进度记录

> status: active
> createdAt: 2026-07-16
> plan: [`order_service_incident_remediation_execution_plan.md`](order_service_incident_remediation_execution_plan.md)

## 执行状态

|Gate|状态|证据|
|-|-|-|
|G0：计划、ADR、文档路由|已完成|ADR-023/024/025 固定所有权、资产能力与类型化修复边界；新计划和文档路由建立；`git diff --check` 与 `make validate-contracts` 通过。|
|G1：Contracts-first|进行中|Business/Operational 与独立 Fault OpenAPI、首批 Incident MCP Tool 输入/输出合同已定义；Go/TypeScript 类型已生成。Go Incident 生成显式使用 `skip-prune` 并以关键类型存在断言防止 MCP component schema 被裁剪。合同覆盖精确告警映射、固定资产引用、可暂停 Playbook、严格诊断 DTO 和有界订单只读查询；写能力与 Approval HTTP 合同将在 G6 实现前补齐。`make validate-contracts`、`make generated-client-diff`、order-demo generated package tests 和 `git diff --check` 通过。|
|G2：order-demo|已完成|Domain/Application、分离的 Business/Operational HTTP、读写 token、Unix Socket Fault、Prometheus collector、loadgen 和三命令镜像已实现。全模块 race tests、重复应用 race tests、合同/生成门禁和 Docker 冷构建通过；真实双容器验证显示 Fault 容器 `network_mode=none`、Business 对 Ops 返回 404、积压 depth=1/configured=0/active=0，CAS `0 -> 2` 后 queue=0 且 probe 203ms 完成。|
|G3：可观测性与隔离拓扑|已完成|新增 incident Compose mode、三个 internal order network、无网络 fault-controller、5s Prometheus scrape、只读 Grafana datasource、10s 规则评估和 HMAC Webhook contact point。冷构建全栈通过；真实 E2E 验证 order target up=1、Business 对 Ops 404、Fault network mode=none、健康 Normal(NoData)、注入后 Pending→Alerting、reset 后恢复 Normal 且 queue=0。Grafana 文件 provisioning 格式以当前镜像实际导出 API 验证。|
|G4：assistant-mcp 资产与能力|只读切片已完成|文件资产与 order-demo Mock/HTTP 只读 Adapter、HMAC checkpoint 和确定性 prepare 已完成。默认 profile 工具仍精确为原三个；incident profile 新增 11 个 closed-world/read-only 工具并要求 `incidents:diagnose`，工具/Port 无 Fault、shell、任意 HTTP、通用 execute 或写入口。MCP transport 覆盖资产→映射→观测→诊断→版本绑定 Intent，以及权限、额外参数、零映射、missing operation、checkpoint 篡改；healthy/slow/dependency 三场景即使请求 restore 也完成为 no-action。race tests 重复 10 次、Docker 冷构建、三层 Compose config 和真实容器 Operational HTTP→MCP（注入 stopped 后固定真实 epoch/version）均通过。写工具与执行审计属于 G6。|
|G5：AI Core 只读 Incident|未开始|—|
|G6：审批、执行、验证与审计|未开始|—|
|G7：Plugin 与 Workbench|未开始|—|
|G8：E2E 与收口|未开始|—|

## 当前边界

G4 的资产、只读诊断与确定性 prepare 切片已完成；下一步进入 G5，先合同化并实现 AI Core Alert ingress、组织 Incident Task、持久化 checkpoint 与只读 Agent 能力过滤。
