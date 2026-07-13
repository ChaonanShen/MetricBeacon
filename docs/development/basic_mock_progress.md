# Basic Mock Skeleton Progress

lastUpdated: 2026-07-13T17:40:00Z
currentGate: G8
status: completed
headCommit: 0a14292
worktreeSummary: remediation R0-R5 complete; original G4-G8 have been re-verified and the worktree is clean

## Passed Gates

- [x] G0 Scaffold
- [x] G1 Contracts
- [x] G2 Domain/SQLite
- [x] G3 assistant-mcp
- [x] G4 AI Workflow
- [x] G5 Plugin Backend
- [x] G6 Frontend
- [x] G7 Mock E2E
- [x] G8 Closeout

## Last Verified Commands

|command|result|timestamp|notes|
|-|-|-|-|
|`go version`|passed|2026-07-13T12:55:42Z|`go1.26.5 darwin/amd64`|
|`node --version` / `npm --version`|passed|2026-07-13T12:55:42Z|`v22.23.1` / `10.9.8`|
|`docker image inspect grafana/grafana:latest`|passed|2026-07-13T12:55:42Z|Grafana `13.1.0`, linux/amd64 digest recorded in toolchain lock|
|`make bootstrap-check`|passed|2026-07-13T13:02:27Z|All five Go modules compile; frontend typecheck and dependency boundary check pass without leaving binaries in source directories|
|`make validate-contracts`|passed|2026-07-13T13:25:05Z|Three OpenAPI documents, 21 JSON Schemas, all examples and the node_exporter fixture validate|
|`make generated-client-diff`|passed|2026-07-13T13:25:05Z|Go and TypeScript OpenAPI/Tool output is reproducible|
|`make bootstrap-check` + generated Go `go test ./...`|passed|2026-07-13T13:25:05Z|Generated AI Core client/server, tool DTOs, all scaffold modules and frontend typecheck compile|
|`go test ./internal/domain/... ./internal/application/... ./internal/ports/...`|passed|2026-07-13T13:30:05Z|Task/Chart state transitions and all pure Domain/Port packages compile|
|`make boundary-check` + `make bootstrap-check`|passed|2026-07-13T13:30:05Z|Domain/Application/Port dependency rule and full scaffold compilation pass|
|`make test-ai-core-domain test-sqlite`|passed|2026-07-13T13:57:23Z|Domain/Port plus SQLite CRUD, tenant isolation, rollback, idempotency, sequence and Replay Contract Tests pass|
|`go vet ./...` (AI Core)|passed|2026-07-13T13:57:23Z|AI Core packages pass vet|
|`make boundary-check` + `make bootstrap-check`|passed|2026-07-13T13:57:23Z|Adapter remains outside Domain/Application/Port; all workspace modules and frontend typecheck compile|
|`make test-assistant-mcp`|passed|2026-07-13T14:06:28Z|Mock Prometheus Adapter Contract Test plus real Streamable HTTP MCP tools/list and all three tool calls pass|
|`go vet ./...` (assistant-mcp)|passed|2026-07-13T14:06:28Z|assistant-mcp packages pass vet|
|`ASSISTANT_MCP_LISTEN_ADDRESS=127.0.0.1:18081 go run ./cmd/server` + health/ready probes|passed|2026-07-13T14:06:28Z|independent process serves both `/healthz` and `/readyz`|
|`make boundary-check` + `make bootstrap-check`|passed|2026-07-13T14:06:28Z|all workspace modules and frontend typecheck compile after G3|
|`go test ./...` + `go vet ./...` (AI Core)|passed|2026-07-13T14:08:46Z|G4 MCP ToolGateway and MetricCatalog adapter foundation compiles without boundary violations|
|`go test ./...` (AI Core) + `make test-ai-core-domain test-sqlite`|passed|2026-07-13T14:12:17Z|typed QueryEngine adapter and DeterministicMockAgentRuntime tests pass; G2 persistence regression remains green|
|`make test`|passed|2026-07-13T14:46:55Z|AI Core workflow/HTTP/SSE/MCP, assistant-mcp and Plugin Backend component suites plus frontend typecheck pass|
|`make lint` + `make boundary-check` + `make secret-scan`|passed|2026-07-13T14:49:14Z|Go formatting, frontend typecheck, production dependency boundary and secret scan pass|
|real assistant-mcp + AI Core process smoke|passed|2026-07-13T14:26:27Z|AI Core readiness reached 200 through real MCP; created Task completed with 30 durable events and three fixed chart executions|
|`docker buildx version` + `docker buildx inspect colima`|passed|2026-07-13T15:13:43Z|Buildx `v0.35.0` is installed; active `colima` builder is running BuildKit `v0.30.0` on the Colima socket|
|`make check`|passed|2026-07-13T15:18:00Z|Generated diff, contracts, Go tests, frontend typecheck, boundary check and secret scan pass at `11d505b`|
|`go vet ./...` in all three runtime Go modules|passed|2026-07-13T15:27:00Z|AI Core, assistant-mcp and Plugin Backend pass vet|
|direct AI Core + real assistant-mcp smoke|passed|2026-07-13T15:25:00Z|Task completed at sequence 30 with 7 `tool.started`, 7 `tool.completed`, 3 `chart.created` and 3 `chart.execution_completed`; replay after 25 returned only 26-30|
|`make e2e-mock` with Buildx and clean ports|failed|2026-07-13T15:24:00Z|All three services became healthy, then the first Grafana Plugin Resource request returned 404 `Plugin not found`|
|retry same relative-duration task body and idempotency key|failed|2026-07-13T15:26:00Z|AI Core returned 409 `idempotency_conflict` because the relative range was resolved against a new current time before hashing|
|authenticated `make e2e-mock` after Grafana remediation|passed|2026-07-13T16:16:00Z|Plugin settings returned 200; Session/Task/SSE traversed Grafana Plugin Backend, AI Core and assistant-mcp and found the final answer and three charts|
|unchanged cached Grafana image rebuild|passed|2026-07-13T16:17:00Z|BuildKit reused dependency and compile layers; rebuild completed in 4.53 seconds after the cold cache population|
|AI Core R2 tests and fault injection|passed|2026-07-13T16:52:00Z|Relative-time retry, failure ordering, live cleanup context, restart recovery and event-append rollback tests pass|
|`make check` + authenticated `make e2e-mock` after R2|passed|2026-07-13T16:59:00Z|Repository checks and the real Grafana → AI Core → assistant-mcp golden path pass after transaction/recovery changes|
|unchanged cached AI Core image rebuild|passed|2026-07-13T17:04:00Z|BuildKit reused module and compile layers; rebuild completed in 1.79 seconds|
|`make test-frontend`|passed|2026-07-13T17:21:00Z|Vitest mapper, reducer, URL and SSE recovery coverage (9 tests) plus TypeScript typecheck pass|
|`make test-ai-mcp`|passed|2026-07-13T17:40:00Z|Original G4 MCP tool gateway, HTTP and workflow suites pass|
|`make test-plugin-backend`|passed|2026-07-13T17:40:00Z|Original G5 authenticated Resource proxy component suite passes|
|`make test-frontend`|passed|2026-07-13T17:40:00Z|Original G6 mapper, reducer, URL and EventSource tests (9) plus typecheck pass|
|`make e2e-mock`|passed|2026-07-13T17:40:00Z|Original G7 runs structured API and authenticated Playwright coverage, then removes only its Compose project and volume|
|`make check`|passed|2026-07-13T17:40:00Z|Original G8 generated diff, contracts, lint, unit suites, boundary and secret checks pass|

