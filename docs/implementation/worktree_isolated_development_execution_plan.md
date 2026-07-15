# 多 Worktree 隔离与一键开发入口执行计划

> status: completed
> createdAt: 2026-07-15
> completedAt: 2026-07-15
> implementationAuthorized: true
> decision: developer-runtime isolation and orchestration only; no ADR or cross-process contract change required
> dependsOn: `current_codebase_overview.md`、`real_backend_test_matrix.md`

## 1. 目标

让多个 Git worktree 能同时开发、构建、运行和测试，而不会共享 Compose project、宿主端口、网络、
AI Core 数据卷、本地 SQLite 或浏览器登录态；同时提供一个统一的一键开发入口，消除当前脚本中分散的
project、端口、URL、依赖准备和清理逻辑。

- 无参数执行 `./scripts/mtb` 时准备依赖、编译前端、构建并启动默认 Mock 开发栈，保留容器供浏览器人工测试。
- `./scripts/mtb verify` 执行快速门禁和 Mock E2E；`--full` 执行 `make check` 和 Mock E2E。
- 自动 E2E 使用唯一临时 project 与 Docker 动态宿主端口，退出只清理本轮资源。
- 每个 linked worktree 使用根 `.env` 中的唯一 ID/slot；主工作区保留 `localhost:3000` 默认体验。

## 2. 锁定配置与命名

唯一工作区配置文件是已被 Git 与 Docker build context 忽略的根 `.env`。新增有效键：

```text
MTB_WORKTREE_ID
MTB_WORKTREE_SLOT
MTB_BIND_HOST
MTB_BROWSER_HOST
GRAFANA_ADMIN_USER
GRAFANA_ADMIN_PASSWORD
DEEPSEEK_API_KEY
DEEPSEEK_BASE_URL
DEEPSEEK_MODEL
```

ID 使用小写字母、数字和连字符，slot 限制为 0..9。开发端口固定由 slot 推导：Grafana 为
`3000 + slot*100`、AI Core 为 `8080 + slot*100`、assistant-mcp 为 `8081 + slot*100`。
slot 0 使用 `localhost`；其他 worktree 默认使用 `<id>.localhost` 隔离 Grafana Cookie。内部服务名、
内部端口、plugin ID、datasource UID 和容器内 SQLite 路径保持不变。

稳定开发 project 为 `mini-torchbearing-<id>-dev-<mode>`；E2E 与诊断 project 追加唯一 run ID。
named volume 和 network 继续由 Compose project 自动命名，不设置全局 `name` 或 `container_name`。

## 3. 命令接口

```text
./scripts/mtb [up] [--mode mock|real-metrics|real-agent]
./scripts/mtb init --slot N [--name ID] [--force]
./scripts/mtb verify [--full] [--mode ...]
./scripts/mtb e2e --mode ...
./scripts/mtb ps|logs|down [--mode ...]
./scripts/mtb reset --yes [--mode ...]
./scripts/mtb diagnose real-metrics|deepseek
./scripts/mtb run assistant-mcp|ai-core
./scripts/mtb doctor
./scripts/mtb config show|check
```

linked worktree 未初始化时必须安全失败；主工作区在没有新键时兼容 `main/slot 0`。`init` 原子更新
受管理键、保留 DeepSeek 配置并移除当前未被消费的旧 `MTB_*_URL` 键。`down` 保留 volume；只有带
`--yes` 的 `reset` 删除当前开发 project 数据。所有配置展示和日志隐藏 API key。

## 4. 执行门

1. G0：修正文档状态，激活本计划和进度记录。
2. G1：实现配置加载、init/check/show、工具链 doctor、前端依赖指纹和 worktree 本地运行目录。
3. G2：统一 Compose mode/project/port 编排，改造 E2E/诊断为唯一 project 与动态端口。
4. G3：实现无参数 up、快速/full verify、生命周期、本地服务入口与旧 Make/脚本兼容层。
5. G4：完成双 worktree 并发 Mock 验收、真实模式回归、全量门禁和当前快照更新。

每个独立行为切片小步提交，并在同一提交更新对应 progress/current snapshot。不 push、不建 PR、不
amend 或重写历史。

## 5. 验收标准

- 两个 worktree 的 ID、slot、开发端口、Compose project、network 和 AI Core volume 均不同。
- 同时运行两个 Mock E2E 均通过；任一测试退出不会停止或删除另一个测试的资源。
- 一个开发栈执行 down/reset 后，另一个栈及其 Session 继续可用。
- `./scripts/mtb` 完成依赖准备、一次前端 build、Compose build/up/wait 并输出浏览器 URL。
- `verify` 不重复安装未变化依赖或重复前端 build；快速和 `--full` 路径均通过。
- 现有三个 E2E、两个诊断入口、直接服务调试和 `make check` 保持可用。
- `.env`、API key、模型原文、内部 URL、完整时序和数据库内容不进入日志或 Git。
