# 多 Worktree 隔离与一键开发入口进度记录

> status: active
> createdAt: 2026-07-15
> plan: [`worktree_isolated_development_execution_plan.md`](worktree_isolated_development_execution_plan.md)

## 执行状态

|阶段|状态|证据|
|-|-|-|
|G0：计划与文档基线|已完成|旧 UI 计划已依据后续 Planner 加固证据收口；本计划已激活。|
|G1：Worktree 配置与依赖准备|已完成|`scripts/mtb` 支持 init/config/doctor/deps；配置、迁移、脱敏、slot 端口和 lockfile 指纹测试通过。|
|G2：Compose 与 E2E 隔离|待执行|—|
|G3：一键入口与兼容层|待执行|—|
|G4：并发与完整验收|待执行|—|

## 安全边界

- 不读取、输出或提交当前 `.env` 的值；配置迁移必须保留 DeepSeek 凭证。
- 不共享写 SQLite，不改变 AI Core 数据所有权，不修改跨进程契约。
- E2E 清理只能命中本轮唯一 Compose project；开发 reset 只能命中当前 worktree/mode。

## G1 验证证据

- `node --test tests/diagnostics/worktree-config.test.mjs`：7 个配置测试通过。
- 当前真实 `.env` 未被改写；`./scripts/mtb config show` 只显示 `<configured>`，不输出 key。
- `.env.example` 已改为实际消费的 ID/slot/host/admin 键，旧未消费 URL 键只由显式 `init` 迁移。
