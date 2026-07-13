Proposals 总览
按《产品设计规范》"一次一个增量"原则，把项目 4 拆成 10 个模块维度 的 Proposal。每个 Proposal 是一个可独立评审、可实现、可验证的窄设计点，对应一份 Issue。

Proposal 不按时间阶段（Milestone）拆分。Milestone 由 Roadmap 文档单独管理，不在 Proposal 字段内。
拆分原则
- 按模块拆：每个 Proposal = 一个子系统的设计点（不是按"用户故事"或"特性"）
- 窄决策：每个 Proposal 解决一个清晰的设计点；相邻决策单独成 Proposal
- 例子即规范：现状 / 提议后的形态 / 边界与非法情况 = 评审和实现的依据
- 边界清晰：明确"做什么 / 不做什么"，不做的不留模糊地带
11 份 Proposal 清单
|#|文件|模块|状态|
|-|-|-|-|
|P1|p1-ai-agent-conversation.md|AI Agent 对话（核心编排）|✅ 草案|
|P2|p2-multi-chart-canvas.md|多图表画布与编辑（前端）|✅ 草案|
|P3|p3-session-management.md|会话持久化与分享|✅ 草案|
|P4|p4-mcp-servers.md|MCP Server 套件（4 server）|✅ 草案|
|P5|p5-knowledge-base.md|知识库联动（Grafana Folder 复用 + Web UI + 文档导入）|✅ 草案|
|P6|p6-playbook-engine.md|Playbook 结构化执行引擎|✅ 草案|
|P7a|p7a-alert-webhook-receiver.md|告警 webhook 接收（HMAC + 幂等）|✅ 草案|
|P7b|p7b-alert-async-analysis.md|告警 → AI 异步分析触发|✅ 草案|
|P8|p8-skill-system.md|Skill 系统（双形态）|✅ 草案|
|P9|p9-audit-log.md|审计日志（合规）|✅ 草案|
|P10|p10-promote-flow.md|Skills / Playbook 个人 → 共享晋升流程|✅ 草案|

拆分理由（与之前对比）：
- 之前 P7（"告警 → AI 异步分析"）混了"接收 webhook"和"异步触发 AI"两个决策。按"一次一个增量"，拆成 P7a / P7b。
模块依赖图（架构设计的输入）
flowchart TB
    P1[AI Agent 对话<br/>P1]
    P2[多图表画布<br/>P2]
    P3[会话持久化<br/>P3]
    P4[MCP Server 套件<br/>P4]
    P5[知识库 RAG<br/>P5]
    P6[Playbook 引擎<br/>P6]
    P7a[告警 webhook 接收<br/>P7a]
    P7b[异步分析触发<br/>P7b]
    P8[Skill 系统<br/>P8]
    P9[审计日志<br/>P9]
P10[Skills / Playbook<br/>晋升流程 P10]
    SharedFolder[Shared Folder<br/>跨项目共享]

    P2 --> P1
    P1 --> P3
    P1 --> P4
    P1 --> P5
    P1 --> P8
    P1 --> P9
    P1 --> P10
    P5 --> GrafanaFolderAPI[Grafana<br/>Folder API]
    P5 --> SharedFolder
    P6 --> P4
    P6 --> P10
    P6 --> GrafanaFolderAPI
    P6 --> SharedFolder
    P8 --> P10
    P8 --> GrafanaFolderAPI
    P8 --> SharedFolder
    P10 --> P9
    P10 --> GrafanaFolderAPI
    P10 --> SharedFolder
    P7a --> P7b
    P7b --> P1
    P7b --> P5
    P7b --> P6
    P4 --> P9
接口契约来源：
- 每个 Proposal 的「基本概念与信息结构」明确给出 对外接口（出参） 和 依赖接口（入参）
- 架构设计（architecture-design.md）据此组织模块串联
重大决策（2026-07-10 拍板，影响多份 Proposal）
- AI Agent 框架：字节 Eino
- Grafana Folder 复用：知识库 / Playbook / Skill 全部按 folder_uid（Grafana Folder UID）隔离；权限走 Grafana Folder Permission API（Admin/Edit/View）
- Folder 选择双机制：手动（UI 下拉 / /switch-folder）+ AI 自动推断（@ service 唯一性 + RAG 命中）
- Skills / Playbook 两层可见性：private（个人，仅 OwnerID）→ shared（绑定 Grafana Folder，复用 Folder Permission）
- P10 晋升流程：用户说"沉淀为 playbook/skill" → private → 申请共享到指定 Folder（项目 Folder 或 Shared Folder）→ Folder Admin 审批 → shared
- 会话级 active_folder_uid（P3 决策 5）：不按 Folder 硬隔离，AI Agent 消费侧按 folder_uid 过滤
- Shared Folder 模式（2026-07-10 拍板）：单一全局 Shared Folder；list 默认聚合 active + Shared + private；跨项目共享靠 Shared Folder 实现（无 global 层）
- MCP 集成：eino-ext/components/tool/mcp（Eino 官方）+ mcp-go 协议层
- HITL 写操作：Eino Interrupt & CheckPoint + approval Middleware（强制）
- LLM 主备切换：Eino ChatModel Failover（原生）
- Skills 双形态：
    - 内部 Agent → Eino Skill Middleware
    - 外部 AI 工具（Cursor 等）→ Skills MCP Server（差异化卖点）
- MCP Server 数量：3 → 4（+Skills MCP）
字段规范（每份 Proposal 必填）
按 01 规范的 8 字段：
1. 动机 / 用户故事 — 哪些真实场景触发，共同需求是什么
2. 目标用户 — 为谁而做
3. 现有做法及其不足 — 现在怎么解决，尚存哪些负担
4. 本提案范围与明确不做 — 做什么 + 明确不做什么
5. 关键决策与依据 — 备选方案 + 选择 + 理由（含竞品/替代）
6. 基本概念与信息结构 — 领域概念 + 数据组织（架构设计的输入），含对外接口 / 数据流向 / 与其他模块对接
7. 原型 / Demo — 文字定稿后才有，不强制
8. 验收标准 — 何为达成，具体可验证
额外规范（强化"例子即规范"）：
- 核心行为用 「现状 / 提议后的形态 / 边界与非法情况」 表达（不是抽象描述）
- 关键决策列备选方案 + 选择 + 理由（不只是"我们选 A"）
- 接口清单以表格 / mermaid 给出（出参 / 入参 / 数据格式）
状态流转
草案（proposal）         设计进行中，可讨论、可修改
  ↓ 讨论
通过（Proposal-Accepted）  定稿并进入开发；此后只读，变更另写新版
拒绝（Proposal-Denied）    方案不成立，记录理由后归档（属正常结果）
暂不排期（Proposal-NoPlan） 方案成立但本期不做
按改动粒度以 FullSpec / MiniSpec 区分：影响面大者用 FullSpec，小改动用 MiniSpec。
关联资源
- 架构设计：architecture-design.md
- 日报：daily-report-2026-07-09.md
- 团队规范 00 / 01 / 02 / 03 wiki
- 真实样例：xgo #2802 Flat Mode（FullSpec）/ #2667 defer（Denied）/ #2797 Function Decorators（MiniSpec）
- Eino 文档：https://www.cloudwego.io/zh/docs/eino/
下一步
1. ✅ 11 份 Proposal 草案已写完（已对齐最新决策）
2. ✅ 重组为模块维度（去掉 MS 标注，强化接口与对接）
3. ⏳ 提交 Proposal Issue（按 03 规范）→ 状态流转 Accepted
4. ⏳ 架构设计 Issue（贴 architecture-design.md）
5. ⏳ 空骨架 PR（所有 module interface + 模块串联）
6. ⏳ Roadmap 文档（管理 Milestone，与 Proposal 解耦）


P1: AI Agent 自然语言对话

标签：proposal FullSpec
所属模块：AI Agent 对话（核心编排）
创建日期：2026-07-10
状态：草案 → 讨论 → Proposal-Accepted / Denied / NoPlan
1. 动机 / 用户故事
真实场景：
- 工程师夜间值班，收到告警："支付服务 latency 升高"。他打开 Grafana，但找不到对应的 dashboard；要手动点击 5+ 个面板、切换时间范围、对比上周同期，耗时 10 分钟。
- 新人入职要看 "checkout 服务最近 7 天有没有异常"，他不知道该看哪些 metric，要问 mentor；mentor 截图截图发 Slack。
- 下午产品复盘，PM 想看"昨天的错误率分布"，但 dashboard 没这个 panel；要找 SRE 加。
共同需求：用一句自然语言描述想看什么，系统自动找出对应的 metric / log / trace、生成可视化。

一句话提炼：让 Grafana 团队成员不用会 PromQL，也能查到想要的指标。
2. 目标用户
- Grafana 团队 工程师（主要）：排查 / 看数 / 写报告
- Grafana 团队 SRE / 值班（高频）：告警触发后第一响应
- 周边团队（次要）：产品、运营，想验证某个假设
不为：
- ❌ 终端用户 / 客户
- ❌ 数据分析师（用专门 BI 工具）
- ❌ 完全不懂技术的角色（本系统仍需基本 Grafana 概念）
3. 现有做法及其不足
现状：
- 团队用 Grafana + Prometheus / Loki / Tempo
- 排查问题靠：手动打开 dashboard → 切换时间范围 → 切数据源 → 看 metric → 找 log → 找 trace
- 经验沉淀靠：wik Confluence 写 SOP、Slack 截图、notebook 记录
不足：
- 视角以事件堆叠：dashboard 是按"服务 / 环境 / 组件"组织的视图，不是按"问题"组织。同一问题跨多个 dashboard
- 手动切图：每次问 "checkout 延迟" 都要点 5+ 面板，新人不会切、老员工浪费切图时间
- 经验不沉淀：SOP 写一次后无人在意，复盘全靠人脑
4. 本期范围与明确不做
做（必做）
- 自然语言对话：用户在 Grafana 插件界面输入文字，系统返回文字 + 图表
- 多轮上下文：连续追问（"哪个服务最严重？" → 自动承接上文）
- 流式输出：LLM 输出逐字回显，体验不卡
- @ 提及上下文：用户主动 @datasource / @dashboard / @service 带入上下文
- 核心工具集：
    - 查询 PromQL / LogQL
    - 列出 / 搜索 dashboard
    - 创建 / 更新 panel
- 错误兜底：LLM 输出非法 JSON 时，将错误注入到工具响应中，让大模型读取错误信息。
- 认证透传：复用 Grafana session，插件必须在登录态使用
不做（明确边界）
- ❌ ML 时序异常检测：仅支持阈值告警
- ❌ 跨 workspace 联邦：单租户内
- ❌ 外部 AI 工具调 AI Agent（Cursor / Claude Desktop / IDE）：不做 agent-mcp
- ❌ 微调模型：仅通用模型
不影响主流程的可叠加能力（留作后续 Proposal）
以下能力不影响本提案的主流程，可以在后续独立 Proposal 中追加，不属于"不做"也不属于本提案展开范围：
- 多模态输入（截图 / 文件）：用户上传截图 / Markdown / PDF 作为上下文，与自然语言消息并列；Agent 解析后注入 context
- 实时语音交互：前端接入语音输入（ASR），Agent 回复 TTS 播报；不影响 Agent 主循环
5. 关键决策与依据
决策 1：AI Agent 框架选 字节 Eino
备选：
- A. 字节 Eino（选）— github.com/cloudwego/eino + eino-ext
- B. LangChain / LangGraph 等其他通用框架
- C. 走 Grafana Assistant 原生（托管云服务，已否决）
选择理由（2026-07-10 拍板）：
- ✅ HITL 原生支持：Eino Interrupt & CheckPoint + ChatModelAgentMiddleware 直接对应流程图第 12 节点（用户是否确认写入？）
- ✅ Skill Middleware 原生：Eino Skill Middleware + FileSystem Backend 直接处理 Skills
- ✅ ChatModel Failover 原生：主备切换 + 熔断内置
- ✅ eino-ext/components/tool/mcp：官方 MCP 工具组件，mcpp.GetTools(ctx, &mcpp.Config{Cli: cli}) 直接用，底层就是 mcp-go 客户端
- ✅ 资料完整 + 社区：字节维护、文档系统、ADK 实战案例多
- ❌ A 缺点：依赖外部框架演进
放弃 B 的理由：LangChain 系生态活跃但文档碎片化、HITL/Skill/Failover/MCP 都要靠社区拼装，与 Eino 中文官方文档 + 国内社区运维相比不利于本团队维护。
放弃 C 的理由：Grafana Assistant 是 Grafana Cloud 托管，不支持自建 / 数据出域。
决策 2：MCP 集成用 eino-ext 官方组件 + mcp-go 底层
备选：
- A. eino-ext/components/tool/mcp + mcp-go client（选）— Eino 官方桥接组件
- B. 自实现 MCP 协议栈
选择理由：
- ✅ eino-ext 是 Eino 官方扩展，与 ChatModelAgent 无缝集成
- ✅ 底层走 mcp-go，Streamable HTTP / OAuth / batching 跟社区演进
- ✅ 协议层零维护
- ❌ A 缺点：依赖外部库版本
放弃 B 的理由：协议演进自跟成本高、重复造轮子。
决策 3：LLM 主备切换用 Eino ChatModel Failover
备选：
- A. Eino ChatModel Failover（选）— 原生支持 primary/secondary，自动熔断
- B. 自写 chatmodel.Client 多路复用
选择理由：
- ✅ 原生支持主备 + 熔断 + 重试
- ✅ 触发条件（HTTP 429/5xx/timeout）成熟
- ❌ B 缺点：重复 Eino 已实现能力
决策 4：前端走 Grafana app plugin，不做独立 web
备选：
- A. Grafana app plugin（选）— 利用 @grafana/create-plugin + Plugin SDK Go
- B. 独立 React web app
选择理由：
- ✅ 复用 Grafana 面板渲染 / 主题 / 权限
- ✅ 与现有 dashboard / panel 集成自然（点 panel → 调 AI 解释）
- ❌ A 缺点：受 Grafana 插件 API 限制
放弃 B 的理由：部署复杂（多前端 + 跨域），风格难统一。
决策 5：MCP 传输协议用 Streamable HTTP（2025-03）
备选：
- A. Streamable HTTP（选）
- B. SSE（旧）
- C. stdio
选择理由：
- ✅ 最新 MCP 标准；mcp-go client 内置支持
- ✅ 跨主机部署友好（私有化部署多在 K8s）
- 放弃 B：被新版协议取代
决策 6：HITL 写操作审批用 Eino Interrupt & CheckPoint（核心新增）
备选：
- A. Eino Interrupt & CheckPoint + approval Middleware（选）— 框架原生
- B. 自实现审批状态机
选择理由：
- ✅ 对应流程图第 12 节点（"用户是否确认写入？"），框架原生能力
- ✅ CheckPoint 自动持久化、Resume 完整恢复状态
- ✅ approval Middleware 在 tool_call 阶段拦截，写操作类工具自动触发
- ❌ B 缺点：自实现约 800 行 + 状态持久化 + 恢复逻辑，得不偿失
决策 7：Skills 双形态（内部 Middleware + 对外 MCP）
备选：
- A. Eino Skill Middleware（内部）+ Skills MCP Server（对外）（选）
- B. 只用 Skill Middleware（不对外暴露）
- C. 只用 Skills MCP（外部 AI 消费）
选择理由：
- ✅ 内部 Agent 通过 Skill Middleware 直接读取（低延迟、无 MCP 协议开销）
- ✅ 对外通过 Skills MCP Server 让 Cursor / Claude Desktop 等外部 AI 工具也能消费（差异化卖点）
- ❌ B 缺点：外部 AI 工具消费不到，丢失差异化
- ❌ C 缺点：Agent 内部调 MCP 走协议栈，没必要
决策 8：Folder 上下文（手动 + AI 自动推断，P3/P5 联动）
核心立场：
- 不强制按 Folder 隔离会话（跨 Folder 对话合理）
- AI Agent 消费侧按 folder_uid 过滤（P5 复用 Grafana Folder 隔离）
- 手动选择 + AI 自动推断双机制
机制：
|机制|触发|实现|
|-|-|-|
|手动切换（主）|用户主动|UI 顶部下拉 / /switch-folder foo / @folder_name|
|AI 自动推断|用户问模糊问题|Agent 根据"服务名唯一性"+"检索命中"推断|

自动推断规则：
- @service_name 在唯一 Folder → 自动切换 Session.active_folder_uid
- @service_name 命中多 Folder → 提示"该服务在多个 Folder 中，请选一个"
- 模糊问题（无 @ 提及）→ Agent RAG 检索 Top-K，按"服务元数据 Folder 权重"打分 → 选 Top-1
- 推断置信度 < 0.7 → 反问用户
权限校验（关键）：
- 切换 Folder 前调 Grafana Folder Permission API 校验用户有 View 权限
- AI Agent 检索时也校验，避免泄漏无权限内容
Session 不按 Folder 硬隔离（P3 决策 5）：
- 同一会话可聊多个 Folder
- Session.active_folder_uid 是"当前上下文"
- 用户说"切到 search" / /switch-folder foo → 切换 active
6. 基本概念与信息结构
6.1 核心对象
// 会话上下文
type ChatContext struct {
    Mentions     []string     `json:"mentions"`        // @datasource/@dashboard/@service
    ServiceNames []string     `json:"service_names"`   // 已在 Service Catalog 的服务名
    Attachments  []Attachment `json:"attachments,omitempty"`
}

type Message struct {
    ID        string    `json:"id"`
    SessionID string    `json:"session_id"`
    Role      string    `json:"role"`     // user | assistant | tool
    Content   string    `json:"content"`
    Charts    []Chart   `json:"charts,omitempty"`
    CreatedAt time.Time `json:"created_at"`
}

// 流式事件（响应给前端）
type AgentEvent struct {
    Type    string          `json:"type"`     // message | tool_call | tool_result | chart | error | done
    Payload json.RawMessage `json:"payload"`
}
6.2 请求 / 响应模型
# 请求
ChatRequest:
  session_id: "uuid"
  user_id: "from-grafana-session"
  message: "checkout 服务这周延迟有没有异常？"
  context:
    mentions: ["@checkout-api", "@prometheus-prod"]
    folder_uid: "<grafana_folder_uid>"  # 可选：覆盖 Session.active_folder_uid（用于单次请求切换 Folder 上下文）
  model: "claude-sonnet-4"     # 可选；默认主模型
  temperature: 0.2             # 可选

# 响应（SSE 流）
events:
  - { type: "message", payload: { delta: "我看了一下" }}
  - { type: "tool_call", payload: { name: "grafana/query_prometheus", args: {...} }}
  - { type: "tool_result", payload: { result: {...} }}
  - { type: "message", payload: { delta: "数据看起来..." }}
  - { type: "chart", payload: { panel_json: {...}, chart_id: "..." }}
  - { type: "message", payload: { delta: "需要我对比上周吗？" }}
  - { type: "done" }
6.3 状态机（Eino ChatModelAgent + Interrupt + Folder 上下文 + Shared 聚合）
ChatRequest 接收
  ↓
加载 Session（含 active_folder_uid） + 历史消息（Eino Memory）
  ↓
确定本次 folder_uids（优先级：context.folder_uid > Session.active_folder_uid > 自动推断）
  ↓
  ├─ context.folder_uid 显式传入 → 校验 Grafana Folder Permission ≥ View → 用之
  ├─ Session.active_folder_uid 已设置 → 校验权限 → 用之
  └─ 都没设置 → AI 自动推断（@ service 唯一性 + RAG 命中权重）
       ├─ 推断置信度 ≥ 0.7 → 校验权限 → 切换 Session.active_folder_uid
       └─ 推断置信度 < 0.7 → 反问用户"请指定 Folder"
  ↓
**计算消费 folder_uids 列表** = [active_folder_uid, shared_folder_uid]（P4 决策 7 聚合）
  ↓
注入 Service Catalog（@ 提及的服务的元数据，按 folder_uids 聚合过滤）
  ↓
Skill Middleware 自动注入相关 Skill 内容到 system（按 folder_uids + visibility 过滤）
  ↓
构造 messages（含 system + 历史 + 当前 + context）
  ↓
Eino ChatModelAgent.Run()  → Stream 输出
  ├─ 普通文本 → 发 SSE message 事件
  ├─ tool_call（read：list_*）→ ToolsNode 调 MCP 工具（不传 folder_uid → 聚合 active + Shared + private）→ 发 SSE tool_call + tool_result 事件
  ├─ tool_call（read：get_*）→ 传 active_folder_uid → 权限校验 → 发 SSE 事件
  ├─ tool_call（write）→ approval Middleware 拦截 → Interrupt
  │     ↓
  │   等待用户审批（SSE interrupt 事件，携带 preview/diff/来源）
  │     ↓
  │   用户确认 → CheckPoint Resume → ToolsNode 执行 → 审计
  │   用户拒绝 → Tool 跳过，Agent 收到 "skipped" 反馈
  └─ 完成 → 发 SSE done 事件
  ↓
持久化本轮 messages + charts 到 Session Store
  ↓
审计：log_llm_call + log_tool_call + log_hitl_decision
关键映射（流程图节点 → Eino 组件）：
- 节点 4（识别意图）：PlanTask Middleware
- 节点 5（构建上下文）：Context Builder + Policy Guard
- 节点 6（RAG 检索）：Eino Retriever + Indexer
- 节点 7（ReAct 主循环）：ChatModelAgent
- 节点 8（工具调用）：ToolsNode + eino-ext/mcp
- 节点 10（结果校验）：Callback + JSON schema 校验
- 节点 11（流式）：SSE
- 节点 12（HITL）：Interrupt & CheckPoint + approval Middleware
- 节点 13（执行 + version check）：MCP 工具内置
- 节点 14（持久化 + 审计）：Callback + audit logger
7. 原型 / Demo
无界面原型（按 01 规范：先文字后原型；本提案定稿后做）。后续发独立原型 PR。

