# Basic Mock Skeleton Remediation Plan

status: completed
createdAt: 2026-07-13T15:30:00Z
sourceCommit: 11d505b
originalPlan: `docs/basic_mock_skeleton_execution_plan.md` v1.2
resumeOriginalPlanAt: completed

## 1. Purpose and boundary

This document is the temporary execution track for correcting defects found while auditing commit
`11d505b`. It does not modify or replace `basic_mock_skeleton_execution_plan.md`. Once every
remediation gate below passes, execution returns to the original plan at G4 and proceeds in order.

The architecture remains unchanged:

```text
Browser -> Grafana App Frontend -> Grafana Plugin Backend -> AI Core
        -> typed MCP adapter -> assistant-mcp -> MockPrometheusAdapter
```

No real LLM, Prometheus, Grafana dashboard write, knowledge, playbook or alert work is added here.

## 2. Audit evidence

### 2.1 What works

- `make check` passes at `11d505b`.
- All three Go runtime modules pass `go vet ./...`.
- AI Core and assistant-mcp become healthy in Compose.
- A direct AI Core request traverses real Streamable HTTP MCP and completes with sequence 30,
  7 `tool.started`, 7 `tool.completed`, 3 `chart.created` and 3
  `chart.execution_completed` events.
- SSE replay after sequence 25 returns only sequences 26 through 30.

### 2.2 Confirmed failures

1. Grafana originally became healthy but did not discover `mini-torchbearing-app`; after correcting
   the package root, Resource requests advanced from 404 to 503 because Grafana intentionally did
   not pass the arbitrary `AI_CORE_ENDPOINT` environment variable to the backend subprocess.
2. Retrying the same relative-duration Create Task body with the same idempotency key returns
   409 `idempotency_conflict`.
3. Workflow state mutations and their durable events are committed separately.
4. Tool failure events, failed status events, timeout cleanup and startup recovery are incomplete.
5. Frontend URL restoration is absent and its SSE subscription reconnects after every sequence.
6. Frontend tests and browser E2E are absent; current targets only typecheck or grep a partial API
   response.

## 3. Root-cause analysis

### 3.1 Grafana App distribution and registration

Current image layout:

```text
mini-torchbearing-app/
  plugin.json
  mini-torchbearing-app_linux_amd64
  dist/
    module.js
```

Grafana treats a child `dist` directory as the installable distribution root. It therefore scans
`dist/`, where neither `plugin.json` nor the backend executable exists. The metadata also omits the
required `dependencies` object. The generic Grafana `/api/health` check cannot detect this because
Grafana itself is healthy even when the application plugin is absent.

The E2E API uses anonymous requests even though Plugin Backend identity construction requires a
trusted Grafana user. Grafana 13 also warns that anonymous `Admin` role configuration is deprecated.
Mock E2E must authenticate as the configured admin user and must not use anonymous Admin as a
substitute for a real PluginContext.

After correcting the install root, container DNS and direct Grafana-container-to-AI-Core requests
both succeeded, but `/proc/<backend-pid>/environ` confirmed that Grafana's backend subprocess only
received Grafana-managed variables. The process therefore used the old localhost fallback. Backend
plugin connection settings belong in App instance `jsonData`, delivered on each Resource request as
`PluginContext.AppInstanceSettings`; the Mock profile now provisions `aiCoreEndpoint` through that
supported path.

### 3.2 Relative time idempotency

The HTTP adapter resolves `relativeDuration` against `time.Now()` before the command service hashes
its input. A retry therefore contains a different absolute `from/to`, even though the wire request
is identical. The idempotency hash must represent the caller's canonical intent, while the absolute
range is frozen only for the first successful creation.

### 3.3 Workflow consistency and recovery

Task state, ToolCall state, Chart/Execution state and corresponding TaskEvents are currently written
through separate repository calls. A failure between calls can expose a state for which no replayable
event exists. Failure cleanup also uses the workflow context after it may already be cancelled and
ignores cleanup errors. Bootstrap never marks interrupted non-terminal Tasks as failed.

### 3.4 Frontend state and verification

