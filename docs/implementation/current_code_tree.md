# 当前代码树（分享版）

> 更新日期：2026-07-14。省略 `node_modules/`、`dist/`、`build/`、测试输出和 Go 模块缓存；
> 该树用于说明当前 Mock 闭环的代码归属，而非逐文件清单。

```text
mini-torchbearing/
├── apps/
│   └── grafana-plugin/                    # Grafana App Plugin
│       ├── frontend/                      # React/Grafana UI 前端
│       │   ├── src/
│       │   │   ├── module.tsx             # Plugin 前端入口
│       │   │   ├── api/
│       │   │   │   ├── resource.ts        # Plugin Resource API 客户端
│       │   │   │   └── generated/         # 由 OpenAPI 生成的前端类型
│       │   │   └── workbench/             # 分析工作台
│       │   │       ├── Workbench.tsx      # 页面编排、任务创建、SSE 订阅
│       │   │       ├── ChartCard.tsx      # 自适应三图卡片与图表状态
│       │   │       ├── mapper.ts          # Chart wire 数据 -> Grafana DataFrame
│       │   │       ├── reducer.ts         # TaskEvent 前端状态归约
│       │   │       ├── sse.ts             # SSE 订阅、重连与 sequence 处理
│       │   │       ├── route.ts           # Session/Task URL 恢复
│       │   │       ├── time-range.ts      # 图表时间范围转换
│       │   │       └── *.test.ts          # 前端单元测试
│       │   ├── backend/                   # Grafana Plugin SDK 后端（Go）
│       │   │   ├── cmd/plugin/            # 后端进程入口
│       │   │   └── internal/
│       │   │       ├── handlers/          # Resource API、历史/有限重放与 SSE 代理
│       │   │       ├── aicore/            # AI Core HTTP Client
│       │   │       ├── context/           # Grafana 身份上下文
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
│   │   │   ├── domain/                    # 纯领域：Session、Task、Chart、Execution
│   │   │   ├── application/
│   │   │   │   ├── commands/              # Session/Task 命令服务
│   │   │   │   ├── workflows/             # 分析工作流与事件持久化
│   │   │   │   └── dto/                   # 应用层数据传递对象
│   │   │   ├── ports/                     # 存储、Agent、MCP、时钟等抽象接口
│   │   │   ├── adapters/
│   │   │   │   ├── inbound/http/          # AI Core HTTP API 与 SSE
│   │   │   │   └── outbound/              # SQLite、MCP、Mock Agent、时钟/ID
│   │   │   └── bootstrap/                 # 服务依赖组装
│   │   ├── migrations/sqlite/             # SQLite 数据库迁移
│   │   └── Dockerfile
│   │
│   └── assistant-mcp/                     # MCP 工具服务（Go）
│       ├── cmd/server/                    # Streamable HTTP MCP 入口
│       ├── internal/
│       │   ├── namespaces/grafana/        # grafana.* Tool 注册与处理
│       │   ├── ports/prometheus/          # Prometheus 查询抽象
│       │   ├── adapters/prometheus/
│       │   │   ├── mock/                  # node_exporter fixture Adapter
│       │   │   └── http/                  # 真实 Prometheus Adapter 预留
│       │   ├── runtime/                   # MCP 运行时与错误处理
│       │   └── bootstrap/                 # 服务依赖组装
│       └── Dockerfile
│
├── contracts/                             # 跨服务协议的单一来源
│   ├── openapi/                           # Plugin Resource API、AI Core API
│   ├── tools/grafana/                     # MCP Tool OpenAPI/JSON Schema
│   ├── schemas/                           # Session、Task、Chart、Event 等 Schema
│   │   └── api/*-page.schema.json          # Message/Task keyset 页面及有限事件重放响应
│   ├── events/                            # TaskEvent 定义
│   ├── errors/                            # 错误码定义
│   └── examples/                          # 合约示例
│
├── packages/                              # 可复用 Go 包及契约生成物
│   ├── generated-clients/                 # AI Core API Client（Go / TypeScript）
│   ├── generated-contracts/               # MCP Tool 类型（Go / TypeScript）
│   ├── request-context-go/                # 租户、用户、角色、请求上下文
│   └── testkit-go/                        # 确定性时钟与 ID 测试工具
│
├── data/
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
│   └── e2e/mock/                          # API 与 Playwright 浏览器 E2E
│       ├── api-e2e.mjs
│       ├── api-e2e.sh
│       └── browser-e2e.spec.ts
│
├── scripts/                               # 生成、契约、边界、Mock E2E 等门禁脚本
├── docs/                                  # 架构、设计、ADR、开发计划与当前说明
├── deploy/                                # 部署相关配置
├── compose.mock-e2e.yaml                  # assistant-mcp + AI Core + Grafana 的 Mock 环境
├── Makefile                               # 测试、校验与 E2E 统一入口
├── go.work                                # Go workspace
└── README.md                              # 项目入口说明
```

## 一句话分层

```text
浏览器 UI (apps/grafana-plugin/frontend)
  -> Grafana Plugin Backend (apps/grafana-plugin/backend)
  -> AI Core (services/ai-core)
  -> MCP Tool Service (services/assistant-mcp)
  -> Mock Prometheus Fixture (data/mock-scenarios)
```

其中 `contracts/` 是各进程间 API、事件与 Tool Schema 的单一来源；`packages/generated-*` 是由
这些契约生成的类型和客户端，不应手工修改。
