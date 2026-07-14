# 真实后端分层诊断进度记录

> status: active
> createdAt: 2026-07-14
> plan: [`real_backend_diagnostics_execution_plan.md`](real_backend_diagnostics_execution_plan.md)

## 执行状态

|阶段|状态|证据|
|-|-|-|
|G0：激活诊断记录|完成|执行计划、进度记录和文档路由已新增；`make validate-contracts` 与 `git diff --check` 通过。|
|P1：Prometheus 与 MCP 断点探针|完成|`make test-diagnostics` 通过 6 个离线响应校验；`make diagnose-real-metrics` 确认三条 Prometheus vector 各 1 series/1 sample，并通过真实 MCP transport 获得 4 个指标候选、4 个 CPU labels 和三条非空 matrix。|
|P2：DeepSeek 直连探针|完成|`make test-diagnostics` 通过 10 个离线测试；从调用进程加载 `.env` 后，`make diagnose-deepseek` 确认 `deepseek-v4-flash` 存在，并在 539 ms 返回严格 `{"status":"ok","answer":"pong"}`。|
|P3：模式切换恢复与收口|待开始|—|

## 基线复现

|检查|结果|说明|
|-|-|-|
|DeepSeek `/models` + 最小 Chat Completion|通过|配置模型为 `deepseek-v4-flash`；返回严格 `{"status":"ok","answer":"pong"}`。|
|`make e2e-real-metrics` API 阶段|通过|三条真实 Prometheus 查询均返回非空 series；浏览器阶段曾被三栏重复标题的旧选择器误报，已在前序提交修复。|
|`make e2e-real-agent`|通过|真实模型完成概览三图和 CPU 单图，并通过 durable tool 配对、恢复与泄漏检查。|

## 边界确认

诊断只调用现有只读接口，不新增业务真相源，不修改契约、SQLite、权限或模型出域范围。

## P1 实现说明

- `make diagnose-real-metrics` 只启动 Prometheus、node_exporter 与 assistant-mcp，使用独立 Compose project，退出时只清理自身资源。
- 原始 Prometheus 阶段检查 CPU、内存和 load 的即时 vector，只打印视图和 series/sample 计数。
- MCP 阶段通过真实 Streamable HTTP transport 检查指标搜索、标签与三条区间查询，只记录候选、标签和 series/sample 计数。
- real-metrics 与 real-agent E2E 已复用同一个 target/two-scrape 等待器，避免诊断和完整验收产生不同的就绪判定。

## P2 实现说明

- `make diagnose-deepseek` 要求调用进程显式提供 `DEEPSEEK_API_KEY`，并沿用业务 Bootstrap 的默认 base URL、model 与 thinking-disabled 配置。
- 探针先调用 `/models` 验证配置模型存在，再发出最多 64 token 的固定 JSON `pong` 请求；成功输出只包含 model、Chat 耗时和受限回复。
- 失败只报告 models/chat 阶段、HTTP 状态或响应分类，不打印响应 body、Authorization、key 或 reasoning。
- 本地 fake HTTP server 覆盖成功、模型缺失、Chat HTTP 错误和非 JSON 模型回复，不需要凭证即可进入 `make check`。