At audit time, `sessionId` and `taskId` existed only in React state. The workbench neither read nor
replaced the URL, so refresh could not restore snapshots or replay events. The SSE effect depended
on `latestSequence`, which tore down the connection after every event. No frontend test runner or
Playwright suite caught either behavior.

## 4. Remediation gates

### R0 - Reproducible environment

- Keep Docker context and Buildx checks documented in the root README.
- Use the active Colima Docker context without setting `BUILDX_BUILDER` for its Docker driver.
- Preserve unrelated user containers and worktree changes.

Acceptance:

```text
docker buildx inspect colima
make check
```

### R1 - Grafana App install artifact and authenticated API E2E

- Produce one plugin install directory containing `plugin.json`, `module.js`, source map and the
  executable as siblings; do not nest another `dist` directory inside it.
- Add required Grafana dependency metadata and deterministic app enablement for the isolated Mock
  profile.
- Validate the built image layout before Grafana starts.
- Change readiness to verify plugin discovery/backend registration, not only `/api/health`.
- Authenticate API E2E with `GRAFANA_ADMIN_USER` and `GRAFANA_ADMIN_PASSWORD`.
- Disable unrelated background plugin installation in the isolated E2E profile.
- Provision `aiCoreEndpoint` as Grafana App `jsonData` and validate the configured URL in the
  backend; do not rely on arbitrary subprocess environment propagation.
- Split Go module download from source compilation and use BuildKit module/build caches.

Acceptance:

- Grafana logs show `mini-torchbearing-app` registration and backend startup.
- Authenticated Plugin settings request succeeds.
- Plugin Resource Create Session/Task/SSE succeeds through Grafana.
- The same request IDs appear through Plugin Backend, AI Core and assistant-mcp where logged.

### R2 - Idempotency and Workflow durability

- Carry a canonical request hash derived from the wire intent into Create Task, or reserve by that
  hash before resolving the relative range.
- Same key/same body returns the original Task; same key/different body returns conflict.
- Wrap every state mutation and its TaskEvent in a minimal store transaction.
- Persist `tool.failed`, `task.status_changed(failed)` and `task.failed` in the specified order.
- Use a bounded live cleanup context after execution cancellation.
- On startup, mark every non-terminal persisted Task as `execution_interrupted` without rerunning it.

Acceptance:

- HTTP-level idempotency tests cover relative and absolute ranges.
- Failure injection tests prove transaction rollback and durable failure events.
- Restart recovery test proves no non-terminal Task is silently stranded or rerun.

### R3 - Frontend restore and unit tests

- Read `sessionId`/`taskId` from the workbench URL.
- On first valid submit, create the fixed-title Session and replace the URL with both IDs.
- On refresh, fetch Session/Task and replay from sequence zero.
- Keep one SSE connection per Task; reconnect only after transport failure and resume after the last
  accepted sequence.
- Use execution/query time ranges for chart rendering.
- Add tests for mapper alignment, reducer de-duplication/gaps, SSE reconnection and URL restoration.

Acceptance:

```text
make test-frontend
```

must execute tests in addition to TypeScript typechecking.

### R4 - Browser and full Mock E2E

- Add Playwright using the locked version and authenticated Grafana admin state.
- Verify submit, assistant response, three non-empty TimeSeries panels and PromQL display.
- Refresh the route and verify identical restored content.
- Strengthen API E2E to assert 7 ToolCalls, 3 Charts, continuous sequence, replay and idempotency.
- Ensure teardown removes only the selected Compose project and volume.

Acceptance:

```text
make e2e-mock
```

runs both API and browser coverage and exits zero.

### R5 - Return to the original plan

After R0-R4 pass:

1. Update `basic_mock_progress.md` with the remediation evidence.
2. Re-run original G4, G5, G6 and G7 commands in order.
3. Run G8 `make check` and documentation verification.
4. Mark this document `completed` and set `resumeOriginalPlanAt: completed`.

## 5. Implementation order and rewrite policy

Prefer focused correction over a full repository rewrite:

1. Rebuild the Grafana distribution/Compose/E2E slice because its current packaging is invalid.
2. Refactor command/workflow transaction boundaries behind existing Ports without changing wire
   contracts.
