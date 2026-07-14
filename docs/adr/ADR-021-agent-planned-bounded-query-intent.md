# ADR-021: Agent-planned bounded query intent

> Status: Accepted
> Date: 2026-07-15
> Supersedes: ADR-020

## Context

ADR-020 removed structured query controls from the Workbench but retained a
deterministic application resolver for range and cadence while the model chose
only views. That split prevents the Agent from interpreting conversational
follow-ups consistently and leaves two independent intent mechanisms around a
single durable query plan.

The existing local safety envelope remains appropriate: only three registered
views are executable, ranges are limited to 30 seconds through 6 hours, steps
come from a fixed allowlist, explicit requests stay below 1,000 points, and CPU
rate windows are selected locally.

## Decision

- The Workbench submits only the natural-language message and fixed logical
  datasource. Optional structured request hints remain API-compatible for
  non-Workbench callers.
- Before writing a new Message or Task, AI Core synchronously calls an
  `IntentPlanner` Port with the current message and bounded conversation
  history. Mock and Eino implementations are outbound Adapters.
- The planner returns strict structured intent containing status, registered
  views, and optional relative range and step. It cannot provide PromQL,
  datasource, absolute timestamps, CPU windows, labels, reasoning, or factual
  metric values.
- Application code merges planner intent with optional API hints and local
  defaults, enforces the existing range/step/point limits, freezes absolute
  time with the injected clock, derives the CPU window locally, and persists
  the selected views in the QueryPlan.
- The asynchronous workflow deterministically executes only the persisted
  views. It does not invoke a model to select views or query parameters again.
- Planner failure or invalid output is retryable and writes no Message, Task,
  or idempotency record. An explicit unsupported result creates a durable Task
  with no views and a local explanation without accessing Prometheus.
- Completed idempotent retries return the existing Task before planning.
  Concurrent first attempts may plan more than once, but the existing
  transaction boundary persists only one Task.

## Consequences

QueryPlan, Task snapshots, events, OpenAPI, JSON Schema, generated clients and
SQLite storage gain durable views. Historical rows are migrated by inferring
known views from persisted canonical Chart PromQL when possible; absence is
kept as an empty list rather than invented.

The Eino Adapter becomes a tool-free synchronous planner. Full time series,
private model reasoning, identities, tokens, internal URLs and secrets remain
outside model input and persistence. `assistant-mcp` remains the only compiler
of registered views to canonical PromQL.

## Alternatives considered

- Keep local range parsing and use the model only for views: rejected because
  conversational intent remains split across two planners.
- Plan again in the asynchronous workflow: rejected because replay would not
  have one frozen authoritative intent.
- Let the model emit PromQL or CPU windows: rejected because it expands the
  data-access and query-safety boundary.
- Fail unsupported requests without a Task: rejected because a durable local
  explanation is a valid conversational result, unlike planner failure.

## Open questions

None for this bounded slice. Adding views, arbitrary PromQL, label filters or
runtime scrape-derived windows requires a separate decision.

## Review conditions

Review this decision if planning becomes asynchronous, if planner output gains
data-access fields, or if the registered view/range/step envelope changes.

## Related documents

- [`ADR-018-multi-turn-real-analysis-boundaries.md`](ADR-018-multi-turn-real-analysis-boundaries.md)
- [`ADR-019-bounded-node-exporter-query-parameters.md`](ADR-019-bounded-node-exporter-query-parameters.md)
- [`ADR-020-natural-language-only-workbench-query-intent.md`](ADR-020-natural-language-only-workbench-query-intent.md)
- [`../implementation/natural_language_query_input_execution_plan.md`](../implementation/natural_language_query_input_execution_plan.md)
