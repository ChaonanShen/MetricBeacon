# Node Exporter Real Analysis Progress

lastUpdated: 2026-07-14
currentGate: P1.2b
status: active
headCommit: pending
worktreeSummary: G0, P1.1 and P1.2a are complete. AI Core now exposes tenant-scoped keyset history and finite durable replay; Plugin proxy, terminal SSE behavior and Agent history assembly remain.

## Gate Status

- [x] G0 Design activation and decision record
- [ ] P1 Multi-turn contracts and persistence
  - [x] P1.1 Task-linked Message persistence and source-call correlation
  - [x] P1.2a AI Core history pages and finite replay
  - [ ] P1.2b Agent context and terminal stream behavior
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
|`make test-ai-core-domain test-sqlite test-ai-mcp test-plugin-backend test-frontend validate-contracts generated-client-diff`|passed|2026-07-14|AI Core history/replay, storage, existing Plugin and frontend suites pass; all 24 JSON Schemas validate and generated artifacts are reproducible.|

## Next Slice

P1.2b supplies the bounded persisted-message context to AgentRuntime and makes terminal SSE close after its durable events are drained. The Plugin proxy work follows in its own focused commit.
