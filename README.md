# mini-torchbearing

Grafana 内嵌的自然语言指标分析工作台。当前默认运行确定性的 `node_exporter` Mock
闭环，并已提供可显式启用的本地真实 Prometheus/node_exporter 指标链路；文档状态、阅读路由和下一阶段方向见
[`docs/CLAUDE.md`](docs/CLAUDE.md)。

## 当前状态

有界 node_exporter 查询和持久化多轮工作台已经实现：Workbench 只提交自然语言，同步 Mock/Eino IntentPlanner 选择 `cpu|memory|load` 并规划可选 range/step；AI Core 本地校验边界、冻结并持久化 QueryPlan，后台只执行已持久化 views。assistant-mcp 生成规范 PromQL，最终数值回复由本地实际样本统计产生。
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

```sh
./scripts/mtb                         # 默认：构建并保留 Mock 开发栈
./scripts/mtb verify                  # 快速门禁 + 隔离 Mock E2E
./scripts/mtb verify --full           # make check + 隔离 Mock E2E
./scripts/mtb e2e --mode real-metrics
./scripts/mtb e2e --mode real-agent   # .env 需配置 DEEPSEEK_API_KEY
```

主工作区在未配置时默认使用 `main`、slot 0 和 `http://localhost:3000`。新建 linked worktree 后，
先选择未使用的 slot 初始化其根 `.env`：

```sh
./scripts/mtb init --slot 1 --name feature-one
./scripts/mtb config check
./scripts/mtb
```

slot 会同时决定该 worktree 的开发端口；非零 slot 默认使用 `<id>.localhost` 隔离 Grafana Cookie。
脚本会在 `package-lock.json` 变化或 `node_modules` 缺失时自动执行 `npm ci`，并为 Compose project、
network、AI Core volume 和自动测试分配 worktree/run 级命名空间。`.env` 不提交；不同 worktree 的
`.env` 可分别用 `init` 生成，已有 DeepSeek 配置会在显式重新初始化时保留。

当前诊断进度和每个 Gate 的验证证据保存在
[`docs/implementation/layered_result_diagnostics_progress.md`](docs/implementation/layered_result_diagnostics_progress.md)。

## 本地演示

`./scripts/mtb up --mode mock|real-metrics|real-agent` 会编译前端、构建并启动当前 worktree 的长期
开发栈；无参数等价于 Mock `up`。命令完成后容器和 AI Core 数据卷会保留，适合浏览器人工测试。
真实指标模式会叠加 Prometheus 和 node_exporter；node_exporter 观察的是 Docker Linux VM/容器宿主
的视图，不是 macOS 内核。

```sh
./scripts/mtb                    # Mock
./scripts/mtb up --mode real-metrics
./scripts/mtb up --mode real-agent
./scripts/mtb ps --mode mock
./scripts/mtb logs --mode mock
./scripts/mtb down --mode mock   # 保留 AI Core volume
./scripts/mtb reset --mode mock --yes  # 删除当前 worktree/mode 的数据
```

若只想判断真实数据断在哪一层，运行 `./scripts/mtb diagnose real-metrics`。它不启动 Grafana 或
AI Core：先直接检查 CPU、内存、load 三条 Prometheus 查询，再通过 assistant-mcp 的真实 transport
重复检查；输出仅包含阶段和 series/sample 计数，并在退出时清理自己的唯一临时 project。

若只检查模型凭证、endpoint 和 model，运行直连探针；统一入口会读取当前 worktree 的 `.env`，但不会
输出 key：

```sh
./scripts/mtb diagnose deepseek
```

该探针先检查配置 model 是否出现在 `/models`，再要求最小 Chat Completion 返回严格 JSON `pong`。它绕过 Grafana、AI Core、Agent 与 MCP，适合把模型连通性问题和编排/工具问题分开。

自动 E2E/诊断使用 Docker 动态宿主端口和唯一 project，退出只清理本轮容器、network、volume 与本地
镜像，不会碰当前 worktree 的长期开发栈或其他 worktree。现有 `make e2e-*`、`make diagnose-*` 和
`scripts/run-*.sh` 保留为兼容入口，内部均转发到 `scripts/mtb`。

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
./scripts/mtb verify
```
