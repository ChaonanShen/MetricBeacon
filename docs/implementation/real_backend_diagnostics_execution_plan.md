# 真实后端分层诊断执行计划

> status: completed
> createdAt: 2026-07-14
> completedAt: 2026-07-14
> implementationAuthorized: true
> dependsOn: `node_exporter_real_analysis_execution_plan.md`、
> `current_codebase_overview.md`、`three_pane_workbench_execution_plan.md`

## 1. 背景与已复现证据

三种 Compose 模式已有完整 E2E，但真实指标和真实 Agent 失败时缺少可单独运行的中间层探针。2026-07-14
已在当前环境重新执行：

- DeepSeek `/models` 返回配置的 `deepseek-v4-flash`，最小 Chat Completion 返回严格 JSON。
- `make e2e-real-metrics` 的 API 阶段从三条规范 PromQL 获得非空真实 series。
- `make e2e-real-agent` 完成“概览”和“只看 CPU”两轮，并通过工具配对、恢复和泄漏检查。

因此本切片不修改 Prometheus Adapter、MCP 契约、Agent Profile 或编排语义，而是把可重复的断点诊断补齐，
并处理独立 Compose project 间切换时 URL 中旧 Session ID 指向另一份 AI Core volume 的恢复问题。

## 2. 范围与边界

本切片包含：

1. 原始 Prometheus 探针：等待 node_exporter target 和 CPU 两次 scrape，直接验证 CPU、内存、负载规范查询有数据。
2. assistant-mcp 探针：绕过 AI Core 和 Grafana，通过真实 Streamable HTTP MCP 调用搜索、标签和三条查询。
3. DeepSeek 探针：绕过 Agent/MCP，用环境配置的 endpoint、key、model 做 `/models` 和最小 Chat Completion。
4. 可离线运行的诊断脚本单元测试和 Shell 语法检查；外部探针不加入无凭证 `make check`。
5. 模式切换恢复：当 URL Session 在当前独立 AI Core volume 中明确返回 `resource_not_found` 时，清除旧路由，
   允许创建新 Session；其他依赖或网络错误不得伪装成新会话。
6. 修订 README、当前代码快照、代码树和本进度记录。

本切片不包含：

- 新指标、开放式 PromQL、模型 fallback 或 readiness 主动调用模型。
- 修改跨进程 OpenAPI、JSON Schema、MCP Tool Schema、SQLite schema 或服务所有权。
- 在日志中打印 API key、完整时序或内部 Prometheus 地址。
- 把实时 Prometheus 数值或模型文本固定成 CI 断言。

上述边界不改变接口、权限、持久化或数据出域决策，因此不需要新 ADR。

## 3. 执行切片

### G0：激活诊断记录

提交：`docs: activate real backend diagnostics plan`

- 新增本计划和进度记录。
- 在 `docs/CLAUDE.md` 登记为当前活动切片。
- 保持已完成 real-analysis 与 three-pane 计划为历史证据。

验证：`make validate-contracts`、`git diff --check`。

### P1：Prometheus 与 MCP 断点探针

提交：`test(diagnostics): probe live prometheus and mcp`

- 抽取 real-metrics 等待脚本，供既有 real-metrics、real-agent E2E 和新探针复用。
- 新增只启动 `prometheus`、`node-exporter`、`assistant-mcp` 的诊断入口。
- 第一阶段直接检查三条规范 PromQL 的非空结果；只输出视图、result type、series/sample 计数。
- 第二阶段通过 assistant-mcp 的真实 MCP transport 检查 4 个注册指标、CPU labels 和三条 matrix 查询。
- 任何阶段失败都以非零状态退出，并保留阶段名；结束后只清理由该入口创建的 Compose project。

验证：`make test-diagnostics`、`make diagnose-real-metrics`、既有 `make e2e-real-metrics`。

### P2：DeepSeek 直连探针

提交：`test(diagnostics): probe configured deepseek model`

- 新增 Node 探针，要求调用进程显式提供 `DEEPSEEK_API_KEY`。
- 验证配置模型出现在 `/models`，再请求固定的严格 JSON `pong`；输出 model、耗时和受限回复，不输出 key。
- 用本地 fake HTTP server 覆盖成功、模型缺失、Chat 错误和非法 JSON。
- Make 目标不自动读取 `.env`，文档提供显式、不会回显 key 的加载方式。

验证：`make test-diagnostics`、加载 `.env` 后执行 `make diagnose-deepseek`。

### P3：模式切换恢复与收口

提交：`fix(frontend): recover stale sessions across run modes`

- 仅在 Session 读取明确返回 404/`resource_not_found` 时清除 URL 的 `sessionId`/`taskId` 和旧 reducer 状态。
- 网络、权限或 dependency 错误保持可见，不自动创建替代会话。
- Playwright 从不存在的 Session URL 恢复后提交一次任务，验证新 Session 和图表正常产生。
- 完成后更新 README、当前代码概览/树和进度，标记本计划 completed。

验证：`make check`、`make e2e-mock`、`make e2e-real-metrics`、加载凭证后执行 `make e2e-real-agent`。

## 4. 完成标准

1. `make diagnose-real-metrics` 能区分原始 Prometheus 与 MCP Adapter 两个阶段，并确认三视图非空。
2. `make diagnose-deepseek` 能独立证明 endpoint、key 和 model 可用，并返回受限合理回复。
3. 三个完整 E2E 保持不变；诊断入口不能成为业务的第二条调用链。
4. 切换使用独立 volume 的 Compose 模式后，旧 URL 不再让新任务静默失败。
5. Secret、内部 URL、完整 series 和模型 reasoning 不进入诊断输出或仓库。
6. 所有脚本自行清理其创建的资源，且不停止用户管理的其他 Compose project。

回滚以每个提交为单位；本切片没有契约或数据库迁移。