示意文本对话：
User: @checkout-api 这周 p95 latency 怎么样？

[Assistant]
我拉了一下数据：
- 本周 p95 中位数: 320ms
- 上周: 280ms
- 差异: +14%（偏离趋势）

[Panel: chart-P95-trend]  ← 自动生成并内嵌

要查错误率吗？

User: 看看今天的

[Assistant]
今天（截至 14:30）p95: 280ms - 410ms（下午开始走高）
[Panel: chart-today-p95]
...
8. 验收标准
必须达成
- [ ]  用户输入自然语言问题，系统返回文字 + 至少 1 个图表
- [ ]  多轮对话：连续 3 轮追问能正确承接上文
- [ ]  @ 提及 datasource / service：上下文正确带入
- [ ]  LLM 输出 panel JSON 非法时，自动降级（截断 + 重试 + 不暴露 stack trace）
- [ ]  流式输出：首字延迟 < 2s（SSE 起）
- [ ]  用户未登录：插件界面禁用 / 跳转登录
- [ ]  LLM 主备切换：注入 5xx 故障后第 2 次请求自动切到备模型
测试
- [ ]  单元测试：context 构建 / 消息格式化 / 错误处理
- [ ]  集成测试：mock LLM 跑 Playbook 端到端
- [ ]  性能：单次对话 LLM 调用 ≤ 8 个 tool rounds
数据
- [ ]  Grafana 团队 5 个工程师试用 1 周，可解决日常 60%+ 问题
- [ ]  平均"找到答案"时间从 10 分钟降到 2 分钟
不验收（明确边界外）
- 自动根因分析
- 多模态输入
- 跨 workspace
9. 关联资源
- 架构设计：architecture-design.md
- 团队规范 00/01/02/03 wiki
- 字节 Eino：https://www.cloudwego.io/zh/docs/eino/
- eino-ext MCP 组件：https://github.com/cloudwego/eino-ext/blob/main/components/tool/mcp/README_zh.md
- mcp-go：https://github.com/mark3labs/mcp-go
- MCP Streamable HTTP：https://modelcontextprotocol.io/specification/2025-03-26/basic/transports
10. 拆分与依赖
10.1 接口与对接
对外接口（本模块提供）：
|接口|类型|消费者|
|-|-|-|
|POST /chat (SSE)|HTTP/SSE 流式|P2 多图表画布|
|POST /chart/generate|HTTP|P2 多图表画布|
|GET /session/{id}/run-playbook|HTTP|P7b 告警异步分析|
|MCP tool: ai_conversation|MCP|外部 AI 工具（可选）|

依赖接口（本模块消费）：
|来源|接口|用途|
|-|-|-|
|P3 会话管理|SessionStore.Get/Put/UpdateActiveFolder|加载 / 保存会话 + 切换 active_folder_uid|
|P4 MCP 套件|tool.BaseTool[]（经 eino-ext/mcp 桥接）|Agent 工具调用|
|P5 知识库|Retriever.Retrieve(folder_uid, query, topK)|RAG 检索（按 folder_uid 过滤）|
|P5 知识库|Catalog.GetService(folder_uid, name)|注入服务元数据（按 folder_uid 过滤）|
|P5 知识库 / Grafana|GrafanaClient.CheckFolderPermission(uid, level)|校验 Folder Permission|
|P6 Playbook|Engine.Run(playbookID, params)|用户主动跑 Playbook|
|P8 Skill|skill.Backend（Middleware 自动注入）|Skill 召回（按 folder_uid + visibility 过滤）|
|P9 审计|Logger.Log*|全链路审计|
|P10 晋升|ApprovalService.SubmitRequest|用户说"沉淀为 playbook/skill"时触发|

数据流向：
flowchart LR
    UI[P2 前端] -->|SSE| P1[AI Agent]
    UI -->|/switch-folder| P3
    P1 --> P3[会话持久化<br/>active_folder_uid]
    P1 --> P4[MCP 工具]
    P1 --> P5[RAG 检索<br/>按 folder_uid 过滤]
    P5 --> GrafanaFolderAPI[Grafana Folder API<br/>Permission 校验]
    P1 --> P6[Playbook 引擎]
    P1 --> P8[Skill Middleware]
    P1 --> P10[P10 晋升<br/>沉淀草稿]
    P1 --> P9[审计日志]
    P7b[P7b 异步分析] -->|调 Run| P1
10.2 模块依赖
- 依赖 P3（会话管理）：本提案需 ChatRequest 中的 session_id + active_folder_uid 由 P3 提供
- 依赖 P4（MCP Server 套件）：本提案的 tool_call 全部走 P4 暴露的工具
- 依赖 P5（知识库）：RAG 检索 + Service Catalog 注入按 folder_uid 过滤
- 依赖 Grafana Folder API：权限校验
- 依赖 P10（晋升流程）：用户说"沉淀为 playbook/skill"时触发
- 被 P2 引用（多图表画布）：前端消费 P1 的 SSE 流 + chart 事件
- 被 P7b 引用（异步分析触发）：告警触发时复用 P1 的 RunPlaybook 子流程
11. 风险
|风险|缓解|
|-|-|
|LLM 输出 panel JSON 不合法|严格 schema 校验 + JSON 修复重试 + 降级提示|
|LLM 调太多工具轮次|Eino MaxToolRounds（默认 20）|
|长会话 token 累积|Eino Memory 短期滑窗（200 messages）|
|流式响应中断|SSE 自动重连 + request id 防重|
|插件鉴权绕过|Grafana session 透传 + 后端再验|
|写操作错误变更|Eino Interrupt & CheckPoint + approval Middleware（强制 HITL）|
|LLM Provider 故障|Eino ChatModel Failover（自动切备）|

12. 状态记录
- 2026-07-10：草案创建（Eino 方案）
- 2026-07-10：新增 Decision 6（HITL）/ Decision 7（Skills 双形态）
- 2026-07-10：新增 Decision 8（Folder 上下文：手动 + AI 自动推断）；状态机加 folder_uid 加载与过滤逻辑


P2: 多图表画布与编辑

标签：proposal FullSpec
所属模块：多图表画布与编辑（前端）
状态：草案
1. 动机 / 用户故事
工程师问 AI："今天上午 11 点的延迟为啥高？" AI 调用 3 个工具生成 3 个图表（p95 趋势 / 错误率 / 下游 trace 占比），用户希望同时看到这 3 个图表横向对比，而不是一个长滚动条看完才算完事。

共同需求：用户在一个视野里看多个图表，且能继续编辑（手动 + 自然语言）。
2. 目标用户
Grafana 团队工程师 / SRE（核心）。PM / 周边团队（次要）。
3. 现有做法及其不足
- AI 回复里嵌 1 个图表，看下一个问题切对话
- 想比较多个视角，必须手动打开多个 dashboard tab
- 编辑 panel 要先点进去 Grafana，再切回 AI
4. 本期范围与明确不做
做
- 同屏显示 4+ 图表（Charts Canvas）
- 手动编辑（拖动 / 删除 / 改变布局）
- 自然语言编辑（"把第二个改成按 region 分组"）
- Canvas 状态随 Session 持久化
- 单 panel 全屏查看（点击放大）
不做
- ❌ Grafana Scenes 复杂编排
- ❌ Canvas 导出 PDF / 图片
- ❌ 跨 Session 共享 Canvas
- ❌ 协作光标 / 多人实时编辑
- ❌ Canvas 模板市场
5. 关键决策
决策 1：UI 走 Grafana app plugin 三栏布局
- 选择：复用 Grafana AppPlugin 配置的页面 path，三栏 = 左对话 / 中画布 / 右上下文
- 理由：插件形态天然契合，与现有 dashboard 互不打扰
- 放弃：Scene App（更灵活但太重）
决策 2：图表渲染走 Grafana React 组件，不自实现
- 选择：用 @grafana/ui 的 PanelChrome + @grafana/runtime 渲染
- 理由：复用主题 / 数据源 API / 与原生 dashboard 一致
- 放弃：自定义 React Chart（重复造轮子）
决策 3：编辑入口双通道
- 自然语言："第二个图改成按 region 分组" → AI 重新生成 panel
- 手动：拖动 / 删除 / 修改参数直接同步
- 理由：自然语言编辑依赖 LLM，慢且不精确；手动实时可靠
6. 基本概念
type Canvas struct {
    ID         string         `json:"id"`
    SessionID  string         `json:"session_id"`
    Layout     string         `json:"layout"`     // grid-2x2 | grid-3x2 | flex
    Charts     []CanvasItem   `json:"charts"`
    UpdatedAt  time.Time      `json:"updated_at"`
}

type CanvasItem struct {
    ChartID  string          `json:"chart_id"`
    X        int             `json:"x"`
    Y        int             `json:"y"`
    W        int             `json:"w"`
    H        int             `json:"h"`
}
Canvas 是 Session 内嵌对象，不独立持久化。删 Session 时连带删。
7. 验收标准
- [ ]  单 Canvas 4+ 图表同屏渲染
- [ ]  手动拖动 / 删除生效，刷新页面后保留
- [ ]  自然语言编辑：1 轮内 panel JSON 更新并展示
- [ ]  单 panel 点击全屏查看
- [ ]  Session 关闭重开后 Canvas 状态恢复
8. 接口与对接
8.1 对外接口（本模块提供）
|接口|类型|消费者|
|-|-|-|
|React 组件 <AssistantCanvas />|前端组件|Grafana 插件主页面|
|POST /canvas/{session_id}/update|HTTP|AI Agent（P1）触发的自然语言编辑|
|GET /canvas/{session_id}|HTTP|页面刷新加载|

8.2 依赖接口（本模块消费）
|来源|接口|用途|
|-|-|-|
|P1 AI Agent|SSE 流式 chart 事件|接收 AI 生成的 chart|
|P1 AI Agent|SSE 流式 chart_update 事件|接收自然语言编辑后的 chart|
|P3 会话管理|SessionStore.GetCanvas(id)|加载 / 保存 Canvas|

8.3 数据流向
flowchart LR
    P1[P1 SSE 流] -->|chart 事件| Canvas[Canvas 组件]
    Canvas -->|手动编辑| Store[本地状态]
    Canvas -->|自然语言编辑| P1
    Store -->|持久化| P3[P3 Canvas Store]
    P3 -->|加载| Canvas
9. 依赖
- 依赖 P1：消费 SSE 流式事件中的 chart 事件
- 依赖 P3：Canvas 随 Session 持久化

P3: 会话持久化与分享

标签：proposal FullSpec
所属模块：会话持久化与分享
状态：草案
1. 动机 / 用户故事
"我上周二的 debug 对话，今天又遇到类似问题，能继续吗？" —— 同一个工程师的会话需要持久化、可检索。

"我找到这个根因了，能把这个会话发给同事参考吗？" —— 团队内需要会话分享。

"我想基于同事的会话自己改改看" —— fork 会话改造。
2. 目标用户
Grafana 团队全员。
3. 现有做法及其不足
- 用 Grafana 注释（Annotations）记录"事件 + 链接"
- Slack 私下发对话截图
- 没有结构化的会话历史
4. 本期范围与明确不做
做
- 会话持久化：UUID / 标题 / 状态 / 创建时间
- 消息持久化：role / content / charts 引用
- 会话列表：按时间 / 标题 / 服务名过滤
- 分享链接：基于 token，TTL 24h
- Fork：深拷贝完整消息历史，独立修改不影响原
- 归档：归档会话可读不可改
- 版本号乐观锁：避免并发覆盖
- 会话级 active_folder_uid（P5 联动，复用 Grafana Folder）：会话内可切换 Folder 上下文（同一会话可聊多个 Folder）
不做
- ❌ 实时协作（多人同时编辑一个会话）
- ❌ 评论 / 标注
- ❌ 跨 Grafana 账号迁移
- ❌ 会话导出 PDF
5. 关键决策
决策 1：存储用 SQLite 起步，可切 pgsql/mysql
- 起步 SQLite（零依赖、文件级）
- Store interface 抽象，配置选择实现
- 后期切 pgsql/mysql 不动业务代码
- 参考：https://github.com/openclaw/openclaw/pull/78595、https://hermes-agent.nousresearch.com/docs/developer-guide/session-storage
决策 2：Fork 深拷贝完整历史 = 创建会话的冻结副本（Snapshot 模型）
\核心立场\：用户只能从***\分享出去的 Snapshot\*** 进行 Fork，不能直接从 Session Fork。
- 拷贝 message + chart
- chart 内容用 inline 存储（不引用原 session id）
- 理由：避免"原会话删除后 fork 失效"
决策 3：分享链接基于短 token
- 8 字符随机 token，存 session_shares 表
- TTL 24h，过期自动清理，可用户自主设置过期时间。
- 撤销 Snapshot 后不再接受新 Fork（已 Fork 的不受影响）
- Snapshot 是"发布"动作的明确边界——分享者决定哪些内容可被 Fork
- 创建者可主动 revoke
- 访问者必须有 Grafana 账号 + 处于分享范围
决策 4：可见性 private / team 二档
- private：仅创建者可见
- team：所在 Grafana team 可见
- 不做"特定用户"或"特定组"细粒度
决策 5：会话级 active_folder_uid（不按 Folder 硬隔离，复用 Grafana Folder）
核心立场：
- Session 不按 Folder 隔离（跨 Folder 对话合理："对比 payment 和 search 的延迟"）
- Session 带 active_folder_uid 字段（当前上下文 Grafana Folder）
- 会话内可切换 Folder（用户说"切到 search" / /switch-folder foo / UI 顶部下拉）
与 P5 的关系：
- P5 知识库按 folder_uid 隔离数据（复用 Grafana Folder）
- Session.active_folder_uid 决定 AI Agent 用哪个 folder_uid 检索
- 用户在同一会话聊多个 Folder 是常见场景
- 权限复用 Grafana：切换 Folder 时校验用户在目标 Folder 有 Permission ≥ View
实现：
type Session struct {
    // ... 原有字段
    ActiveFolderUID   string   `json:"active_folder_uid,omitempty"`  // 当前上下文 Grafana Folder UID
    FolderHistory     []string `json:"folder_history,omitempty"`      // 切换历史（可选）
    // ... 原有字段
}
6. 基本概念
type Session struct {
    ID         string    `json:"id"`
    UserID     string    `json:"user_id"`
    Title      string    `json:"title"`
    Status     string    `json:"status"`      // active | archived
    ForkedFrom *string   `json:"forked_from,omitempty"`
    Visibility string    `json:"visibility"`  // private | team
    ActiveFolderUID string `json:"active_folder_uid,omitempty"`  // P5 联动：当前上下文 Grafana Folder
    Version    int64     `json:"version"`     // 乐观锁
    CreatedAt  time.Time `json:"created_at"`
    UpdatedAt  time.Time `json:"updated_at"`
}

type SessionShare struct {
    SessionID  string    `json:"session_id"`
    Token      string    `json:"token"`       // 8 字符
    CreatedBy  string    `json:"created_by"`
    ExpiresAt  time.Time `json:"expires_at"`
    Revoked    bool      `json:"revoked"`
}
7. 验收标准
- [ ]  Session CRUD 完整工作
- [ ]  多端编辑：同一 session 用乐观锁防止覆盖
- [ ]  Fork 后独立修改不影响原 session
- [ ]  分享链接：未登录访问被拒 / 登录访问者不在范围被拒
- [ ]  过期 / revoke 后访问被拒
- [ ]  删除 Session：所有相关消息 / chart / share 级联清理
8. 不做（边界）
- 跨账号迁移
- 全文搜索
9. 接口与对接
9.1 对外接口（本模块提供）
|接口|类型|消费者|
|-|-|-|
|SessionStore.Get/Put/List/Delete/Fork|Go interface|P1 AI Agent|
|SessionStore.UpdateActiveFolder(sessionID, folderUID)|Go function|P1 / Web UI|
|CanvasStore.Get/Put|Go interface|P2 多图表画布|
|ShareToken.Create/Verify/Revoke|Go interface|P1 / P2|

9.2 依赖接口（本模块消费）
|来源|接口|用途|
|-|-|-|
|SQLite / pgsql / mysql|Store interface 实现|持久化（SQLite 起步，可切换）|
|Grafana Folder API|GET /api/folders/:uid|校验 active_folder_uid 合法 + 用户权限|

9.3 数据流向
flowchart LR
    P1[P1 AI Agent] -->|Get/Put Session| P3
    P1 -->|UpdateActiveProject| P3
    P2[P2 Canvas] -->|Get/Put Canvas| P3
    UI[Web UI 顶部下拉] -->|切换项目| P3
    P5[P5 Catalog] -->|校验 project_id| P3
    P3[Session Store] --> SQLite[(SQLite)]
9.4 例子即规范
场景：用户在同一会话聊多个 Folder
User: "看看 payment 的 checkout 延迟"
[P1] 加载 Session.active_folder_uid=null → AI 提示"请选择 Folder"
User: 点 UI 顶部下拉选 "payment"
[前端] 校验用户在 payment Folder 有 View 权限（调 Grafana API）
[前端] P3.UpdateActiveFolder(sessionID, "payment")
User: "查一下 checkout 的 p95"
[P1] 加载 Session.active_folder_uid=payment → 按 payment Folder 检索
User: "再看看 search 的搜索 QPS"
[P1] AI 提示"是否切换到 search Folder？"
User: "是的" → UI 切换 → P3.UpdateActiveFolder(sessionID, "search")
User: "查一下搜索服务的 QPS"
[P1] 按 search Folder 检索
边界与非法情况：
- ❌ 用户切换到无权限的 Folder → 拒绝（基于 Grafana Folder Permission 校验）
- ❌ active_folder_uid 在 Grafana 已被删除 → Session 启动时检测 + 提示用户重选
- ❌ 用户没有目标 Folder 的 View 权限 → 拒绝切换
- ✅ 会话可跨 Folder 切换，不丢失历史消息
10. 依赖
- 被 P1 依赖：ChatRequest 需要 session_id + active_folder_uid
- 被 P2 依赖：Canvas 随 session 存
- 被 P5 联动：active_folder_uid 决定知识库检索范围（复用 Grafana Folder）
- 依赖 Grafana Folder API：校验 folder_uid 合法 + 用户 Permission ≥ View

P4: MCP Server 套件

标签：proposal FullSpec
所属模块：MCP Server 套件（4 server）
状态：草案（2026-07-10 修订：4 server + Eino 集成）
1. 动机 / 用户故事
AI Agent 要查 Grafana 数据、要跑 Playbook、要检索知识库、要消费 Skills。这些能力不能塞进 Agent 内部（绑死、难调试、难复用）。把它们做成 MCP 工具，AI Agent 通过 MCP 协议调用。

当前实现：1 个总 MCP server 进程，按 namespace 分组（grafana / knowledge / playbook / skills）。

额外价值：Skills MCP Server 同时供外部 AI 工具（Cursor / Claude Desktop / IDE）消费，是项目 4 差异化卖点之一。
2. 目标用户
- 主要：P1（AI Agent），Agent 是 Server 的客户端
- 次要：外部 AI 工具（Cursor / Claude Desktop），通过 MCP 客户端连接 Skills MCP
3. 现有做法及其不足
- AI Agent 内部硬编码所有能力
- 不同能力混在一个代码包里，调试和替换都难
- 不同能力可能由不同团队维护（grafana 工具由 SRE 团队、knowledge 工具由文档团队等，当前都在同一个 MCP server 进程内）
- 沉淀的 SOP / Skills 无法被外部 AI 工具复用
4. 本期范围与明确不做
做（1 个总 MCP server，namespace 分组）
核心立场：当前 4 类能力（grafana / knowledge / playbook / skills）合并为1 个总 MCP server，通过 namespace 区分工具来源。
|Namespace|工具集（v1 实现）|协议|
|-|-|-|
|grafana|query_prometheus / list_dashboards / list_alerts / create_dashboard / update_panel / delete_dashboard|Streamable HTTP|
|knowledge|list_services / get_service / upsert_service / delete_service / list_runbooks / get_runbook / upsert_runbook / delete_runbook / search_docs / get_doc / ingest_doc / ingest_documents / update_document / delete_document|Streamable HTTP|
|playbook|list_playbooks / get_playbook / explain_playbook / run_playbook / create_playbook / update_playbook / delete_playbook|Streamable HTTP|
|skills|list_skills / search_skills / get_skill / load_skill_for_agent / run_skill / create_skill / update_skill / delete_skill / promote_skill|Streamable HTTP|

