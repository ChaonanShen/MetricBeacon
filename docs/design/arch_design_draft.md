Grafana AI 助手架构设计说明
一、目标与定位
本系统是一套与 Grafana 强绑定的 AI 助手，面向 SRE、后端工程师、平台工程师等日常使用 Grafana 进行观测、排障、看板维护和告警处理的用户。用户可以用自然语言描述目标，例如“帮我生成某服务 P95 延迟图”“把这个面板改成按地域分组”“解释这次告警并补充排障图表”。AI 助手负责理解意图、收集 Grafana 上下文、调用数据和图表工具、生成或修改面板草稿，并在用户确认后写回 Grafana。

系统的核心设计原则是：Grafana 插件负责贴近用户和 Grafana 会话，AI 核心服务负责推理、编排、工具调用与持久化，MCP Server 负责把可执行能力以受控工具形式暴露出来，LLM 基础设施负责模型选择、降级和成本治理。
二、范围与边界
本期要做：
- 在 Grafana 内提供对话式入口、当前 dashboard / panel 上下文感知、多图表预览、图表 diff、会话列表和任务状态。
- 支持自然语言生成图表、编辑图表、解释图表、生成查询、补充变量、调整可视化配置。
- 支持 Grafana Alert Webhook 触发的告警解释、上下文聚合、排障建议和图表草稿生成。
- 支持人工确认后写回 Grafana，所有写操作可审计、可回滚、可追踪。
- 支持可复用能力沉淀，例如图表生成 skill、图表编辑 skill、告警解释 skill、报告 / 截图 / 生图 skill。
本期不做：
- 不绕过 Grafana 权限体系直接使用全局管理员凭证改 dashboard。
- 不把 AI 核心逻辑塞进 Grafana 前端或插件后端，插件后端只做薄代理、鉴权、上下文透传和少量边界校验。
- 不自动执行破坏性变更。删除面板、覆盖 dashboard、修改 alert rule 等动作必须进入确认流。
- 不承诺一次覆盖所有数据源。先以 Prometheus / Loki / Tempo / SQL 类主流数据源为主，通过 MCP Server 扩展。
三、总体架构
系统分为六层：
1. 输入与触发层：用户自然语言、Grafana 当前上下文、Grafana Alert Webhook。
2. Grafana 插件层：App Plugin 前端、Plugin Backend、Grafana Session / Org / RBAC 透传、Grafana API 受控代理。
3. AI 核心服务层：任务编排、意图识别、上下文构建、RAG 检索、MCP Client、skill 调度、结果合成、持久化。
4. MCP 工具层：Grafana 读写工具、数据查询工具、生图 / 改图 / 截图工具、外部知识和 runbook 工具。
5. LLM 基础设施层：模型路由、自动选择、失败降级、超时重试、预算控制、评测与安全策略。
6. 数据与治理层：会话、任务、草稿、审计、向量索引、对象存储、观测指标和告警。
推荐插件形态为 Grafana App Plugin + Backend Component。App Plugin 适合承载完整的产品体验，可以提供自定义页面、UI 扩展、会话列表和多图表工作区；Backend Component 通过 Resource Handler 暴露后端接口，用于接收前端请求、拿到 Grafana 插件上下文、透传用户身份和组织信息，并作为 AI 核心服务与 Grafana API 之间的受控边界。
四、模块设计
1. 输入与触发层
自然语言输入来自 Grafana 内的 AI 对话窗、dashboard 侧边栏、panel 编辑页、Explore 页面或独立 App 页面。每次请求除用户文本外，还要附带当前 Grafana 上下文：orgId、user、roles、dashboard UID、panel ID、time range、template variables、datasource UID、当前查询、选中数据点、告警规则等。

