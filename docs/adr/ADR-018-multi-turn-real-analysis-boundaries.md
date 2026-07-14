# ADR-018: Multi-turn real-analysis boundaries

> Status: Accepted
> Date: 2026-07-14

## Context

The completed Mock slice persists Sessions, Tasks and events but has a
single-turn workbench and a deterministic fixture-only metrics path. The next
slice adds durable multi-turn recovery, a replaceable real Prometheus adapter
and an opt-in external model. These changes affect persistent relationships,
cross-process read contracts and what may leave the service boundary.

## Decision

- `Session` remains the conversation boundary and `Task` remains one analysis
  turn. Every Message has a required `taskId`; a Task retains
  `inputMessageId`. The User Message and Task are one-to-one, an Assistant
  Message is optional and one-to-one, and tenant/session equality is enforced
  by migration checks and database constraints.
- A tenant/session can have only one non-terminal Task. The application gives
  a useful conflict response, while a SQLite partial unique index is the
  final concurrency guard.
- Historical TaskEvent reads are bounded JSON replay with a fixed target
  sequence. SSE is reserved for durable catch-up plus follow of a
  non-terminal Task and closes after durable terminal events are drained.
- Mock and HTTP Prometheus adapters use the one logical datasource UID
  `prometheus-main`. The adapter endpoint is configuration only, never a
  browser or model input.
- PromQL is a registry of exactly CPU, memory-available percentage and load
  views. Adapter-local AST validation normalizes only those canonical
  expressions; arbitrary metrics, labels, functions and datasource access are
  rejected.
- The Eino adapter is an outbound implementation of the existing AgentRuntime
  Port. It operates serially with stable source call IDs, carries at most 12
  preceding persisted User/Assistant messages and 12,000 Unicode characters,
  and keeps the current message separate.
- Full query results remain in a per-run local accumulator. A model receives
  only bounded summaries. Identities, tokens, secrets, internal URLs, raw
  series/labels and private reasoning must not be persisted in model-facing
  data, events or logs.

## Consequences

The first migration is forward-only and must reject ambiguous old data rather
than infer message/task relationships. New read endpoints and generated types
are required before handlers and frontend state. Mock remains the default
driver and the real-agent route is explicitly opt-in; no API key is needed by
Mock or real-metrics checks.

## Alternatives considered

- Keep one unbounded SSE stream per historical Task: rejected because it does
  not terminate and complicates refresh recovery.
- Replace `inputMessageId` with only `Message.taskId`: rejected because it
  creates an unnecessary breaking refactor during the migration.
- Let the model compose arbitrary PromQL: rejected because it expands read
  scope and risks unbounded data access.
- Return full query series to the model: rejected because it violates data
  minimization and leaks labels/timestamps beyond the service boundary.

## Review conditions

Any expansion beyond the three-view registry, concurrent active Tasks or
parallel tools, a different data owner, or model-facing raw series requires a
new ADR. The Grafana delegation decision remains governed independently by
ADR-017.
