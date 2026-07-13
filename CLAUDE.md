# CLAUDE.md

## 项目概览

本项目要做一个嵌入 Grafana 的“自然语言可观测性指标分析工作台”：用户用自然语言生成和编辑 PromQL/临时图表，在同一工作台中对比多张图，并把完整分析过程保存为可恢复、可分享、可 Fork 的会话；后续再从真实会话中沉淀 Skill、Playbook，并扩展告警分析。

当前仓库只有 `docs/` 下的设计文档，尚无实际骨架代码。文档中的目录、接口、数据表和部署方式均是待实现设计，不代表已经存在。

## 文档地图

- `docs/original_task.md`：最原始题目。定义问题、基础目标和远期愿景；用于确认项目初心，不用于判断当前阶段的详细范围。
- `docs/product_design.md`：当前产品范围和里程碑的主要依据。明确工作台形态、用户流程、MS1–MS4、验收指标和“不做”边界。决定“当前阶段做什么”。
- `docs/arch_design_draft.md`：长期总体架构草案。描述 Grafana App Plugin、薄 Plugin Backend、AI Core、MCP、模型基础设施和数据治理等六层，以及告警、审批、回滚和安全链路。
- `docs/arch_design_detail.md`：P1–P10（P7 拆为 P7a/P7b）共 11 份模块 Proposal 合集。包含更具体的模块接口、对象和已拍板决策；Proposal 本身仍是 Draft，而且不按 Milestone 排期，不能据此扩大当前阶段范围。
- `docs/code_skeleton_design.md`：后续生成骨架代码的权威实现蓝图。重点是 Monorepo、契约、Port/Adapter、状态机、SSE、SQLite→PostgreSQL、Mock、测试和分级验收。实现前应优先按此文档建立边界；发现缺口先修改文档或新增 ADR。
- `docs/adr/`：记录会影响模块边界、安全或长期演进的具体决策；Provisional 表示暂定方案，达到复审条件时必须重新评估。

阅读顺序建议：`original_task` → `product_design` → `arch_design_draft` → `arch_design_detail` → `code_skeleton_design`。

遇到口径差异时按问题类型判断：阶段范围以 `product_design.md` 为准；明确标注“拍板”的模块决策看 `arch_design_detail.md`；代码结构、Port、Adapter、数据所有权和验收方式以 `code_skeleton_design.md` 为准。若仍冲突，不要静默混用，先指出差异并向用户确认。

## 当前产品主线

- 用户：SRE、后端和平台工程师，尤其是告警排查、临时分析和发布验证场景。
- 入口：Grafana App Plugin，而非独立 Web、Panel Plugin 或修改 Grafana Core。
- 布局：会话/对话 + 多图画布 + 上下文区；文档里有左右/三栏措辞差异，核心都是“对话与多图工作区并存”。
- 默认产物：会话内的 Temporary Chart，不自动污染共享 Dashboard。
- 图表支持自然语言和手动编辑；必须能查看、修改、校验并重跑 PromQL。
- 会话是一级资产，不只是聊天文本；需要保存消息、上下文、图表、查询修订、执行结果、备注、分享/Fork 和写入记录。
- v1/MS1 聚焦 Prometheus/PromQL。Loki、Tempo、SQL、自动 RCA、语音等属于预留或后续演进。
- Skill/Playbook 的长期方向是“AI 辅助生成草稿 + 用户编辑确认”，不允许无人工直接沉淀或共享。

## 阶段理解

MS1 的目标是战略决策和可串联骨架，允许大量 Mock；不要把 MS2–MS4 或 Proposal 中的完整能力提前实现成 MS1 必需项。

首条纵向演示链路是：

1. 在 Grafana 插件中新建会话并输入“帮我排查 checkout 服务过去 30 分钟错误率升高”。
2. Plugin Backend 透传 Grafana 用户/组织/上下文并创建 AI Core Task。
3. Mock 指标目录、PromQL Skill 和查询引擎生成三张固定图。
4. 用户编辑一条 PromQL、重新执行，刷新后会话和图表仍可恢复。
5. 用户发起保存到 Dashboard，系统生成草稿和保存意图，确认后由 Mock Grafana Write 写入。
6. 能稳定重放无权限、版本冲突、AI 不可用等错误场景。

## 目标架构与关键决策