3. Rewrite the small Workbench/SSE implementation where necessary; preserving its current code is
   less important than satisfying URL/replay semantics.
4. Preserve generated contracts, domain state machines, MCP namespace and MockPrometheusAdapter
   unless a failing contract test demonstrates a defect.

A full rewrite of AI Core, assistant-mcp or shared contracts is explicitly out of scope because the
real MCP golden path already passes and those boundaries are structurally sound.

## 6. Current status

- [x] R0 environment reproduced and documented
- [x] R1 Grafana App install and authenticated API E2E
- [x] R2 idempotency and Workflow durability
- [x] R3 Frontend restore and unit tests
- [x] R4 browser/full E2E
- [x] R5 original Gate re-verification

R1 evidence (2026-07-13):

- Plugin settings returned HTTP 200 with `enabled=true`, and Grafana logged
  `Plugin registered pluginId=mini-torchbearing-app`.
- `make e2e-mock` completed through authenticated Grafana Resource APIs and found the final
  assistant message plus all three expected chart titles in the SSE stream.
- Plugin Backend unit tests cover provisioned endpoint selection, missing configuration and trusted
  Grafana identity propagation; `go test ./...` and `go vet ./...` pass.
- A cold dependency population took 131 seconds and the first SDK compile took 162 seconds; an
  unchanged cached Grafana image rebuild then completed in 4.53 seconds with every dependency and
  compile layer cached.

R2 evidence (2026-07-13):

- HTTP tests prove identical relative-duration intent with the same idempotency key returns the
  original Task, while a changed message returns 409.
- Task transitions, failure transitions, ToolCalls, assistant messages, Charts and ChartExecutions
  now commit with their durable TaskEvents in minimal SQLite transactions.
- Cancellation failure tests prove a fresh bounded cleanup context persists
  `tool.failed`, `task.status_changed(failed)`, then `task.failed`.
- Startup recovery fails non-terminal Tasks and open ToolCalls as `execution_interrupted`, does not
  invoke the runtime and is idempotent on a second pass.
- An injected event append failure proves the paired Task mutation rolls back.
- Repository `make check` and authenticated `make e2e-mock` both pass after the R2 changes.
- AI Core image dependency/build caching reduced an unchanged rebuild from a 124-second compile
  path to 1.79 seconds.

R3 evidence (2026-07-13):

- Workbench reads `sessionId` and `taskId` from the URL, fetches their snapshots on refresh and
  replays Task events from sequence zero. Its first successful submission uses the fixed
  `Node exporter mock analysis` Session title and replaces the route with both identifiers.
- One EventSource remains open while contiguous events are accepted. Duplicate events are ignored;
  transport, decode or sequence-gap failures close it and reconnect from the last accepted
  sequence.
- Chart rendering uses the durable execution sample range, falling back to the persisted query
  range rather than a fresh browser-relative range.
- `make test-frontend` passed with Vitest 4.1.10: mapper alignment, reducer de-duplication/gap
  rejection, URL restore/replace, SSE gap recovery and transport resumption all pass before
  TypeScript typechecking.

R4 evidence (2026-07-13):

- The locked Playwright 1.61.1 browser test authenticates as the Grafana admin, submits a request,
  verifies the assistant response, three titled non-empty TimeSeries panels and their PromQL,
  then refreshes the route and verifies the same restored content.
- The API E2E parses durable SSE rather than grepping text. It asserts sequences are continuous,
  exactly 7 `tool.started`/`tool.completed` pairs and 3 Chart/Execution pairs, exact replay suffix,
  same-intent idempotent reuse and changed-intent conflict.
- Browser execution exposed and corrected the production bundle `process` reference, missing visible
  Card titles, and the relative Compose cleanup path after the browser test. `make e2e-mock` now
  exits zero and removes only `mini-torchbearing-mock` resources.

R5 evidence (2026-07-13):

- Re-ran original G4 `make test-ai-mcp`, G5 `make test-plugin-backend`, G6 `make test-frontend`,
  G7 `make e2e-mock` and G8 `make check` in order; every command passed.