工具命名约定：{namespace}.{tool_name}，例如 grafana.query_prometheus / knowledge.list_services
- 1 个 server 跑在 Streamable HTTP 协议上
- Server 用 github.com/mark3labs/mcp-go 库实现
- 1 个进程 1 个部署单元
- AI Agent 用 github.com/cloudwego/eino-ext/components/tool/mcp 通过 mcp-go client 一次连接拿所有工具
不做
- ❌ agent-mcp（不让外部 AI 工具调我们的 AI Agent）
- ❌ MCP Server OAuth（用 header bearer token 即可）
- ❌ MCP Server 集群 / 高可用
- ❌ MCP Sampling（客户端向 LLM 采样的能力）
5. 关键决策
决策 1：Server 端用 mcp-go 库实现，不用 JSON-RPC 手搓
- mcp-go 库提供 server 模板，直接定义 tools / handlers
- 一个 server 一个进程，简单清晰
- 与 eino-ext/mcp 共享协议层（mcp-go client + server）
决策 2：协议统一用 Streamable HTTP
- 跨主机部署、私有化部署友好
- 不用 stdio（同主机耦合）
- 不用 SSE（旧协议）
决策 3：权限分 read / write 两档 + HITL
- read：默认无需审批
- write（create/update dashboard / create_skill / update_skill / ingest_doc）：HITL 审批
- 标记在 MCP tool 元数据上，Agent 调工具时由 approval Middleware 拦截
- HITL 由 Eino Interrupt & CheckPoint 实现（见 P1 Decision 6）
决策 4：每个 Server 独立部署
- 不在一个进程里多 server
- 部署独立（Grafana MCP 可能 SRE 团队部署，Knowledge MCP 可能文档团队部署）
- 配置中心化（MCP config 文件统一管理）
决策 5：AI Agent 端集成用 eino-ext/mcp（2026-07-10 新增）
备选：
- A. eino-ext/components/tool/mcp（选）— Eino 官方 MCP 工具组件
- B. 自实现 eino tool.BaseTool 适配器
选择理由：
- ✅ eino-ext 是 Eino 官方扩展，与 ChatModelAgent 无缝集成
- ✅ mcpp.GetTools(ctx, &mcpp.Config{Cli: cli}) 一行拿到全部工具
- ❌ B 缺点：重复 eino-ext 已实现能力（schema 转换 / 参数序列化 / 错误处理约 400 行）
import mcpp "github.com/cloudwego/eino-ext/components/tool/mcp"