告警输入来自 Grafana Alerting 的 Webhook Contact Point。Webhook 入口必须校验 HMAC 签名或等效认证方式，提取 status、labels、annotations、values、generatorURL、dashboardURL、panelURL、fingerprint、startsAt 等字段，并将同一 fingerprint / groupKey 的事件做去重、聚合和状态更新。
2. Grafana 插件层
前端采用 React，主要组件包括：
- AI 对话窗：支持流式回复、引用来源、工具调用状态、错误恢复。
- 上下文面板：展示当前 dashboard、panel、数据源、变量、时间范围和告警上下文。
- 多图表工作区：展示 AI 生成的多个 panel 草稿、查询结果预览、截图预览。
- 图表 diff 视图：对比原 panel JSON 和 AI 修改后的 panel JSON，突出 datasource、query、fieldConfig、threshold、legend、layout 等变化。
- 会话列表：按 dashboard、alert fingerprint、用户、时间维度组织历史会话。
- 确认与回滚入口：写操作前展示计划、影响范围和回滚点。
插件后端采用 Go 和 Grafana Plugin SDK，职责保持轻量：
- 暴露 Resource Handler，例如 /chat/stream、/sessions、/tasks、/drafts、/approve、/grafana/proxy。
- 从 Grafana plugin context 中获取当前用户、组织、插件配置和安全配置。
- 将 Grafana session、org、user、roles、datasource UID、dashboard UID 等上下文透传给 AI 核心服务。
- 对 AI 核心服务的 Grafana 读写请求做权限检查、白名单路由、速率限制和审计埋点。
- 管理插件级配置，例如 AI Core 地址、租户映射、是否启用告警助手、可用 MCP 工具、默认模型策略。
插件后端不负责复杂推理，不保存长期会话状态，不直接持有可跨租户使用的全局写权限。
3. AI 核心服务层
AI 核心服务是系统的大脑，建议拆分为以下内部模块：
- API Gateway：接收插件后端和 Alert Receiver 的请求，处理鉴权、租户解析、幂等、限流、流式响应。
- Workflow Orchestrator：把用户需求转为可执行任务，按“理解意图、收集上下文、制定计划、调用工具、生成草稿、验证结果、等待确认、执行写入”的方式编排。
- Intent Router：识别任务类型，例如生成图表、编辑图表、解释面板、生成查询、解释告警、创建排障看板、总结事故。
- Context Builder：从 Grafana、数据源、告警事件、历史会话和知识库中组装最小必要上下文，避免把无关数据全部塞给模型。
- Policy Guard：检查权限、工具调用边界、危险操作、提示注入、敏感数据外泄和成本预算。
- MCP Client：统一调用 MCP Server，记录每次工具调用的输入、输出、耗时、错误和可重放信息。
- RAG Retriever：检索 dashboard 说明、服务目录、指标口径、日志字段、runbook、历史告警、最佳实践和图表样例。
- Skill Registry：沉淀可复用能力，例如 generate_panel、edit_panel、explain_alert、build_promql、build_loki_query、render_preview、generate_report_image。
- Result Composer：将模型输出、工具结果和验证反馈合成为用户可读结果，并给出可执行 diff 或草稿。
- Persistence Worker：持久化会话、任务、草稿、审批、审计、成本和评测数据。
4. MCP Server 层
MCP Server 是“可执行能力”的边界，每个工具必须有清晰 schema、权限要求、超时、幂等策略和审计记录。

