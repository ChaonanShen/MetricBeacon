# 自然语言单输入查询执行计划

> status: active
> createdAt: 2026-07-15
> implementationAuthorized: true
> decision: [`../adr/ADR-020-natural-language-only-workbench-query-intent.md`](../adr/ADR-020-natural-language-only-workbench-query-intent.md)
> dependsOn: `bounded_node_exporter_query_parameters_execution_plan.md`、`current_codebase_overview.md`

## 1. 目标

Workbench 只提交自然语言和固定逻辑数据源。AI Core 从当前消息解析有界查询范围与采样间隔，持久化有效
QueryPlan，再由现有 view-only Agent 和 assistant-mcp 注册表形成规范 PromQL 并查询数据。

```text
自然语言消息
  -> AI Core bounded QueryIntentResolver
  -> durable Task.timeRange + QueryPlan
  -> Agent selects registered views
  -> assistant-mcp compiles canonical PromQL
  -> Prometheus query_range
```

## 2. 范围与非目标

本切片包含：

- 删除 Workbench 的默认时间范围与采样分辨率控件及其请求映射；
- 让 Workbench CreateTask 只携带 message 与 `prometheus-main`；
- 覆盖 `每隔 5s 采集` 等自然语言 cadence；
- 更新浏览器 E2E、当前代码快照、文档路由和验证证据。

本切片不包含：任意 PromQL、模型生成时间/step、额外指标/view、label 过滤、范围/点数上限调整、CPU rate
window 策略调整、数据库或跨进程契约变更。

## 3. 分阶段执行

### G0：决策与计划

- 新增 ADR-020、本计划和进度记录；标明仅 supersede ADR-019 的 Workbench 控件决策。
- 更新 ADR 索引与结构蓝图。
- 验证：`make validate-contracts`、`git diff --check`。

### G1：自然语言 Workbench

- 删除范围/resolution state、props、select 和选项映射。
- 请求 builder 只提交 `datasourceUid` 与 trim 后的 message。
- 浏览器测试断言两个控件不存在，并用自然语言指定五分钟/五秒。
- 验证：`make test-frontend`。

### G2：解析回归

- cadence parser 支持 `每`、`每个`、`每隔`。
- commands 与 Mock API E2E 使用用户示例 wording，证明不是 auto 偶合。
- 验证：AI Core commands tests、Mock API E2E。

### G3：收口

- 更新 README、current overview/tree、runbook 和本进度记录。
- 验证：`make check`、`make e2e-mock`；若本地真实指标环境可用，再运行 `make e2e-real-metrics`。

## 4. 验收标准

- Workbench 不渲染“默认时间范围”和“采样分辨率”控件。
- Workbench CreateTask request 不含 `timeRange` 或 `resolution`。
- `查看最近1分钟cpu的使用率变化，每隔5s采集个数据` 解析为 60 秒范围、step 5 秒。
- 使用一个不会与 auto step 重合的 cadence 用例，证明 `每隔` 是显式解析结果。
- Task、Chart、Execution 与 Assistant 回复仍反映同一个有效 QueryPlan。
- CPU 30 秒 rate window 保持为计算窗口，不被误改为五秒 query step。
