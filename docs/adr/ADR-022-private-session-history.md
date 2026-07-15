# ADR-022: Private session history and activity ordering

> Status: Accepted
> Date: 2026-07-15

## Context

AI Core already persists Sessions, Messages, Tasks, TaskEvents, Charts and
Executions, and the Workbench can restore one Session from its URL. It cannot
list Sessions, and creating a follow-up Task does not update
`AnalysisSession.updatedAt`. A browser-only list would not survive devices or
cleared storage, while a tenant-wide list would expose conversations before a
sharing model exists.

## Decision

- AI Core exposes an owner-scoped, keyset-paginated Session list through the
  Plugin Resource API. The browser continues to call only Plugin resources.
- The current private profile permits only the Session creator to list, open,
  continue, or read resources derived from a Session. A foreign resource is
  reported as `resource_not_found` to avoid disclosing its existence.
- The list includes only Sessions with at least one persisted Task. An empty
  Session left by a failed pre-persistence planner attempt is not conversation
  history.
- Session list order is `updatedAt DESC, id DESC`. Accepting a new durable User
  Message and Task touches the Session in the same transaction; background
  Task transitions do not reorder it.
- A fresh Workbench remains an unsaved local empty state until first submit.
  New titles are deterministically derived from the first submitted message;
  no model call or browser storage is used.

## Consequences

The two Session APIs gain a backward-compatible `GET /sessions` operation and
a `SessionPage` schema. AI Core gains an owner-and-activity index and increments
Session version whenever a new Task is accepted. Existing stored rows require
no backfill.

The current private access rule is intentionally narrower than the long-term
product proposal for team visibility, sharing and Fork. Those capabilities
require an explicit visibility/authorization contract and a later ADR.

## Alternatives considered

- Keep Session IDs in localStorage: rejected because it creates a browser-only
  competing truth source.
- List every tenant Session: rejected because no team visibility permission
  model exists.
- Sort by creation time: rejected because continued conversations would not
  return to the top.
- Auto-open the latest Session: rejected because it changes the existing fresh
  Workbench entry behavior.
- Generate titles with the external model: rejected because titles do not
  justify an additional model dependency or data disclosure.

## Open questions

None for the private list-and-switch slice. Search, rename, delete, archive,
share, team visibility and Fork remain separate work.

## Review conditions

Review this decision before adding any non-owner visibility, a Session delete
or archive lifecycle, server-side title mutation, or searchable Session
content.

## Related documents

- [`ADR-018-multi-turn-real-analysis-boundaries.md`](ADR-018-multi-turn-real-analysis-boundaries.md)
- [`../implementation/session_history_workbench_execution_plan.md`](../implementation/session_history_workbench_execution_plan.md)

