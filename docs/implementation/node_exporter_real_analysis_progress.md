# Node Exporter Real Analysis Progress

lastUpdated: 2026-07-14
currentGate: complete
status: completed
headCommit: 68eac9d
worktreeSummary: G0 through P5 are complete. The final gate passed `make check`, fresh Mock and real-metrics Compose E2E, and credentialed DeepSeek Eino smoke against live node_exporter data; every E2E stack removed its containers and volume.

## Gate Status

- [x] G0 Design activation and decision record
- [x] P1 Multi-turn contracts and persistence
  - [x] P1.1 Task-linked Message persistence and source-call correlation
  - [x] P1.2a AI Core history pages and finite replay
  - [x] P1.2b Agent context and terminal stream behavior
  - [x] P1.3 Plugin history/replay proxy
- [x] P2 Persistent Session workbench
  - [x] P2.1 Session reducer, page recovery, finite replay and terminal SSE handling
  - [x] P2.2 Container E2E execution
- [x] P3 Real Prometheus and node_exporter
  - [x] P3.1 Shared `prometheus-main` datasource UID, canonical-expression contract and migration
  - [x] P3.2 Shared node_exporter registry and PromQL AST policy
  - [x] P3.3 Real Prometheus HTTP Adapter, driver config and readiness probe
  - [x] P3.4 Real-metrics Compose topology and live series E2E
- [x] P4 Static Profile and constrained Eino runtime
  - [x] P4.1 Read-only node_exporter Profile and local view metadata
  - [x] P4.2 Constrained Eino runtime, strict tools and summary isolation
  - [x] P4.3 Explicit DeepSeek configuration and Bootstrap wiring
- [x] P5 End-to-end closeout
  - [x] P5.1 Credential-gated real-agent smoke harness and leak checks
  - [x] P5.2a Execute Mock and real-metrics container gates
  - [x] P5.2b Execute credentialed real-agent container gate

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
|`make e2e-mock`|passed|2026-07-14|Rebuilt the Mock Compose stack, passed the three-turn API E2E and Playwright refresh/recovery suite, then removed its containers and volume.|
|`make validate-contracts generated-client-diff test-sqlite test-assistant-mcp test-ai-mcp`|passed|2026-07-14|P3.1 validates all generated UID/canonical-expression contracts; SQLite verifies Task and Chart query JSON forward migration from `mock-prometheus`.|
|`make test-assistant-mcp test-ai-mcp`|passed|2026-07-14|P3.2 accepts only the three registered normalized PromQL expressions and rejects out-of-registry ASTs before Mock fixture access.|
|`make test-assistant-mcp`|passed|2026-07-14|P3.3 `httptest` coverage verifies the opt-in HTTP driver, canonical query POST, local validation, response/series limits, non-finite filtering, timeout/status/error mapping, redirect refusal and Prometheus readiness behavior.|
|`make check`|passed|2026-07-14|P3.3 passes generated-client reproducibility, contract validation, Go/TypeScript tests, formatting, boundaries and secret scan.|
|`sh -n scripts/run-real-metrics-e2e.sh && node --check tests/e2e/mock/api-e2e.mjs && docker compose -f compose.mock-e2e.yaml -f compose.real-metrics-e2e.yaml config`|passed|2026-07-14|P3.4 validates the real-metrics script syntax, optional real-series assertion and merged Compose topology without starting containers.|
|`make e2e-real-metrics`|passed|2026-07-14|Started Prometheus/node_exporter, waited for `up=1` and two CPU idle scrapes, passed the real-series API E2E and browser suite, then removed its containers and volume.|
|`cd services/ai-core && go test ./internal/adapters/outbound/agent/profile`|passed|2026-07-14|P4.1 validates the repository Profile, fixed local view metadata and rejection of missing guidance, invalid UTF-8 and files over 64 KiB.|
|`make test-ai-agent test-ai-core-domain test-ai-mcp`|passed|2026-07-14|P4.2 validates the Eino fake-model seam, strict tool JSON, stable source-call pairing, canonical one-view execution, final-result consistency and the absence of raw series, labels, upstream warnings and internal URLs from model inputs.|
|`make check`|passed|2026-07-14|P4.2 passes generated-client reproducibility, contract validation, formatting, all Go/TypeScript tests including `test-ai-agent`, dependency-boundary checks and secret scan.|
|`make test-ai-agent && docker compose -f compose.mock-e2e.yaml -f compose.real-metrics-e2e.yaml -f compose.real-agent-e2e.yaml config`|passed|2026-07-14|P4.3 verifies default Mock startup without a DeepSeek key, rejection of Eino without key or Profile, Profile image path/config limits and the merged opt-in real-agent Compose topology.|
|`make check`|passed|2026-07-14|P4.3 keeps the new Bootstrap configuration tests in the Agent test target and passes the full local quality gate.|
|`sh -n scripts/run-real-agent-e2e.sh && node --check tests/e2e/real-agent/api-smoke.mjs && DEEPSEEK_API_KEY=placeholder docker compose -f compose.mock-e2e.yaml -f compose.real-metrics-e2e.yaml -f compose.real-agent-e2e.yaml config`|passed|2026-07-14|P5.1 validates the real-agent harness syntax, API smoke discovery and credentialed Compose topology without making a model call.|
|`make e2e-real-agent` (without key)|passed|2026-07-14|Fails immediately with exit code 2 and a clear key-required message; it does not start containers or silently fall back to Mock.|
|`GOWORK=off go test ./...` in `packages/generated-contracts/go`; assistant-mcp image build|passed|2026-07-14|The standalone generated-contracts Go module now declares the `oapi-codegen` runtime used by generated Grafana tool types, so the isolated assistant-mcp Docker build resolves its imports.|
|`sh -n scripts/run-real-metrics-e2e.sh scripts/run-real-agent-e2e.sh`; live Prometheus readiness probe|passed|2026-07-14|The real-metrics and real-agent harnesses use URL-encoded GET queries: `up == 1` and a boolean two-scrape CPU condition, avoiding BusyBox `wget` POST form parsing differences.|
|`zsh -lc 'set -a; source .env; set +a; make e2e-real-agent'`|passed|2026-07-14|DeepSeek Eino completed overview and CPU follow-up against live node_exporter data; the harness verified durable tool-pair events, history/replay recovery, terminal stream closure, and API/log/SQLite leak markers. The key was loaded only into the invoking process and was not logged or persisted.|
|`make e2e-mock`|passed|2026-07-14|Fresh Mock Compose gate verifies explicitly encoded history/replay query parameters, reducer-owned finite replay completion, two consecutive tasks, SSE follow, refresh recovery, six-card desktop layout and two-column mid-width layout; containers and volume were removed.|
|`make e2e-real-metrics`|passed|2026-07-14|Fresh real-metrics Compose gate waited for node_exporter target and two CPU idle scrapes, verified non-empty live series through API E2E, then verified browser task/replay/layout behavior without assuming Mock fixture instance labels; containers and volume were removed.|
|`make check`|passed|2026-07-14|Final local quality gate passed after the replay, layout, contract-module and real-metrics test fixes: generated artifacts, contracts, Go/TypeScript suites, boundary checks and secret scan are clean.|
|`zsh -lc 'set -a; source .env; set +a; make e2e-real-agent'`|passed|2026-07-14|Final credentialed gate repeated the live overview/CPU DeepSeek smoke, durable tool-pair/replay checks and API/log/SQLite leak scans; the key remained process-local and the Compose stack was removed.|

## Next Slice

Completed. `make check`, fresh Mock and real-metrics E2E, and the credentialed real-agent gate all passed. This file is retained as the implementation evidence for the completed slice.
