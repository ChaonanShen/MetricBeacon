# Architecture Decision Records

本目录记录会影响模块边界、协议、权限、安全、数据所有权或长期演进成本的架构决策。

状态约定：

- Proposed：待讨论。
- Provisional：为推进骨架而暂定，达到复审条件前不能视为最终结论。
- Accepted：已确认，后续变更需要新 ADR supersede。
- Rejected：未采用，保留原因。
- Superseded：已被新 ADR 替代。

## 当前 ADR

|ADR|状态|主题|
|-|-|-|
|[ADR-017](ADR-017-grafana-delegation-grant.md)|Provisional|assistant-mcp 使用短期 Delegation Grant 回调 Plugin Backend 的 Grafana 受控代理|
|[ADR-018](ADR-018-multi-turn-real-analysis-boundaries.md)|Accepted|多轮会话、有限事件重放、受限 node_exporter 查询与模型数据隔离|
|[ADR-019](ADR-019-bounded-node-exporter-query-parameters.md)|Accepted|三视图内的有界时间、resolution、CPU window 与极简 Agent|
|[ADR-020](ADR-020-natural-language-only-workbench-query-intent.md)|Accepted|Workbench 只提交自然语言，由 AI Core 解析有界查询意图|

新增 ADR 时应至少包含：背景、决策、备选方案、影响、开放问题、复审条件和关联文档。
