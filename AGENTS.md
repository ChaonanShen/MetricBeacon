# AGENTS.md

## Purpose and precedence

This is the repository-wide, tool-agnostic instruction file for coding agents.
It is the single source of truth for implementation rules. `CLAUDE.md` is a
small Claude-specific entry point that refers here; do not duplicate these
rules there.

Read this file before changing code. When a task concerns documentation, also
read [`docs/CLAUDE.md`](docs/CLAUDE.md) before deciding scope or editing a
document. A nested `AGENTS.md` takes precedence within its directory if one is
added later.

## Current delivery state

The basic deterministic Mock slice is complete. It currently provides:

```text
Grafana App Plugin -> Plugin Backend -> AI Core -> assistant-mcp
-> Mock Prometheus -> fixed node_exporter CPU / memory / load charts
```

The next agreed direction is to establish a persistent multi-turn workbench,
then replace adapters incrementally with real Prometheus/node_exporter and a
minimal Eino-backed Agent. Skill, Playbook, Dashboard write, and alert
integration are not part of that next slice unless a later execution plan
explicitly adds them. The document statuses and reading routes live in
[`docs/CLAUDE.md`](docs/CLAUDE.md).

## Non-negotiable architecture rules

- The entry point is a Grafana App Plugin. The browser calls only the Plugin
  Resource API; it never calls AI Core, MCP, Prometheus, or Grafana APIs
  directly.
- Plugin Backend is a thin Grafana identity, authorization, controlled-proxy,
  error-mapping, and SSE-proxy boundary. It does not own Session/Task data or
  implement Agent orchestration.
- AI Core owns Sessions, Messages, Tasks, TaskEvents, Charts, and its own
  database. assistant-mcp owns its own future knowledge/skill/playbook data.
  Services must not share-write SQLite files or query each other's database.
- Domain and application code depend on Ports, never on Grafana SDKs,
  Prometheus clients, Eino, MCP, model SDKs, or database drivers. Only
  Adapters may import those implementations.
- Mocks replace Adapters only. Do not add `mockMode` branches to Domain,
  Application, handlers, or frontend business state.
- Define or update cross-process OpenAPI, JSON Schema, MCP Tool Schema, SSE
  schema, and error codes before the corresponding implementation. Generate
  clients/types from `contracts/`; do not hand-maintain duplicate wire DTOs.
- `ChartDraft` and Grafana-specific `PanelDraft` are separate concepts. Any
  Grafana write must remain Prepare -> Intent/Diff -> Approval -> Execute ->
  Audit with version checks.
- Persist Task state and TaskEvents before SSE notification. Events carry
  `taskId`, `sessionId`, and monotonically increasing `sequence` and must be
  replayable.
- Carry tenant/org/user context through every data and tool access. Do not
  persist model private reasoning or leak secrets, Grafana tokens, identities,
  internal URLs, or complete raw time series to an external model.

## Working rules

- Verify the actual repository and relevant ADRs before relying on a design
  document. Use the documentation routing in `docs/CLAUDE.md` to resolve
  conflicts.
- Treat evolution records as part of the implementation, not follow-up work.
  Every completed code slice must update the affected contract, implementation
  plan/progress record, and current-code snapshot in the same commit when its
  behavior, module ownership, runnable topology, verification evidence, or
  code tree changes. Record an ADR in the same slice when the change affects a
  decision boundary. A code change is not complete while its authoritative
  documentation describes a different system.
- Keep implementation scope to the active execution slice. If a choice would
  alter product scope, module ownership, permissions, or irreversible storage
  structure, stop and ask for direction or record an ADR as appropriate.
- Make small, focused commits. After each independently verifiable slice, run
  proportionate checks and create a commit containing only that slice. The
  user has authorized these commits; do not push, create a PR, amend, or
  rewrite history unless explicitly asked.
- Preserve unrelated worktree changes. Never reset, checkout, or otherwise
  discard user changes without explicit authorization.
- When a cross-module field changes, update its contract, generated client,
  mock fixture, and contract tests together. Each Port needs deterministic
  Mock and Real Adapter consistency coverage as its Real Adapter is added.
