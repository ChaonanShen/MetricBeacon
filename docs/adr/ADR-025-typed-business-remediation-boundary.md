# ADR-025: Typed business remediation and fault isolation

> Status: Accepted
> Date: 2026-07-16

## Context

A useful remediation demonstration must change real business behavior while
remaining safe enough to repeat. A generic execute endpoint would make approval
cosmetic, and exposing fault ground truth to the Agent would reduce diagnosis
to toggling a known switch.

## Decision

- The order demonstration exposes separate Business, Operational and Fault
  Injection interfaces. Fault control is test-only, network-isolated and never
  registered as an Agent or Playbook capability.
- Remediation uses a typed worker-concurrency operation with an operation ID,
  instance epoch, expected version, expected old value, fixed new value,
  Intent digest and Approval reference.
- AI Core persists an immutable Intent/Diff and Approval. After approval it
  issues a short-lived signed ApprovalEvidence bound to the exact operation.
- assistant-mcp verifies ApprovalEvidence and uses a distinct remediation
  credential. The order service independently enforces the typed operation,
  compare-and-swap preconditions and idempotency.
- AI Core TaskEvents, an authoritative AI Core audit, assistant-mcp append-only
  execution audit and the order-service operation receipt share correlation
  identifiers. TaskEvents are not treated as the audit store.
- Success requires runtime state, registered Prometheus views and a bounded
  real business probe. A successful configuration write alone is insufficient.
- No shell, raw command, arbitrary HTTP, generic execute, raw PromQL or Fault
  Injection capability is introduced.

## Consequences

The first writable action is deliberately narrow: restore worker concurrency
from zero to the pinned healthy value two. AI Core and assistant-mcp share the
ApprovalEvidence verification secret in the local demonstration; the order
service trusts only assistant-mcp's separate remediation credential and repeats
all target precondition checks. Production key distribution is deferred.

## Alternatives considered

- Give the Agent the Fault API: rejected because it reveals ground truth and
  bypasses diagnostic evidence.
- Approve a natural-language instruction: rejected because the approved object
  would not bind an exact side effect.
- Treat a successful API response as recovery: rejected because it does not
  prove customer-visible service restoration.

## Review conditions

Review before a second write action, automatic approval, production key
management, cross-service remediation, rollback automation or durable business
state in the demonstration service.

## Related documents

- [`ADR-017-grafana-delegation-grant.md`](ADR-017-grafana-delegation-grant.md)
- [`../implementation/order_service_incident_remediation_execution_plan.md`](../implementation/order_service_incident_remediation_execution_plan.md)

