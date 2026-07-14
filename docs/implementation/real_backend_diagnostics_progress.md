# 真实后端分层诊断进度记录

> status: completed
> createdAt: 2026-07-14
> completedAt: 2026-07-14
> plan: [`real_backend_diagnostics_execution_plan.md`](real_backend_diagnostics_execution_plan.md)

## 执行状态

|阶段|状态|证据|
|-|-|-|
|G0：激活诊断记录|完成|执行计划、进度记录和文档路由已新增；`make validate-contracts` 与 `git diff --check` 通过。|
|P1：Prometheus 与 MCP 断点探针|完成|`make test-diagnostics` 通过 6 个离线响应校验；`make diagnose-real-metrics` 确认三条 Prometheus vector 各 1 series/1 sample，并通过真实 MCP transport 获得 4 个指标候选、4 个 CPU labels 和三条非空 matrix。|
|P2：DeepSeek 直连探针|完成|`make test-diagnostics` 通过 10 个离线测试；从调用进程加载 `.env` 后，`make diagnose-deepseek` 确认 `deepseek-v4-flash` 存在，并在 539 ms 返回严格 `{"status":"ok","answer":"pong"}`。|
|P3：模式切换恢复与收口|完成|前端单测 7 文件/18 用例通过；Mock 浏览器从不存在的 Session URL 清理旧 ID 后成功创建新任务和三图；`make e2e-mock`、最终两轮 `make e2e-real-metrics` 与有凭证 `make e2e-real-agent` 通过。|

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

## P3 实现说明

- 三种手动 Compose 命令使用不同 project 名和 AI Core named volume；从上一模式保留的 URL Session 在下一模式中会得到 404/`resource_not_found`。
- Workbench 现在只对这一明确错误清除旧 `sessionId`/`taskId`、reducer、replay sequence 和待提交幂等状态，同时保留 URL 的其他 Grafana 参数并显示恢复提示。
- 明确的 HTTP 404/`resource_not_found` 之外，Session/history/create/load-more 错误不会被伪装成新会话，而是使用 Plugin Resource API 的结构化错误或安全的 HTTP/依赖分类显示；任意异常文本不会直接进入页面。
- replay 请求在 Session 被清除后会检查 effect cancellation，避免旧环境返回的事件重新写入已重置状态。
- Playwright 覆盖“不存在的 Session URL -> URL 清理 -> 新 Session/Task -> 三图”的完整浏览器路径，Mock 与 real-metrics 均通过。

## 最终验证与结论

|检查|最终结果|
|-|-|
|`make check`|通过：生成物、24 份 JSON Schema/fixture、Go、前端 7 文件/18 用例、10 个诊断测试、边界和密钥扫描。|
|`make diagnose-real-metrics`|通过：Prometheus 三条 vector 与 MCP 三条 matrix 均为非空 1 series/1 sample。|
|`make diagnose-deepseek`|通过：`deepseek-v4-flash` 返回严格 JSON `pong`，未输出 key。|
|`make e2e-mock`|通过：多轮、恢复、布局以及 stale-Session 自动恢复。|
|`make e2e-real-metrics`|最终连续两轮通过：真实三视图、API/SSE、浏览器 stale-Session 恢复与清理均通过。|
|`make e2e-real-agent`|通过：真实模型概览/CPU、真实 series、工具配对、replay 与泄漏检查。|

P3 的首次 real-metrics 预检曾出现一次 API 审计断言只观察到 3/7 个 `tool.started`；未改业务代码的后续两轮完整命令连续通过，且原始 Prometheus/MCP 探针始终通过。本记录保留该瞬时现象，当前没有证据把它归因于指标无数据或本次前端恢复改动。

当前环境的模式二、模式三后端均能返回数据。已确认的手工模式切换故障点是独立 volume 与旧 URL Session 的组合；本切片已修复其静默失败表现，并提供 Prometheus、MCP、DeepSeek 三个独立断点入口。