## Files/Interfaces Completed

- G0 root/module/frontend skeleton: complete
- `build/toolchain.lock`: initial host and Grafana runtime lock recorded
- `scripts/check-boundaries.sh`: initial AI Core boundary rule recorded
- G1 OpenAPI/JSON Schema/SSE/MCP contracts, examples and node_exporter fixture: complete
- G1 generated Go/TypeScript clients, server interface and schema validation: complete
- G2 pure Domain, application DTO, typed Port and deterministic testkit baseline: complete
- G2 SQLite migration, ApplicationStore, durable TaskEvent Store and in-memory notifier: complete
- G3 `assistant-mcp` Grafana namespace, MockPrometheusAdapter, fixture/schema readiness and Streamable HTTP MCP transport: complete
- G4 AI Core Workflow, durable TaskEvents, SQLite Chart/ToolCall persistence, generated HTTP handlers, SSE replay and real MCP golden path: remediation complete; original Gate re-verification remains deferred until R4
- G5 Grafana Plugin Backend generated-client Resource proxy and chunked SSE proxy: registration, App `jsonData` endpoint provisioning and authenticated Plugin Backend → AI Core integration now pass; original Gate re-verification remains deferred until R2-R4 complete
- G6 Grafana Workbench, Query cache, SSE reducer and DataFrame mapper: URL restore, stable resumable SSE, persisted chart ranges and frontend unit tests complete; original Gate re-verification remains deferred until R4
- G7 Dockerfiles and Compose topology: structured Plugin Resource API E2E and authenticated browser E2E pass; selected Compose resources are removed on completion

