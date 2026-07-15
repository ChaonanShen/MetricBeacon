---
kind: skill
id: diagnose-order-backlog
version: "1"
title: Diagnose an order queue backlog
serviceRef: order-demo
allowedCapabilities:
  - order_service.get_runtime
  - order_service.get_queue_snapshot
  - order_service.get_worker_state
  - order_service.get_worker_policy
  - order_service.get_recent_outcomes
  - order_service.get_operation
---
# Diagnostic method

Establish runtime identity first, then compare queue health, worker state, and the pinned worker policy. Use recent outcomes only to distinguish dependency failures from slow or stopped processing. Do not infer a repair solely from an alert or one metric.

Classify the primary hypothesis as one of: stopped worker, slow processing, dependency errors, already recovered, or insufficient evidence. Preserve evidence references and at least one plausible alternative when evidence permits.

Recommend `restore_worker_concurrency` only when configured concurrency, effective concurrency, and active workers are all zero in the same fresh observation, and policy says the healthy value is 2. If workers are active, configuration is already healthy, epochs differ, observations are stale, or dependency failures explain the symptom, choose `no_action`. Never request shell, arbitrary HTTP, PromQL, fault injection, or a generic execute capability.