建议的 MCP Server：
- Grafana MCP：搜索 dashboard、读取 dashboard JSON、读取 panel、读取 alert rule、读取 datasource metadata、解析变量、创建 panel 草稿、生成 dashboard patch、应用 panel patch、保存 dashboard version、回滚 dashboard。
- Load Skills MCP：获取Skills列表，加载对应skills的信息。
- Playbook MCP：获取Playbook列表，使用对应的Playbook功能。
- Knowledge MCP：访问 runbook、CMDB、服务目录、指标口径文档、历史事故库。
MCP 工具返回结果应尽量结构化，例如 panel JSON、JSON Patch、query validation result、data sample、preview image URL、risk assessment，而不是只返回自然语言。
5. LLM 基础设施层
LLM 基础设施应提供统一 Model Gateway：
- 自动选择模型：简单分类和字段抽取用低成本模型，复杂规划和多工具编排用强推理模型，截图理解和图表识别用视觉模型，向量检索用 embedding 模型。
- 失败降级：模型超时、限流或质量不达标时，按任务类型降级到备用模型或退回只读解释模式。
- 成本治理：按租户、用户、会话、任务类型统计 token、工具调用、模型费用，支持预算阈值和熔断。
- 安全治理：统一做敏感信息脱敏、提示注入检测、输出 schema 校验、工具调用 allowlist。
- 质量闭环：保留计划、工具调用、草稿和用户确认结果，用于离线评测和 prompt / skill 迭代。
6. 数据与持久化
建议的核心数据对象：
- Conversation：一次用户与 AI 的上下文会话，绑定 org、user、dashboard、alert fingerprint。
- Message：用户消息、AI 回复、工具调用摘要、引用来源。
- Plan：AI 生成的执行计划，包含步骤、风险、所需工具和确认点。
- ToolCall：每次 MCP 调用的输入输出摘要、耗时、状态、错误。
- PanelDraft：AI 生成的 panel JSON、JSON Patch、预览图、验证结果。
- Approval：用户对写操作的确认记录，包含确认人、时间、影响范围。
- AlertEvent：Webhook 事件、fingerprint、groupKey、状态、关联会话。
- AuditLog：所有读写 Grafana、调用外部数据源、模型调用和审批的审计记录。
存储建议：
- PostgreSQL：会话、任务、草稿、审批、审计、租户配置。
- Vector DB：runbook、dashboard 样例、历史事故、指标说明、查询样例的向量索引。
五、关键链路
1. 自然语言生成 / 编辑图表
1. 用户在 Grafana 内输入需求。
2. 前端附带 dashboard、panel、时间范围、变量、数据源等上下文，调用插件后端 Resource Handler。
3. 插件后端校验 Grafana session，补充 org、user、role、插件配置，并把请求转发给 AI Core。
4. AI Core 识别意图，构建上下文。
5. RAG 检索指标口径、runbook、历史 dashboard 和查询样例。
6. Workflow 调用 LLM 生成计划，再通过 MCP 查询数据源、生成 query、生成 panel JSON 或 JSON Patch。
7. 工具层验证 query 是否可执行、panel JSON 是否符合 Grafana 结构、变更是否触及危险字段。
8. AI Core 返回草稿、预览、diff、风险说明和建议。
9. 前端展示预览和 diff，用户确认后才进入写入。
10. 插件后端或受控 Grafana MCP 以当前用户权限应用变更，保存 dashboard 新版本。
11. 系统持久化会话、草稿、审批和审计。
2. Grafana Alert Webhook 触发
1. Grafana Alerting 向 Alert Receiver 发送 Webhook。
2. Alert Receiver 校验签名、解析 payload、按 fingerprint / groupKey 去重聚合。
3. AI Core 读取 alert rule、关联 dashboard、panel、query、最近数据点、日志和 trace。
4. RAG 检索 runbook、服务目录、历史相似告警和处理记录。
5. Workflow 生成告警摘要、可能原因、排障步骤、建议查询和可选图表草稿。
6. 如果用户打开 Grafana 插件，会看到该告警关联会话；也可以通过通知渠道发送摘要链接。
7. 任何自动生成的 dashboard / panel 修改仍需用户确认后写回。
3. 写操作确认与回滚
所有写操作采用“草稿优先”的模式：AI 先生成 draft 和 diff，用户确认后再执行。保存 dashboard 时必须带上 dashboard version 或等效并发控制；若版本冲突，返回前端重新合并。每次写入前保存回滚点，回滚操作也进入审计。
六、权限、安全与治理
- 权限继承：默认继承 Grafana 当前用户、组织和角色，不使用跨租户全局管理员 token。
- 最小权限：AI Core 不直接拥有任意 Grafana 写权限，写操作经插件后端或受控 MCP 代理校验。
- 人工确认：创建 / 修改 / 删除 dashboard、panel、alert rule、datasource 配置前必须确认。
- 工具白名单：每个租户可配置可用 MCP 工具和最大权限，例如只读、可写草稿、可写 dashboard。
- Webhook 安全：告警 Webhook 必须使用 HMAC、Bearer token、mTLS 或内网入口策略，防止伪造事件。
- 租户隔离：会话、向量索引、对象存储路径、模型调用上下文按 org / tenant 隔离。
- 敏感信息处理：对 secret、token、个人信息、日志中的敏感字段做脱敏，禁止进入长期记忆。
- Prompt Injection 防护：来自 dashboard 描述、panel 文本、日志、runbook 的内容都视为不可信数据，只能作为引用内容，不能提升工具权限。
- 审计与可观测性：记录用户、输入、计划、工具调用、写入 diff、审批人、模型、成本、耗时和错误。
七、接口草案
插件前端调用插件后端：
- POST /api/plugins/<PLUGIN_ID>/resources/chat/stream：提交用户消息，返回流式 AI 响应。
- GET /api/plugins/<PLUGIN_ID>/resources/sessions：查询会话列表。
- GET /api/plugins/<PLUGIN_ID>/resources/tasks/{taskId}：查询任务状态和草稿。
- POST /api/plugins/<PLUGIN_ID>/resources/tasks/{taskId}/approve：确认执行写操作。
- POST /api/plugins/<PLUGIN_ID>/resources/grafana/proxy：受控 Grafana API 代理，仅允许白名单接口。
Alert Receiver：
- POST /alerts/grafana/webhook：接收 Grafana Alert Webhook，校验签名后创建或更新 AlertEvent。
AI Core 内部 API：
- POST /v1/tasks：创建自然语言任务或告警任务。
- GET /v1/tasks/{taskId}：查询任务。
- POST /v1/tasks/{taskId}/approve：确认计划。
- POST /v1/tool-calls：MCP 工具调用网关。
- POST /v1/evaluations：记录用户反馈和质量评测。
八、部署建议
生产部署建议采用以下拓扑：
- Grafana 安装 AI Assistant App Plugin。
- Plugin Backend 随 Grafana 插件进程运行，连接 AI Core 内网地址。
- AI Core 部署在 Kubernetes 或等效平台，水平扩展 API Gateway、Workflow Worker、RAG Worker。
- MCP Server 按能力拆分部署，靠近数据源和 Grafana 所在网络。
- Model Gateway 统一连接内外部模型供应商，并提供降级、审计和预算控制。
- PostgreSQL、Vector DB、Redis / Queue、Object Store 作为共享基础设施。

