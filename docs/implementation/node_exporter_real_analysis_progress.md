# Node Exporter Real Analysis Progress

lastUpdated: 2026-07-14
currentGate: P3.3
status: active
headCommit: pending
worktreeSummary: G0 and P1 are complete; P2 Session workbench and multi-turn E2E coverage are implemented. P3 has adopted the shared datasource UID, canonical-expression contract, AST registry and an opt-in real Prometheus HTTP Adapter. Container E2E remains unverified in this workspace because the user's existing local stack owns ports 3000, 8080 and 8081.

## Gate Status

- [x] G0 Design activation and decision record
- [x] P1 Multi-turn contracts and persistence
  - [x] P1.1 Task-linked Message persistence and source-call correlation
  - [x] P1.2a AI Core history pages and finite replay
  - [x] P1.2b Agent context and terminal stream behavior
  - [x] P1.3 Plugin history/replay proxy
- [ ] P2 Persistent Session workbench
  - [x] P2.1 Session reducer, page recovery, finite replay and terminal SSE handling
  - [ ] P2.2 Container E2E execution (blocked by existing local stack port bindings)
- [ ] P3 Real Prometheus and node_exporter
  - [x] P3.1 Shared `prometheus-main` datasource UID, canonical-expression contract and migration
  - [x] P3.2 Shared node_exporter registry and PromQL AST policy
  - [x] P3.3 Real Prometheus HTTP Adapter, driver config and readiness probe
  - [ ] P3.4 Real-metrics Compose topology (implemented; container E2E awaits port availability)
- [ ] P4 Static Profile and constrained Eino runtime
  - [x] P4.1 Read-only node_exporter Profile and local view metadata
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
|`make test-ai-core-domain test-sqlite`|passed|2026-07-14|Conversation context includes only the preceding twelve persisted messages in chronological order; SQLite regression suite remains green.|
|`make test-plugin-backend`|passed|2026-07-14|Plugin Resource API proxies tenant-scoped Message/Task history and finite event replay; SSE uses a request-context-controlled client without a global timeout.|
|`make test-frontend`|passed|2026-07-14|Session reducer merges history pages, preserves task runtimes, completes finite replay and closes the active SSE stream on a terminal event.|
|`node --check tests/e2e/mock/api-e2e.mjs && npx playwright test --list`|passed|2026-07-14|Expanded Mock API/Playwright E2E scripts parse and test discovery succeeds.|
|`make e2e-mock`|blocked|2026-07-14|Docker cannot bind 127.0.0.1 ports 3000, 8080 and 8081 because an existing user-managed `mini-torchbearing-*` Compose stack is running; it was not stopped.|
|`make validate-contracts generated-client-diff test-sqlite test-assistant-mcp test-ai-mcp`|passed|2026-07-14|P3.1 validates all generated UID/canonical-expression contracts; SQLite verifies Task and Chart query JSON forward migration from `mock-prometheus`.|
|`make test-assistant-mcp test-ai-mcp`|passed|2026-07-14|P3.2 accepts only the three registered normalized PromQL expressions and rejects out-of-registry ASTs before Mock fixture access.|
|`make test-assistant-mcp`|passed|2026-07-14|P3.3 `httptest` coverage verifies the opt-in HTTP driver, canonical query POST, local validation, response/series limits, non-finite filtering, timeout/status/error mapping, redirect refusal and Prometheus readiness behavior.|
|`make check`|passed|2026-07-14|P3.3 passes generated-client reproducibility, contract validation, Go/TypeScript tests, formatting, boundaries and secret scan.|
|`sh -n scripts/run-real-metrics-e2e.sh && node --check tests/e2e/mock/api-e2e.mjs && docker compose -f compose.mock-e2e.yaml -f compose.real-metrics-e2e.yaml config`|passed|2026-07-14|P3.4 validates the real-metrics script syntax, optional real-series assertion and merged Compose topology without starting containers.|
|`cd services/ai-core && go test ./internal/adapters/outbound/agent/profile`|passed|2026-07-14|P4.1 validates the repository Profile, fixed local view metadata and rejection of missing guidance, invalid UTF-8 and files over 64 KiB.|

## Next Slice

Run P3.4's `make e2e-real-metrics` and re-run `make e2e-mock` after the user-managed local stack releases ports 3000, 8080 and 8081. Those pending container gates remain required for final closeout; P4 implementation can proceed independently.
