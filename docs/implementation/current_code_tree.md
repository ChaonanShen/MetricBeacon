# 当前代码树（分享版）

> 更新日期：2026-07-16。省略 `node_modules/`、`dist/`、`build/`、测试输出和 Go 模块缓存；
> 该树用于说明当前有界 node_exporter 查询闭环的代码归属，而非逐文件清单。

```text
mini-torchbearing/
├── apps/
│   └── grafana-plugin/                    # Grafana App Plugin
│       ├── frontend/                      # React/Grafana UI 前端
│       │   ├── src/
│       │   │   ├── module.tsx             # Plugin 前端入口
│       │   │   ├── api/
│       │   │   │   ├── resource.ts        # Plugin Resource API 客户端
│       │   │   │   ├── resource-error.ts  # Resource 404 恢复判定与安全错误展示
│       │   │   │   └── generated/         # 由 OpenAPI 生成的前端类型
│       │   │   └── workbench/             # 分析工作台
│       │   │       ├── Workbench.tsx      # 常驻 controller：Session list/切换、任务创建、恢复与 SSE 编排
│       │   │       ├── WorkbenchShell.tsx # 产品页面壳与展示 slot，不复制请求或服务端状态
│       │   │       ├── WorkbenchHeader.tsx # 页内产品标题与当前会话状态
│       │   │       ├── ContextPane.tsx    # 真实 Session/Task/QueryPlan 只读上下文
│       │   │       ├── workbench-view.ts  # Context、状态与示例问题的纯展示派生
│       │   │       ├── workbench-styles.ts # Grafana theme 到 scoped 产品 CSS variables 的映射
│       │   │       ├── ChatPane.tsx       # 消息、assistant draft、状态、示例与单 form composer
│       │   │       ├── SessionMenu.tsx    # owner Session 有界分页菜单、新建与选择
│       │   │       ├── ChartCanvas.tsx    # Task 分组、最多两列画布与滚动控制
│       │   │       ├── chart-groups.ts    # oldest-first 分组与 Task 滚动纯逻辑
│       │   │       ├── ChartCard.tsx      # 时序卡片、图表状态与只读 PromQL
│       │   │       ├── chart-view.ts      # Execution 到卡片状态的纯展示映射
│       │   │       ├── mapper.ts          # Chart wire 数据 -> Grafana DataFrame
│       │   │       ├── reducer.ts         # TaskEvent 前端状态归约
│       │   │       ├── session-reducer.ts # Session 历史、Task runtimes 与活动流状态
│       │   │       ├── sse.ts             # SSE 订阅、重连与 sequence 处理
│       │   │       ├── route.ts           # Session/Task URL 恢复与旧 ID 清理
│       │   │       ├── session-list.ts    # Session page 去重与首消息标题纯逻辑
│       │   │       ├── time-range.ts      # 图表时间范围转换
│       │   │       ├── query-input.ts     # 自然语言与固定 datasource 请求映射
│       │   │       └── *.test.ts          # 前端单元测试
│       │   ├── backend/                   # Grafana Plugin SDK 后端（Go）
│       │   │   ├── cmd/plugin/            # 后端进程入口
│       │   │   └── internal/
│       │   │       ├── handlers/          # Session/Task/Incident/Approval Resource API、有限重放与 SSE 代理
│       │   │       ├── aicore/            # AI Core HTTP Client
│       │   │       ├── context/           # Grafana org/user/role 到 query/read/Admin-approve 权限
│       │   │       ├── config/            # 插件实例配置
│       │   │       └── bootstrap/         # 依赖组装
│       │   ├── provisioning/              # Grafana App 配置
│       │   ├── plugin.json                # Plugin 元数据
│       │   └── Dockerfile                 # Grafana Plugin 镜像
│
├── services/
│   ├── ai-core/                           # 核心业务服务（Go）
│   │   ├── cmd/server/                    # HTTP 服务入口
│   │   ├── internal/
│   │   │   ├── domain/                    # 纯领域：Session/Task、Incident、Approval/Execution/Audit、QueryPlan、Chart
│   │   │   ├── application/
│   │   │   │   ├── commands/              # Session/Task 命令、Planner 合并与 QueryPlan 冻结
│   │   │   │   ├── incidents/             # 告警生命周期幂等建组织 Incident 与异步调度
│   │   │   │   ├── approvals/             # Admin、幂等、双版本 CAS 的 Approval 决策与审计
│   │   │   │   ├── workflows/             # 指标分析、Incident 诊断/prepare 与 write-once Execute/Reconcile/三重 Verify
│   │   │   │   └── dto/                   # 应用层数据传递对象
│   │   │   ├── ports/                     # 存储、Agent、MCP、Incident 诊断/修复分离 Toolset、时钟等抽象接口
│   │   │   ├── adapters/
│   │   │   │   ├── inbound/http/          # AI Core API、SSE 与 source/HMAC/time-window Grafana ingress
│   │   │   │   └── outbound/              # SQLite、MCP（指标 + Incident 诊断/受控修复）、Mock/Eino Agent、时钟/ID
│   │   │   │       ├── agent/eino/         # JSON mode、结构化历史和一次重试的 IntentPlanner Adapter
│   │   │   │       ├── agent/mock/         # 确定性意图解析与 persisted-view 执行器
│   │   │   │       └── agent/localresult/  # Mock/Eino 共用的可信事实 formatter
│   │   │   └── bootstrap/                 # 分析 + Incident 诊断/Approval/执行恢复的原子依赖组装
│   │   ├── migrations/sqlite/             # SQLite 迁移（0007 Incident union；0008 Intent/Approval/Execution/Audit）
│   │   └── Dockerfile
│   │
│   ├── assistant-mcp/                     # MCP 工具服务（Go）
│       ├── cmd/server/                    # Streamable HTTP MCP 入口
│       ├── internal/
│       │   ├── namespaces/grafana/        # grafana.* Tool 注册与处理
│       │   ├── namespaces/incident/       # opt-in 资产、Playbook、订单诊断与审批证据约束的类型化处置工具
│       │   ├── ports/prometheus/          # Prometheus 查询抽象
│       │   ├── ports/orderdemo/           # 隔离的只读诊断 Port 与唯一 0 -> 2 RemediationPort
│       │   ├── ports/incidentmetrics/     # 固定恢复指标视图 Port
│       │   ├── ports/executionaudit/      # assistant-mcp 边界执行审计 Port
│       │   ├── adapters/prometheus/
│       │   │   ├── mock/                  # 按请求范围/step 重采样的 fixture Adapter
│       │   │   ├── registry/              # view/window -> PromQL 与 AST policy
│       │   │   └── http/                  # 动态 step、受注册表约束的真实 HTTP Adapter
│       │   ├── adapters/orderdemo/
│       │   │   ├── mock/                  # 四类诊断证据一致的确定性 Adapter
│       │   │   └── http/                  # 生成客户端、独立读写 token、CAS/幂等与响应边界
│       │   ├── adapters/incidentmetrics/  # 固定四条 PromQL 的 HTTP 与同形 Mock
│       │   ├── adapters/executionaudit/   # append-only、同步落盘的 JSONL 边界审计
│       │   ├── adapters/assets/filesystem/ # Schema、引用、能力和 SHA-256 固定的只读资产
│       │   ├── playbook/                   # HMAC checkpoint 与确定性 prepare policy
│       │   ├── ports/assets/               # 运行资产与精确 Alert Mapping 抽象
│       │   ├── runtime/                   # MCP 运行时与错误处理
│       │   └── bootstrap/                 # 服务依赖组装
│       └── Dockerfile
│   └── order-demo/                        # 可控订单业务服务（Go）
│       ├── cmd/                           # order-service、fault-controller、loadgen
│       ├── internal/
│           ├── domain/                    # 订单状态机与版本化 worker 配置策略
│           ├── application/               # bounded queue、动态 worker、故障、CAS 与业务 probe
│           ├── ports/metrics/             # 业务指标记录 Port
│           └── adapters/
│               ├── inbound/http/          # 分离路由、token、Unix Fault 与生成类型
│               └── outbound/prometheus/   # 低基数业务 counter/histogram/gauge
│       └── Dockerfile                     # 同镜像中的三个受限进程入口
│
├── contracts/                             # 跨服务协议的单一来源
│   ├── openapi/                           # Plugin/AI Core 与订单 Business/Operational/Fault API
│   ├── tools/grafana/                     # MCP Tool OpenAPI/JSON Schema
│   ├── schemas/                           # Session/Task tagged union、IncidentPlan、Approval、Alert、Chart 等 Schema
│   │   ├── incident/                      # 有界 Grafana Webhook 与接收响应
│   │   ├── approval/                      # 审批公开快照
│   │   └── api/                           # 页面、有限重放与审批 decision 请求
│   ├── events/                            # TaskEvent 定义
│   ├── errors/                            # 错误码定义
│   └── examples/                          # 合约示例
│
├── packages/                              # 可复用 Go 包及契约生成物
│   ├── approval-evidence-go/              # 60 秒、全 scope HMAC ApprovalEvidence codec
│   ├── generated-clients/                 # AI Core 与订单 API Client/类型
│   ├── generated-contracts/               # MCP Tool 类型（Go / TypeScript）
│   ├── request-context-go/                # 租户、用户、角色、请求上下文
│   └── testkit-go/                        # 确定性时钟与 ID 测试工具
│
├── data/
│   ├── agent-knowledge/node_exporter.md   # 受校验的只读 node_exporter Agent Profile
│   └── mock-scenarios/
│       └── node_exporter_overview/        # 固定 Mock 场景
│           ├── search_metrics.json
│           ├── metric_labels.json
│           ├── query_cpu.json
│           ├── query_memory.json
│           ├── query_load.json
│           └── expected_task_events.json
│
├── tests/
│   ├── diagnostics/                       # Prometheus/DeepSeek 探针、指标及 durable Task 语义分析与 fake 测试
│   ├── e2e/mock/                          # 六种有界输入的 Mock API 与 Playwright E2E
│   │   ├── api-e2e.mjs
│   │   ├── api-e2e.sh
│   │   ├── browser-e2e.spec.ts            # 真实多轮、Session、replay 与图表纵向链
│   │   ├── browser-shell.spec.ts          # 主题、响应式、a11y 与请求/存储边界
│   │   └── browser-errors.spec.ts         # Task 依赖错误与同幂等键重试
│   └── e2e/real-agent/api-smoke.mjs        # view-only Eino、8 轮重复规划、本地回复、replay 与泄漏检查
│
├── scripts/
│   ├── mtb                               # 无参数一键开发与统一 lifecycle/verify 入口
│   ├── mtb.mjs                           # 配置、依赖、Compose namespace/动态端口和运行编排
│   └── run-*.sh                          # 保留原 Make/脚本入口的薄兼容层
├── docs/                                  # 架构、设计、ADR、开发计划与当前说明
│   └── implementation/real_backend_test_matrix.md # code agent 分层测试与结果判读手册
├── deploy/
│   ├── prometheus/                        # node-exporter 与 incident/order-demo 的 5 秒 scrape 配置
│   └── grafana/provisioning/              # incident Prometheus datasource、Alert rule 与 HMAC Webhook
├── compose.mock-e2e.yaml                  # assistant-mcp + AI Core + Grafana 的 Mock 环境
├── compose.real-metrics-e2e.yaml          # Prometheus/node-exporter real-metrics Compose overlay
├── compose.real-agent-e2e.yaml            # opt-in DeepSeek Eino Agent Compose overlay
├── compose.incident-e2e.yaml              # order/loadgen/fault、隔离网络与 Grafana Alert overlay
├── Makefile                               # 测试、校验与 E2E 统一入口
├── go.work                                # Go workspace
└── README.md                              # 项目入口说明
```

## 一句话分层

```text
浏览器 UI (apps/grafana-plugin/frontend)
  -> Grafana Plugin Backend (apps/grafana-plugin/backend)
  -> AI Core QueryPlan + view-only Agent (services/ai-core)
  -> MCP Tool Service + PromQL Registry (services/assistant-mcp)
  -> Mock Fixture 或 Prometheus/node_exporter
```

其中 `contracts/` 是各进程间 API、事件与 Tool Schema 的单一来源；`packages/generated-*` 是由
这些契约生成的类型和客户端，不应手工修改。
