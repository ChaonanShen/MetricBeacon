# 真实后端分层测试矩阵

> status: current runbook
> updatedAt: 2026-07-15

本文供开发者或 code agent 判断“第二、第三种模式为什么没有数据”。测试从最窄的解析与连接开始，逐层走到
完整浏览器链路；不要只以 HTTP 200 或数组非空作为通过条件。

## 1. 推荐执行顺序

|顺序|命令|通过证明|失败优先检查|
|-|-|-|-|
|1|`make validate-contracts`|OpenAPI、JSON Schema 和 MCP Tool Schema 可生成且一致。|契约或生成物。|
|2|`make test-diagnostics`|离线解析、指标语义、Task/Event/Chart 分析和探针错误分类均正确。|测试分析器或 wire shape 假设。|
|3|`make test-assistant-mcp`|Mock/Real Prometheus Adapter 与 MCP 工具接线单测通过。|assistant-mcp Adapter、registry 或 Tool handler。|
|4|`make diagnose-real-metrics`|原始 Prometheus 和 MCP 都返回 CPU/内存/load 的合理结果。|Prometheus scrape、PromQL、MCP transport/Adapter。|
|5|`make diagnose-deepseek`|配置模型存在，并返回可解析的严格 `{"answer":"pong"}`。|模型凭证、endpoint、model 或供应商响应。|
|6|`make e2e-mock`|确定性数据通过 Plugin/API/持久化/SSE/浏览器全链路。|与真实数据无关的应用链路回归。|
|7|`make e2e-real-metrics`|六种自然语言 range/step/view 输入能把真实 Prometheus 数据变成 durable Chart，其中包括 30 分钟范围、每 5 分钟采样。|IntentPlanner→QueryPlan→MCP→Prometheus 的真实数据路径。|
|8|`make e2e-real-agent`|真实模型同步规划概览三图和 CPU 追问一图，并在同一 Session 连续 8 次正确规划相同 CPU/内存请求；确定性执行结果可持久化/重放。|Planner 严格 JSON、结构化历史、冻结 views 或本地结果格式化。|
|9|`make check`|仓库全部静态检查、单元/集成测试通过。|对应失败目标。|

需要模型的第 5、8 步不会自动读取 `.env`。执行前显式加载，且不要在日志中打印变量：

```sh
set -a
. ./.env
set +a
make diagnose-deepseek
make e2e-real-agent
```

自动 E2E 默认把 Grafana、AI Core、assistant-mcp 映射到 `13000`、`18080`、`18081`，避免干扰使用
`3000`、`8080`、`8081` 的手工栈。必要时可用 `GRAFANA_HOST_PORT`、`AI_CORE_HOST_PORT`、
`ASSISTANT_MCP_HOST_PORT` 覆盖。诊断 MCP 端口则用 `MTB_DIAGNOSTIC_MCP_PORT` 覆盖。

## 2. 各层合理返回的大致形式

以下示例只描述结构。真实数值、时间戳、instance 名和事件总数可随机器负载与 Agent 事件变化，code agent
不得把示例瞬时值复制成固定断言。

### L1：原始 Prometheus

CPU、内存和 load 即时查询应是 `status=success`、`resultType=vector`，并至少有一个带 `instance` 的 sample：

```json
{
  "status": "success",
  "data": {
    "resultType": "vector",
    "result": [
      {"metric": {"instance": "node-exporter:9100"}, "value": [1234567890, "<finite-number>"]}
    ]
  }
}
```

稳定语义：CPU 使用率和内存可用率在 `0..100`，load 有限且非负。空 result、缺少 instance、`NaN`/
`Inf`、CPU/内存越界或负 load 均失败。诊断只输出计数和 `min/max/latest`，不输出完整时序。

### L2：assistant-mcp

`grafana.query_prometheus` 的工具结果应包含 `resultType=matrix` 和非空 `series`；每条 series 含 labels 与
按时间升序的 points。安全摘要近似如下：

