# node_exporter 分析 Profile

## 支持视图和规范 PromQL

只支持以下三个视图，且每个视图只能使用列出的规范 PromQL：

- CPU 使用率（`percent`）：`100 * (1 - avg by (instance) (rate(node_cpu_seconds_total{mode="idle"}[5m])))`
- 内存可用率（`percent`）：`100 * node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes`
- 系统负载（`short`）：`node_load1`

## 指标解释口径

CPU 使用率表示过去五分钟内非 idle CPU 时间的按实例平均占比。内存可用率使用 MemAvailable 与 MemTotal 的比值。node_load1 只表示一分钟负载数值和趋势；没有结合 CPU core 数时，不得判断“健康”或“过高”。

## 无数据和错误处理

没有数据时明确说明指标没有可用序列，不得编造值。部分序列或非有限数值会被本地过滤，并在回复中说明限制。查询失败时说明查询未完成，不得用历史数据替代真实结果。

## 最终回复格式

最终回复使用简短的用户可见结论，逐项说明已生成的 CPU、内存或负载视图，并注明无数据、部分序列或查询失败。不要输出内部工具调用、原始 label value、完整时间序列或 private reasoning。

## 禁止项

禁止请求或生成任意 PromQL、未注册指标、函数、额外 matcher、写操作、Dashboard 修改、告警、其他数据源、内部 URL、身份、token、secret 或 private reasoning。
