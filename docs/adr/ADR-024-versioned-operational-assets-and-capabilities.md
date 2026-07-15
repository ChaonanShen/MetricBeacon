# ADR-024: Versioned operational assets and bounded capabilities

> Status: Accepted
> Date: 2026-07-16

## Context

The existing assistant-mcp exposes three bounded node_exporter-oriented
Prometheus tools. Incident diagnosis needs business facts, diagnostic guidance,
resumable execution steps and business-system observations without giving the
model raw PromQL, arbitrary HTTP or write capabilities.

## Decision

- assistant-mcp owns deployment-provided Knowledge, Skill, Playbook and alert
  mapping assets through a read-only filesystem Adapter.
- Knowledge contains stable business facts and healthy state; Skill contains
  diagnostic method; Playbook contains versioned resumable steps and pause
  points. Authorization policy remains executable configuration/code.
- Assets are schema-validated at startup and addressed by ID, version and
  SHA-256 digest. A Task pins those values for its lifetime.
- Alert-to-Playbook resolution is an exact match over a trusted source ID,
  alert name and required stable labels. Zero or multiple matches fail closed.
- Playbooks can reference only registered typed capability IDs. They cannot
  contain scripts, URLs, raw queries, dynamic tool names or arbitrary argument
  forwarding.
- The Incident Agent receives only Playbook-allowlisted read-only tools and
  bounded summaries. Intent preparation and every write remain deterministic
  application operations outside the model toolset.
- Existing registered metric views expand from the node_exporter slice to
  explicit order-service views; callers still cannot supply PromQL.

## Consequences

This ADR extends the service scope recorded in ADR-018, ADR-019 and ADR-021
without weakening their view-only model boundary. Mock and real capability
Adapters must satisfy the same contract. Asset management, semantic retrieval
and online editing remain out of scope.

## Alternatives considered

- Put business facts in the AI Core system prompt: rejected because it creates
  a second asset owner and makes deployment facts inseparable from orchestration.
- Let the model select arbitrary MCP tools or PromQL: rejected because the
  result is not locally bounded or auditable.
- Encode authorization in natural-language Playbooks: rejected because prose
  cannot be the enforcement boundary.

## Review conditions

Review before online asset mutation, semantic retrieval, nested Playbooks,
multi-MCP discovery, or any model-visible write capability.

## Related documents

- [`ADR-018-multi-turn-real-analysis-boundaries.md`](ADR-018-multi-turn-real-analysis-boundaries.md)
- [`ADR-019-bounded-node-exporter-query-parameters.md`](ADR-019-bounded-node-exporter-query-parameters.md)
- [`ADR-021-agent-planned-bounded-query-intent.md`](ADR-021-agent-planned-bounded-query-intent.md)
- [`../implementation/order_service_incident_remediation_execution_plan.md`](../implementation/order_service_incident_remediation_execution_plan.md)

