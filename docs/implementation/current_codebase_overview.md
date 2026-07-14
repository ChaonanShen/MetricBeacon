# 当前骨架代码说明

> 本文以 2026-07-14 的实际代码为准，说明已可工作的基本 Mock 闭环；长期设计请参照
> [`code_skeleton_design.md`](code_skeleton_design.md)。

## 当前可演示的能力

用户在 Grafana App Plugin 中提交任意非空分析请求后，系统会创建持久化的 Session 和 Task，并通过 SSE 返回固定的 `node_exporter` CPU、内存可用率和系统负载三张时序图。数据与 Agent 计划都是确定性的 Mock，因此该闭环用于验证模块边界、协议、持久化和前端恢复能力，不代表自然语言理解或真实 Prometheus 查询已经实现。

```text
Grafana 浏览器前端
  -> Plugin Resource API
  -> Grafana Plugin Backend（认证上下文、受控代理）
  -> AI Core（Session/Task 工作流、SQLite、SSE）
  -> assistant-mcp（MCP transport、grafana.* 只读工具）
  -> Mock Prometheus Adapter -> node_exporter fixture

SSE TaskEvent 按原路径回传到前端，前端恢复状态并渲染三张图。
```

## 模块与职责

|位置|职责|当前实现边界|
|-|-|-|
|`contracts/`|跨进程协议的单一来源：Plugin Resource API、AI Core API、SSE 事件、MCP Tool Schema、错误码和示例。|通过 OpenAPI/JSON Schema 校验，并据此生成 Go/TypeScript 类型；业务模块不应另写重复 DTO。|
|`packages/generated-clients`、`packages/generated-contracts`|契约生成物。前者是 AI Core HTTP Client，后者是 Grafana MCP 工具类型。|由 `scripts/generate-clients.sh` 生成，`make generated-client-diff` 检查生成结果可复现。|
|`packages/request-context-go`|跨服务传递的租户、组织、用户、角色、权限、请求与 Trace 上下文。|Plugin Backend 从 Grafana 请求上下文生成它；浏览器传入的身份头不会被信任。|
|`packages/testkit-go`|测试用的确定性时钟与 ID 生成器。|仅为测试可重复性服务。|
|`services/ai-core/internal/domain`|核心领域模型和状态规则：Session/Message、AnalysisTask/TaskEvent/ToolCall、Chart/Execution、时间范围与领域错误。|不依赖数据库、MCP、Grafana 或模型 SDK。|
|`services/ai-core/internal/application` 与 `internal/ports`|命令服务和分析工作流；Port 定义存储、事件通知、Agent、工具、时钟和 ID 等外部能力。|工作流先持久化状态/事件，再通知 SSE；重启后将不能安全续跑的任务标记为失败。|
|`services/ai-core/internal/adapters`|将 Port 接到具体实现：SQLite、内存通知器、MCP 客户端、系统时钟/随机 ID 与确定性 Mock Agent。HTTP 入站 Adapter 暴露会话、任务、读取与 SSE 重放接口。|AI Core 是 Session、Task、Event、Chart 和 SQLite 数据的唯一所有者。它不直接读取 fixture，也不承载 Grafana 鉴权。|
|`services/ai-core/internal/bootstrap`|组装依赖：SQLite Store、Mock Agent、MCP Gateway、工作流、HTTP API。|`/readyz` 会同时检查 SQLite 及 MCP 工具是否就绪。|
|`services/assistant-mcp`|以 Streamable HTTP（`/mcp`）暴露只读的 `grafana.*` MCP 工具：`search_metrics`、`get_metric_labels`、`query_prometheus`。|工具先做权限和 Schema 校验，再调用 Prometheus Port；该服务不拥有 AI Core 的任务或数据库。|
|`services/assistant-mcp/internal/adapters/prometheus/mock`|Mock Prometheus Port 实现。|唯一允许读取 `data/mock-scenarios` 的代码；按请求的 PromQL 返回固定 fixture。真实 HTTP Adapter 目前显式 `not_implemented`。|
|`apps/grafana-plugin/frontend`|React/Grafana 工作台：创建 Session/Task、消费 Resource API、以连续 sequence 消费/重连 SSE、还原 URL、把执行结果映射为 Grafana DataFrame 与时序图。|当前页面只展示输入、状态、助手文本与三张图；未实现图表编辑和 Dashboard 写入。|
|`apps/grafana-plugin/backend`|Grafana Plugin SDK 的薄 Resource API 层。|从 Grafana 上下文提取身份、读取 `aiCoreEndpoint` 配置、代理请求与 SSE 字节流，并映射错误；不持久化业务数据、不调用 MCP。|
|`data/mock-scenarios/node_exporter_overview`|确定性场景数据：指标搜索、标签、三条查询结果、期望事件。|只供 MCP 的 Mock Prometheus Adapter 使用，并受 Schema 校验。|
|`scripts/`、`Makefile`、`tests/e2e/`|工程门禁、代码生成、契约/边界检查与端到端验收入口。|`compose.mock-e2e.yaml` 启动 assistant-mcp、AI Core 与 Grafana 三个容器。|