## Remaining Work For Current Gate

None. The remediation track and original G4-G8 re-verification are complete.

## Resolved Environment Issues

- symptom: `docker compose up --build` warned that the Buildx plugin was missing and then spent several minutes with little output while downloading and compiling Go dependencies.
- root cause: Homebrew installs Docker CLI, Compose and Buildx separately. Buildx was absent, so Compose fell back to the classic builder, duplicated backend builds and lost effective BuildKit dependency caching. The build later completed, so this was not a confirmed Go module proxy outage.
- prevention: install `docker-buildx`, expose its Homebrew plugin directory through `cliPluginsExtraDirs`, and verify the active `colima` builder before G7. See the root README Docker/Colima section.
- harmless secondary symptom: `docker buildx ls` reports the reserved `default` builder as unavailable because it points to `/var/run/docker.sock`; Colima uses `~/.colima/default/docker.sock`. The active `colima*` builder is the acceptance condition.

## Known Failures

- command: `make e2e-mock`
- relevant output summary: Buildx built all images and all three containers became healthy; `POST /api/plugins/mini-torchbearing-app/resources/sessions` then returned 404. Grafana settings reported `no installed plugin with that id`, and startup logs contained no registration for `mini-torchbearing-app`.
- confirmed causes: the plugin root contained a nested `dist` directory, metadata omitted `dependencies`, and—after those were fixed—the backend process did not receive arbitrary `AI_CORE_ENDPOINT` environment variables from Grafana.
- resolution: flatten the install artifact, add metadata, provision `aiCoreEndpoint` as App `jsonData`, read it from `PluginContext.AppInstanceSettings`, require authenticated E2E, and use plugin discovery as readiness. The rerun passed.
- command: same `POST /v1/tasks` relative-duration body with the same `Idempotency-Key`
- relevant output summary: the retry returned 409 `idempotency_conflict` instead of the original Task.
- suspected cause: the HTTP adapter resolves `relativeDuration` with `time.Now()` before the command hashes `CreateTaskInput`, so every retry hashes a different absolute range.
- resolution: hash the canonical wire intent before freezing the relative range and cover same-intent
  reuse plus changed-intent conflict at the HTTP and full Mock E2E layers.

## Decisions Made Within Plan Defaults

- decision: use `mini-torchbearing.local` module base
- reason: repository has no Git remote
- affected files: `go.work`, all G0 Go modules
- decision: lock Grafana frontend package family to the locally verified Grafana `13.1.0`
- reason: Plugin frontend and runtime image must track the same Grafana major/minor baseline
- affected files: `build/toolchain.lock`
- decision: preserve OpenAPI 3.1 as the authority and project the Redocly-bundled AI Core/Tool schemas to temporary OpenAPI 3.0 only for Go generation
- reason: `oapi-codegen` `v2.7.2` reports that OpenAPI 3.1 is unsupported and fails to generate directly from `$defs`/null-capable 3.1 schemas
- affected files: `scripts/generate-clients.sh`, `apps/grafana-plugin/frontend/scripts/project-oas31-to-oas30.mjs`, ignored `build/generated/openapi/`
- decision: use React `18.3.1` with Grafana `13.1.0`, not React 19
- reason: Grafana `13.1.0` declares a React 18 peer dependency
- affected files: frontend `package.json` and lockfile
- decision: use Vitest `4.1.10` for frontend unit tests
- reason: it runs the mapper, reducer, URL and EventSource tests under the locked Node 22 toolchain
- affected files: frontend `package.json` and lockfile, `build/toolchain.lock`

## Blockers Requiring User/Approval

- none
