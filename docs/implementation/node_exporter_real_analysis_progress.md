# Node Exporter Real Analysis Progress

lastUpdated: 2026-07-14
currentGate: P1.2
status: active
headCommit: pending
worktreeSummary: G0 and P1.1 are complete; task-linked Message persistence, source-call correlation and database concurrency constraints are now in place. Session-history and finite-replay APIs are next.

## Gate Status

- [x] G0 Design activation and decision record
- [ ] P1 Multi-turn contracts and persistence
  - [x] P1.1 Task-linked Message persistence and source-call correlation
  - [ ] P1.2 Session history, bounded replay and Agent context
- [ ] P2 Persistent Session workbench
- [ ] P3 Real Prometheus and node_exporter
- [ ] P4 Static Profile and constrained Eino runtime
- [ ] P5 End-to-end closeout

## Locked G0 Decisions

- Mock and Real adapters share the logical datasource UID `prometheus-main`.
- Message/task association, one-active-task enforcement, source-call correlation and migration/backfill rules are defined by ADR-018 and execution-plan section 2.2.
- History uses bounded JSON replay. Only a non-terminal Task keeps an SSE follow stream.
- Agent context is the previous 12 persisted User/Assistant messages, capped at 12,000 Unicode characters; the current message is separate and capped at 4,000 characters.
- The only query semantics are the CPU, memory and load registry entries in the execution plan. Tools run serially with stable source call IDs.
- External models receive only the Profile, approved message context and bounded summaries; full series, identities, URLs, secrets and private reasoning remain local.

## Verification Evidence

|command|result|timestamp|notes|
|-|-|-|-|
|`make validate-contracts`|passed|2026-07-14|Three OpenAPI documents, 21 JSON Schemas and the Mock fixture validate unchanged.|
|`make test-ai-core-domain test-sqlite test-ai-mcp`|passed|2026-07-14|Domain/application, SQLite migration/backfill/concurrency, HTTP/MCP and workflow suites pass after P1.1.|
|`make validate-contracts generated-client-diff`|passed|2026-07-14|Updated Message and ToolCall contract types validate and generated artifacts are reproducible.|

## Next Slice

P1.2 adds Session Message/Task keyset paging, bounded JSON TaskEvent replay and the limited persisted-message context supplied to AgentRuntime.
