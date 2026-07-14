# ADR-019: Bounded node_exporter query parameters

> Status: Accepted
> Date: 2026-07-14
> Note: The Workbench authoring-control clause is superseded by ADR-020; the
> bounded QueryPlan and execution policy remains in force.

## Context

ADR-018 deliberately limited the first real Agent to three canonical
node_exporter views. That slice proved the model, MCP and Prometheus path, but
the Workbench always submits a 30-minute range and the Eino Adapter always
queries with a 300-second step. The CPU expression also always uses a five-minute
`rate` window. Natural-language requests such as “最近五分钟、每五秒一个点”
therefore select the right view but do not affect the executed parameters.

The product design calls for natural-language-first analysis with visible,
editable time parameters. We need that flexibility without turning the small
node_exporter Agent into an arbitrary PromQL author or widening data access.

## Decision

- The registry remains exactly three semantic views: CPU usage, memory available
  percentage and one-minute system load. No new metric, label matcher, grouping,
  datasource or arbitrary expression is introduced by this decision.
- A persisted `QueryPlan` freezes the effective query step and CPU rate window
  alongside the Task's existing absolute time range. The initial bounded policy
  is:
  - query range: 30 seconds through 6 hours;
  - explicit step: 5, 10, 15, 30, 60, 120 or 300 seconds;
  - auto step: the smallest allowed step producing at most 300 theoretical
    evaluation points;
  - explicit requests producing more than 1,000 theoretical points are rejected;
  - CPU rate window: locally selected from 30, 60 or 300 seconds according to
    the query duration.
- A deterministic, application-local resolver recognizes only bounded relative
  time and resolution phrases. It does not call a model. Absolute caller ranges
  take precedence; otherwise an explicit current-message phrase overrides the
  caller's relative default. Unrecognized input keeps the structured caller
  value, then falls back to 30 minutes and auto resolution.
- The model chooses only one or more view keys. It never supplies PromQL,
  timestamps, step, window, datasource or label values.
- `assistant-mcp` owns rendering the registered view into the effective canonical
  expression and returns that expression in validation output. CPU may render
  only the three registered rate-window variants; memory and load remain fixed.
- Full series remain local. The safe model summary may add effective query/data
  ranges, step, calculation window, first/latest/delta and sample counts, but no
  raw points, timestamps per point, real labels or internal endpoints.
- The durable Assistant answer is formatted locally from the effective plan and
  accumulated query results. Model prose is not persisted as the factual query
  summary, and the permissive plain-text final fallback is removed.
- The Workbench exposes time-range and resolution controls and displays the
  effective persisted plan. It still reaches data only through the Plugin
  Resource API.

## Consequences

CreateTask, Task, Chart, Execution and MCP Tool contracts change before their
implementations. AI Core requires a forward-only SQLite migration; historical
Tasks are backfilled with the previous behavior (`stepSeconds=300`,
`cpuRateWindowSeconds=300`). Chart queries persist their step, and executions
record an optional actual data range in addition to the requested query range.

Mock and real Prometheus adapters must share consistency coverage for the same
registered view/window policy. Mock behavior may deterministically resample
fixture values inside the Adapter, but Domain, Application and frontend state
must not gain a Mock branch.

## Alternatives considered

- Let the model generate arbitrary PromQL and query parameters: rejected because
  it widens the read boundary and makes limits dependent on prompt compliance.
- Parse all parameters only in the browser: rejected because API callers and
  replayed Tasks would not share one authoritative policy.
- Keep the five-minute CPU window for every range: rejected because short-range
  charts would remain unnecessarily smoothed and misleading.
- Send full time series back to the model for a better explanation: rejected;
  bounded local statistics are sufficient and preserve ADR-018's minimization.

## Review conditions

Adding another view, metric, datasource, label filter, grouping choice, arbitrary
PromQL, a CPU window outside the registered set, more than 6 hours, or model-facing
raw points requires a new ADR. Changing only the bounded step list or auto-step
target requires contract, limit and consistency-test updates but not necessarily
a new ADR if the security envelope does not expand.

## Related documents

- [`ADR-018-multi-turn-real-analysis-boundaries.md`](ADR-018-multi-turn-real-analysis-boundaries.md)
- [`../implementation/bounded_node_exporter_query_parameters_execution_plan.md`](../implementation/bounded_node_exporter_query_parameters_execution_plan.md)
- [`../implementation/code_skeleton_design.md`](../implementation/code_skeleton_design.md)
