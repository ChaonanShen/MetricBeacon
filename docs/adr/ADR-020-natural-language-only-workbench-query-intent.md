# ADR-020: Natural-language-only Workbench query intent

> Status: Superseded by ADR-021
> Date: 2026-07-15
> Supersedes: the Workbench authoring-control clause of ADR-019

## Context

ADR-019 added bounded query ranges, steps and CPU rate windows, but also made
the Workbench submit a selected default range and resolution. That UI makes the
structured fallback look like a required part of the analysis request and
weakens the intended natural-language-first interaction. It also hides a test
gap: the phrase `每隔 5s` was not recognized by the application resolver, while
a one-minute query happened to choose the same five-second step in auto mode.

The bounded execution and persistence policy remains useful. The change is in
how the Grafana Workbench expresses user intent, not in the Prometheus access
envelope.

## Decision

- The Workbench exposes one analysis-message input. It does not expose or
  submit a default time-range or sampling-resolution control.
- Its CreateTask request contains the user message and the fixed logical
  datasource only. The existing optional `analysisContext.timeRange` and
  `analysisContext.resolution` contract remains available to non-Workbench API
  callers and compatibility tests.
- AI Core remains the authoritative natural-language intent boundary. Before
  creating the Task it recognizes supported range and cadence phrases,
  validates them against the existing bounds, freezes the absolute range and
  persists the effective QueryPlan. Cadence phrases include `每 5s`, `每个 5s`
  and `每隔 5s` forms.
- When the current message omits a supported phrase, AI Core applies its local
  30-minute/auto policy. That is a server policy, not a browser-selected value.
- The Eino model continues to select only registered semantic views. It does
  not supply timestamps, step, CPU rate window or arbitrary PromQL.
  `assistant-mcp` continues to compile the selected view and effective bounded
  parameters into canonical PromQL.
- Read-only effective query metadata may remain visible after submission for
  auditability. It is not an authoring control.
- CPU rate window semantics do not change. The window is the lookback used by
  each `rate()` evaluation, while step is the distance between evaluations.

## Consequences

The frontend request builder and browser E2E tests become simpler and prove
that a natural-language-only request reaches AI Core without structured range
or resolution defaults. The resolver requires regression coverage using the
exact `每隔 5s` wording.

No OpenAPI, JSON Schema, generated client, database migration or MCP Tool Schema
changes are required: ADR-019 already made both request hints optional and the
effective Task QueryPlan remains required and durable.

## Alternatives considered

- Keep the controls but visually de-emphasize them: rejected because the
  Workbench would still author and submit structured defaults.
- Move range and step extraction into the external model: rejected for this
  bounded slice because Task creation must freeze a replayable plan before the
  Agent workflow runs, and model availability should not be required to create
  a deterministic Mock task.
- Allow the model to submit arbitrary PromQL: rejected because it expands the
  data-access boundary governed by ADR-018 and ADR-019.
- Set the CPU `rate()` range equal to the five-second query step: rejected
  because step and rate lookback have different semantics and a single scrape
  interval is not a reliable rate window.

## Review conditions

Moving query-intent ownership into a model, allowing arbitrary PromQL, changing
the three-view registry or deriving rate windows from runtime datasource scrape
metadata requires a new ADR. Adding bounded phrase variants to the local
resolver does not.

## Related documents

- [`ADR-018-multi-turn-real-analysis-boundaries.md`](ADR-018-multi-turn-real-analysis-boundaries.md)
- [`ADR-019-bounded-node-exporter-query-parameters.md`](ADR-019-bounded-node-exporter-query-parameters.md)
- [`../implementation/natural_language_query_input_execution_plan.md`](../implementation/natural_language_query_input_execution_plan.md)