## 关键数据与依赖边界

- AI Core 独占业务持久化。SQLite 迁移在 `services/ai-core/migrations/sqlite/`，Plugin 和 MCP 都不能直接读写它。每条 Message 已持久化关联其 Task：User Message 与 `Task.inputMessageId` 双向一致，Assistant Message 也归属产生它的 Task；迁移会拒绝无法无歧义关联的旧数据。
- 同一 tenant/Session 最多允许一个非终态 Task，SQLite partial unique index 是并发竞争的最终约束。工具审计以内部稳定 source call ID 关联 start/completed/failed 记录，Mock Runtime 使用可重复的 source call ID。
- AI Core 内部 API 已提供按 `createdAt DESC,id DESC` 的 Session Message/Task keyset 分页，以及固定 `targetSequence` 的有限 JSON TaskEvent replay。page token 绑定资源类型及 Session/Task，不能跨接口或跨资源复用；当前浏览器仍只通过尚待补齐代理的 Plugin Resource API 访问这些能力。
- Mock 只位于 Adapter 层：Mock Agent 在 AI Core 的出站 Adapter，Mock Prometheus 在 MCP 的出站 Adapter；领域和工作流中没有 `mockMode` 分支。
- SSE 事件带有 Task、Session 和单调递增 sequence。事件先写入 durable store，客户端可通过 `afterSequence` 或 `Last-Event-ID` 获取断线后的后缀。
- 前端只能访问 Grafana Plugin Resource API；它不直连 AI Core、MCP 或 Prometheus。
- `scripts/check-boundaries.sh` 会阻止 AI Core 的 domain/application/ports import 外部 SDK、Adapter 或 Mock fixture。

## 尚未实现的范围

当前只覆盖一个固定 Mock 场景。以下是明确保留的后续能力，而非现有功能：真实 Agent/LLM、真实 Prometheus、PromQL 或图表编辑/重跑、Dashboard 写入与审批、真实 Grafana 写权限、知识库/Skill/Playbook、会话分享/Fork、告警和其他数据源。对应的部分 Port 或 Schema 已预留，但不能按“已实现”理解。

## 上手使用

### 环境准备

仓库锁定的版本是 Go `1.26.5`、Node.js `22.23.1` 和 npm `10.9.8`。体验完整 UI 还需要可用的 Docker Engine 与 Docker Compose。首次使用先安装前端依赖并执行启动检查：

```sh
cd apps/grafana-plugin/frontend && npm ci
cd ../../..
make bootstrap-check
```

`make bootstrap-check` 会明确报告版本不一致、Go 依赖/编译问题、前端类型问题或 AI Core 分层违规。

这里的各命令并不启动业务服务：`npm ci` 严格按锁文件安装前端依赖，`bootstrap-check` 负责确认本机能编译和测试当前骨架。需要看到实际页面时，再执行下一节的 Compose 启动命令。

### 在浏览器体验完整闭环

这是最适合手动试用的方式。以下命令会构建 Plugin 前端和三个服务，并在后台启动 Grafana、AI Core、assistant-mcp：

```sh
(cd apps/grafana-plugin/frontend && npm run build)
docker compose -f compose.mock-e2e.yaml up --build --wait
```

第一条命令用 Rollup 把 React 工作台打包为 `apps/grafana-plugin/dist/module.js`。第二条命令会构建并启动三个容器：

|容器|启动时做什么|它在本次体验中提供什么|
|-|-|-|
|`assistant-mcp`|读取工具 Schema 和 `node_exporter_overview` fixture，注册三个 `grafana.*` MCP Tool。|接受指标搜索、标签和 PromQL 查询请求，返回确定性 Mock 结果。|
|`ai-core`|创建/迁移自己的 SQLite 数据库，连接 `assistant-mcp` 的 `/mcp`；`/readyz` 还会确认三个工具均可列出。|持久化 Session、Task、Event、Chart 和执行结果，并对外提供 SSE。|
|`grafana`|安装前端产物和 Plugin Backend 二进制，通过 provisioning 写入 AI Core 地址。|提供登录、Plugin Resource API 和浏览器工作台；它不直接访问 fixture 或 MCP。|

