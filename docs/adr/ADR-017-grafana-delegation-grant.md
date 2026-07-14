# ADR-017：使用短期 Delegation Grant 访问 Plugin Backend 的 Grafana 受控代理

## 状态

Provisional（暂定，2026-07-13）

本 ADR 用于固定骨架接口方向，不代表真实 Grafana 写入机制已经完成验证。在实现真实 Grafana Write Adapter 前必须按“复审条件”做 Spike 和安全复审。

## 背景

系统需要同时满足：

- Grafana Plugin Backend 是登录态、Org、RBAC 和 Grafana API allowlist 的安全边界。
- AI Core 负责工作流、审批状态和 Eino Interrupt/CheckPoint 恢复。
- assistant-mcp 提供结构化 Grafana Tool，但不能持有跨租户管理员凭证。
- AI Core 和 assistant-mcp 不得保存或转发 Grafana Session、Cookie、Token、数据源 Secret。
- 用户确认写入后，恢复的 Agent Workflow 需要继续调用原 write tool，并记录完整审批和审计信息。

因此需要决定：审批通过后，assistant-mcp 如何在不获得用户原始凭证的前提下调用 Grafana。

## 暂定决策

采用短期、限定权限的 Delegation Grant：

1. 浏览器只调用 Plugin Backend。
2. Plugin Backend 校验当前 Grafana 登录态、Org、角色和权限。
3. 创建只读任务时，Plugin Backend 签发 read grant；用户确认写入时重新校验登录态和目标资源权限，再签发本次操作专用 write grant。
4. AI Core 和 assistant-mcp 只把 grant 当作不透明敏感值透传，不解析、不持久化、不记录日志。Grant 通过内部 transport metadata/header 传递，不进入 LLM 可见的 Tool 参数或 Tool Schema。
5. assistant-mcp 的 Grafana Adapter 携带 grant 回调 Plugin Backend 的内部受控代理。
6. Plugin Backend 验证 grant 的签发者、受众、有效期、租户、Org、用户、操作 scope、目标资源、approval ID 和幂等键。
7. 验证通过后，Plugin Backend 才调用 allowlist 内的 Grafana API，并返回结构化结果。
8. AI Core 持久化 DashboardSaveResult、任务事件和审计摘要；不持久化 grant。

该决策只规定授权边界和调用方向。Grant 的具体编码采用“签名自包含 Token”还是“服务端保存、调用方只持引用 Token”，由实现 Spike 后决定，但必须隐藏在以下接口后：

```go
type GrafanaGrantIssuer interface {
    Issue(ctx context.Context, request GrantRequest) (OpaqueGrant, error)
}

type GrafanaGrantVerifier interface {
    Verify(ctx context.Context, grant OpaqueGrant, expected GrantScope) (GrantClaims, error)
}
```

## Grant 最小语义

Grant 至少约束：

```text
grant_id
issuer / audience
tenant_id / org_id / user_id
allowed_operations
dashboard_uid / folder_uid / datasource_uid 等资源范围
approval_id（write grant 必填）
idempotency_key（write grant 必填）
issued_at / expires_at
```

约束：

- read grant 和 write grant 分开，不能升级 scope。
- write grant 单次、短时有效，只允许一个明确操作和目标资源。
- grant 不包含 Grafana Session、Cookie、API Key、Token 或其他可复用凭证。
- grant 不是 `grafana.add_panel` 等 MCP Tool 的业务入参；ToolGateway Adapter 从内部 RequestContext 注入 transport metadata，assistant-mcp 再映射为 ToolContext。
- grant 不能进入数据库、TaskEvent payload、Checkpoint、ToolCall summary、AuditEvent 或普通日志。
- assistant-mcp 不能绕过 Plugin Backend 直接访问 Grafana Write API。
- Plugin Backend 的代理不是通用 URL proxy，只能调用固定方法和路由。

## 调用流程