```text
[mcp] view=cpu series=1 samples=1 min=98.1 max=98.1 latest=98.1
[mcp] view=memory series=1 samples=1 min=64.6 max=64.6 latest=64.6
[mcp] view=load series=1 samples=1 min=3.2 max=3.2 latest=3.2
```

同一最新时间点有多个 instance 时，`latest` 可显示为范围。单条 series 时间必须严格递增；总量上限为
20 series、5,000 samples。

### L3/L4：durable Task 与 Agent

Task API 的通过条件不是“收到了 assistant 文本”，而是同时满足：

- sequence 从 1 连续递增，所有事件的 `taskId`/`sessionId` 一致；
- 恰好一个最终 `task.completed`，没有 `task.failed`；
- 每个 `tool.started` 有且只有一个同 source-call ID 的成功 `tool.completed`；
- 每张 Chart 有一个成功 Execution，`seriesCount` 等于实际 series 数；
- Task 的 QueryPlan、Chart step、Execution 实际样本范围与返回 series 一致；
- Chart view、规范 PromQL 和 CPU/内存/load 的指标语义一致；
- 恰好一个由本地 formatter 生成的最终 `assistant.message.completed`，包含有效范围/step/window 和本地样本统计。
- 重复请求验收中，每一轮都必须得到与当前显式输入一致的 QueryPlan；不能因历史增长退化为 prose、空 content 或沿用旧参数。

Mock/真实指标概览为 3 次 query 工具调用和 3 张图；真实 Agent 概览也应规划 CPU、内存、load 三图，而
“只看 CPU”的追问应只有 1 次查询工具调用和 1 张 CPU 图。安全输出近似如下：

```text
[agent-task] events=33 toolCalls=7 charts=3 terminal=task.completed
[agent-task] view=cpu series=1 samples=1 min=97.1 max=97.1 latest=97.1
[agent-task] view=memory series=1 samples=1 min=54.3 max=54.3 latest=54.3
[agent-task] view=load series=1 samples=1 min=2.3 max=2.3 latest=2.3
```

## 3. 失败定位

|观察结果|故障边界|
|-|-|
|原始 Prometheus 失败|先查 node_exporter target、scrape 历史和规范 PromQL；问题尚未进入 MCP。|
|原始 Prometheus 通过、MCP 失败|查 assistant-mcp transport、身份上下文、Real Adapter 解码或 Tool Schema。|
|MCP 通过、真实指标 E2E 失败|查 AI Core workflow、事件持久化、Chart/Execution 组装或 Plugin 代理。|
|DeepSeek 直连失败|查 key、endpoint、model 和供应商 HTTP；无需先改 Agent prompt。|
|DeepSeek 直连通过、真实 Agent E2E 失败|查 Eino IntentPlanner 的严格 JSON、历史边界和本地冻结校验；执行阶段不再调用模型。|
|API E2E 通过、浏览器失败|后端数据已经成立；查 SSE 恢复、URL Session、DataFrame mapper 或图表渲染。|
|Mock E2E 也失败|这是公共应用链路回归，不应归因于真实 Prometheus 或模型。|

失败报告应包含失败层、命令、view、series/sample/event/tool/chart 计数和安全错误分类。不要附完整 raw series、
模型私有推理、key、Grafana token、内部 URL 或持久化数据库全文。

## 4. 本次基线证据

2026-07-14 的验证中，离线诊断 35/35 通过；Mock、真实指标和真实 Agent E2E 均通过。Mock 与真实指标
E2E 覆盖同一组五种有界输入。真实 Agent 概览为 21 events/3 query tool calls/3 charts，CPU 追问为
13 events/1 query tool call/1 chart；这些计数和真实瞬时值仅是本次执行证据，不是未来运行的期望常量。

2026-07-15 的 Planner 加固验证中，Mock E2E 扩展为六种输入并通过；真实 Agent 在同一 Session 连续 8 次
提交“最近 10 分钟 CPU 和内存、每隔 2 分钟采样”，每轮均形成 `rangeSeconds=600`、`stepSeconds=120`
的双视图计划，并各完成 2 次 query 工具调用和 2 张图。测试只记录计划和事件计数，不记录模型原文、凭证或
完整时序。
