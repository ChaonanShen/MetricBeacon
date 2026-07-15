---
kind: knowledge
id: order-service
version: "1"
title: Order service facts and healthy state
serviceRef: order-demo
healthyState:
  workerConcurrency: 2
  queueTrend: stable_or_decreasing
  businessProbe: completed_within_5s
---
# Order service facts

An accepted order enters a bounded in-memory queue. A worker moves it through queued, processing, and completed or failed. Queue and worker metrics are derived from these real states; no metric is a fault switch.

Healthy operation has configured and effective worker concurrency 2, active workers 2, a stable or decreasing queue, and completed orders continuing to increase. A fixed business probe must be accepted and complete within five seconds.

Backlog is a symptom, not a cause. A stopped worker, slow processing, or dependency errors can all increase queue depth and oldest queued age. Runtime epoch, configuration version, worker state, recent outcomes, metrics, and the business probe are distinct evidence sources.