```text
Browser
  → Plugin Backend：用户点击确认
  → 重新校验登录态、Folder/Dashboard 权限
  → 签发 write grant（scope = add_panel + target + approval）
  → AI Core：Approve + Resume CheckPoint
  → assistant-mcp：grafana.add_panel(intent, approval evidence) + grant transport metadata
  → Plugin Backend controlled proxy：Verify grant + allowlist + version
  → Grafana API
  → assistant-mcp → AI Core：DashboardSaveResult
```

## 备选方案

### 方案 A：Plugin Backend 在审批请求中直接执行写入

优点：不需要跨服务 grant，授权链较直观。

未选择原因：需要新增“AI Core 返回写命令 → Plugin Backend 执行 → 回报 Tool Result → AI Core 恢复 Workflow”的外部工具执行协议，与 Eino Tool/Interrupt 的自然恢复路径结合更复杂。

### 方案 B：assistant-mcp 使用受控 Service Account

优点：调用链简单，适合后台任务。

未选择原因：容易弱化“继承当前用户权限”，扩大凭证权限和租户风险；除非真实 Grafana API 能力证明无法完成代理方案，否则不采用。

### 方案 C：把用户 Grafana Session/Token 透传给 AI Core 或 assistant-mcp

拒绝。会扩大敏感凭证暴露面，并与现有安全边界冲突。

## 影响

正面影响：

- 保持 Plugin Backend 为 Grafana 授权和写入边界。
- Eino Workflow 审批恢复后仍可按普通 Tool 流程继续执行。
- AI Core、assistant-mcp 无需持有长期 Grafana 管理员凭证。
- Grant issuer/verifier 和 Grafana executor 均可通过 Port/Adapter 替换。

代价和风险：

- 形成 `Plugin Backend → AI Core → assistant-mcp → Plugin Backend` 的调用环，需要严格超时、幂等和防循环设计。
- 多实例 Plugin Backend 下需要共享验证密钥或共享 grant 状态，并支持轮换。
- 真实 Grafana Plugin SDK/API 是否支持以原用户身份执行或等价地强制原用户权限，尚未完成 Spike。
- 长任务、审批等待和短 grant TTL 之间需要重新签发机制。
- Grant 泄漏虽然影响小于原始 Session，但在有效期和 scope 内仍是敏感授权。

## 复审条件

出现以下任一情况必须重新评审本 ADR：

1. 开始实现真实 `GrafanaDashboardWriteAdapter`。
2. Grafana 权限/代理 Spike 证明无法以当前用户身份或等价权限执行写入。
3. Plugin Backend 需要多副本部署。
4. 需要后台无用户在线的写操作。
5. 写操作从“新增 Panel”扩大到更新 Panel、Dashboard 或 Alert Rule。
6. 出现 grant 续期、撤销、重放或密钥轮换需求。
7. 安全评审认为回调环或 grant 模型不可接受。

## 实现前必须回答的开放问题

- Plugin Backend 调 Grafana API 时的实际执行身份是什么，如何证明没有权限提升？
- Grant 使用自包含签名 Token 还是服务端引用 Token？
- 多副本部署时如何验证、撤销和轮换？
- write grant 的 TTL、单次消费和重放防护如何实现？
- Eino CheckPoint Resume 失败或 Grafana 写入成功但结果回报失败时，如何用幂等键恢复？
- 审计日志如何关联 grant_id、approval_id、tool_call_id，同时确保不记录 grant 本体？

## 验收要求

真实写入启用前至少需要：

- Grant issuer/verifier 单元测试：过期、错误 audience、跨租户、scope 不符、资源不符、approval 不符。
- write grant 重放与幂等测试。
- Plugin Backend allowlist 测试，证明不能代理任意 URL/方法。
- 不落盘、不进日志、不进 CheckPoint 的敏感信息测试。
- 无权限用户和权限变更后的拒绝测试。
- Grafana Dashboard version conflict 测试。
- “Grafana 已写成功但回报超时”的恢复测试。

## 关联文档

- `docs/implementation/code_skeleton_design.md`：第 8、10、11、14、23、30、33 节。
- `docs/design/arch_design_draft.md`：Plugin Backend 受控边界、写操作确认。
- `docs/design/arch_design_detail.md`：P1 HITL、P4 Grafana Tool、Folder Permission。
