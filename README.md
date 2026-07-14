# mini-torchbearing

Grafana 内嵌的自然语言指标分析工作台。当前默认运行确定性的 `node_exporter` Mock
闭环，并已提供可显式启用的本地真实 Prometheus/node_exporter 指标链路；文档状态、阅读路由和下一阶段方向见
[`docs/CLAUDE.md`](docs/CLAUDE.md)。

## 当前状态

有界 node_exporter 查询和持久化多轮工作台已经实现：用户可选择或用有限自然语言指定最近 30 秒至 6 小时、auto/注册 step；AI Core 持久化有效 QueryPlan，assistant-mcp 只按 `cpu|memory|load` view 生成规范 PromQL，最终数值回复由本地实际样本统计产生。Mock Agent 固定生成三图，显式 `eino` 模式下 DeepSeek 只选择所需 view。
`make e2e-mock`、`make e2e-real-metrics` 和有凭证的 `make e2e-real-agent` 已通过；Prometheus、MCP、DeepSeek 分层诊断、durable Task 结果语义分析和跨模式旧 Session 恢复也已完成。Grafana Dashboard 写入仍未实现。最终执行计划与证据见
[`docs/implementation/bounded_node_exporter_query_parameters_execution_plan.md`](docs/implementation/bounded_node_exporter_query_parameters_execution_plan.md) 和
[`docs/implementation/bounded_node_exporter_query_parameters_progress.md`](docs/implementation/bounded_node_exporter_query_parameters_progress.md)。独立探针设计见
[`docs/implementation/real_backend_diagnostics_execution_plan.md`](docs/implementation/real_backend_diagnostics_execution_plan.md)，分层命令顺序、预期结果形式和故障定位见
[`docs/implementation/real_backend_test_matrix.md`](docs/implementation/real_backend_test_matrix.md)。

## 模块边界

- `apps/grafana-plugin`：Grafana App Plugin。Frontend 只调用 Plugin Resource API；
  Backend 只代理到 AI Core。
- `services/ai-core`：会话、任务、事件、工作流和 SQLite 的唯一所有者。
- `services/assistant-mcp`：MCP transport、`grafana.*` 只读工具与指标源 Adapter。
- `contracts`：跨进程 OpenAPI、JSON Schema、SSE 和 Tool Schema 的唯一来源。
- `data/mock-scenarios`：仅供 Mock Prometheus Adapter 读取的确定性 fixture。

## 开发入口

```text
make bootstrap-check
make test
make check
make diagnose-real-metrics
make diagnose-deepseek  # requires DEEPSEEK_API_KEY
make e2e-real-agent  # requires DEEPSEEK_API_KEY
```

安装前端锁定依赖后再执行该命令：

```text
cd apps/grafana-plugin/frontend && npm ci
```

当前诊断进度和每个 Gate 的验证证据保存在
[`docs/implementation/layered_result_diagnostics_progress.md`](docs/implementation/layered_result_diagnostics_progress.md)。

## 本地演示

分别启动 `assistant-mcp`（`:8081`）和 AI Core（`:8080`），或运行 `make e2e-mock` 构建 Mock Compose；浏览器只通过 Grafana Plugin Resource API 访问系统。运行 `make e2e-real-metrics` 会在相同栈上叠加 Prometheus 和 node_exporter，并轮询 `up=1` 及至少两个 CPU idle scrape 后才发起真实查询。node_exporter 观察的是 Docker Linux VM/容器宿主的视图，不是 macOS 内核。

若只想判断真实数据断在哪一层，运行 `make diagnose-real-metrics`。它不启动 Grafana 或 AI Core：先直接检查 CPU、内存、load 三条 Prometheus 查询，再通过 assistant-mcp 的真实 transport 重复检查；输出仅包含阶段和 series/sample 计数，并在退出时清理自己的容器。

若只检查模型凭证、endpoint 和 model，可显式加载本地环境后运行直连探针；它不会自动读取 `.env`，也不会输出 key：

```text
set -a
. ./.env
set +a
make diagnose-deepseek
```

该探针先检查配置 model 是否出现在 `/models`，再要求最小 Chat Completion 返回严格 JSON `pong`。它绕过 Grafana、AI Core、Agent 与 MCP，适合把模型连通性问题和编排/工具问题分开。

三种手动 Compose 示例使用独立 project/AI Core volume。切换模式后若浏览器 URL 仍带上一模式的 Session/Task，Workbench 会在当前环境明确返回 `resource_not_found` 时清理旧 ID 并提示重新提交；网络、权限或其他依赖错误不会触发自动清理，而会显示在对话栏。

### Docker/Colima 前置检查

`make e2e-mock` 依赖 Docker Compose 和 BuildKit。使用 Homebrew Docker CLI + Colima
时，必须另外安装 `docker-buildx`；否则 Compose 会提示
`Docker Compose requires buildx plugin to be installed`，回退到 classic builder。该回退会
重复构建镜像、无法有效复用依赖缓存，首次下载和编译 Grafana Backend SDK 时容易表现为长时间
没有输出，但不代表 Go module proxy 一定不可用。

```text
brew install docker-buildx
docker context use colima
docker buildx use colima
docker buildx inspect colima
```

期望结果是 `colima` builder 的状态为 `running`。如 Homebrew 已安装插件但 Docker 找不到它，
按 `brew info docker-buildx` 的提示把 `/usr/local/lib/docker/cli-plugins`（Intel Mac）或
`/opt/homebrew/lib/docker/cli-plugins`（Apple Silicon）加入
`~/.docker/config.json` 的 `cliPluginsExtraDirs`。

在 Colima 环境中，`docker buildx ls` 可能同时显示：

```text
colima*   ...   running
default         error
```

这是因为 Colima 使用 `~/.colima/default/docker.sock`，而 Docker 的保留 `default` context
仍尝试连接 Linux 默认的 `/var/run/docker.sock`。只要当前 Docker context 和带 `*` 的 builder
都是 `colima`，该 `default` 错误不影响构建；不要为消除提示而创建
`/var/run/docker.sock` 软链接。Colima 的 `docker` driver builder 跟随当前 Docker context，
不要额外设置 `BUILDX_BUILDER=colima`；该环境变量会让部分 Compose/Buildx 版本把 builder
和 context 判为不匹配。确认 context 后直接执行：

```text
docker context use colima
docker buildx inspect colima
make e2e-mock
```
