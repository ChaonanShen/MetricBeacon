# Basic Mock Skeleton Progress

lastUpdated: 2026-07-13T16:22:00Z
currentGate: G4
status: in_progress
headCommit: be17ac3
worktreeSummary: remediation R0-R1 complete; Grafana registration and authenticated Plugin Resource API E2E now pass, R2 workflow durability is next

## Passed Gates

- [x] G0 Scaffold
- [x] G1 Contracts
- [x] G2 Domain/SQLite
- [x] G3 assistant-mcp
- [ ] G4 AI Workflow
- [ ] G5 Plugin Backend
- [ ] G6 Frontend
- [ ] G7 Mock E2E
- [ ] G8 Closeout

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

## Files/Interfaces Completed

- G0 root/module/frontend skeleton: complete
- `build/toolchain.lock`: initial host and Grafana runtime lock recorded
- `scripts/check-boundaries.sh`: initial AI Core boundary rule recorded
- G1 OpenAPI/JSON Schema/SSE/MCP contracts, examples and node_exporter fixture: complete
- G1 generated Go/TypeScript clients, server interface and schema validation: complete
- G2 pure Domain, application DTO, typed Port and deterministic testkit baseline: complete
- G2 SQLite migration, ApplicationStore, durable TaskEvent Store and in-memory notifier: complete
- G3 `assistant-mcp` Grafana namespace, MockPrometheusAdapter, fixture/schema readiness and Streamable HTTP MCP transport: complete
- G4 AI Core Workflow, durable TaskEvents, SQLite Chart/ToolCall persistence, generated HTTP handlers, SSE replay and real MCP golden path: implemented; failure events, restart recovery and mutation/event transaction boundaries remain incomplete
- G5 Grafana Plugin Backend generated-client Resource proxy and chunked SSE proxy: registration, App `jsonData` endpoint provisioning and authenticated Plugin Backend → AI Core integration now pass; original Gate re-verification remains deferred until R2-R4 complete
- G6 Grafana Workbench, Query cache, SSE reducer and DataFrame mapper: UI skeleton exists; URL restore, stable SSE subscription and frontend tests are incomplete
- G7 Dockerfiles and Compose topology: containers become healthy, but Plugin Resource API E2E fails and browser E2E is absent

## Remaining Work For Current Gate

1. Make Workflow state/ToolCall/Chart mutations atomic with their TaskEvents, and emit `tool.failed` plus both failed-state events.
2. Add startup recovery for non-terminal Tasks and ensure timeout failure persistence uses a live cleanup context.
3. Fix relative-time idempotency hashing and add a same-key/same-body HTTP retry assertion.
4. Then fix frontend restore/tests and rerun G5 through G8 in order.

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
- next safe action: hash the canonical wire intent (or otherwise preserve the first frozen range for the reserved key) and add an HTTP-level retry test.

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

## Blockers Requiring User/Approval

- none
