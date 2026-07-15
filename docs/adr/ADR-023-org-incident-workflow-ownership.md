# ADR-023: Organization incident workflow ownership

> Status: Accepted
> Date: 2026-07-16

## Context

AI Core currently owns private Sessions, Messages, metric-analysis Tasks and
replayable TaskEvents. The next slice needs alert-created work that is visible
to an organization, can pause for approval, and can resume after a process
restart. Putting that state in Grafana, Plugin Backend or assistant-mcp would
split the workflow truth across services and conflict with the existing data
ownership boundary.

## Decision

- AI Core owns Incident Tasks, organization-visible incident Sessions,
  checkpoints, remediation intents, approvals, execution records and the
  authoritative workflow audit.
- `Task` becomes a tagged union of the existing `metric_analysis/QueryPlan`
  branch and a new `incident_remediation/IncidentPlan` branch. QueryPlan stays
  unchanged.
- `Session.visibility` is `private` or `org_incident`. Private access remains
  owner-only; any member of the same Grafana organization can read an incident
  Session, while only an explicitly authorized administrator can approve it.
- Plugin Backend remains a thin identity, authorization and proxy boundary. It
  does not persist or orchestrate Incident state.
- assistant-mcp interprets versioned Playbooks and returns opaque checkpoints;
  AI Core persists but does not interpret those checkpoints.
- Every state change and TaskEvent is committed before SSE notification.

## Consequences

ADR-022 remains authoritative for private history but is intentionally
extended by a second, separately queried organization-visible Session class.
SQLite requires a preserving table rebuild for the constrained Session,
Message and Task tables. Incident recovery can resume from durable state
without sharing databases between services.

## Alternatives considered

- Store incidents in Plugin Backend: rejected because it would make the proxy
  an orchestration and data owner.
- Store Playbook runs in assistant-mcp: rejected because Task state and user
  approval would then be split across databases.
- Represent remediation as QueryPlan steps: rejected because metric selection
  and resumable side effects have different invariants.

## Review conditions

Review before adding cross-organization sharing, incident mutation by normal
chat messages, multi-Task incident rooms, or an external workflow engine.

## Related documents

- [`ADR-022-private-session-history.md`](ADR-022-private-session-history.md)
- [`../implementation/order_service_incident_remediation_execution_plan.md`](../implementation/order_service_incident_remediation_execution_plan.md)

