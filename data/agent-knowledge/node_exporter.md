# node_exporter 分析 Profile

## 支持视图

只支持以下三个视图。调用 `query_prometheus` 时只提交 `view`；时间范围、step、CPU rate window、数据源和规范 PromQL 均由本地应用注入，模型不得生成或修改它们：

- CPU 使用率（`cpu`，`percent`，来自 `node_cpu_seconds_total`）
- 内存可用率（`memory`，`percent`，来自 `node_memory_MemAvailable_bytes` / `node_memory_MemTotal_bytes`）
- 系统负载（`load`，`short`，来自 `node_load1`）

## 指标解释口径

CPU 使用率表示本地 QueryPlan 所选 30 秒、1 分钟或 5 分钟 rate window 内非 idle CPU 时间的按实例平均占比。内存可用率使用 MemAvailable 与 MemTotal 的比值。node_load1 只表示一分钟负载数值和趋势；没有结合 CPU core 数时，不得判断“健康”或“过高”。

## 无数据和错误处理

没有数据时明确说明指标没有可用序列，不得编造值。部分序列或非有限数值会被本地过滤，并在回复中说明限制。查询失败时说明查询未完成，不得用历史数据替代真实结果。

## 最终回复格式

模型最终只返回严格 JSON 状态和已成功查询的 view keys。用户可见事实回复由本地 formatter 根据有效 QueryPlan 和实际查询统计生成；模型不得自行陈述数值或查询参数。不要输出内部工具调用、原始 label value、完整时间序列或 private reasoning。

## 禁止项

禁止请求或生成任意 PromQL、未注册指标、函数、额外 matcher、写操作、Dashboard 修改、告警、其他数据源、内部 URL、身份、token、secret 或 private reasoning。