Compose 为 AI Core 挂载了单独的 named volume，所以在容器运行期间的数据会保留；执行带 `-v` 的清理命令才会移除这次体验的数据。前端没有单独的开发服务器，页面和 Plugin Backend 都由 Grafana 容器提供。

容器均就绪后：

1. 打开 `http://127.0.0.1:3000`，用 `admin` / `admin` 登录。
2. 访问 `http://127.0.0.1:3000/a/mini-torchbearing-app/workbench`。
3. 输入任意非空分析请求，例如“帮我看看 node_exporter 最近 30 分钟的 CPU、内存和系统负载”，点击“开始分析”。
4. 应看到完成状态、固定的助手说明，以及“CPU 使用率”“内存可用率”“系统负载”三张图。刷新页面后，URL 中的 Session/Task 标识会使页面恢复结果。

当前实现不根据输入生成不同的分析计划；任何非空输入都会走同一个确定性 node_exporter Mock 场景。

### 点击“开始分析”后实际发生的事

下面是手动输入一段文本后的一次完整链路。理解这条链路基本就能把握当前骨架的运行方式。

|阶段|输入与去向|内部处理|可见/持久化输出|
|-|-|-|-|
|1. 创建会话|前端在没有 URL Session 时调用 `POST .../resources/sessions`，标题固定为 `Node exporter mock analysis`。|Plugin Backend 从 Grafana 登录态构造用户/组织/权限上下文，并代理到 AI Core。AI Core 在 SQLite 中创建 Session。|浏览器获得 `sessionId`。|
|2. 创建任务|前端把用户输入、`datasourceUid: mock-prometheus`、`relativeDuration: 30m` 和一个新的 idempotency key 发送到 `POST .../resources/tasks`。|AI Core 将相对时间冻结为当前时刻前 30 分钟到当前时刻；写入用户 Message、Task 和首个 `task.created` 事件。相同 key 与相同原始请求会返回同一个 Task，不同请求会得到 `idempotency_conflict`。|浏览器获得 `taskId`，并把 `sessionId`/`taskId` 写入 URL。|
|3. 启动工作流|Task 提交成功后，AI Core 在事务提交后异步运行工作流。|Task 依次进入 planning、running_tools、validating、completed；每次状态改变及后续事件都先写 SQLite，再通知 SSE 订阅者。|任务状态会实时变化，即使 SSE 断开也能从数据库重放。|
|4. Mock Agent 计划|Agent 只检查输入是否非空，不解析自然语言内容。|它固定搜索 `node exporter cpu memory load`，并固定选择 CPU、内存和负载三个指标；这正是未来真实 Agent 的替换点。|先看到“正在生成固定的 node_exporter 分析视图…”，最终固定文本为“已生成 node_exporter 的 CPU、内存和系统负载视图。”|
|5. 真实 MCP 通信|AI Core 的 MCP Gateway 通过 HTTP 调用 assistant-mcp，并携带从 Grafana 派生的身份上下文。|依次调用 1 次 `grafana.search_metrics`、3 次 `grafana.get_metric_labels`、3 次 `grafana.query_prometheus`。MCP 服务做权限、输入/输出 Schema 校验后才进入 Adapter。|SSE 出现对应的 `tool.started` / `tool.completed` 和指标候选事件。|
|6. Mock 数据读取|MCP 的 Mock Prometheus Adapter 按固定 PromQL expression 从 `data/mock-scenarios/node_exporter_overview` 读取数据。|查询不是访问真实 Prometheus；fixture 时间点会平移到本次请求的时间范围。固定表达式是 CPU 使用率、内存可用率和 `node_load1`。|每个查询返回 matrix 时序数据；没有匹配 fixture 的 PromQL 会被拒绝，而不会返回伪造成功。|
|7. 图表与页面恢复|AI Core 为三项结果创建 Chart 和 Execution，并写入事件流；前端订阅 `.../events?afterSequence=N`。|前端 reducer 只接收连续 sequence，缺号或断线会从最后序号重连；mapper 将持久化 series 转成 Grafana DataFrame 并交给 `TimeSeries`。|出现三张图及 PromQL 折叠区。刷新页面时，前端从 URL 读取 ID，再从 sequence 0 重放事件，因此能恢复相同结果。|

