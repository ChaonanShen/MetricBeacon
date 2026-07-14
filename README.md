# mini-torchbearing

Grafana 内嵌的自然语言指标分析工作台。当前默认运行确定性的 `node_exporter` Mock
闭环，并已提供可显式启用的本地真实 Prometheus/node_exporter 指标链路；文档状态、阅读路由和下一阶段方向见
[`docs/CLAUDE.md`](docs/CLAUDE.md)。

## 当前状态

基本 Mock 闭环和持久化多轮工作台已经实现：输入任意非空消息会产生 node_exporter CPU、内存和负载三图。
`make e2e-real-metrics` 使用本地 Prometheus/node_exporter 验证同一 MCP/API 链路；真实 Agent/LLM 和 Grafana Dashboard 写入尚未实现。当前路线图正在执行，范围、安全边界和 Gate 见
[`docs/implementation/node_exporter_real_analysis_plan.md`](docs/implementation/node_exporter_real_analysis_plan.md)。

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
```

安装前端锁定依赖后再执行该命令：

```text
cd apps/grafana-plugin/frontend && npm ci
```

进度和每个 Gate 的验证证据保存在
[`docs/implementation/basic_mock_progress.md`](docs/implementation/basic_mock_progress.md)。

## 本地演示

分别启动 `assistant-mcp`（`:8081`）和 AI Core（`:8080`），或运行 `make e2e-mock` 构建 Mock Compose；浏览器只通过 Grafana Plugin Resource API 访问系统。运行 `make e2e-real-metrics` 会在相同栈上叠加 Prometheus 和 node_exporter，并轮询 `up=1` 及至少两个 CPU idle scrape 后才发起真实查询。node_exporter 观察的是 Docker Linux VM/容器宿主的视图，不是 macOS 内核。

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
