# 多 Worktree 隔离与一键开发入口进度记录

> status: completed
> createdAt: 2026-07-15
> plan: [`worktree_isolated_development_execution_plan.md`](worktree_isolated_development_execution_plan.md)

## 执行状态

|阶段|状态|证据|
|-|-|-|
|G0：计划与文档基线|已完成|旧 UI 计划已依据后续 Planner 加固证据收口；本计划已激活。|
|G1：Worktree 配置与依赖准备|已完成|`scripts/mtb` 支持 init/config/doctor/deps；配置、迁移、脱敏、slot 端口和 lockfile 指纹测试通过。|
|G2：Compose 与 E2E 隔离|已完成|三种 mode 共用 worktree namespace；Mock E2E 以动态端口和唯一 project 完整通过并清理自身资源。|
|G3：一键入口与兼容层|已完成|无参数/up、快速/full verify、生命周期、诊断、本地服务和旧脚本/Make 兼容层已接入。|
|G4：并发与完整验收|已完成|两个真实 linked worktree 并发 E2E/开发栈隔离、full gate、三模式与诊断回归全部通过。|

## 安全边界

- 不读取、输出或提交当前 `.env` 的值；配置迁移必须保留 DeepSeek 凭证。
- 不共享写 SQLite，不改变 AI Core 数据所有权，不修改跨进程契约。
- E2E 清理只能命中本轮唯一 Compose project；开发 reset 只能命中当前 worktree/mode。

## G1 验证证据

- `node --test tests/diagnostics/worktree-config.test.mjs`：9 个配置/Compose 编排测试通过。
- 当前真实 `.env` 未被改写；`./scripts/mtb config show` 只显示 `<configured>`，不输出 key。
- `.env.example` 已改为实际消费的 ID/slot/host/admin 键，旧未消费 URL 键只由显式 `init` 迁移。

## G2/G3 验证证据

- `./scripts/mtb e2e --mode mock`：Docker 自动分配宿主端口；API 六种输入和 Playwright 1/1 通过；
  带 run ID 的容器、network 和 AI Core volume 在退出时删除。
- `./scripts/mtb verify`：契约、边界、AI Core MCP/API/workflow、Plugin Backend、26 个前端单测、
  44 个 diagnostics 测试及 Mock API/浏览器 E2E 全部通过。
- 使用临时 `main-test/slot 9` 启动长期开发栈后，`main-test.localhost:3900` 可访问；`down` 保留
  AI Core volume，`reset --yes` 只删除该 project 的 volume。
- 默认 slot 0 被现有手工栈占用时，`up` 在前端 build 前安全失败并提示选择其他 slot；未停止未知容器。
- `./scripts/mtb diagnose real-metrics`：动态端口上的原始 Prometheus 与 MCP CPU/内存/load 语义检查
  通过；本轮临时容器、network 和本地镜像已删除。
- `./scripts/mtb e2e --mode real-metrics`：六组真实指标 API 用例和 Playwright 1/1 通过；动态端口、
  唯一 project、volume 与 `--rmi local` 清理通过。
- `./scripts/mtb diagnose deepseek`：自动读取当前 `.env`，配置模型返回严格 JSON `pong`；未输出 key。

## G4 验证证据

- 从提交 `a9c3cde` 创建两个 detached linked worktree；分别执行 `init --slot 7 --name accept-a` 和
  `init --slot 8 --name accept-b`，未复制主工作区 `.env`。配置检查得到独立浏览器主机、开发端口、
  runtime 目录和 Compose namespace。
- 两个 worktree 同时执行 `./scripts/mtb e2e --mode mock`，均自动安装 556 个锁定前端包、完成六组
  API 用例和 Playwright 1/1。运行中 A/B 的 Grafana、AI Core、MCP 分别使用动态端口
  `32782/32780/32778` 与 `32783/32781/32779`；两个 project、network、volume 和镜像名互不相同，
  结束后分别完成自清理。
- 两个长期 Mock 开发栈同时在 `accept-a.localhost:3700` 与 `accept-b.localhost:3800` 返回 200。
  A 执行 `down` 后其 volume 保留，B 仍返回 200；A 执行 `reset --yes` 删除自己的 volume 后，B 仍
  返回 200。最后单独 reset B，并删除两个临时 linked worktree。
- `./scripts/mtb verify --full` 通过：生成物无差异、3 份 OpenAPI、24 份 JSON Schema、全部 Go
  测试、前端 9 files/26 tests、44 个 diagnostics、边界/密钥门禁以及动态端口 Mock API/Playwright
  E2E 全部成功。
- `./scripts/mtb e2e --mode real-agent` 通过：概览 21 events/3 query calls/3 charts，CPU 追问
  13 events/1 query call/1 chart，连续 8 轮 CPU+内存请求均为 17 events/2 query calls/2 charts；
  日志/SQLite 凭证与禁止标记扫描通过，临时 project、volume、network 和本地镜像已删除。
- 验收过程中未停止或修改用户已有的 `mini-torchbearing-real-agent-manual-*` 栈；收口时其五个容器
  仍在运行，其中三个应用容器保持 healthy。仅精确删除了本切片创建的临时 worktree 与测试资源。

## 提交记录

- `f0371d7 docs: plan isolated worktree development`
- `4cb9aed feat(dev): add worktree configuration tooling`
- `a9c3cde feat(dev): isolate compose development workflows`