因此，当前“输入什么”只影响保存下来的用户 Message 和 Task 的幂等语义；“输出什么”由 fixture 和固定的三条 PromQL 决定。这样刻意收敛，便于先验证真实的跨进程协议、权限传递、持久化和 SSE 恢复，再替换 Agent 或 Prometheus Adapter。

运行中可查看日志：

```sh
docker compose -f compose.mock-e2e.yaml logs -f
```

结束体验并清理容器及本次 AI Core 数据：

```sh
docker compose -f compose.mock-e2e.yaml down -v --remove-orphans
```

### 单独调试后端服务

不需要 Grafana UI 时，可以在两个终端分别运行 MCP 与 AI Core：

```sh
# 终端 1
cd services/assistant-mcp && go run ./cmd/server
```

```sh
# 终端 2
cd services/ai-core
AI_CORE_SQLITE_PATH=/tmp/mini-torchbearing-ai-core.sqlite \
ASSISTANT_MCP_ENDPOINT=http://127.0.0.1:8081/mcp \
go run ./cmd/server
```

随后可检查健康状态：

```sh
curl -i http://127.0.0.1:8081/readyz
curl -i http://127.0.0.1:8080/readyz
```

assistant-mcp 会从当前目录向上寻找 fixture 和 Tool Schema，并在 `/mcp` 暴露 Streamable HTTP；AI Core 启动时会迁移 `/tmp/mini-torchbearing-ai-core.sqlite`，且 `/readyz` 会通过 MCP 实际列出工具，所以它同时验证 SQLite 与服务间通信。若 MCP 尚未启动或三个工具未注册，AI Core 的就绪检查会失败。这条路径方便观察服务启动与依赖故障；若要通过浏览器发起任务，仍应使用前一节的 Compose 方式，因为 Plugin Backend 由 Grafana 承载。

## 测试与检查入口

|命令|覆盖内容|本次结果（2026-07-14）|
|-|-|-|
|`make bootstrap-check`|固定 Go/Node/npm 版本；三个运行时 Go 模块全量编译测试；前端 typecheck；依赖边界。|通过。|
|`make test-ai-core-domain`|AI Core 领域、应用和 Port 的单元测试。|由 `make check` 通过。|
|`make test-sqlite`|SQLite Store 与内存事件通知器：CRUD、租户隔离、事务/幂等、sequence 与重放。|由 `make check` 通过。|
|`make test-assistant-mcp`|Mock Prometheus Adapter、MCP 接线和工具调用。|由 `make check` 通过。|
|`make test-ai-mcp`|AI Core MCP Gateway/查询 Adapter、HTTP API、分析工作流与 SSE。|由 `make check` 通过。|
|`make test-plugin-backend`|Grafana Resource API 代理、身份上下文、错误与 SSE 转发。|由 `make check` 通过。|
|`make test-frontend`|Vitest 工作台状态、SSE、路由、时间范围和 DataFrame mapper；随后 TypeScript typecheck。|通过：5 个测试文件、9 个用例。|
|`make test`|上述 Go 和前端测试的聚合入口。|由 `make check` 通过。|
|`make validate-contracts`|3 份 OpenAPI、21 份 JSON Schema 与 node_exporter fixture。|通过。|
|`make generated-client-diff`|重新生成 Client/类型后确认 Git 无差异。|通过。|
|`make lint`|Go 格式检查和前端 typecheck。|通过。|
|`make boundary-check`、`make secret-scan`|AI Core 依赖边界和常见私钥/AKIA 模式扫描。|通过。|
|`make check`|除容器 E2E 外的完整质量门禁：生成物、契约、lint、`make test`、边界与密钥扫描。|通过。|
|`make e2e-mock`|构建前端与三个容器；API E2E 校验幂等、事件 sequence 连续性、7 次工具调用、3 张图和 SSE 重放；Playwright 再验证浏览器提交和刷新恢复。|通过：容器健康后，API 脚本与 Playwright 用例均通过（1/1）。|

日常开发先运行 `make check`。需要一次性验证完整链路时，执行：

```sh
make e2e-mock
```

它会自行构建前端、启动 Compose、运行 API E2E 和 Playwright，然后删除它创建的容器与 volume；因此它是验收命令，不适合在结束后继续手动浏览。若已按“在浏览器体验完整闭环”启动容器，可分阶段执行相同的 E2E 用例：

```sh
tests/e2e/mock/api-e2e.sh
(cd apps/grafana-plugin/frontend && npm run test:e2e)
```

只调试某一层时，可使用表格中的 `make test-ai-mcp`、`make test-assistant-mcp`、`make test-plugin-backend` 或 `make test-frontend`；完整单元测试聚合入口为 `make test`。