- 前端：React + TypeScript + Grafana UI/Runtime，消费任务事件并渲染多图画布。
- Plugin Backend：Go + Grafana Plugin SDK；保持薄层，只做登录态/RBAC、Grafana Context、受控代理、限流、追踪和错误映射。前端只调用它，不直连 AI、Prometheus 或 AI Core。
- AI Core：业务编排中心，负责会话、Agent/Workflow、上下文、策略、工具调用、修订、审批和持久化。
- Agent 的明确方向是字节 Eino：ChatModelAgent、Interrupt/CheckPoint、ChatModel Failover、Skill Middleware，以及 `eino-ext/components/tool/mcp`。
- MCP v1 的最新 Proposal 口径是一个 `assistant-mcp` 进程，通过 `grafana.*`、`knowledge.*`、`playbook.*`、`skills.*` 四个 namespace 暴露工具，使用 mcp-go + Streamable HTTP；达到复杂度阈值后再拆 server。
- 数据源 v1 只实现 Prometheus；其他 dialect 保留扩展点。
- Grafana Folder 是知识库、shared Skill 和 shared Playbook 的隔离及权限边界，不另造 Project ACL。
- Session 不按 Folder 硬隔离，而保存 `active_folder_uid`，允许会话内切换；消费侧通常聚合 active Folder、单一 Shared Folder 和用户自己的 private 对象。
- Skill 有双消费形态：内部 Agent 走 Eino Skill Middleware，外部 AI 工具走 Skills MCP；两者共享同一份 Skill 数据源。
- Skill/Playbook 可见性只有 `private` 和 `shared`。private 仅 Owner；shared 绑定 Grafana Folder。晋升必须由目标 Folder Admin 人工审批并审计。
- 告警链路拆成 P7a（HMAC、时间戳、防重放、fingerprint 幂等、异步投递）和 P7b（配置映射 Playbook、执行、超时、结果推送）；按产品 Roadmap 属于后续阶段。
- 持久化首选 SQLite，后续切换 PostgreSQL；两者必须实现相同 Repository/Store Port 和 Contract Test，业务层不得接触驱动、SQL 错误或数据库专有类型。
- AI Core 与 assistant-mcp 各自拥有存储，禁止共享写 SQLite 文件或跨服务直查数据库。
- Grafana 写入暂定采用 assistant-mcp 携带短期、限定 scope 的 delegation grant 回调 Plugin Backend 受控代理；该决策仍待真实 Grafana 权限 Spike，详见 `docs/adr/ADR-017-grafana-delegation-grant.md`。

## 不可破坏的工程约束

- 契约先行：跨模块先定义 OpenAPI、JSON Schema、MCP Tool Schema、SSE Event Schema、错误码和状态枚举，再写实现；不要在多个模块手写重复类型。
- 核心业务依赖 Port，不直接依赖 Grafana SDK、Prometheus API、模型厂商 SDK 或具体数据库驱动；Real/Mock 都通过 Adapter 注入。
- Mock 只能替换 Adapter，业务代码中禁止散落 `if mockMode`。
- `ChartDraft`（工作台临时图）与 `PanelDraft`（面向特定 Grafana 版本的待写对象）必须分离。
- 所有写操作走 Prepare → Intent/Diff → Approval → Execute → Audit，并使用版本检查；不得无确认覆盖或删除 Dashboard/Panel。
- AI Core 不得绕过 Plugin Backend 获得任意 Grafana 写权限；默认继承当前 Grafana 用户、Org 和 Folder 权限。
- Task 状态需要持久化；SSE 事件至少带 `taskId`、`sessionId` 和单调递增 `sequence`，支持断线重放。
- 结构化保存可展示计划、工具调用、查询修订和验证结果；不要持久化模型私有思维链。
- 外部模型只接收最小且脱敏的上下文；默认不发送完整时间序列、日志、Trace、身份、Token、内部 URL 或数据源 Secret。
- 所有工具调用、LLM 调用、用户操作、Playbook、Webhook、HITL 和晋升行为必须可审计；Secret 不得进入日志。
- 所有数据访问必须贯穿 tenant/org/user 上下文，禁止跨租户查询。

## 实现时的确认原则

- 技术基线已收敛为 Go + Eino、一个 `assistant-mcp`/四 namespace、SQLite 起步并通过 Adapter 切换 PostgreSQL。
- `arch_design_detail.md` 的旧段落仍可能保留多 server 等历史措辞；以其“重大决策”和 `code_skeleton_design.md` v1.1 的收敛口径为准。
- 产品 Roadmap 在 MS1 只要求 Playbook/Alert 等结构预留，而详细 Proposal 描述了完整模块；排期必须服从产品里程碑，Proposal 只提供目标设计。
- 如果疑问会改变模块边界、产品范围、权限模型、数据所有权或不可逆存储结构，应停止实现并直接向用户确认；不要自行补一个隐含决策。

## 开发时的默认做法

- 先查看仓库当前真实代码和 ADR，再对照文档；不要假设设计已实现。
- 修改跨模块字段时同步契约、生成客户端、Mock Fixture 和 Contract Test。
- 每个 Port 同时提供确定性 Mock 和 Real Adapter 的一致性测试。
- 优先让固定 Mock 场景跑通完整 E2E，再逐阶段替换 Adapter，避免重写上层流程。
- 保留用户确认、权限错误、无数据、超时、版本冲突和 AI 不可用时的可恢复路径。