// 每个 server 一个 client
func setupMCPTools(ctx context.Context, servers []string) ([]tool.BaseTool, error) {
    var allTools []tool.BaseTool
    for _, name := range servers {
        cfg := loadMCPConfig(name)
        cli, _ := client.NewStreamableHttpClient(cfg.URL)
        ts, _ := mcpp.GetTools(ctx, &mcpp.Config{Cli: cli})
        allTools = append(allTools, ts...)
    }
    return allTools, nil
}
决策 6：Skills MCP 对外暴露（差异化卖点）
备选：
- A. Skills 既走 Eino Skill Middleware（内部）又走 Skills MCP（对外）（选）—— 双形态
- B. 只用 Skill Middleware（不对外暴露）
选择理由：
- ✅ 内部 Agent 通过 Skill Middleware 直接读取（低延迟、零协议开销）
- ✅ 对外通过 Skills MCP 让 Cursor / Claude Desktop / IDE 等外部 AI 工具也能消费项目 4 沉淀的 Skills（差异化卖点）
- ✅ Skills 物理文件只有一份（data/skills/*.md），Eino Skill Backend 和 Skills MCP Server 都从同一目录加载
- ❌ B 缺点：外部 AI 工具消费不到，差异化卖点丢失
决策 7：MCP tool 入参含 Shared Folder 聚合逻辑（2026-07-10 新增）
Shared Folder 约定：
- Grafana 团队约定一个单一全局 Shared Folder（UID 由 config 配置）
- 所有跨项目共享的 playbook / skill / runbook 申请时目标 Folder 选这个
- 默认约定名称：Shared（可在 Grafana 中手动建）
List 类工具默认行为（不带 folder_uid 参数时）：
// GrafanaFolderUIDConfig 约定的 Shared Folder UID
type FolderConfig struct {
    SharedFolderUID string `yaml:"shared_folder_uid"`  // Grafana Folder UID
}

// 内部逻辑：list_* 不传 folder_uid → 默认聚合
func (s *Service) ListPlaybooks(ctx, userID string, folderUID *string) ([]*Playbook, error) {
    if folderUID == nil {
        // 默认行为：active_folder + Shared Folder + 自己的 private
        userFolders, _ := s.grafanaClient.ListUserFolders(ctx, userID, LevelView)
        folderUIDs := []string{s.sessionStore.GetActiveFolder(userID)}
        if s.config.SharedFolderUID != "" {
            folderUIDs = append(folderUIDs, s.config.SharedFolderUID)
        }
        // 加上用户自己 private 的（folder_uid = ""）
        return s.catalog.ListPlaybooksByFolders(ctx, folderUIDs, userID)
    }
    // 精确指定 folder_uid → 只查该 Folder
    return s.catalog.ListPlaybooks(ctx, *folderUID)
}
MCP tool 入参（增加 folder_uid 可选参数）：
playbook.list_playbooks() → 默认聚合（active + Shared + private）
playbook.list_playbooks(folder_uid: "abc") → 只查 abc Folder
playbook.list_playbooks(folder_uid: "Shared") → 只查 Shared Folder

skills.list_skills() → 默认聚合
skills.list_skills(folder_uid: "abc") → 只查 abc Folder

knowledge.list_services() → 默认聚合
knowledge.list_services(folder_uid: "abc") → 只查 abc Folder
Get / Run / Upsert 类工具：必须传 folder_uid（用于权限校验）

为什么 AI Core 自动注入：
- AI Core 加载 Session.active_folder_uid 后
- List 类工具自动传 active + Shared（聚合）
- Get / Run 类工具自动传 active（用于精确权限校验）
决策 8：权限校验统一走 FolderPermissionService（2026-07-10 新增）
三个 MCP server 共享一个 FolderPermissionService（避免重复实现）：
// 由 knowledge / playbook / skills namespace 共享
type FolderPermissionService interface {
    CheckPermission(ctx, userID, folderUID string, required Level) (bool, error)
    ListUserFolders(ctx, userID string, minLevel Level) ([]Folder, error)
    BatchCheckPermission(ctx, userID string, folderUIDs []string, required Level) (map[string]bool, error)
}

type Level int
const (
    LevelView  Level = 1
    LevelEdit  Level = 2
    LevelAdmin Level = 4
)
缓存：5min TTL（避免每个 tool call 都打 Grafana API）

AI Core 调用时透传 user_id：不用 MCP server 自己的 Service Account 权限
决策 9：MCP server 拆分原则（先 1 个后按需拆）
核心立场：v1 用 1 个总 MCP server（namespace 分组），按需拆为多 server。

当前实现（v1）：
- 1 个 MCP server 进程
- 4 个 namespace：grafana / knowledge / playbook / skills
- 工具命名：{namespace}.{tool_name}，例如 grafana.query_prometheus / knowledge.list_services
- 总工具数：~36（6 + 14 + 7 + 9）
为什么先合成 1 个：
- ✅ 部署简单（1 进程 vs 4 进程）
- ✅ 共享 FolderPermissionService、日志、监控
- ✅ 团队规模小时维护成本低
- ✅ mcp-go 1 个 server 可注册多个 tool，按 namespace 分组
- ✅ 减法设计：能用简单方案就别过度设计
什么时候拆（按需触发）：
|触发条件|阈值|动作|
|-|-|-|
|单 server 工具数过多|> 20|拆为多 server|
|数据源增多|≥ 3 个不同数据源|按数据源拆（grafana-prometheus-mcp / grafana-loki-mcp / ...）|
|团队增多|≥ 3 个团队维护|按团队拆|
|单 server 性能瓶颈|tool list 加载 > 1s / 选 tool 准确率 < 80%|拆|
|故障频发|某个 namespace 频繁重启|隔离|

拆的规则（参考 mcp-go 实践）：
- 拆时按 namespace 切分（每个新 server 接一个 namespace 集）
- 拆后 AI Core config 多加一条 MCP_SERVERS 项
- 工具命名保留 {namespace}.{tool_name}（不破坏 LLM 调用习惯）
- 改代码量：~100 行（注册 + config）
v1 数据源支持：
- ✅ Prometheus（grafana.query_prometheus 等）
- ❌ Loki / Tempo / Elasticsearch / 其他：不做（v1）
加新数据源的动作清单（未来用）：
1. 在 v1 单一 server 注册新 tool（不加进程）
2. 在 P5 KeyMetric.dialect 加新类型（traceql / es_dsl / sql）
3. 在 P6 PlaybookStep.query 加新 dialect
4. 加新数据源对应的 Skill（按 dialect 模板）
5. 改代码量：~100-200 行
当工具数 > 20 时再考虑：
- 拆为多 server（按 namespace 或按数据源）
- AI Core config 加多 MCP_SERVERS 项
6. 基本概念
// MCP tool 元数据
type MCPTool struct {
    Name        string          `json:"name"`
    Description string          `json:"description"`
    InputSchema json.RawMessage `json:"inputSchema"`
    Permission  string          `json:"permission"`  // read | write
}
每个 server 暴露的工具列表（1 个总 server 按 namespace 分组，完备 CRUD）：

namespace = grafana（CUD 完整）
- grafana.query_prometheus（read）
- grafana.list_dashboards（read）
- grafana.list_alerts（read）
- grafana.create_dashboard（write，HITL）
- grafana.update_panel（write，HITL）
- grafana.delete_dashboard（write，HITL）
namespace = playbook
- playbook.list_playbooks（read，默认聚合）
- playbook.get_playbook（read）
- playbook.explain_playbook（read，返回 DAG 说明）
- playbook.run_playbook（write/execute，HITL）
- playbook.create_playbook（write，HITL）
- playbook.update_playbook（write，HITL）
- playbook.delete_playbook（write，HITL）
namespace = knowledge
- knowledge.list_services（read）
- knowledge.get_service（read）
- knowledge.upsert_service（write，HITL）
- knowledge.delete_service（write，HITL）
- knowledge.list_runbooks（read）
- knowledge.get_runbook（read）
- knowledge.upsert_runbook（write，HITL）
- knowledge.delete_runbook（write，HITL）
- knowledge.search_docs（read）
- knowledge.get_doc（read）
- knowledge.ingest_doc（write，HITL，单条）
- knowledge.ingest_documents（write，HITL，批量）
- knowledge.update_document（write，HITL）
- knowledge.delete_document（write，HITL）
namespace = skills
- skills.list_skills（read，默认聚合）
- skills.search_skills（read）
- skills.get_skill（read）
- skills.load_skill_for_agent（read）
- skills.run_skill（write/execute，HITL）
- skills.create_skill（write，HITL）
- skills.update_skill（write，HITL）
- skills.delete_skill（write，HITL）
- skills.promote_skill（write，HITL，P10 联动）
7. 验收标准
- [ ]  1 个 MCP server 用 mcp-go 起，监听 Streamable HTTP endpoint，按 4 个 namespace（grafana / knowledge / playbook / skills）注册工具
- [ ]  AI Agent 用 eino-ext/components/tool/mcp 1 个 client 连接，按 namespace 过滤/聚合工具
- [ ]  read 工具调一次成功
- [ ]  write 工具触发 Eino Interrupt & CheckPoint（HITL 审批）
- [ ]  server 故障后 client 自动重连（mcp-go 内置）
- [ ]  Skills MCP 可被外部 MCP 客户端连接：curl /mcp 调用 list_skills 返回 JSON-RPC 响应
- [ ]  Skills MCP 与 Eino Skill Middleware 共用同一 data/skills/ 目录（文件单一来源）
8. 接口与对接
8.1 对外接口（每个 server 独立暴露）
|Server|传输协议|工具集合|消费者|
|-|-|-|-|
|assistant-mcp（总 server）|Streamable HTTP|36 个工具（4 namespace：grafana / knowledge / playbook / skills）|P1 / P5 / P6 / P7b / P8 / 外部 AI 工具|

8.2 依赖接口（本模块消费）
|来源|接口|用途|
|-|-|-|
|Grafana API|HTTP|grafana.* 工具数据源|
|Prometheus|PromQL|grafana.query_prometheus 查询|
|data/services/*.yaml|文件|knowledge.* 工具加载|
|data/playbooks/*.yaml|文件|playbook.* 工具加载|
|data/skills/*.md|文件|skills.* 工具加载（与 Skill Middleware 共享）|

8.3 数据流向
flowchart TB
    P1[P1 AI Agent<br/>eino-ext/mcp 桥接] -->|tool_call<br/>{namespace}.{tool}| AssistantMCP[assistant-mcp<br/>单 server 4 namespace]
    P5[P5 知识库] --> AssistantMCP
    P6[P6 Playbook] --> AssistantMCP
    P7b[P7b 异步分析] --> AssistantMCP
    P8[P8 Skill] --> AssistantMCP
    Cursor[外部 AI 工具<br/>Cursor/Claude Desktop] -->|MCP client| AssistantMCP

    AssistantMCP -->|grafana.*| Grafana[Grafana API]
    AssistantMCP -->|knowledge.*| Catalog[(data/services/ + runbooks/ + documents/)]
    AssistantMCP -->|playbook.*| Playbooks[(data/playbooks/)]
    AssistantMCP -->|skills.*| Skills[(data/skills/)]
8.4 例子即规范
现状：所有能力写死在 AI Agent 代码里，难以替换 / 调试 / 跨团队复用。

提议后的形态：
- 1 个 MCP server，每个 namespace 一个进程（v1 合成一个，按需拆分）
- AI Agent 通过 eino-ext/components/tool/mcp 一行 GetTools 拿到所有工具
- 外部 AI 工具（Cursor）通过 MCP client 连接 assistant-mcp:8080/mcp 消费 skills.* 工具
边界与非法情况：
- ❌ MCP server 不可用 → eino-ext/mcp client 自动重连（mcp-go 内置）
- ❌ write 工具 → 触发 Eino Interrupt（不在 MCP server 内处理）
- ❌ OAuth 不支持 → header bearer token
- ✅ 任意 MCP 客户端可连接任意 server（仅权限受限）
9. 依赖
- 被 P1 依赖：AI Agent 通过本提案 server 调用所有工具
- 被 P5 依赖：knowledge.* 工具由本提案提供
- 被 P6 依赖：playbook 工具由本提案提供
- 被 P8 依赖：skills 工具由本提案提供
- 被 P7b 依赖：告警触发时通过 grafana.* / playbook.* 工具查询
10. 状态记录
- 2026-07-10：草案创建（4 server 含 Skills MCP）
- 2026-07-10：新增 Decision 5（eino-ext 集成）/ Decision 6（Skills 双形态）

P5: 知识库联动（基于 Grafana Folder + Web UI + 文档导入）

标签：proposal FullSpec
所属模块：知识库联动
状态：草案（2026-07-10 重写：复用 Grafana Folder 权限模型，Web UI + DB 主存，文档导入）
1. 动机 / 用户故事
- 工程师问 AI "checkout 服务是什么"，AI 一无所知；问 "用什么 metric 看支付服务的错误率"，AI 给不出准确答案
- SRE 在 Confluence / Lark Doc / Markdown 散落写了 50+ 份 runbook，没人搬到 AI 能用的地方
- Grafana 团队用 Folder 组织 Dashboard，每个 Folder = 一个业务域（payment / search / infra）
- 新人想加 runbook，但不愿 clone 仓库 / 写 YAML
共同需求：
1. AI 知道团队自己的"行话"（服务名 / metric / runbook）
2. 每个 Grafana Folder 内的知识库是隔离的——AI 检索自动按用户当前 Folder 过滤
3. 支持批量导入已有文档（Confluence 导出 / Markdown / PDF / Word）
4. 普通人能编辑，不用懂 Git / YAML
2. 目标用户
- 服务 owner / SRE：维护服务元数据 + runbook（绑定到 Grafana Folder）
- 值班 / SRE Lead：写故障复盘
- PM / 运营（次要）：看样例问答、加业务背景文档
- 所有 Grafana 团队成员：消费知识库（@ 提及 / 提问时 RAG）
3. 现有做法及其不足
- Confluence 写"服务地图"wiki，过期无人在意
- Dashboard 标题 / panel 标题里有线索，但 LLM 不查
- 新人问 mentor，mentor 离开就丢
- 每个 Folder 在 Grafana 有独立权限（已存在），但 AI 知识库没复用
- 没有导入工具：已经写好的 runbook 要手动搬到 AI 知识库
4. 本提案范围与明确不做
做
核心立场：复用 Grafana Folder 体系做隔离和权限，不发明新概念。

Grafana Folder 复用点：
- 隔离单位 = Folder UID（不是自建 Project）
- 权限 = Grafana Folder Permission（User / Team / Role 三种分配对象）
- 前端显示 = Grafana Folder 树（直接调 Grafana API）
- 校验 = 调 Grafana API（GET /api/folders/:uid）
数据模型：
- Service Entry：服务的结构化元数据，关联 folder_uid（Grafana Folder）
- Runbook：Markdown 自由格式 + YAML frontmatter，关联 folder_uid
- Document：导入的原始文档，关联 folder_uid
- Sample Q&A：样例问答，关联 folder_uid
添加 / 编辑：
- Web UI（主路径）：所见即所得
    - 服务元数据：表单 + YAML 预览
    - Runbook：Markdown 编辑器
    - 文档导入：拖拽上传 + 解析预览
- ingest_doc MCP tool（辅助）：外部 AI 工具 / CLI 写入单条（HITL）
- ingest_documents MCP tool（批量）：外部工具批量导入
- 导入流程（独立功能）：
    - 拖拽 / 选择多个文件
    - 异步解析（PDF / DOCX / HTML / MD / Confluence ZIP）
    - 解析进度 + 失败列表
    - 用户确认（调整 folder_uid / tags / 作者）
    - 入库 + embedding
消费：
- @ 提及：用户 @service_name → 按当前 folder_uid 检索服务元数据
- AI 自动 RAG：用户问问题时按当前 folder_uid 检索
权限校验（复用 Grafana）：
- AI Core 调 Grafana API：GET /api/folders/:uid/permissions 拿当前用户在该 Folder 的权限
- 仅当权限 ≥ View 时，才返回该 Folder 的知识库内容
- 写操作要求 Edit / Admin 权限
不做
- ❌ 自建 Project / ProjectMember 表（Grafana Folder 已有）
- ❌ 自建权限模型（Grafana Folder Permission 已有）
- ❌ Web UI 实时多人协作（仅单人编辑，乐观锁）
- ❌ 外部 Confluence / Lark Doc 实时同步：只支持导出 → 导入
- ❌ OCR（图片中的文字识别）
- ❌ 自动从聊天对话提取 runbook
- ❌ 复杂权限模型（用 Grafana Folder Permission：Admin / Edit / View 三档）
5. 关键决策
决策 1：MVP 不上向量库
- 起步：服务元数据走 DB（结构化字段），文档走关键词检索 + 全量 prompt
- 理由：MVP 不引入额外组件；文档 < 1000 时精度足够
决策 2：复用 Grafana Folder 做隔离（核心决策）
核心立场：Grafana Folder 已经是 Grafana 团队组织的核心单元 + 权限边界。AI 知识库应复用而非发明 Project 概念。

复用映射：
|设计|来源|
|-|-|
|隔离单位 = Grafana Folder|Grafana 原生|
|权限 = Grafana Folder Permission（User / Team / Role / Service Account）|Grafana 原生|
|字段 folder_uid 引用 Grafana Folder|Grafana Folder UID|
|角色 = Grafana Folder Permission 等级（Admin=4 / Edit=2 / View=1）|Grafana 原生|
|团队成员管理 = Grafana Team + Folder Permission 分配给 Team|Grafana 原生|

具体机制：
- AI Core 收到请求时，调 Grafana API：
    - GET /api/folders?query=...（列出用户可访问的 Folder）
    - GET /api/folders/:uid/permissions（查该 Folder 权限）
- 校验当前用户在该 Folder 的 Permission ≥ 1（View）才能检索
- 写操作校验 Permission ≥ 2（Edit）
为什么不再自建：
- ✅ Grafana Folder 已有完善的 Permission 模型（User / Team / Role 三种分配对象）
- ✅ 前端 Folder 树已有，不用做选择器
- ✅ SRE 已会用 Grafana Folder 管理 Dashboard，零学习成本
- ❌ 自建 Project 会引入"另一套权限体系"，与 Grafana 冲突
- ❌ 双体系维护成本高（数据同步、权限冲突）
决策 3：存储用 DB（SQLite 起步，可切 pgsql/mysql）
- 主存储：DB（结构化字段 + Markdown body）
- 文件镜像：Git 作为审计 / 备份 / diff 源（异步 commit，非主路径）
- 理由：Web UI 友好、Agent 读 DB 快、用户零学习成本
- Store interface 抽象，配置选择实现
决策 4：Embedding 用云端 API
- OpenAI text-embedding-3 或 BGE（智源）
- 不引入本地 embedding 模型（避免 GPU 依赖）
- 文档库 < 1000 文档时 token 成本可接受
决策 5：RAG 触发时机由 Agent 决定
- Agent 判断"该用户问题需要查文档吗"
- 不是每个问题都查（避免 token 浪费）
- 规则简单：用户 @ 文档名 / 问具体 runbook 名称 / 提问语言是 SOP 类型 → 查
- 所有 RAG 检索按当前 folder_uid 过滤
决策 6：知识库添加走 Web UI + MCP
|入口|适用|是否 HITL|
|-|-|-|
|Web UI（主路径）|服务 owner / SRE 编辑服务元数据或 Runbook|否（Web UI 自带乐观锁）|
|ingest_doc MCP|外部 AI 工具 / CLI 写入单条|是（Eino Interrupt）|
|ingest_documents MCP（批量）|批量导入|是（Eino Interrupt + 进度展示）|
|用户主动存档（可选）|对话中说"保存这个排查为 runbook" → Agent 总结草稿 → 走 Web UI|否|

Web UI 的乐观锁：
- 编辑时基于 version 字段，提交时校验
- 冲突提示"XXX 正在编辑"
决策 8：完备 CRUD（ServiceEntry / Runbook / Document 三类资源）
核心立场：知识库三类资源都需完整 CRUD，不仅是 read。

CRUD 矩阵：
|资源|Create|Read|Update|Delete|
|-|-|-|-|-|
|ServiceEntry|upsert_service|list_services / get_service|upsert_service|delete_service|
|Runbook|upsert_runbook|list_runbooks / get_runbook / search_docs / get_doc|upsert_runbook|delete_runbook|
|Document|ingest_doc / ingest_documents|search_docs / get_doc|update_document|delete_document|

权限校验：
- 写操作（create / update / delete）：Folder Permission ≥ Edit（HITL）
- 读操作（list / get / search）：Folder Permission ≥ View
Update 语义：
- 保留 version 字段（乐观锁）
- 写冲突时返回 409 + 提示
- 不做版本历史（DB 自身有 updated_at）
Delete 语义：
- 软删除 vs 硬删除：硬删除（删除前弹窗确认）
- cascade：删除关联的引用（如 ServiceEntry 删除时清理其 runbook_ids / playbook_ids 引用——但不删除引用的 Runbook / Playbook 本身）
Document 特殊性：
- Document 来自导入（PDF / DOCX / HTML），元数据自动提取
- Update 可修 frontmatter / tags / 关联 service
- Delete = 从 DB + embedding index 移除
决策 7：支持文档批量导入
导入流程：
flowchart LR
    User[用户] -->|拖拽 / 选择| UploadUI[Web UI 上传]
    UploadUI -->|multipart/form-data| IngestAPI[ingest API]
    IngestAPI -->|异步任务| Parser[解析器]
    Parser -->|PDF/DOCX/HTML/MD/ZIP| Chunker[Chunker]
    Chunker -->|chunk + metadata| Reviewer[导入预览]
    Reviewer -->|用户确认| Persist[入库 + embedding]
    Persist --> DB[(DB)]
    Persist --> Index[Embedding Index]
支持格式：
|格式|解析库|注意事项|
|-|-|-|
|Markdown|goldmark|保留 frontmatter|
|PDF|unidoc/unipdf 或 pdfcpu|提取文本 + 元数据|
|DOCX|fumiama/go-docx 或 unidoc/unioffice|提取段落 + 表格|
|HTML|PuerkitoBio/goquery|去 CSS / JS，提取正文|
|Confluence 导出（HTML ZIP）|标准 unzip + HTML 解析|保留页面间链接|

导入元数据自动提取：
- 标题（文件名 / HTML title / PDF metadata）
- 作者（PDF Author / DOCX core property / Frontmatter author）
- 创建时间（PDF CreationDate / DOCX created / Frontmatter date）
- Tags（Frontmatter tags / 文件夹路径 / 用户手动指定）
- Folder 关联（用户手动选）
导入预览：
- 显示每个文件的解析结果（标题 / chunk 数 / 提取的元数据）
- 用户可调整：folder_uid / tags / 作者
- 部分失败的文件允许"跳过失败文件，仅导入成功"
任务管理：
- 每个批量导入是一个 ImportTask（UUID）
- 状态：pending → parsing → reviewing → persisting → done / failed
- 失败可重试
边界与限制：
- 单文件 ≤ 10MB
- 单次批量 ≤ 100 文件
- 单 Folder ≤ 5000 文档
- 失败文件不阻塞整体任务
6. 基本概念
6.1 Service Entry 模型（绑定 Grafana Folder）
# 数据存储于 DB；以下为字段定义
name: checkout-api
folder_uid: <grafana_folder_uid>      # 必填：引用 Grafana Folder
namespace: production
display_name: Checkout API
owner: payments-team
key_metrics:
  - name: p95_latency
    description: 95 分位延迟
    dialect: promql              # 查询方言（promql 当前唯一实现）
    expr: histogram_quantile(0.95, ...)
    threshold: "500ms"
    severity: warning
dependencies:
  - payment-service
runbook_ids:                          # 背景文档引用（P5）
  - <runbook_uuid>
playbook_ids:                         # 可执行流程引用（P6）
  - <playbook_uuid>
dashboard_uids:                       # 关联 Grafana Dashboard
  - checkout-overview
labels:
  tier: critical
version: 3                           # 乐观锁
type ServiceEntry struct {
    ID           string         `db:"id"`           // UUID
    FolderUID    string         `db:"folder_uid"`   // Grafana Folder UID（必填）
    Name         string         `db:"name"`
    Namespace    string         `db:"namespace"`
    DisplayName  string         `db:"display_name"`
    Owner        string         `db:"owner"`
    KeyMetrics   []KeyMetric    `db:"key_metrics"`  // JSON：含 dialect 字段（多数据源支持）
    Dependencies []string       `db:"dependencies"` // JSON
    RunbookIDs   []string       `db:"runbook_ids"`   // JSON：引用 P5 Runbook.id（背景文档）
    PlaybookIDs  []string       `db:"playbook_ids"`  // JSON：引用 P6 Playbook.id（可执行流程）
    DashboardUIDs []string      `db:"dashboard_uids"` // JSON
    Labels       map[string]string `db:"labels"`    // JSON
    Version      int64          `db:"version"`
    CreatedAt    time.Time      `db:"created_at"`
    UpdatedAt    time.Time      `db:"updated_at"`
}

// KeyMetric：服务关键指标（多数据源支持）
type KeyMetric struct {
    Name        string `json:"name"`         // metric 名（如 p95_latency）
    Description string `json:"description"`  // 人类可读描述
    Dialect     string `json:"dialect"`      // 查询方言：promql（v1 唯一实现）| logql | traceql | es_dsl | sql
    Expr        string `json:"expr"`         // 查询表达式
    Threshold   string `json:"threshold,omitempty"` // 阈值（人类可读）
    Severity    string `json:"severity"`     // info | warning | critical
}
6.2 Runbook 模型（绑定 Grafana Folder）
---
title: Checkout 服务 p95 延迟升高排查
folder_uid: <grafana_folder_uid>      # 必填
service: checkout-api                # 关联 ServiceEntry.name
tags: [latency, oncall, p0]
severity: warning
author: alice
updated_at: 2026-07-10
source: manual                       # manual | imported
source_meta:                          # 导入时的来源信息
  format: pdf
  original_path: /uploads/checkout-latency.pdf
  imported_at: 2026-07-10T10:00:00Z
  importer: alice
---

# Checkout 服务 p95 延迟升高排查
...
type Runbook struct {
    ID          string            `db:"id"`
    FolderUID   string            `db:"folder_uid"`   // Grafana Folder UID
    Title       string            `db:"title"`
    Service     string            `db:"service"`
    Tags        []string          `db:"tags"`
    Severity    string            `db:"severity"`
    Author      string            `db:"author"`
    Frontmatter map[string]any    `db:"frontmatter"`
    Body        string            `db:"body"`        // Markdown 全文
    Source      string            `db:"source"`      // manual | imported
    SourceMeta  map[string]any    `db:"source_meta"` // JSON
    Version     int64             `db:"version"`
    CreatedAt   time.Time         `db:"created_at"`
    UpdatedAt   time.Time         `db:"updated_at"`
}
6.3 Document 模型（导入的原始文档，绑定 Grafana Folder）
type Document struct {
    ID           string    `db:"id"`
    FolderUID    string    `db:"folder_uid"`    // Grafana Folder UID
    Path         string    `db:"path"`
    Title        string    `db:"title"`
    Format       string    `db:"format"`        // md | pdf | docx | html | confluence
    Chunks       []Chunk   `db:"chunks"`        // JSON
    Embedding    []float32 `db:"embedding"`
    ChunkCount   int       `db:"chunk_count"`
    ImportedBy   string    `db:"imported_by"`
    ImportedAt   time.Time `db:"imported_at"`
    SourceMeta   map[string]any `db:"source_meta"` // JSON
}
6.4 Catalog / Importer / GrafanaClient 接口
type Catalog interface {
    // Service Entry（按 folder_uid 过滤 + Shared Folder 聚合）
    GetService(ctx context.Context, folderUID, name string) (*ServiceEntry, error)
    ListServices(ctx context.Context, folderUIDs []string, filter ListFilter) ([]*ServiceEntry, error)  // folderUIDs 含 active + Shared
    UpsertService(ctx context.Context, s ServiceEntry) error  // Web UI 用

    // Runbook（按 folder_uid 过滤 + Shared Folder 聚合）
    GetRunbook(ctx context.Context, folderUID, id string) (*Runbook, error)
    ListRunbooks(ctx context.Context, folderUIDs []string, filter ListFilter) ([]*Runbook, error)
    UpsertRunbook(ctx context.Context, r Runbook) error
    DeleteRunbook(ctx context.Context, folderUID, id string) error

    // RAG（按 folder_uid 过滤 + Shared Folder 聚合）
    Retrieve(ctx context.Context, folderUIDs []string, query string, topK int) ([]*Chunk, error)

    // Import
    CreateImportTask(ctx context.Context, task ImportTask) error
    GetImportTask(ctx context.Context, id string) (*ImportTask, error)
    UpdateImportTask(ctx context.Context, task ImportTask) error

    Reload(ctx context.Context) error
}

type Importer interface {
    Parse(ctx context.Context, file io.Reader, format string) (*ParsedDoc, error)
}

type GrafanaClient interface {
    // 列出当前用户可访问的 Folder
    ListFolders(ctx context.Context) ([]*GrafanaFolder, error)
    // 查 Folder 详情
    GetFolder(ctx context.Context, uid string) (*GrafanaFolder, error)
    // 查 Folder Permission（用于权限校验）
    GetFolderPermissions(ctx context.Context, uid string) ([]*GrafanaPermission, error)
    // 校验当前用户在 Folder 的 Permission（View / Edit / Admin）
    CheckFolderPermission(ctx context.Context, uid string, required int) (bool, error)
}

type GrafanaFolder struct {
    UID   string `json:"uid"`
    Title string `json:"title"`
    // ... 其他字段
}

type GrafanaPermission struct {
    UserID     int    `json:"userId,omitempty"`
    TeamID     int    `json:"teamId,omitempty"`
    Role       string `json:"role,omitempty"`     // Viewer | Editor | Admin
    Permission int    `json:"permission"`          // 1=View, 2=Edit, 4=Admin
}
6.5 ImportTask / ParsedDoc
type ImportTask struct {
    ID          string         `db:"id"`
    FolderUID   string         `db:"folder_uid"`     // 用户选的目标 Folder
    UserID      string         `db:"user_id"`
    Files       []ImportFile   `db:"files"`          // JSON
    Status      string         `db:"status"`         // pending | parsing | reviewing | persisting | done | failed
    Progress    int            `db:"progress"`       // 0-100
    ErrorMsg    string         `db:"error_msg,omitempty"`
    CreatedAt   time.Time      `db:"created_at"`
    UpdatedAt   time.Time      `db:"updated_at"`
}

type ImportFile struct {
    Filename   string      `json:"filename"`
    Format     string      `json:"format"`
    Size       int64       `json:"size"`
    Status     string      `json:"status"`           // pending | parsed | failed | skipped | imported
    ParsedDoc  *ParsedDoc  `json:"parsed_doc,omitempty"`
    ErrorMsg   string      `json:"error_msg,omitempty"`
}

type ParsedDoc struct {
    Title       string            `json:"title"`
    Author      string            `json:"author,omitempty"`
    CreatedAt   time.Time         `json:"created_at,omitempty"`
    Tags        []string          `json:"tags,omitempty"`
    Body        string            `json:"body"`
    Frontmatter map[string]any    `json:"frontmatter,omitempty"`
    SourceMeta  map[string]any    `json:"source_meta"`
}
7. 验收标准
Folder 复用
- [ ]  AI Core 调 Grafana GET /api/folders?query=... 列出当前用户可访问的 Folder
- [ ]  Web UI 显示 Folder 树（直接用 Grafana Folder 树组件）
- [ ]  AI Core 校验用户对当前 folder_uid 的 Permission ≥ View 才能检索
- [ ]  写操作要求 Permission ≥ Edit
消费（按 folder_uid 隔离）
- [ ]  用户 @service_name → Catalog 按当前 folder_uid 查
- [ ]  AI 自动 RAG 按当前 folder_uid 检索
- [ ]  用户无权限的 Folder 不可见（Web UI + AI Agent 双重校验）
添加（Web UI 主路径）
- [ ]  服务元数据：表单编辑 + YAML 预览
- [ ]  Runbook：Markdown 编辑器（带 frontmatter 表单）
- [ ]  编辑时乐观锁防止覆盖
- [ ]  创建后立即可被检索（RAG 索引 ≤ 5s 更新）
添加（MCP 辅助路径）
- [ ]  ingest_doc MCP tool 写入单条 runbook（触发 Eino Interrupt）
- [ ]  ingest_documents MCP tool 批量导入（带进度）
- [ ]  用户确认 → 入库
文档导入
- [ ]  Web UI 支持拖拽 / 选择多个文件
- [ ]  支持格式：Markdown / PDF / DOCX / HTML / Confluence 导出 ZIP
- [ ]  异步解析 + 进度显示
- [ ]  解析预览（标题 / 作者 / tags / folder_uid 关联）可编辑
- [ ]  用户确认后入库 + 自动 embedding
- [ ]  单文件 ≤ 10MB / 单次 ≤ 100 文件 / 单 Folder ≤ 5000 文档
- [ ]  部分失败不阻塞整体任务
不做的验收边界
- [ ]  没有自建 Project / ProjectMember 表（全部复用 Grafana Folder）
- [ ]  没有外部 Confluence 实时同步
- [ ]  没有 OCR
- [ ]  没有自动从聊天提取 runbook
8. 接口与对接
8.1 对外接口（本模块提供）
Go 接口：
|接口|类型|消费者|
|-|-|-|
|Catalog.*|Go interface|P1 AI Agent / P5 Web UI|
|Importer.Parse|Go interface|P5 Web UI|
|GrafanaClient.*|Go interface|内部权限校验|

MCP tools（knowledge.* namespace）：
|Tool|权限|消费者|默认行为|
|-|-|-|-|
|list_folders（调 Grafana）|read|P1 / 外部工具|列出用户可访问 Folder|
|list_services(folder_uid?)|read|P1 / 外部工具|不传 → active + Shared + private|
|get_service|read|P1 / 外部工具|必须传 folder_uid（权限校验）|
|upsert_service / delete_service|write（HITL）|P1 / 外部工具|必须传 folder_uid|
|list_runbooks(folder_uid?)|read|P1 / 外部工具|不传 → active + Shared + private|
|get_runbook|read|P1 / 外部工具|必须传 folder_uid|
|upsert_runbook / delete_runbook|write（HITL）|P1 / 外部工具|必须传 folder_uid|
|search_docs / get_doc|read|P1 / 外部工具|必须传 folder_uid|
|ingest_doc（单条）|write（HITL）|P1 / 外部工具|必须传 folder_uid|
|ingest_documents（批量）|write（HITL）|P1 / 外部工具 / CLI|必须传 folder_uid|
|update_document|write（HITL）|P1 / 外部工具|必须传 folder_uid（修 frontmatter / tags / service 关联）|
|delete_document|write（HITL）|P1 / 外部工具|必须传 folder_uid|

8.2 依赖接口（本模块消费）
|来源|接口|用途|
|-|-|-|
|Grafana Folder API|GET /api/folders/:uid/permissions|权限校验|
|Grafana Folder API|GET /api/folders?query=...|列出可访问 Folder|
|data/services/*.yaml|文件系统|（仅初始 seed）|
|OpenAI / BGE Embedding API|HTTP|文档 embedding|
|Eino Callback|框架 API|RAG 检索触发|
|GitHub API（gh CLI）|HTTP|Git 镜像（异步、非主路径）|

8.3 数据流向
flowchart TB
    %% Folder 维度（Grafana 原生）
    GrafanaFolder[Grafana Folder<br/>已有 Permission 体系]

    %% 消费侧
    P1[P1 AI Agent<br/>active_folder_uid] -->|GetService/Retrieve| Catalog[Catalog]
    P1 -->|knowledge.* tools| MCP[assistant-mcp]
    Catalog -->|按 folder_uid 过滤| DB[(DB: services / runbooks / documents)]
    Catalog -->|调 Grafana| GrafanaFolder
    GrafanaFolder -->|Permission ≥ View| Catalog
    EmbeddingAPI[OpenAI/BGE] --> Catalog

    %% 添加侧（Web UI）
    User[用户] -->|所见即所得| WebUI[Web UI<br/>Folder 树选择器]
    WebUI -->|CRUD + 乐观锁| DB
    WebUI -->|上传文件| Importer[Importer]
    Importer -->|Parse| ParserLib[PDF/DOCX/HTML/MD 解析]
    ParserLib -->|ParsedDoc| WebUI
    WebUI -->|用户确认| DB

    %% 添加侧（MCP）
    External[外部 AI 工具 / CLI] -->|ingest_doc / ingest_documents| MCP
    MCP -->|HITL Interrupt| P1
    P1 -->|用户确认| DB
    MCP -->|写 DB + 异步 Git mirror| GitMirror[Git 镜像<br/>非主路径]
8.4 例子即规范
消费 - 现状：工程师问 AI "checkout 服务是什么"，AI 一无所知。

消费 - 提议后的形态：
- 用户在 Grafana 切到 "payment" Folder
- 输入"@checkout-api"
- AI Core 校验：用户在 payment Folder 有 View 权限 ✓
- Catalog 按 folder_uid=payment 查服务元数据 → 注入 LLM context
添加 - 现状：新人想加一个 runbook，不愿 clone 仓库 / 写 YAML / 走 PR。

添加 - 提议后的形态（Web UI 编辑）：
1. 用户在 Grafana Folder "payment" 内
2. 点"新建 Runbook" → Markdown 编辑器
3. 填 frontmatter（title / service / tags / severity）→ 写 body
4. 保存 → DB 立即可被 RAG 检索（≤ 5s）
批量导入 - 现状：团队有 50+ Confluence runbook，没人搬到 AI 能用的地方。

批量导入 - 提议后的形态：
1. Web UI 进入"payment" Folder → "导入文档"
2. 拖拽 50 个 PDF / Markdown / HTML 文件
3. 后端异步解析，显示进度条
4. 解析完成 → 预览页面显示每个文件的：标题 / 作者 / tags / folder_uid（默认当前 Folder）
5. 用户调整（部分文件跳过失败、tags 修正、folder_uid 改到 search）
6. 点确认 → 批量入库 + embedding
7. 任务列表显示 ImportTask 状态 = done
边界与非法情况：
- ❌ YAML / Markdown 文件解析失败 → 启动时拒绝加载该文件 + 日志报警
- ❌ Embedding API 不可用 → 启动失败（fail fast）
- ❌ RAG 召回 0 命中 → Agent 主动告诉用户"未找到"
- ❌ 外部 Confluence 同步 → 不做
- ❌ Reload 失败 → 保留旧版本 + 报警（不阻塞服务）
- ❌ ingest_doc 无 frontmatter → 拒绝 + 报错
- ❌ 用户无 Folder 权限 → Web UI 不显示 / AI Agent 拒绝返回
- ✅ Web UI 编辑 / 批量导入 / MCP ingest 都按 folder_uid 隔离 + Grafana Permission 校验
9. 不做（边界）
- 自建 Project / ProjectMember（已复用 Grafana Folder）
- Web UI 实时多人协作
- 外部 Confluence / Lark Doc 实时同步（只支持导出 → 导入）
- OCR（图片中的文字识别）
- 自动从聊天对话提取 runbook
- 复杂权限模型（用 Grafana Folder Permission：Admin / Edit / View）
- 文档版本控制 / 时间机器（DB 自身有 updated_at；不提供历史 diff UI）
10. 依赖
- 依赖 P4：knowledge.* 工具
- 依赖 Grafana Folder API：隔离 + 权限（核心复用）
- 被 P1 依赖：AI Agent 通过 Catalog / Retriever 调用（按 active_folder_uid 过滤）
- 被 P3 依赖：会话级 active_folder_uid
- 被 P7b 依赖：告警分析时按告警所属 Folder 查 runbook
- 被 P10 依赖：晋升审批通过后，shared 对象关联到 Folder


P6: Playbook 结构化执行引擎

标签：proposal FullSpec
所属模块：Playbook 结构化执行引擎
状态：草案
1. 动机 / 用户故事
团队已经写好了一份 "checkout 延迟排查" SOP，放在 Confluence。每当告警触发值班同事都要按这份 SOP 一二三四步操作。

共同需求：把这份 SOP 变成可执行的 playbook，让 AI 直接跑。
2. 目标用户
SRE / 值班（执行 playbook），值班主管 / SRE 工程师（编写 playbook）。
3. 现有做法及其不足
- Confluence / Lark Doc 写 SOP
- 走一步看一步，依赖操作员记性
- 不能复用（每个事故又从头来）
- 经验不沉淀（每次走的路径略有不同，难以归纳）
4. 本期范围与明确不做
做
- 结构化 YAML 定义 playbook（步骤 / 参数 / 条件 / 副作用）
- step 类型：query / branch / loop / mcp_call / template / parallel
- 引擎基于 Eino Graph / Workflow + Lambda（数据驱动 DAG）
- 步骤级 expect 校验（CEL / JSONLogic 表达式）
- 副作用 step 支持 dry_run
- 经验字段（notes / authors / revisions）记录沉淀
- 版本管理（Git 仓）
- AI 辅助步骤推断（P10 沉淀流程联动）：从对话中推断步骤骨架（query / mcp_call / branch / template），用户编辑确认后再写入
- 跑结果记录到 Session
- 完备 CRUD（Web UI + playbook.* MCP 工具）：
    - create_playbook：从 YAML / Web UI 表单创建（private / shared）
    - list_playbooks(folder_uid?)：默认聚合（active + Shared + private）
    - get_playbook：按 ID 查（必须传 folder_uid 校验）
    - update_playbook：编辑现有（保留 version 字段 + 乐观锁）
    - delete_playbook：删除（cascade 删除关联 session run / shared permission）
    - run_playbook：执行（HITL，write 类）
    - explain_playbook：返回步骤说明 + DAG 可视化（read 类）
不做
❌ PlanExecutor 模式（LLM 自主规划步骤）
- Playbook 是结构化 YAML，LLM 自主规划会破坏确定性和可审计性
- 不适合 SOP 性质的操作流程
❌ 跨 playbook 调用
- 避免依赖复杂化
- 跨 playbook 用例通过 Playbook 嵌套引用
❌ 沙箱执行任意代码
- Playbook 是结构化步骤（6 种类型），不执行任意 Python / JS
- 避免引入代码执行沙箱的安全风险
❌ Playbook marketplace / 跨团队共享
核心立场：Playbook 不引入 marketplace 概念；跨项目共享通过 Grafana Shared Folder 模式实现（已在 P6 决策 6 明确）。

为什么不做 marketplace：
- Marketplace 是 SaaS 概念（公共仓库 + 评分 + 下载）
- 与 Grafana Folder Permission 体系重复
- 引入新概念（marketplace owner / 评分 / 评论）
- 维护成本高
做的是什么（替代方案）：
- ✅ Shared Folder 模式（所有项目共享的 Playbook 放在 Shared Folder）
- ✅ P10 晋升流程（private → shared，Folder 选择器）
- ✅ Folder Admin 审批（Permission=4 的 User / Team / Role）
对比：
|维度|Marketplace（不做）|Shared Folder 模式（做）|
|-|-|-|
|隔离单元|Public / Private 仓库|Grafana Folder|
|权限|自建 ACL|Grafana Folder Permission|
|跨项目共享|下载 / 复制|绑 Shared Folder|
|审批|Marketplace owner|Folder Admin|
|评分|⭐ + 评论|无|

❌ LLM 完全无人工生成 playbook
核心立场：不做完全无人工的 Playbook 自动生成。做的是"AI 辅助生成 + 人工确认"（详见 P6 决策 7）。
|模式|含义|是否做|
|-|-|-|
|完全无人工|LLM 写完整 Playbook YAML → 直接入库|❌ 不做|
|AI 辅助 + 人工确认|LLM 从对话推断步骤骨架 → 用户编辑 → 入库|✅ 做（P10 沉淀流程）|
|完全人工|用户手写 YAML（Web UI 表单 / YAML 编辑器）|✅ 做|

为什么不做"完全无人工"：
- 自动生成的 Playbook 不可控（幻觉 / 错误步骤 / 含糊条件）
- 引入低质量 Playbook 污染执行
- 跑出问题难调试（用户不理解内容）
- 不实现完全自动生成
做的是什么（替代方案）：
- ✅ AI 辅助推断 + Web UI 编辑确认（P6 决策 7 + P10 沉淀流程）
- ✅ 完全人工（用户手写 YAML，Web UI 表单 / YAML 编辑器）
5. 关键决策
决策 1：用 Eino Graph / Workflow 作执行引擎，不用 PlanExecutor
备选：
- A. Eino Graph / Workflow（选）— 数据驱动 DAG
- B. PlanExecutor（LLM 自主规划步骤）
选择理由：
- ✅ Playbook 是结构化 YAML（步骤确定性强），Eino Graph 数据驱动 DAG 完美匹配
- ✅ 可重放、可审计、可测试
- ✅ Eino Aspect（Retry / Timeout / CircuitBreaker）开箱即用
放弃 B 的理由：LLM 自主规划步骤不可控、不适合 SOP 性质的 Playbook。
决策 2：Step 类型限制 6 种
- query / branch / loop / template / mcp_call / parallel
- 不引入 free-form text step（避免 LLM 改语义）
query step 的 dialect 字段（v1 唯一实现：promql）：
- 加新数据源时加新 dialect 值（logql / traceql / es_dsl / sql）
- 不改 PlaybookStep struct（Config 已是开放 map）
- 对应 P4 决策 9（按数据源拆 MCP server）
决策 3：副作用 step 默认 dry_run
- write / update 类 step 第一次跑 dry_run 模式，只输出"将做什么"
- 显式确认后才真正执行（或者配置信任策略跳过）
决策 4：参数走"位置 + 命名 + 上下文"三层
- 位置：命令行参数直接覆盖
- 命名：playbook 顶部 parameters 块定义
- 上下文：上一步输出作为本步 inputs
决策 5：经验沉淀字段强制
- experience.notes：每次修订记录原因 + 作者
- 不强制填但强烈建议（为空时模板提醒）
决策 7：AI 辅助步骤推断（P10 沉淀流程联动）
核心立场：Playbook 是结构化 YAML，AI 不完全自动生成，但AI 可推断步骤骨架，用户编辑确认后入库。
|模式|含义|是否做|
|-|-|-|
|完全自动（无人工）|LLM 写完整 playbook → 直接入库|❌ 不做|
|辅助推断 + 人工确认|LLM 从对话推断步骤骨架（query / mcp_call / branch / template）→ 用户编辑 → 入库|✅ 做（P10 沉淀流程）|
|完全人工|用户手写 YAML|✅ 做（高级用户 / 复杂场景）|

实现：
type StepInferrer interface {
    // 从对话历史推断步骤骨架
    InferSteps(ctx, conversationHistory []Message) ([]*PlaybookStep, error)
}

// P10 沉淀流程
func GeneratePlaybookDraft(ctx, sessionID, ownerID string) (*Playbook, error) {
    // 1. 拉最近 5 轮对话
    history := sessionStore.GetRecentMessages(sessionID, 5)
    // 2. LLM 推断步骤
    steps := stepInferrer.InferSteps(ctx, history)
    // 3. 生成草稿（private, owner = ownerID, folder_uid = ""）
    return &Playbook{
        ID: uuid.New(),
        Visibility: VisibilityPrivate,
        OwnerID: ownerID,
        Steps: steps,
        // ...
    }, nil
}
AI 推断步骤的边界（P10 已定义）：
- ✅ "查 checkout p95" → query (PromQL)
- ✅ "如果错误率高" → branch
- ✅ "查 Loki 错误" → mcp_call (grafana.query_loki，v1 不做 Loki)
- ✅ "汇总报告" → template
- ❌ "先做 A 再做 B 但要看 C" → 模糊，要求用户手填确认
- ❌ "循环 / 遍历" → 模糊，要求用户手填
为什么不做"完全自动"：
- 完全自动生成的 playbook 不可控、不易调试
- 引入低质量 / 错误内容的风险
- P10 沉淀流程（人工确认）已经覆盖大部分场景
决策 6：Playbook 可见性（private / shared 两层，P10 联动，shared 可绑 Shared Folder）
核心立场：Playbook 可见性分两层，shared 直接绑定 Grafana Folder，复用 Folder Permission：
|可见性|谁能看 / 跑 / 编辑|
|-|-|
|private|仅 OwnerID（创建者）|
|shared|Grafana Folder Permission ≥ View 可见 / 可跑；≥ Edit 可编辑|

shared 绑定的 Folder 两种：
|Folder 类型|FolderUID|可见范围|
|-|-|-|
|项目 Folder（如 Payment）|真实 Grafana Folder UID|仅该 Folder 成员|
|Shared Folder（跨项目）|配置的 shared_folder_uid（约定单一全局）|所有项目成员|

创建方式：
- 手工创建：默认 private（owner = 当前用户）
- 对话沉淀（P10）：AI 生成草稿 → private
- 晋升通过（P10）：private → shared，目标 Folder = 用户选的（项目 Folder 或 Shared）
运行时校验：
- AI Core 调 Grafana Folder Permission API 校验用户是否可看 / 跑
- 写操作（创建 / 修改 / 跑）校验 Permission ≥ Edit
- 跑完 UsageCount++（用于 P10 审批参考）
List 默认行为（不传 folder_uid）：
- 返回 active_folder_uid + Shared Folder + 用户 private
- 跨项目共享的 Playbook 通过 Shared Folder 聚合进来
决策 8：完备 CRUD（create / list / get / update / delete）
核心立场：Playbook 必须有完整 CRUD，不仅是 read 类。Web UI + playbook.* MCP 工具都必须支持。

CRUD 矩阵：
|操作|MCP tool|权限|HITL|Web UI|
|-|-|-|-|-|
|Create|create_playbook|write|✅|✅（表单 + YAML 编辑器）|
|List|list_playbooks(folder_uid?)|read|❌|✅|
|Get|get_playbook|read|❌|✅|
|Update|update_playbook|write|✅|✅|
|Delete|delete_playbook|write|✅|✅|
|Run|run_playbook|execute|✅|✅（dry_run 预览 + 确认）|
|Explain|explain_playbook|read|❌|✅（DAG 可视化）|

权限校验（MCP server 统一）：
- create / update / delete：校验 Folder Permission ≥ Edit
- run：write 操作 + 副作用 step 时必须 HITL
- list / get / explain：read 操作
Update 语义：
- 保留 version 字段（乐观锁）
- 写冲突时返回 409 + 提示"XXX 正在编辑"
- 不做版本历史（不做 diff UI；DB 自身有 updated_at）
Delete 语义：
- private delete：仅 OwnerID 可删
- shared delete：Folder Admin 或 OwnerID 可删
- cascade：删除关联的 playbook_run / approval_request
- 不做软删除（避免 DB 复杂度；删除前确认弹窗）
Run 语义：
- 默认 dry_run 模式（先看"将做什么"）
- 用户确认后真正执行
- 执行中可中断（Eino Interrupt）
- 跑完 UsageCount++（P10 审批参考）
6. 基本概念
# data/playbooks/checkout-latency-investigation.yaml
id: checkout-latency-investigation
name: Checkout 延迟升高排查
description: 收到 checkout 服务 p95 latency > 500ms 告警后自动排查
version: 1
trigger:
  type: alert
  pattern: "CheckoutLatencyHigh"
  alert_labels:
    severity: warning
parameters:
  - name: env
    type: string
    default: production
    required: true
  - name: time_range
    type: string
    default: 1h
steps:
  - id: baseline_p95
    type: query
    config:
      dialect: promql              # 查询方言（v1 唯一实现：promql）
      datasource: prometheus
      expr: histogram_quantile(0.95, sum by (le) (rate(http_request_duration_seconds_bucket{job="checkout-api"}[5m])))
      output: p95_value
    expect:
      expression: "p95_value < 500"
      on_fail: continue
  - id: check_errors
    type: query
    config:
      dialect: promql
      datasource: prometheus
      expr: sum(rate(http_requests_total{job="checkout-api",status=~"5.."}[5m])) / sum(rate(http_requests_total{job="checkout-api"}[5m]))
      output: error_rate
    expect:
      expression: "error_rate < 0.01"
      on_fail: continue
  - id: if_errors_high
    type: branch
    depends_on: [check_errors]
    config:
      condition: "error_rate > 0.01"
      then:
        - id: query_loki_errors
          type: mcp_call
          config:
            server: grafana
            tool: query_loki
            args:
              query: '{job="checkout-api"} |= "ERROR" | json'
            output: error_logs
      else:
        - id: query_loki_latency
          type: mcp_call
          depends_on: [baseline_p95]
          config:
            server: grafana
            tool: query_loki
            args:
              query: '{job="checkout-api"} |= "slow"'
            output: slow_logs
  - id: summarize
    type: template
    depends_on: [if_errors_high]
    config:
      template: |
        ## 排查结果
        - 当前 p95: {{p95_value}}
        - 当前错误率: {{error_rate}}
        - 异常定位: {{error_logs | default: "无明显异常"}}
experience:
  notes:
    - date: 2026-06-23
      author: alice
      body: 真实事故发现 PG connection pool 满比 latency 早告警，可加 query
  revisions: 3
type Playbook struct {
    ID          string             `yaml:"id"`
    Name        string             `yaml:"name"`
    Description string             `yaml:"description"`
    Version     string             `yaml:"version"`
    Trigger     PlaybookTrigger    `yaml:"trigger"`
    Parameters  []PlaybookParam    `yaml:"parameters"`
    Steps       []PlaybookStep     `yaml:"steps"`
    Experience  PlaybookExperience `yaml:"experience"`

    // P10 晋升流程联动（两层可见性）
    Visibility   Visibility        `yaml:"-" db:"visibility"`        // private | shared
    FolderUID    string             `yaml:"-" db:"folder_uid"`        // shared 时必填：Grafana Folder UID
    OwnerID      string             `yaml:"-" db:"owner_id"`          // 创建者 user_id
    UsageCount   int64              `yaml:"-" db:"usage_count"`       // 跑过几次（用于审批参考）
}

type Visibility string

const (
    VisibilityPrivate Visibility = "private"
    VisibilityShared  Visibility = "shared"
)

type PlaybookStep struct {
    ID         string         `yaml:"id"`
    Type       string         `yaml:"type"`   // query | branch | loop | template | mcp_call | parallel
    DependsOn  []string       `yaml:"depends_on,omitempty"`
    Config     map[string]any `yaml:"config"`  // type=query 时含 dialect 字段（v1 唯一：promql）
    Expect     *PlaybookExpect `yaml:"expect,omitempty"`
    SideEffect bool           `yaml:"side_effect,omitempty"`
    DryRun     bool           `yaml:"dry_run,omitempty"`
}
Query Step 的 Config Schema（v1 实现）：
# type: query 的步骤
- id: baseline_p95
  type: query
  config:
    dialect: promql                # 必填：promql（v1 唯一实现）
    datasource: prometheus         # 必填：数据源 ID
    expr: histogram_quantile(0.95, ...)  # 必填：查询表达式
    output: p95_value              # 可选：本步输出变量名
    time_range: "5m"              # 可选：覆盖 Playbook 全局 time_range
Dialect 取值（v1 唯一）：promql

未来加数据源时（决策 7 联动）：dialect 加新值（如 logql / traceql / es_dsl / sql），不需改 PlaybookStep struct（Config 已是开放 map）。
可见性语义（与 P10 一致）
|可见性|谁能看|谁能跑|谁能编辑|
|-|-|-|-|
|private|仅 OwnerID 匹配|仅 OwnerID 匹配|仅 OwnerID 匹配|
|shared|Grafana Folder FolderUID Permission ≥ View|Permission ≥ View|Permission ≥ Edit（含 owner）|

可见性来源：
- 手工创建：直接填 private（owner = 当前用户）
- 从对话沉淀：AI 总结生成 → private（P10 沉淀流程）
- 晋升通过：private → shared，关联 target_folder_uid（P10 晋升流程）
- shared 跨 Folder 复制：不支持（要换 Folder 走删除 + 重新申请）
7. 验收标准
- [ ]  至少 1 个 playbook YAML 可被解析 + 执行
- [ ]  6 种 step 类型各实现至少 1 个 runner
- [ ]  副作用 step 首次跑输出"将做什么"，确认后才真正执行
- [ ]  expect 校验失败按 on_fail 配置处理
- [ ]  失败步骤有清晰报错，session 可查看
- [ ]  经验字段支持修订和 authors
8. 不做（边界）
- 跨 playbook 调用
- LLM 自动生成
- 跨团队共享 / 市场
9. 接口与对接
9.1 对外接口（本模块提供）
|接口|类型|消费者|
|-|-|-|
|Engine.Run(playbookID, params, ctx)|Go function|P1 / P7b|
|Engine.List() / Engine.Get(id)|Go function|P1（用户列举）|
|playbook.* list_playbooks / get_playbook / run_playbook / explain_playbook / create_playbook / update_playbook / delete_playbook|MCP tools|P1 / 外部 AI 工具|

9.2 依赖接口（本模块消费）
|来源|接口|用途|
|-|-|-|
|data/playbooks/*.yaml|文件系统|Playbook 定义|
|P4 grafana.* / knowledge.*|MCP tools|step 类型 mcp_call 执行|
|Eino Graph / Workflow|框架 API|DAG 编排底座|
|P9 审计|Logger.LogPlaybookRun|跑结果审计|

9.3 数据流向
flowchart TB
    P1[P1 AI Agent] -->|Run| Engine[Playbook Engine]
    P7b[P7b 异步分析] -->|Run| Engine
    Engine -->|mcp_call step| MCP[P4 assistant-mcp<br/>grafana.* / knowledge.*]
    Engine -->|query step| PromQL[Prometheus/Loki]
    Engine -->|执行结果| P9[P9 审计]
    MCP --> Engine
    Engine -->|Run ID + 结果| P1
    Engine -->|Run ID + 结果| P7b
9.4 例子即规范
现状：值班按 SOP 一二三四步走，依赖记忆；不能复用，每次事故略不同。

提议后的形态：
- 告警触发 → Engine.Run("checkout-latency-investigation", {env, time_range})
- 引擎按 YAML 步骤执行（query p95 → query 错误率 → branch → 汇总）
- 副作用 step 默认 dry_run（需 HITL 确认）
边界与非法情况：
- ❌ 跨 playbook 调用 → 不做（避免依赖复杂化）
- ❌ LLM 自主规划步骤 → 不做（用结构化 YAML）
- ❌ 沙箱执行任意代码 → 不做（Playbook 是结构化的）
- ❌ 副作用 step 默认 dry_run（首次跑不真执行）
- ✅ 6 种 step 类型确定 / 可重放 / 可审计
10. 依赖
- 依赖 P4：playbook.* 工具
- 依赖 P10：可见性 / 晋升（private / shared 两层，审批通过后转 shared）
- 被 P7b 依赖：告警触发链路
- 被 P1 依赖：用户手动调
- 被 P5 依赖：ServiceEntry.playbook_ids 引用 P6 Playbook.id
- 被 Grafana Folder API 依赖：shared 对象校验 Permission


P7a: 告警 Webhook 接收（HMAC + 幂等 + 投递）

标签：proposal FullSpec
所属模块：告警接收
状态：草案
1. 动机 / 用户故事
Grafana Alert 触发时需要把告警投递到 AI Core，由 AI 自动跑 Playbook 出结论。Grafana 侧的 webhook 没有自带安全验证（任何 HTTP 请求都能打），且重复触发 / 重放攻击会让 AI Core 跑重复的分析。

共同需求：可信、幂等、防重放地把告警从 Grafana 投递到 AI Core。
2. 目标用户
Grafana Alert 配置者 / SRE 配置 webhook 时关心"我要传什么 secret / 怎么验签"。
3. 现有做法及其不足
- 裸 HTTP webhook（无签名）：Grafana 默认走这个，任何能打到 URL 的人都能伪造告警
- Slack Incoming Webhook（带签名）：强依赖 Slack 协议，不能复用
- Alertmanager：完整但重，本期不上
4. 本提案范围与明确不做
做
- 单端点：POST /webhook/alert，监听 Grafana Alerting → Contact Point 推送
- HMAC-SHA256 验签：Header X-AI-Signature = HMAC(secret, request_body)；secret 通过环境变量注入
- 时间戳防重放：Header X-AI-Timestamp，超过 timestamp_skew（默认 300s）拒绝
- 幂等去重：用 Grafana Alert 的 fingerprint 字段作为 key，1 小时内同一 fingerprint 只投递 1 次到下游
- 异步投递：接到 webhook 验签通过后立即返回 200，后台 goroutine 投递给 P7b（异步分析触发）
- 失败重试：下游投递失败指数退避 3 次，3 次仍失败入死信（仅日志，不入 DB）
- 监控指标：暴露 Prometheus 指标（接收数 / 验签失败数 / 重放数 / 投递成功数 / 失败数）
不做
- ❌ Alertmanager 协议集成
- ❌ 多 webhook 端点（只一个 /webhook/alert）
- ❌ 复杂 deduplication 规则（仅 fingerprint 兜底）
- ❌ 自动抑制 / 静默（用 Grafana 原生 silences）
- ❌ 飞书 / 钉钉 / 邮件通知（属于 P7b 投递目标，不在接收端）
5. 关键决策
决策 1：签名算法用 HMAC-SHA256（与 GitHub / Stripe 同款）
备选：
- A. HMAC-SHA256（选）
- B. RSA 签名
- C. 无签名 + IP 白名单
选择理由：
- ✅ 简单、广泛采用、与 Grafana 现有 Contact Point 集成无门槛
- ✅ secret 可轮换
- ❌ A 缺点：secret 泄露需要轮换
- 放弃 B：复杂、慢，无必要
- 放弃 C：私有化部署 IP 不固定（K8s）
决策 2：幂等用 Grafana Alert fingerprint
- Grafana Alert 自动给每个 alert 一个 fingerprint 字段（基于 labels 哈希）
- 同 fingerprint 1h 内只投递 1 次（LRU 缓存 10000 条）
- 不持久化（重启清空；接受——重启期间告警丢失可接受）
决策 3：异步投递，立即 200
- 接到 webhook 后立即 c.Writer.WriteHeader(200) 返回
- 后台 goroutine 通过内部 channel 投递给 P7b
- 理由：Grafana Alert webhook 有超时，不阻塞 Alert 链路
决策 4：失败重试 + 死信日志
- 重试：指数退避（1s, 5s, 30s），最多 3 次
- 死信：3 次仍失败，写 audit-YYYY-MM-DD.jsonl 的 webhook_dead_letter 事件，不入 DB
- 理由：失败原因通常是下游 bug，不该用 DB 永久存
6. 基本概念与信息结构
6.1 接口与对接
入参（HTTP 端点）：
POST /webhook/alert
Headers:
  X-AI-Signature: hex(HMAC-SHA256(secret, body))
  X-AI-Timestamp: unix_seconds
  Content-Type: application/json
Body: Grafana Alert webhook 标准 payload（含 alerts[], fingerprint 等字段）
出参（内部 channel）：
// 投递到 P7b 的内部事件
type AlertEvent struct {
    Fingerprint  string                 `json:"fingerprint"`
    AlertName    string                 `json:"alert_name"`
    Status       string                 `json:"status"`      // firing | resolved
    Labels       map[string]string      `json:"labels"`
    Annotations  map[string]string      `json:"annotations"`
    StartsAt     time.Time              `json:"starts_at"`
    EndsAt       time.Time              `json:"ends_at,omitempty"`
    ReceivedAt   time.Time              `json:"received_at"`
    RawPayload   json.RawMessage        `json:"raw_payload"`  // 完整原文
}
对接关系：
- 上游消费方：Grafana Alerting Contact Point（HTTP POST）
- 下游消费方：P7b 告警 → AI 异步分析触发（内部 channel）
- 配置：config/webhook.yaml（secret / timestamp_skew）
数据流向：
flowchart LR
    Grafana[Grafana Alert] -->|HTTP POST<br/>+ HMAC + Timestamp| Receiver[Webhook 接收端]
    Receiver -->|验签失败| Reject[立即 401]
    Receiver -->|验签通过<br/>指纹去重| Channel[(内部 channel)]
    Receiver -->|死信| Audit[P9 审计]
    Channel --> P7b[P7b 异步分析触发]
    Receiver --> Metrics[Prometheus 指标]
6.2 数据结构
type WebhookConfig struct {
    Secret         string `yaml:"secret"`           // 从 ${GRAFANA_WEBHOOK_SECRET} 注入
    TimestampSkew  int    `yaml:"timestamp_skew"`  // 秒，默认 300
}

// 内部去重 LRU
type Deduper struct {
    mu       sync.Mutex
    seen     map[string]time.Time  // fingerprint -> first seen
    capacity int
    ttl      time.Duration
}
6.3 例子即规范
现状：Grafana Alert webhook 打到裸 HTTP 端点，任何能访问 URL 的人都能伪造告警；重复告警会触发 AI 重复分析。

提议后的形态：
- Grafana Contact Point 配置 Contact Point URL = https://ai-core.internal/webhook/alert
- Contact Point headers 加 X-AI-Signature 和 X-AI-Timestamp
- secret 配置在双方环境变量
- 1h 内同一 fingerprint 不重复跑下游
边界与非法情况：
- ❌ 缺 X-AI-Signature → 401
- ❌ 签名不匹配 → 401
- ❌ 时间戳与服务器差 > 300s → 401（防重放）
- ❌ fingerprint 1h 内重复 → 200 但不投递下游（去重生效）
- ❌ 下游 channel 满 → 重试 3 次，仍失败入死信日志
- ✅ 正常首次告警 → 200 + 后台投递下游
7. 验收标准
- [ ]  Grafana Alert 触发 → 端到端 200 + 后台投递
- [ ]  错误签名 / 缺签名 / 时间戳过期 → 401 + 不投递
- [ ]  同一 fingerprint 1h 内重复触发只投递 1 次
- [ ]  下游 channel 满时重试 3 次后入死信日志
- [ ]  Prometheus 指标可观察（接收数 / 验签失败数 / 投递成功数 / 死信数）
8. 不做（边界）
- Alertmanager 协议
- 多端点 / 多 secret
- 复杂去重规则
- 自动抑制 / 静默
9. 依赖
- 上游（谁调我）：Grafana Alerting Contact Point
- 下游（我调谁）：P7b 异步分析触发（通过内部 channel）
- 被依赖：P9 审计（接收死信日志）
10. 状态记录
- 2026-07-10：草案创建（与 P7b 配套）



P7b: 告警 → AI 异步分析触发

标签：proposal FullSpec
所属模块：异步分析触发
状态：草案
1. 动机 / 用户故事
P7a 验签完、把告警投递到内部 channel 之后，需要决定：该告警应该触发哪个 Playbook？参数怎么映射？结果推到哪里？

共同需求：从告警到 AI 结论 + 推送的完整链路，由配置驱动，无代码改动即可适配新告警。
2. 目标用户
SRE / 值班主管（编写 mapping + 推送目标配置）；值班（接收结论推送）。
3. 现有做法及其不足
- Grafana Alert → Slack webhook（纯文本，无分析）
- 每个新告警都要 SRE 写新 Playbook + 新推送代码
- 没有复用：告警 / Playbook / 推送三者关系混乱
4. 本提案范围与明确不做
做
- Alert → Playbook 映射：config/alert-mapping.yaml，按 alert_name + labels 匹配 → playbook_id
- 参数映射：把 alert labels 提取成 Playbook 参数（param_mapping: { env: "alert.labels.env" }）
- 异步任务：从 P7a channel 接收 AlertEvent，按 mapping 启动 Playbook（走 P6）
- 结果推送：跑完后推到 webhook URL（Slack / 飞书 / 自定义），Markdown 格式
- 超时与取消：单个 Playbook run 有 timeout（默认 5min），超时强制取消
- 审计：每个告警触发 / 跑结果 / 推送结果入 P9 审计
不做
- ❌ Alertmanager 集成
- ❌ Email / 钉钉 推送
- ❌ 多推送通道并行（单 webhook URL）
- ❌ 推送模板引擎复杂化（用 Go text/template 即可）
5. 关键决策
决策 1：Alert → Playbook 映射走配置（不是代码）
备选：
- A. 配置驱动 config/alert-mapping.yaml（选）
- B. 写死在代码里
- C. 数据库动态管理
选择理由：
- ✅ 加新告警 → 加 YAML 配置 → 重启生效，无代码改动
- ✅ 与 Playbook 文件位置一致（data/playbooks/），Git 仓管理
- ❌ B 缺点：每次新告警要改代码、测、部署
- 放弃 C：MVP 不引入 DB 复杂度
决策 2：推送格式用 Markdown + 单 webhook URL
- 推送目标 1 个 webhook URL（环境变量 PUSH_WEBHOOK_URL）
- 推送内容：告警信息 + Playbook 步骤摘要 + AI 结论 + 关键证据 + 链接
- Slack / 飞书原生支持 Markdown，无需额外适配
决策 3：超时与取消
- 单 Playbook run timeout 默认 5min（可在 mapping 覆盖）
- 超时后 context.Cancel，子 goroutine 退出
- 推送消息带 [超时] 前缀
决策 4：审计由 P9 统一处理
- 不在 P7b 单独写审计代码
- 通过 P9 的 Logger interface 上报：LogWebhookTrigger / LogPlaybookRun
6. 基本概念与信息结构
6.1 接口与对接
入参（内部 channel，来自 P7a）：
// 来自 P7a 的 AlertEvent（参见 p7a §6.1）
type AlertEvent struct {
    Fingerprint  string            `json:"fingerprint"`
    AlertName    string            `json:"alert_name"`
    Status       string            `json:"status"`
    Labels       map[string]string `json:"labels"`
    Annotations  map[string]string `json:"annotations"`
    StartsAt     time.Time         `json:"starts_at"`
    ReceivedAt   time.Time         `json:"received_at"`
    RawPayload   json.RawMessage   `json:"raw_payload"`
}
出参（推送 webhook）：
type PushMessage struct {
    AlertName   string    `json:"alert_name"`
    Status      string    `json:"status"`
    Title       string    `json:"title"`
    Summary     string    `json:"summary"`       // Markdown
    Evidence    []string  `json:"evidence"`      // 关键证据链接
    PlaybookRun string    `json:"playbook_run"`  // Run ID
    Duration    int64     `json:"duration_ms"`
    Timestamp   time.Time `json:"timestamp"`
}
对接关系：
- 上游：P7a 告警 webhook 接收（内部 channel）
- 下游消费方 1：P6 Playbook 引擎（执行 Run）
- 下游消费方 2：推送 webhook URL（Slack / 飞书）
- 审计：P9 审计日志（通过 Logger interface）
- 配置：config/alert-mapping.yaml + 环境变量 PUSH_WEBHOOK_URL
数据流向：
flowchart TB
    Channel[(P7a 内部 channel)] --> Dispatcher[Mapping 匹配]
    Dispatcher -->|命中| Runner[启动 P6 Playbook Run]
    Dispatcher -->|未命中| Default[推送 Default 消息<br/>仅告警 + 链接]
    Runner -->|正常完成| Formatter[格式化结果]
    Runner -->|超时| Timeout[格式化超时消息]
    Runner -->|失败| Failed[格式化失败消息]
    Formatter --> Push[推送到 webhook]
    Timeout --> Push
    Failed --> Push
    Default --> Push
    Runner --> Audit[P9 审计: playbook_run]
    Push --> Audit2[P9 审计: webhook_push]
6.2 数据结构
type AlertMapping struct {
    AlertName    string            `yaml:"alert_name"`         // 精确匹配
    LabelMatcher map[string]string `yaml:"label_matcher,omitempty"`  // labels 包含匹配
    PlaybookID   string            `yaml:"playbook_id"`        // 命中后跑哪个 playbook
    Timeout      string            `yaml:"timeout,omitempty"`  // 如 "10m"
    ParamMapping map[string]string `yaml:"param_mapping"`      // alert.labels.xxx → playbook param
    Default      bool              `yaml:"default,omitempty"`  // 默认 mapping（兜底）
}

type AlertMappingConfig struct {
    Mappings []AlertMapping `yaml:"mappings"`
}
配置文件示例：
# config/alert-mapping.yaml
mappings:
  - alert_name: "CheckoutLatencyHigh"
    label_matcher:
      severity: warning
    playbook_id: checkout-latency-investigation
    timeout: 10m
    param_mapping:
      env: "alert.labels.env"
      time_range: "1h"
  - alert_name: "PaymentErrorSpike"
    playbook_id: payment-error-spike-triage
    param_mapping:
      env: "alert.labels.env"
  - default: true   # 未匹配时：仅推送告警原文 + 跳转 Grafana 链接
6.3 例子即规范
现状：值班收到告警后手动跑 Playbook + 自己写结论 + 自己复制粘贴到 Slack。

提议后的形态：
1. Grafana Alert 触发 → P7a 验签 → P7b 接收
2. 按 mapping 匹配 → 启动 checkout-latency-investigation Playbook
3. Playbook 跑完（或超时）→ 格式化结果 → 推送到 Slack
4. 值班看到 Slack 消息 = 告警信息 + AI 结论 + 关键证据 + Playbook run 链接
边界与非法情况：
- ❌ alert_name 不匹配任何 mapping → 走 default: true 兜底（仅推送告警原文）
- ❌ Playbook 不存在 / 解析失败 → 推送"Playbook 配置错误"消息 + 审计 + 不重试
- ❌ Playbook run 超时 → 推送带 [超时] 前缀
- ❌ 推送 webhook 返回 5xx → 重试 3 次，仍失败入死信
- ✅ 正常告警 + 命中 mapping → 跑 Playbook + 推 Slack + 审计
7. 验收标准
- [ ]  alert_name 命中 mapping → 自动跑对应 Playbook + 推送结论
- [ ]  alert_name 未命中 → 走 default 推送
- [ ]  alert labels 按 param_mapping 提取成 Playbook 参数
- [ ]  Playbook run 超时 → 推送带 [超时] 前缀
- [ ]  推送失败重试 3 次后入死信日志
- [ ]  每个告警触发 / Playbook 跑 / 推送结果入 P9 审计
8. 不做（边界）
- Alertmanager 协议
- Email / 钉钉
- 多推送通道并行
- 推送模板复杂化
9. 依赖
- 上游：P7a 告警 webhook 接收（AlertEvent 来自 P7a channel）
- 依赖：P6 Playbook 引擎（执行 Run）
- 依赖：P9 审计日志（上报所有事件）
- 被 P1 依赖（可选）：用户手动 run_playbook 走相同链路
10. 状态记录
- 2026-07-10：草案创建（与 P7a 配套）


P8: Skill 系统（双形态：Middleware + Skills MCP）

标签：proposal FullSpec
所属模块：Skill 系统（双形态：Middleware + Skills MCP）
状态：草案（2026-07-10 重写：原 SkillLoader → Skills 双形态）
1. 动机 / 用户故事
团队已经有 SOP 文档（runbook）。某些 SOP 不想每次都让用户写 prompt 描述，而是用一条命令触发："/check-cart" 自动跑 "Checkout 服务健康检查"。

额外价值（差异化卖点）：沉淀的 Skills 不仅能被项目 4 内部 Agent 消费，还能通过 Skills MCP Server 让 Cursor / Claude Desktop / IDE 等外部 AI 工具消费——团队所有 AI 工具共享一份 SOP。

共同需求：把常用 SOP 沉淀成 Skill，内部 Agent 用 Skill Middleware 高效消费，外部 AI 工具通过 MCP 协议消费。
2. 目标用户
- 主要：项目 4 内部 AI Agent（通过 Skill Middleware）
- 次要：外部 AI 工具（Cursor / Claude Desktop / IDE），通过 Skills MCP Server
3. 现有做法及其不足
- 每次值班都重新描述一遍排查步骤
- SOP 散落在 Confluence，找不到
- 团队多 AI 工具（Cursor、Grafana 插件、独立 Agent）各自维护一份 SOP，无法共享
4. 本期范围与明确不做
做
形态 A：Eino Skill Middleware（内部 Agent 消费）
- Skill 定义：Markdown + YAML frontmatter（name / description / allowed-tools / parameters）
- Eino skill.NewBackendFromFilesystem 加载 data/skills/
- skill.NewTyped[Message] 注入 ChatModelAgent Handlers
- 用户输入 /xxx slash command 自动调对应 skill
- 用户问相关问题时 Agent 语义匹配相关 skill（embedding Top-K，阈值 0.7）
形态 B：Skills MCP Server（对外暴露）
- 与 Skill Middleware 共用同一 data/skills/ 目录（文件单一来源）
- 通过 mcp-go 起 MCP Server，监听 Streamable HTTP
- 提供 9 个工具（见 §6）
共用部分
- Skill 文件结构统一（Markdown + YAML frontmatter）
- Skill 注册中心：Git 管理（data/skills/*.md）+ DB 索引（folder_uid 过滤）
- 内置示例：3+ skill
- 完备 CRUD（Web UI + skills.* MCP 工具，详见 P8 决策 6）
- AI 辅助生成 + 人工确认（P10 沉淀流程，详见 P8 决策 7）
- 跨项目共享通过 Shared Folder 模式（详见 P8 决策 5）
Skill Web 可视化
- Web UI 编辑器：Markdown 编辑器 + frontmatter 表单
- CRUD 完整（见 P8 决策 6）：
    - Create：表单 / MCP create_skill
    - Read：列表 + 详情
    - Update：编辑器 / MCP update_skill
    - Delete：弹窗确认 / MCP delete_skill
- AI 辅助生成预览：用户说"沉淀为 skill" → AI 生成草稿 → 编辑器中预览
- 跨项目共享入口：申请共享时 Folder 选择器（含 Shared Folder）
- 审批中心：pending 列表 + 草稿预览 + approve / reject
不做
❌ Skill marketplace / 跨团队共享
核心立场：跨项目共享通过 Grafana Shared Folder 模式实现（P8 决策 5），不引入 marketplace 概念。

为什么不做：
- Marketplace 是 SaaS 概念（公共仓库 + 评分 + 下载）
- 与 Grafana Folder Permission 体系重复
- 引入新概念（marketplace owner / 评分 / 评论）
- 维护成本高
替代方案（Shared Folder 模式）：
- 所有项目共享的 Skill 放在 Shared Folder
- P10 晋升流程（private → shared，Folder 选择器）
- Folder Admin 审批（Permission=4 的 User / Team / Role）
|维度|Shared Folder 模式|
|-|-|
|隔离单元|Grafana Folder|
|权限|Grafana Folder Permission|
|跨项目共享|绑 Shared Folder|
|审批|Folder Admin|
|评分|无|

❌ Skill 完全无人工自动生成 / AI 编写
核心立场：不做完全无人工的 Skill 自动生成。做的是"AI 辅助生成 + 人工确认"（和 P6 Playbook 一致，P8 决策 7）。

为什么不做：
- 自动生成的 Skill 不可控（幻觉 / 错误步骤 / 含糊描述）
- 引入低质量 Skill 污染知识库
- 用户无法理解 Skill 内容（不调试）
三种 Skill 生成模式：
|模式|含义|是否做|
|-|-|-|
|完全无人工|LLM 写完整 Skill Markdown → 直接入库|❌ 不做|
|AI 辅助 + 人工确认|LLM 从对话总结 Skill 骨架 → 用户编辑 → 入库|✅ 做（P10 沉淀流程）|
|完全人工|用户手写 Markdown（高级用户 / 复杂 Skill）|✅ 做（Web UI 编辑器）|

AI 辅助生成的边界：

✅ AI 能做的：
- 从对话历史提取 SOP 步骤
- 生成 frontmatter（name / description / tags / allowed-tools）
- 推断 Skill 类型（debug / runbook / template / check）
- 复用对话中提到的 metric / service / dashboard
❌ AI 不能做的（要求用户补）：
- 模糊的"如果 X 就 Y 否则 Z" 条件分支
- 业务特有的"踩坑提醒"（隐性知识）
- 跨服务依赖的优先级判断
实际流程（P10 沉淀联动）：
User: "保存这个排查为 skill"
  ↓
AI Agent 从 Session 历史提取最近 5 轮对话
  ↓
Skill Generator 总结 SOP（Markdown + frontmatter）
  ↓
Web UI 弹出预览（Markdown 编辑器 + 表单）
  ↓
User 编辑 / 补全 / 确认
  ↓
写入 private（owner = user_id, folder_uid = ""）
其他不做项
- ❌ Skill 调用统计仪表盘（用 Grafana + Loki 解决）
- ❌ Skill 调用链 / 嵌套（Skill 内不调 Skill；避免依赖复杂化）
- ❌ Skill 计费 / 付费分发（内部工具，无商业化）
- ❌ Skill Web 端市场页 / 评分 / 评论（不做 marketplace）
5. 关键决策
决策 1：使用 Eino 的 Skill 能力
备选：
- A. Eino Skill Middleware + FileSystem Backend（选）—— 字节官方
- B. 自实现 SkillLoader + RAG 检索
选择理由：
- ✅ Eino 原生 Skill Middleware + skill.NewBackendFromFilesystem 直接加载目录
- ✅ 与 ChatModelAgent Handlers 集成（1-2 行）
- ✅ 自实现约 500 行（Markdown 解析 + frontmatter + RAG + 注入）+ 维护成本
- ❌ B 缺点：重复造轮子
决策 2：Skills 双形态（内部 Middleware + 对外 MCP）
核心洞察：Skills 物理文件只有一份（data/skills/*.md），但消费方式分两种：
|消费方|协议|工具|
|-|-|-|
|项目 4 内部 Agent|进程内|Eino Skill Middleware|
|外部 AI 工具（Cursor 等）|MCP 协议|Skills MCP Server|

为什么不是单纯用一种：
- 只用 Skill Middleware：外部 AI 工具消费不到，差异化卖点丢失
- 只用 Skills MCP：内部 Agent 走协议栈，没必要且增加延迟
复用点：两个 consumer 都从 data/skills/ 读，统一 Skill 加载器接口（internal/skills/loader.go）。
决策 3：自动检索基于 embedding 相似度（内部 Agent）
- 用户问题 embedding vs skill description embedding
- 召回 Top-3 skill 注入 system prompt
- 阈值 0.7 以下不注入（避免噪音）
决策 4：Slash command 与自动检索并行
- 用户输入 / 看到命令提示列表
- 选中即调
- 与自动检索不冲突（slash 是显式、自动检索是隐式）
决策 5：Skill 可见性（private / shared 两层，P10 联动，shared 可绑 Shared Folder）
核心立场：Skill 可见性分两层，shared 直接绑定 Grafana Folder，复用 Folder Permission：
|可见性|谁能看|谁能跑|谁能编辑|
|-|-|-|-|
|private|仅 OwnerID（创建者）|仅 OwnerID|仅 OwnerID|
|shared|Grafana Folder Permission ≥ View|Permission ≥ View|Permission ≥ Edit（含 owner）|

shared 绑定的 Folder 两种：
|Folder 类型|FolderUID|可见范围|
|-|-|-|
|项目 Folder（如 Payment）|真实 Grafana Folder UID|仅该 Folder 成员|
|Shared Folder（跨项目）|配置的 shared_folder_uid（约定单一全局）|所有项目成员|

为什么不再分三层（去掉 global）：
- ✅ Grafana Folder 本身是"项目隔离"单元
- ✅ shared 绑定 Folder（含 Shared Folder）后权限自动复用 Grafana Permission
- ❌ 三层会让审批 / 权限模型变复杂
来源：
- 手工创建：默认 private（owner = 当前用户）
- 对话沉淀（P10）：AI 总结生成 → private
- 晋升通过（P10）：private → shared，目标 Folder = 用户选的（项目 Folder 或 Shared）
决策 6：Skills MCP 工具集设计（含完备 CRUD）
|工具|权限|说明|
|-|-|-|
|list_skills|read|列出 skills（默认聚合 active + Shared + private）|
|search_skills|read|关键词 / 语义搜索（按 folder_uid）|
|get_skill|read|获取 skill Markdown 完整内容（必须传 folder_uid）|
|load_skill_for_agent|read|加载 skill 注入到指定 Agent|
|run_skill|write（HITL）|执行 skill|
|create_skill|write（HITL）|创建新 skill|
|update_skill|write（HITL）|修改现有 skill|
|delete_skill|write（HITL）|删除 skill（2026-07-10 新增）|
|promote_skill|write（HITL）|申请晋升（private → shared，2026-07-10 新增）|

write 工具触发 Eino Interrupt & CheckPoint，HITL 由 AI Core 统一调度（不是 MCP Server 自己处理）。

CRUD 矩阵：
|操作|MCP tool|权限|HITL|Web UI|
|-|-|-|-|-|
|Create|create_skill|write|✅|✅|
|List|list_skills|read|❌|✅|
|Get|get_skill|read|❌|✅|
|Update|update_skill|write|✅|✅|
|Delete|delete_skill|write|✅|✅（弹窗确认）|
|Run|run_skill|execute|✅|✅（dry_run 预览）|

权限校验：
- write 操作：Folder Permission ≥ Edit
- read 操作：Folder Permission ≥ View
- private：仅 OwnerID 可操作
Delete 语义：
- private delete：仅 OwnerID
- shared delete：Folder Admin 或 OwnerID
- 不做软删除（弹窗确认后硬删除）
write 工具触发 Eino Interrupt & CheckPoint，HITL 由 AI Core 统一调度（不是 MCP Server 自己处理）。
决策 7：AI 辅助生成 Skill + 人工确认（P10 沉淀流程联动，2026-07-10 新增）
核心立场：不做完全无人工的 Skill 自动生成。做的是"AI 辅助生成 + 人工确认"（和 P6 Playbook 决策 7 一致）。
|模式|含义|是否做|
|-|-|-|
|完全无人工|LLM 写完整 Skill Markdown → 直接入库|❌ 不做|
|AI 辅助 + 人工确认|LLM 从对话总结 Skill 骨架 → 用户编辑 → 入库|✅ 做（P10 沉淀流程）|
|完全人工|用户手写 Markdown（Web UI 编辑器）|✅ 做|

为什么不做"完全无人工"：
- 自动生成的 Skill 不可控（幻觉 / 错误步骤 / 含糊描述）
- 引入低质量 Skill 污染知识库
- 用户无法理解 Skill 内容（不调试）
- 不实现完全自动生成
AI 辅助生成的实现：
type SkillGenerator interface {
    // 从对话历史生成 Skill 草稿
    GenerateDraft(ctx, conversationHistory []Message) (*SkillDraft, error)
}

type SkillDraft struct {
    Frontmatter SkillFrontmatter  // name / description / tags / allowed-tools
    Body        string            // Markdown body
    SourceMeta  map[string]any    // 来源信息（session_id / 提取的对话）
}

func GenerateSkillDraft(ctx, sessionID, ownerID string) (*Skill, error) {
    // 1. 拉最近 5 轮对话
    history := sessionStore.GetRecentMessages(sessionID, 5)
    // 2. LLM 总结 SOP
    draft := skillGenerator.GenerateDraft(ctx, history)
    // 3. 返回草稿（不直接入库）
    return &Skill{
        ID:          uuid.New(),
        Visibility:  VisibilityPrivate,
        OwnerID:     ownerID,
        Frontmatter: draft.Frontmatter,
        Body:        draft.Body,
        SourceMeta:  draft.SourceMeta,
    }, nil
}
AI 能做的（自动生成）：

✅ 从对话历史提取 SOP 步骤
✅ 生成 frontmatter（name / description / tags / allowed-tools）
✅ 推断 Skill 类型（debug / runbook / template / check / guide）
✅ 复用对话中提到的 metric / service / dashboard
✅ 总结代码片段 / 错误信息

AI 不能做的（要求用户补）：

❌ 模糊的"如果 X 就 Y 否则 Z" 条件分支
❌ 业务特有的"踩坑提醒"（隐性知识）
❌ 跨服务依赖的优先级判断
❌ 不在对话中出现的上下文（如团队规范）

实际流程（P10 沉淀联动）：
User: "保存这个排查为 skill"
  ↓
AI Agent 从 Session 历史提取最近 5 轮对话
  ↓
Skill Generator 总结 SOP（Markdown + frontmatter）
  ↓
Web UI 弹出预览（Markdown 编辑器 + 表单）
  ↓
User 编辑 / 补全 / 确认
  ↓
写入 private（owner = user_id, folder_uid = ""）
  ↓
可选：用户可申请晋升（private → shared）
Edit 后续：用户编辑后保存的 Skill 走 update_skill MCP tool（HITL）。
6. 基本概念
6.1 Skill 文件格式
---
name: checkout-troubleshoot
description: checkout 服务健康检查与故障排查
timeout: 60s
visible: team
allowed-tools: [grafana_mcp/query_prometheus, grafana_mcp/query_loki]
parameters:
  - name: time_range
    type: string
    default: 1h
slash-command: /check-cart
---

# Checkout 服务健康检查

按以下顺序检查：

1. 查 p95 latency
2. 查错误率
3. 如果错误率高，查 Loki 错误日志
4. 列出可疑下游服务

每步用 Grafana MCP 工具查询，结果汇总到 Markdown 报告。
6.2 Skill 结构体（P10 联动）
type Skill struct {
    ID           string       `yaml:"name"`
    Description  string       `yaml:"description"`
    Body         string       // Markdown body
    Visible      string       `yaml:"visible"`     // legacy: private | team（保留兼容）
    SlashCmd     string       `yaml:"slash-command,omitempty"`
    AllowedTools []string     `yaml:"allowed-tools,omitempty"`
    Parameters   []SkillParam `yaml:"parameters,omitempty"`
    Timeout      string       `yaml:"timeout,omitempty"`

    // P10 晋升流程联动（两层可见性，优先于 Visible）
    Visibility   Visibility   `yaml:"-" db:"visibility"`         // private | shared
    FolderUID    string       `yaml:"-" db:"folder_uid"`         // shared 时必填：Grafana Folder UID
    OwnerID      string       `yaml:"-" db:"owner_id"`           // 创建者 user_id
    UsageCount   int64        `yaml:"-" db:"usage_count"`        // 调用次数（用于审批参考）
}

type Visibility string

const (
    VisibilityPrivate Visibility = "private"
    VisibilityShared  Visibility = "shared"
)

type SkillParam struct {
    Name    string `yaml:"name"`
    Type    string `yaml:"type"`
    Default string `yaml:"default,omitempty"`
}
可见性语义（与 P10 / P6 一致）：
|可见性|谁能看|谁能跑|谁能编辑|
|-|-|-|-|
|private|仅 OwnerID|仅 OwnerID|仅 OwnerID|
|shared|Grafana Folder FolderUID Permission ≥ View|Permission ≥ View|Permission ≥ Edit（含 owner）|

可见性来源：
- 手工创建：默认 private（owner = 当前用户）
- 从对话沉淀：AI 总结生成 → private（P10）
- 晋升通过：private → shared，关联 target_folder_uid（P10）
type SkillParam struct {
    Name    string yaml:"name"
    Type    string yaml:"type"
    Default string yaml:"default,omitempty"
}

### 6.3 架构

data/skills/*.md          ←  物理文件单一来源
       ↓
internal/skills/loader.go ←  统一加载器（解析 + 缓存）
       ↓                        ↓
  ┌────┴────┐               ┌────┴────┐
  │  Eino   │               │ Skills  │
  │  Skill  │               │  MCP    │
  │ Backend │               │ Server  │
  └────┬────┘               └────┬────┘
       ↓                          ↓
  ChatModelAgent           外部 AI 工具
  (项目 4 内部)         (Cursor/Claude Desktop)

## 7. Skills MCP Server 实现（差异化关键）

```go
// cmd/assistant-mcp/main.go（P4 单 server，含 skills namespace 工具）
package main

import (
    "github.com/mark3labs/mcp-go/server"
    "github.com/mark3labs/mcp-go/mcp"
)

func main() {
    skills, _ := loadSkillsFromDir("./data/skills")
    srv := server.NewMCPServer("skills", "1.0.0")

    srv.AddTool(mcp.NewTool("list_skills", ...), handleListSkills(skills))
    srv.AddTool(mcp.NewTool("search_skills", ...), handleSearchSkills(skills))
    srv.AddTool(mcp.NewTool("get_skill", ...), handleGetSkill(skills))
    srv.AddTool(mcp.NewTool("load_skill_for_agent", ...), handleLoadSkill(skills))
    srv.AddTool(mcp.NewTool("run_skill", ...), handleRunSkill(skills))
    srv.AddTool(mcp.NewTool("create_skill", ...), handleCreateSkill(skills))
    srv.AddTool(mcp.NewTool("update_skill", ...), handleUpdateSkill(skills))

    server.NewStreamableHTTPServer(srv,
        server.WithEndpointPath("/mcp"),
    ).Start(":8083")
}
外部 AI 工具（如 Cursor）通过 MCP 客户端连接 http://assistant-mcp:8080/mcp，可消费所有 skills.* 工具。
8. 验收标准
形态 A（内部）
- [ ]  内置 3+ skill 示例
- [ ]  用户输入 /check-cart 触发对应 skill（Eino Skill Middleware 路径）
- [ ]  用户问相关问题时 Agent 自动召回 skill（embedding 阈值 0.7）
- [ ]  Skill allowed-tools 限制生效
形态 B（外部）
- [ ]  Skills MCP Server 启动成功，监听 :8083/mcp
- [ ]  外部 MCP 客户端（curl / mcp-go client / Cursor）能调 list_skills 返回 JSON-RPC 响应
- [ ]  get_skill 返回 Skill 完整 Markdown
- [ ]  write 类工具（run_skill / create_skill / update_skill）触发 HITL（HITL 由 AI Core 统一处理，不在 MCP Server 内）
共用
- [ ]  内部 Middleware 与外部 MCP Server 共用同一 data/skills/ 目录（修改一个文件两边都生效）
- [ ]  Web UI 编辑立即生效
- [ ]  Skill 结构校验：缺 name/description 自动拒绝加载
9. 不做（边界）
与 §4 不做段对齐：
- ❌ Skill marketplace / 跨团队 marketplace（用 Grafana Shared Folder 模式替代）
- ❌ 完全无人工 AI 自动生成 Skill（用 AI 辅助 + 人工确认替代）
- ❌ Skill 调用统计仪表盘（用 Grafana + Loki 解决）
- ❌ Skill 调用链 / 嵌套（Skill 内不调 Skill）
- ❌ Skill 计费 / 付费分发（内部工具）
- ❌ Skill Web 端市场页 / 评分 / 评论（不做 marketplace）
10. 接口与对接
10.1 对外接口（双形态）
形态 A：Eino Skill Middleware（内部 Agent）
|接口|类型|消费者|
|-|-|-|
|skill.Backend（FileSystem Backend）|Eino interface|P1 ChatModelAgent Handlers|
|Skill 自动注入 system prompt|Eino Middleware|P1 Agent 内部消费|

形态 B：Skills MCP Server（对外暴露）
|工具|权限|消费者|
|-|-|-|
|list_skills / search_skills / get_skill|read|外部 AI 工具（Cursor / Claude Desktop）|
|load_skill_for_agent|read|外部 AI 工具（注入指定 Agent）|
|run_skill / create_skill / update_skill|write（HITL）|外部 AI 工具|

10.2 依赖接口（本模块消费）
|来源|接口|用途|
|-|-|-|
|data/skills/*.md|文件系统|Skill 物理文件单一来源|
|Eino Skill API|skill.NewBackendFromFilesystem|内部加载|
|mcp-go Server API|server.NewMCPServer|对外暴露|

10.3 数据流向
flowchart TB
    Skills[(data/skills/*.md<br/>物理文件单一来源)]
    Skills --> Loader[Skill 加载器]
    Loader -->|形式 A| EinoBackend[Eino Skill Backend]
    Loader -->|形式 B| SkillsMCP[Skills MCP Server]
    EinoBackend --> P1[P1 AI Agent<br/>Middleware 注入]
    SkillsMCP --> Cursor[外部 AI 工具<br/>Cursor/Claude Desktop]
    SkillsMCP --> P1[Eino Skill Middleware<br/>通过 P4 MCP 套件]
10.4 例子即规范
现状：每次值班重新描述 SOP；SOP 散落在 Confluence；多 AI 工具各自维护 SOP。

提议后的形态：
- 内部：用户在 Grafana 插件输入"checkout 出问题了" → Agent 自动召回 checkout-troubleshoot skill → 按 SOP 排查
- 对外：Cursor 用户连接 assistant-mcp:8080/mcp → 输入"列出排查 skills" → 调 skills.list_skills → 获得清单
边界与非法情况：
- ❌ Skill 文件缺 name / description → 拒绝加载 + 日志报警
- ❌ write 工具（run_skill / create_skill / update_skill）→ 触发 Eino Interrupt（HITL）
- ❌ Skill marketplace / 跨团队共享 → 不做
- ✅ 内部 Agent 与外部 MCP 客户端共用同一文件源
11. 依赖
- 依赖 P1：AI Agent 通过 Skill Middleware 加载 Skills
- 依赖 P4：MCP Server 套件提供 Skills MCP Server 进程
- 依赖 P10：可见性 / 晋升（private / shared 两层，审批通过后转 shared）
- 被 P5 依赖（可选）：Knowledge MCP 可引用 Skills 作为 runbook 元数据
- 被 P6 依赖（可选）：Playbook 步骤可调用 Skill
- 被 Grafana Folder API 依赖：shared 对象校验 Permission
11. 状态记录
- 2026-07-10：草案创建
- 2026-07-10：定型为双形态（Middleware + Skills MCP）

P9: 审计日志（合规）

标签：proposal FullSpec
所属模块：审计日志（合规）
状态：草案
1. 动机 / 用户故事
合规 / 安全团队问"上周谁调过 create_dashboard 这个工具？参数是什么？结果如何？"

共同需求：所有 LLM 调用、工具调用、用户操作都有审计日志。
2. 目标用户
合规 / 安全 / SRE Lead（审计查询）；全体（被审计对象）。
3. 现有做法及其不足
- 通用 LLM 框架（Eino）的 Callback 机制仅输出到 slog，没落盘
- 仅 stdout，重启清空
- 无 Session / User 关联（审计需要的核心信息）
- 缺 HITL 审批事件记录（合规要求"谁批的什么操作"必须可查）
4. 本期范围与明确不做
做
- 统一审计接口（项目 4 内部 internal/audit/）
- 6 类事件：tool_call / llm_call / user_action / playbook_run / webhook_trigger / hitl_approval（新增）
- 按天 JSON Lines 文件
- 异步批量写（buffer 4096，5s flush）
- 保留时长动态配置（默认 90 天）
- 敏感字段脱敏（默认规则 + 可自定义）
- SessionID + UserID 全链路关联
- HITL 审批事件：write 类工具调用的审批人 / 决策（approve/reject/modify）/ 时间 / 审批前后参数 diff / CheckPoint ID
- 跨进程投递：通过 Eino Callback 机制自动 capture（tool_call / llm_call）；HITL 决策由 approval Middleware 上报
不做
- ❌ 实时告警 / 可视化仪表盘（用 Grafana + Loki 解决）
- ❌ SQL 查询接口（jq / grep 足够）
- ❌ 跨租户聚合（单租户）
- ❌ 审计日志自身审计（避免无限递归）
- ❌ 告警级别 / 严重度（用 Status + Error 表达）
5. 关键决策
决策 1：审计放在项目 4，不放 Eino 框架
- Eino 是通用 AI 框架，不该管业务合规
- 项目 4 有自己的审计 schema（特别是 HITL 审批事件）
- 通过 Eino Callback 机制注入到项目 4 的 audit logger
- 跨项目复用：抽到内部库
决策 2：异步 buffer + 降级同步
- 正常：写 channel → 异步 flush 到文件
- buffer 满：降级同步写，不丢日志
- 写失败：不阻塞业务（业务已经用日志上报过）
决策 3：敏感字段默认脱敏
- 默认 key 列表：password / api_key / token / authorization / cookie / 等
- 用户可加自定义 key
- 嵌套 map 递归处理
决策 4：保留时长可配置，不删除
- 默认 90 天（环境变量）
- 每日清理一次
- 不允许短期保留（如 < 7 天）但默认安全
6. 基本概念
type Event struct {
    ID         string         `json:"id"`               // UUID
    Type       string         `json:"type"`             // tool_call | llm_call | user_action | playbook_run | webhook_trigger | hitl_approval | object_personal_created | object_promote_requested | object_promote_approved | object_promote_rejected | error
    Timestamp  time.Time      `json:"timestamp"`
    SessionID  string         `json:"session_id,omitempty"`
    UserID     string         `json:"user_id,omitempty"`
    Actor      string         `json:"actor,omitempty"`       // user | system | webhook | agent
    Action     string         `json:"action,omitempty"`     // 子动作
    Resource   string         `json:"resource,omitempty"`   // 资源标识
    Status     string         `json:"status,omitempty"`     // success | failed | skipped | approved | rejected
    LatencyMs  int64          `json:"latency_ms,omitempty"`
    Input      map[string]any `json:"input,omitempty"`      // 自动脱敏
    Output     any            `json:"output,omitempty"`
    Error      string         `json:"error,omitempty"`
    Metadata   map[string]any `json:"metadata,omitempty"`
}

// HITL 审批专用字段（通过 Metadata 扩展）
type HITLMetadata struct {
    CheckPointID string         `json:"checkpoint_id"`      // Eino Interrupt CheckPoint ID
    Decision     string         `json:"decision"`           // approve | reject | modify
    DiffBefore   map[string]any `json:"diff_before,omitempty"`
    DiffAfter    map[string]any `json:"diff_after,omitempty"`
    Reason       string         `json:"reason,omitempty"`   // 拒绝 / 修改原因
    WaitSeconds  int64          `json:"wait_seconds"`       // 用户决策耗时
}

type Logger interface {
    Log(ctx context.Context, e Event) error
    LogToolCall(ctx context.Context, e ToolCallEvent) error
    LogLLMCall(ctx context.Context, e LLMCallEvent) error
    LogUserAction(ctx context.Context, e UserActionEvent) error
    LogPlaybookRun(ctx context.Context, e PlaybookRunEvent) error
    LogWebhookTrigger(ctx context.Context, e WebhookEvent) error
    LogHITLDecision(ctx context.Context, e Event, m HITLMetadata) error

    // P10 晋升流程联动
    LogObjectPersonalCreated(ctx context.Context, e Event, m PromoteMetadata) error
    LogObjectPromoteRequested(ctx context.Context, e Event, m PromoteMetadata) error
    LogObjectPromoteApproved(ctx context.Context, e Event, m PromoteMetadata) error
    LogObjectPromoteRejected(ctx context.Context, e Event, m PromoteMetadata) error

    Close() error
}

// PromoteMetadata（P10 晋升审计元数据）
type PromoteMetadata struct {
    ObjectType       string `json:"object_type"`       // skill | playbook
    ObjectID         string `json:"object_id"`
    ObjectName       string `json:"object_name,omitempty"`
    TargetFolderUID  string `json:"target_folder_uid,omitempty"`  // 申请的目标 Grafana Folder
    Decision         string `json:"decision,omitempty"`            // approve | reject
    RejectReason     string `json:"reject_reason,omitempty"`        // 拒绝原因
    ReviewerID       string `json:"reviewer_id,omitempty"`          // 审批者 user_id
    ApprovalID       string `json:"approval_id,omitempty"`          // ApprovalRequest.id
    WaitSeconds      int64  `json:"wait_seconds,omitempty"`         // 审批耗时
}
7. 验收标准
- [ ]  10 类事件全落盘（tool_call / llm_call / user_action / playbook_run / webhook_trigger / hitl_approval / object_personal_created / object_promote_requested / object_promote_approved / object_promote_rejected）
- [ ]  JSON Lines 格式可被 jq 解析
- [ ]  SessionID 通过 context 自动关联
- [ ]  敏感字段自动脱敏（默认规则覆盖 password / token 等）
- [ ]  异步批量：buffer 满时降级同步不丢日志
- [ ]  保留时长动态配置生效（环境变量）
- [ ]  写失败不影响业务（业务继续跑）
- [ ]  HITL 审批可查：jq 'select(.type=="hitl_approval")' audit-2026-07-10.jsonl | head 能看到完整决策记录（审批人/决策/diff/耗时）
- [ ]  审批人追溯：所有 write 类工具调用的 Event.UserID 字段 = 实际审批人（非 LLM 调用发起人）
- [ ]  P10 晋升可查：
    - jq 'select(.type=="object_personal_created")' 能看到私人对象创建记录
    - jq 'select(.type=="object_promote_requested")' 能看到申请共享的目标 folder_uid
    - jq 'select(.type=="object_promote_approved")' 能看到审批者 ID + 目标 folder_uid
    - jq 'select(.type=="object_promote_rejected")' 能看到拒绝原因（必填）
8. 不做（边界）
- 跨租户
- 实时告警
- SQL 查询接口
- 审计可视化
9. 接口与对接
9.1 对外接口（本模块提供）
|接口|类型|消费者|
|-|-|-|
|Logger.Log(ctx, Event)|Go interface|P1 / P5 / P6 / P7a / P7b / P10|
|Logger.LogToolCall / LogLLMCall / LogHITLDecision ...|Go interface|各模块按事件类型|
|Logger.LogObjectPersonalCreated / LogObjectPromoteRequested / LogObjectPromoteApproved / LogObjectPromoteRejected|Go interface|P10 晋升流程|
|按天 JSON Lines 文件 audit-YYYY-MM-DD.jsonl|文件|合规查询（jq / grep）|

9.2 依赖接口（本模块消费）
|来源|接口|用途|
|-|-|-|
|Eino Callback 机制|框架 API|自动 capture tool_call / llm_call|
|文件系统|本地目录|落盘|
|配置|环境变量|保留时长 / 路径|

9.3 数据流向
flowchart LR
    P1[P1 AI Agent<br/>Eino Callback] -->|Log| Logger[Audit Logger]
    P6[P6 Playbook] --> Logger
    P7a[P7a Webhook] -->|死信事件| Logger
    P7b[P7b 异步分析] --> Logger
    Logger -->|异步批量| File[(audit-YYYY-MM-DD.jsonl)]
    Logger -->|降级同步| File
9.4 例子即规范
现状：合规 / 安全问"上周谁调过 create_dashboard"，答不上来——日志只在 stdout。

提议后的形态：
$ jq 'select(.type=="hitl_approval" and .user_id=="alice")' audit-2026-07-10.jsonl
{"id":"...","type":"hitl_approval","user_id":"alice","action":"create_dashboard",
 "decision":"approve","checkpoint_id":"cp-xxx","diff_before":{...},"diff_after":{...}}
边界与非法情况：
- ❌ buffer 满 → 降级同步写（不丢日志）
- ❌ 文件写入失败 → 不阻塞业务 + 错误日志
- ❌ 审计日志自身审计 → 不做（避免无限递归）
- ✅ SessionID 通过 context 自动关联（无需业务传）
10. 依赖
- 被 P1 依赖：所有 LLM 调用 / 工具调用 / 用户操作均记录
- 被 P6 依赖：Playbook 运行结果
- 被 P7a 依赖：Webhook 触发 / 死信
- 被 P7b 依赖：告警异步分析
- 被 P10 依赖：晋升流程审计（object_personal_created / object_promote_*）
- 被 P1 Decision 6 依赖：HITL 审批事件由 approval Middleware 上报
10. 状态记录
- 2026-07-10：草案创建
- 2026-07-10：新增 hitl_approval 事件类型（合规要求）


P10: Skills / Playbook 个人 → 共享晋升流程

标签：proposal FullSpec
所属模块：Skills / Playbook 晋升流程
状态：草案（2026-07-10 简化：visibility 改为 private / shared 两层）
1. 动机 / 用户故事
- 工程师小王值班时发现一个排查模式有效，想沉淀为 playbook 但不愿直接公开（怕不完善、有错误）
- 工程师小李用 AI 跑了 5 次 /check-cart，想把这个 SOP 沉淀为 skill 但只想自己先用
- 团队积累了 20+ 个人 playbook/skill，部分好用值得共享，但没有晋升渠道
- Admin 没法发现"哪些值得共享"，没有审批入口
共同需求：
1. 用户可沉淀对话内容为 private 的 skill / playbook
2. private 用得好后，可申请共享到指定 Grafana Folder
3. Admin / Folder owner 在审批入口看 pending 请求，逐个通过 / 拒绝 + 评论
4. 通过后成为 shared（关联到 Grafana Folder，自动复用 Folder 权限）
5. 全过程有审计
2. 目标用户
- 所有 Grafana 团队成员：沉淀自己的 skill / playbook、申请共享
- Admin / Folder Admin（Grafana Folder Permission ≥ Admin）：审批共享申请
- 其他 Folder 成员（Permission ≥ View）：消费 shared 的 skill / playbook
3. 现有做法及其不足
- Confluence 写 SOP，没人看，过期无人在意
- 没有"先用后共享"的渐进机制
- 共享渠道靠私下传文件，没审计
- Admin 不知道"哪些内容值得共享"
4. 本提案范围与明确不做
做
对象：
- Skill（P8）：Markdown 格式的 SOP
- Playbook（P6）：YAML 结构化执行步骤
两层可见性：
|层级|谁能看|谁能编辑|谁能跑|
|-|-|-|-|
|private（个人）|仅创建者（user_id 匹配）|仅创建者|仅创建者|
|shared（Folder 内共享）|Folder Permission ≥ View 的成员|Folder Permission ≥ Edit（含创建者）|Folder Permission ≥ View|

关键立场：shared 直接复用 Grafana Folder Permission，不发明新权限模型。晋升到 shared 时由用户指定目标 Folder，权限随 Folder 走。

沉淀流程（用户 → private）：
1. 用户对话中说"沉淀为 playbook" / "保存为 skill"
2. AI Agent 总结对话内容
3. Skill：生成 Markdown 草稿（含 frontmatter）
4. Playbook：自动推断步骤结构（query / mcp_call / branch），生成 YAML 草稿
5. 写入 private/{user_id}/{object_type}/{uuid}.{ext}
6. 用户在 Web UI 看到草稿，可编辑后再确认
晋升流程（private → shared）：
1. 用户在 Web UI 看到自己的 private 对象
2. 点"申请共享" → Folder 选择器（列出 Grafana 团队所有 Folder + Shared Folder 选项）
3. 用户选择目标 Folder：
    - 项目 Folder（如 Payment）→ 项目内可见
    - Shared Folder → 跨项目可见
4. 创建 ApprovalRequest（pending, target_folder_uid = 用户选的 Folder）
5. Folder Admin 在审批入口看到 pending
    - 项目 Folder 申请 → 该 Folder Admin 审批
    - Shared Folder 申请 → Shared Folder Admin 审批
6. 审批动作：通过（→ shared，关联到 target_folder_uid）/ 拒绝 + 评论（→ 仍 private，通知用户）
7. 状态变更触发：移动文件 / 改 DB 状态 / 通知创建者
审批入口：
- Web UI 审批中心：pending 列表 + 历史
- 筛选：按类型（skill / playbook）/ 按申请人 / 按时间 / 按目标 Folder
- 详情：看草稿内容 + 创建者历史（跑过几次 / 反馈）
- 动作：通过 / 拒绝 + 评论
权限模型（复用 Grafana Folder Permission）：
- private：user_id == creator.user_id
- shared：Grafana Folder Permission ≥ View / Edit / Admin
审计（P9 联动）：
- object_personal_created：沉淀草稿
- object_promote_requested：申请共享
- object_promote_approved：审批通过
- object_promote_rejected：审批拒绝
不做
- ❌ 三层可见性（private / shared / global）：Grafana Folder 已覆盖"跨 Folder"概念，不需要 global 层
- ❌ 从聊天对话自动检测"应该沉淀"（只做显式"用户说沉淀"）
- ❌ AI 自动审批（必须人工，避免共享低质量内容）
- ❌ 跨 Folder 复制：shared 一旦绑定 Folder 即固定；要换 Folder 走删除 + 重新申请
- ❌ 私有对象加密（不做，DB 权限隔离即可）
- ❌ 评分 / 投票机制
- ❌ 版本控制（DB 自身有 updated_at，不做历史 diff UI）
5. 关键决策
决策 1：两层可见性（private + shared），不引入 global；shared 绑 Grafana Folder（含 Shared Folder）
备选：
- A. private + shared 两层（选）—— 共享靠 Grafana Folder Permission
- B. 三层（private + shared + global）
- C. 多层级（按 Grafana Role / Team 区分可见）
选择理由：
- ✅ Grafana Folder 本身就是"项目隔离单元"+ 权限边界
- ✅ 共享申请指定目标 Folder（含 Shared Folder 跨项目），权限自动复用 Grafana Permission
- ✅ 责任清晰：Folder Admin 审批自己 Folder；Shared Folder Admin 审批跨项目共享
- ❌ B 缺点：global 层与 Grafana 已有 Folder 重复，引入双体系
- 放弃 C：复杂度高，团队规模小不需要
Shared Folder 约定（2026-07-10 拍板）：
- 单一全局 Shared Folder（Grafana Folder UID 由 config 配置）
- 所有跨项目共享都绑这个 Folder
- Shared Folder Admin = 跨项目共享的审批者
决策 2：沉淀触发只支持显式（用户说"沉淀为 X"）
备选：
- A. 显式触发（选）—— 用户对话中说
- B. AI 自动检测 + 建议
选择理由：
- ✅ 简单：避免误判，用户意图明确
- ✅ 不引入 AI 检测逻辑
- ❌ B 缺点：误判会让用户烦，"建议你沉淀" 可能频繁出现
决策 3：审批必须人工，不做 AI 自动审批
选择理由：
- ✅ 内容质量把控：人审才能发现错误 / 不严谨
- ✅ 责任清晰：admin 拍板，避免 AI 推卸责任
- ✅ 简单：审批流是 Web UI 标准流程（CRUD + 状态机）
决策 4：审批者权限用 Grafana Folder Permission
复用 Grafana：
- 审批者必须是 target_folder_uid 的 Admin（Permission = 4）
- 普通用户能提交申请，但无法审批
- 不发明审批者角色，全部走 Grafana Folder Permission
决策 5：AI 辅助生成 Playbook / Skill 草稿 + 人工确认（2026-07-10 统一）
核心立场：不做完全无人工的 Playbook / Skill 自动生成。做的是"AI 辅助生成 + 人工确认"（和 P6 决策 7 / P8 决策 7 一致）。
|模式|含义|是否做|
|-|-|-|
|完全无人工|LLM 写完整 Playbook / Skill → 直接入库|❌ 不做|
|AI 辅助 + 人工确认|LLM 从对话总结骨架 → 用户编辑 → 入库|✅ 做（P10 沉淀流程）|
|完全人工|用户手写 YAML / Markdown（Web UI 编辑器）|✅ 做|

为什么不做"完全无人工"：
- 自动生成的 Playbook / Skill 不可控（幻觉 / 错误步骤 / 含糊描述）
- 引入低质量内容污染知识库
- 用户无法理解内容（不调试）
- 不实现完全自动生成
Playbook 步骤类型（AI 推断）：
|对话中识别|生成 step 类型|
|-|-|
|"查 checkout p95"|query（PromQL）|
|"如果错误率高就查 Loki"|branch + mcp_call（Loki）|
|"汇总报告"|template|
|"通知 Slack"|mcp_call（webhook）|

Skill 草稿生成（AI 总结）：
|对话中识别|生成 Skill 内容|
|-|-|
|"这个排查流程很好用"|SOP Markdown（步骤 + 现象 + 解决）|
|"我们用什么 metric 看错误率"|runbook 类型（步骤 + metric 表达式）|
|"下次遇到 X 就做 Y"|template 类型（通用模板）|

AI 不能做的（要求用户补）：
- 模糊的"如果 X 就 Y 否则 Z" 条件分支
- 业务特有的"踩坑提醒"（隐性知识）
- 跨服务依赖的优先级判断
- 不在对话中出现的上下文（如团队规范）
实际流程：
User: "把刚才的排查沉淀为 playbook" / "保存为 skill"
  ↓
AI Agent 从 Session 历史提取最近 5 轮对话
  ↓
Generator 推断步骤 / 总结 SOP
  ↓
Web UI 弹出预览（编辑器 + 表单）
  ↓
User 编辑 / 补全 / 确认
  ↓
写入 private（owner = user_id, folder_uid = ""）
决策 6：拒绝审批必须填评论
理由：
- ✅ 创建者知道为什么被拒，便于改进
- ✅ admin 不能随便拒绝（必须有理由）
- ✅ 拒绝记录可追溯
6. 基本概念与信息结构
6.1 接口与对接
对外接口（本模块提供）：
|接口|类型|消费者|
|-|-|-|
|Web UI 沉淀入口（前端组件）|React 组件|用户|
|Web UI 审批中心（前端组件）|React 组件|admin / folder admin|
|ApprovalService.SubmitRequest|Go function|P1 AI Agent（用户说"沉淀"时）|
|ApprovalService.Approve / Reject|Go function|Web UI 审批中心|
|ApprovalService.ListPending(role, folderUID)|Go function|Web UI 审批中心|
|PromotionService.MoveToShared(objID, folderUID)|Go function|ApprovalService|
|MCP tool promote_object|MCP tool|外部 AI 工具|

依赖接口（本模块消费）：
|来源|接口|用途|
|-|-|-|
|P6 Playbook 引擎|Engine.Upsert / Run / Get|Playbook CRUD + 跑|
|P8 Skill 系统|SkillBackend.Upsert / Search / Get|Skill CRUD|
|P3 会话|Session.GetMessageHistory|提取对话内容生成草稿|
|P5 知识库|Catalog.GetService(folderUID, name)|推断 service 关联|
|Grafana Folder API|GET /api/folders?query=...|列出可选目标 Folder|
|Grafana Folder API|GET /api/folders/:uid/permissions|校验当前用户是 Folder Admin|
|P9 审计|Logger.Log*|全链路审计|

数据流向：
flowchart TB
    %% 沉淀阶段
    User[用户] -->|"沉淀为 playbook"| Agent[P1 AI Agent]
    Agent -->|提取对话历史| Session[P3 Session]
    Agent -->|推断步骤结构| Generator[Playbook Generator]
    Generator -->|YAML 草稿| Private[(private<br/>user_id 隔离)]
    Agent -->|"沉淀为 skill"| SkillGen[Skill Generator]
    SkillGen -->|Markdown 草稿| Private

    %% 跑阶段
    User -->|跑自己的 private| Engine[P6 Engine]
    User -->|跑 shared| Engine
    Engine -->|审计| P9[P9 审计]

    %% 晋升阶段
    User -->|申请共享<br/>选 Folder| Submit[ApprovalService.Submit]
    Submit -->|target_folder_uid| Pending[(Approval<br/>Request)]
    Pending -->|目标 = 项目 Folder| FolderAdmin[项目 Folder Admin]
    Pending -->|目标 = Shared Folder| SharedAdmin[Shared Folder Admin]
    FolderAdmin -->|Web UI 审批中心| Approve[Approve / Reject + 评论]
    SharedAdmin -->|Web UI 审批中心| Approve
    Approve -->|通过| Promote[PromotionService.MoveToShared]
    Promote --> Private
    Promote --> Shared1[(shared<br/>绑项目 Folder)]
    Promote --> Shared2[(shared<br/>绑 Shared Folder<br/>跨项目可见)]
    Approve -.->|拒绝 + 评论| Notify[通知创建者]
    Notify --> User

    Approve -->|审计| P9
    Submit -->|审计| P9
6.2 数据结构
// Visibility 层级（两层）
type Visibility string

const (
    VisibilityPrivate Visibility = "private"
    VisibilityShared  Visibility = "shared"
)

// Object 类型
type ObjectType string

const (
    ObjectTypeSkill    ObjectType = "skill"
    ObjectTypePlaybook ObjectType = "playbook"
)

// ApprovalRequest（简化为 to_level 必为 shared）
type ApprovalRequest struct {
    ID              string     `db:"id"`                  // UUID
    ObjectType      ObjectType `db:"object_type"`
    ObjectID        string     `db:"object_id"`            // skill or playbook UUID
    TargetFolderUID string     `db:"target_folder_uid"`   // 用户选的目标 Grafana Folder
    RequestedBy     string     `db:"requested_by"`        // user_id
    Status          string     `db:"status"`              // pending | approved | rejected
    ReviewerID      string     `db:"reviewer_id,omitempty"`
    Comment         string     `db:"comment,omitempty"`        // 审批评论
    RejectReason    string     `db:"reject_reason,omitempty"`  // 拒绝时必填
    CreatedAt       time.Time  `db:"created_at"`
    ReviewedAt      *time.Time `db:"reviewed_at,omitempty"`
}

// P6 Playbook 增加字段（在原 P6 基础上）
type Playbook struct {
    // ... 原有字段
    Visibility   Visibility     `db:"visibility"`         // private | shared
    FolderUID    string         `db:"folder_uid"`         // shared 时必填（Grafana Folder）
    OwnerID      string         `db:"owner_id"`           // 创建者 user_id
    UsageCount   int64          `db:"usage_count"`        // 跑过几次（用于审批参考）
    // ... 原有字段
}

// P8 Skill 增加字段
type Skill struct {
    // ... 原有字段
    Visibility   Visibility     `db:"visibility"`
    FolderUID    string         `db:"folder_uid"`         // shared 时必填
    OwnerID      string         `db:"owner_id"`
    UsageCount   int64          `db:"usage_count"`
    // ... 原有字段
}
6.3 例子即规范
沉淀流程 - 现状：用户对话中说"这个排查流程很好用"，但没法保存；第二天又得重新描述。

沉淀流程 - 提议后的形态：
1. 用户对话中说"把刚才的排查沉淀为 playbook"
2. AI Agent 从 Session 历史提取最近 5 轮对话
3. 调用 Playbook Generator 推断步骤：
    - "查 checkout p95" → query (PromQL)
    - "如果错误率高" → branch + 条件
    - "查 Loki 错误" → mcp_call (grafana.query_loki，v1 不做 Loki)
    - "汇总报告" → template
4. 生成 YAML 草稿（含 metadata）
5. Web UI 弹出预览："已为您生成 playbook 草稿，请确认" → 用户可编辑 → 确认
6. 写入 private（owner = user_id, visibility = private, folder_uid = ""）
晋升流程 - 现状：用户想把好用的私有 playbook 共享给同事，只能私下传文件，没审计。

晋升流程 - 提议后的形态：
1. 用户在 Web UI 看到自己的 private playbook（用过 5 次）
2. 点"申请共享" → 列出用户有 Edit 权限的 Grafana Folder（payment / search）
3. 选 "payment" → 创建 ApprovalRequest（pending, target_folder_uid = payment）
4. payment Folder Admin 收到通知，在"审批中心"看到 pending
5. 点开看草稿 + 看到 usage_count = 5（跑过 5 次）
6. 通过 → Playbook.Visibility = shared, FolderUID = payment
7. payment Folder 所有成员（Permission ≥ View）可在 Skill Middleware / playbook.* 工具中看到
拒绝流程：
1. Folder Admin 拒绝 + 评论"YAML 步骤不合理，第二步和第三步应该交换"
2. 创建者收到通知（带评论）
3. 仍为 private，可修改后重新申请
AI 推断步骤的边界：
- ✅ 对话含"查 PromQL / LogQL / Grafana 指标" → query
- ✅ 对话含"如果...就..." → branch
- ✅ 对话含"用工具 X 调用 Y" → mcp_call
- ✅ 对话含"总结 / 汇总 / 报告" → template
- ❌ 对话含"先做 A 再做 B 但要看 C" → 模糊，要求用户手填确认
- ❌ 对话含"循环 / 遍历" → 模糊，要求用户手填
7. 验收标准
沉淀
- [ ]  用户对话中说"沉淀为 playbook" → AI 生成 YAML 草稿 → 写入 private
- [ ]  用户对话中说"保存为 skill" → AI 生成 Markdown 草稿 → 写入 private
- [ ]  草稿在 Web UI 可编辑后再确认
- [ ]  确认后 usage_count = 0，visibility = private，folder_uid = ""
跑
- [ ]  用户可跑自己的 private object
- [ ]  shared object：Folder Permission ≥ View 的成员可跑
- [ ]  每次跑后 usage_count++
申请共享
- [ ]  用户在 Web UI 可申请 private → shared
- [ ]  列出用户有 Edit 权限的 Grafana Folder 供选择
- [ ]  创建 ApprovalRequest（pending, target_folder_uid = 用户选的 Folder）
- [ ]  通知审批者（站内 / 邮件可选）
审批
- [ ]  审批者必须是 target_folder_uid 的 Grafana Folder Admin（Permission = 4）
- [ ]  通过 → Object.Visibility = shared, FolderUID = target_folder_uid
- [ ]  拒绝 + 评论（必填）→ Object 仍 private + 通知创建者（带评论）
- [ ]  审批后 usage_count 累计数可看（用于决策）
权限（复用 Grafana Folder Permission）
- [ ]  private：仅 owner 能看 / 跑 / 编辑
- [ ]  shared：Folder Permission ≥ View 可看 / 跑；≥ Edit 可编辑
- [ ]  AI Agent 调 Grafana Folder Permission API 校验
审计（P9 联动）
- [ ]  object_personal_created 事件：含 user_id / object_type / object_id
- [ ]  object_promote_requested：含 ApprovalRequest.ID / requested_by / target_folder_uid
- [ ]  object_promote_approved：含 reviewer_id / target_folder_uid
- [ ]  object_promote_rejected：含 reviewer_id / reject_reason
不做的验收边界
- [ ]  不做三层可见性（global 层）
- [ ]  不做 AI 自动检测"应该沉淀"
- [ ]  不做 AI 自动审批
- [ ]  不做评分 / 投票机制
8. 不做（边界）
- 三层可见性（global 层由 Grafana Folder 替代）
- 隐式沉淀（AI 自动检测）
- AI 自动审批
- 跨 Folder 复制
- 评分 / 投票
- 版本控制 / 历史 diff UI
9. 依赖
- 依赖 P1：AI Agent 提取对话历史生成草稿
- 依赖 P3：Session（提取 message history）
- 依赖 P5：Catalog（推断 service 关联）
- 依赖 P6：Playbook CRUD（被本模块晋升）
- 依赖 P8：Skill CRUD（被本模块晋升）
- 依赖 P9：审计
- 依赖 Grafana Folder API：列出可选 Folder + 校验 Folder Admin 权限
- 被 P1 依赖：AI Agent 内部检测"用户说沉淀" → 触发本模块
10. 状态记录
- 2026-07-10：草案创建（从讨论洞察衍生：Skills / Playbook 需要"先用再共享"机制）
- 2026-07-10：简化 visibility 为两层（private + shared），复用 Grafana Folder Permission