本地骨架集成采用收敛的 Docker 混合 E2E 拓扑：
- 五个常驻容器：Grafana（承载 Plugin Frontend/Backend）、AI Core、assistant-mcp、Prometheus、node_exporter；Plugin build 是一次性构建任务。
- Prometheus 抓取 `node-exporter:9100`，Grafana provisioning 指向 `http://prometheus:9090` 的 datasource；AI Core 与 assistant-mcp 分别独占自己的 SQLite volume。
- Model 默认使用 Deterministic Mock，Knowledge/Playbook 可以使用 fixture；HTTP、SSE、MCP、Prometheus 查询、SQLite 恢复和 Grafana 新增 Panel 走真实链路。
- 保留 `mock-e2e` 与 `local-real` 两个 profile。前者负责可重复输出和权限/冲突/超时等错误注入，后者负责容器网络、插件加载、真实 API、Adapter 替换和写入授权回程。
- `local-real` 首个场景使用 node_exporter 的 CPU、内存和系统负载指标。真实数值不稳定，只断言 target 健康、查询结果非空、Schema/状态迁移正确以及审批后的 Dashboard 版本和 Panel 变化。
- 本地 Prometheus Adapter 可以直连 Prometheus，但不能据此声称已验证 Grafana datasource RBAC；生产查询是否必须经过 Grafana datasource proxy 由独立 Adapter/ADR 决定。

本地混合 E2E 是骨架后的集成增强目标，不替代契约、单元、Adapter Contract 和 Mock E2E，也不成为首个接口骨架提交的硬门槛。真实 Grafana 写入仍必须经过 `Draft → SaveIntent → Approval → Execute → Audit`，测试初始化凭证不得进入应用运行链路。
九、主要风险与应对
- 图表生成质量不稳定：用结构化 panel schema、查询验证、截图预览和用户确认降低风险。
- Grafana JSON 兼容性问题：按 Grafana 版本维护 panel 模板和迁移器，优先生成 patch 而非整份覆盖。
- 权限绕过风险：AI Core 不直接写 Grafana，所有写入经 session 透传和后端校验。
- 告警噪声过多：按 fingerprint / groupKey 去重，加入冷却窗口和事件聚合。
- 数据源差异大：通过 MCP 工具把不同数据源能力抽象为统一 schema，逐步扩展。
- 成本不可控：按任务类型选择模型，缓存上下文摘要，设置租户预算和熔断。
十、演进路线
MS1：完成插件骨架、AI Core 骨架、Grafana Context MCP、会话与任务模型，打通只读解释链路；建立 `mock-e2e`/`local-real` 双跑道及 Compose 骨架，真实链路跑通不作为接口骨架硬门槛。

MS2：支持自然语言生成 panel 草稿、查询验证、预览图、diff 和人工确认写回；用真实 Grafana、Prometheus、node_exporter 跑通本地 Golden Path。

MS3：支持编辑现有 panel、dashboard 多图表生成、回滚、审计和质量反馈。

MS4：接入 Grafana Alert Webhook，完成告警解释、排障建议、关联会话和通知链接。
沉淀 skill registry、更多数据源 MCP。
参考资料
- Grafana Plugin Tools: https://grafana.com/developers/plugin-tools/
- Grafana plugin types and usage: https://grafana.com/developers/plugin-tools/key-concepts/plugin-types-usage
- Grafana plugin backend system: https://grafana.com/developers/plugin-tools/key-concepts/backend-plugins/
- Add a backend component to an app plugin: https://grafana.com/developers/plugin-tools/how-to-guides/app-plugins/add-backend-component
- Add resource handler for app plugins: https://grafana.com/developers/plugin-tools/how-to-guides/app-plugins/add-resource-handler
- Grafana webhook notifications: https://grafana.com/docs/grafana/latest/alerting/configure-notifications/manage-contact-points/integrations/webhook-notifier/
