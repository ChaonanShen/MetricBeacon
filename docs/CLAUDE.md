# Documentation guide and status

> Last reviewed: 2026-07-14
> Scope: documentation ownership, reading routes, and lifecycle status.

This file is the documentation counterpart to the repository-wide
[`../AGENTS.md`](../AGENTS.md). It is deliberately the only place that keeps a
detailed document map and current-document status. Do not copy this map into
the root `CLAUDE.md` or `AGENTS.md`.

## Current state

The deterministic Mock node_exporter skeleton is complete and verified. The
actual codebase, rather than the old design-only description, is documented in
[`implementation/current_codebase_overview.md`](implementation/current_codebase_overview.md)
and [`implementation/current_code_tree.md`](implementation/current_code_tree.md).

The node_exporter real-analysis slice is complete. Its roadmap, detailed
execution plan, and verification record remain the authoritative historical evidence:

```text
documentation governance
  -> persistent multi-turn Session/Message workbench
  -> real Prometheus + node_exporter adapter
  -> static node_exporter Agent profile document
  -> minimal Eino-backed Agent
  -> real end-to-end demonstration
```

Skill, Playbook, Dashboard write, and alert integration remain outside this
slice. Its proposed scope, safety boundaries, and gate order are recorded in
[`implementation/node_exporter_real_analysis_plan.md`](implementation/node_exporter_real_analysis_plan.md).
[`implementation/node_exporter_real_analysis_execution_plan.md`](implementation/node_exporter_real_analysis_execution_plan.md)
contains the detailed, decision-complete execution sequence and its progress
record is [`implementation/node_exporter_real_analysis_progress.md`](implementation/node_exporter_real_analysis_progress.md).

The completed UI slice is
[`implementation/three_pane_workbench_execution_plan.md`](implementation/three_pane_workbench_execution_plan.md).
It narrows P2 to a read-only three-pane Workbench shell; its execution evidence
is tracked in [`implementation/three_pane_workbench_progress.md`](implementation/three_pane_workbench_progress.md).

The completed diagnostic slice is
[`implementation/real_backend_diagnostics_execution_plan.md`](implementation/real_backend_diagnostics_execution_plan.md).
It adds independent Prometheus, MCP, and DeepSeek probes plus safe recovery when switching between run modes with
independent AI Core volumes. Progress is tracked in
[`implementation/real_backend_diagnostics_progress.md`](implementation/real_backend_diagnostics_progress.md).

The completed layered-result test slice is
[`implementation/layered_result_diagnostics_execution_plan.md`](implementation/layered_result_diagnostics_execution_plan.md).
It adds code-agent-readable semantic checks and safe summaries above the existing connectivity probes. Progress is tracked in
[`implementation/layered_result_diagnostics_progress.md`](implementation/layered_result_diagnostics_progress.md).

The completed bounded-query slice is
[`implementation/bounded_node_exporter_query_parameters_execution_plan.md`](implementation/bounded_node_exporter_query_parameters_execution_plan.md).
It keeps the three-view registry while adding persisted time/resolution parameters, bounded CPU windows, a view-only Agent and locally truthful result summaries. Progress is tracked in
[`implementation/bounded_node_exporter_query_parameters_progress.md`](implementation/bounded_node_exporter_query_parameters_progress.md), and its boundary decision is
[`adr/ADR-019-bounded-node-exporter-query-parameters.md`](adr/ADR-019-bounded-node-exporter-query-parameters.md).

## Reading and conflict rules

| Question | Read first | Authority when documents disagree |
|---|---|---|
| Product scope and milestone | `design/product_design.md` | Product design |
| Existing code, runnable behavior, and test evidence | `implementation/current_codebase_overview.md`, `implementation/current_code_tree.md`, then code/tests | Actual code and verification evidence |
| Module boundaries, ports, data ownership, contracts, and acceptance design | `implementation/code_skeleton_design.md` | Code skeleton design |
| A decision explicitly marked as decided | relevant section of `design/arch_design_detail.md` or an ADR | ADR first; otherwise the explicit detailed decision |
| Grafana delegation grant | `adr/ADR-017-grafana-delegation-grant.md` | ADR-017; it is still Provisional |
| A finished historical implementation | its completed plan plus `implementation/basic_mock_progress.md` | Historical record only; do not reactivate it |

If the disagreement changes product scope, module ownership, permissions, or an
irreversible data structure, do not silently merge the two interpretations.
Surface the conflict and obtain a decision or add an ADR.

## Document map

| Document | Status | Use |
|---|---|---|
| `design/` | Mostly stable | Project origin, product scope, and long-term architecture Proposals. See `design/README.md`. |
| `implementation/code_skeleton_design.md` | Active, Implementation Blueprint | Mutable structural authority for code, contracts, Ports/Adapters, ownership, and verification. |
| `implementation/node_exporter_real_analysis_plan.md` | Completed roadmap | Scope, safety boundaries and gate order for the finished multi-turn, real Prometheus/node_exporter and minimal Eino Agent slice. |
| `implementation/node_exporter_real_analysis_execution_plan.md` | Completed execution plan | Decision-complete implementation sequence and completion evidence for that slice. |
| `implementation/node_exporter_real_analysis_progress.md` | Completed progress record | Gate status and verification evidence for that slice. |
| `implementation/three_pane_workbench_execution_plan.md` | Completed execution plan | Narrow read-only three-pane Workbench UI slice; progress is tracked in `implementation/three_pane_workbench_progress.md`. |
| `implementation/three_pane_workbench_progress.md` | Completed progress record | Gate status and verification evidence for the three-pane Workbench UI slice. |
| `implementation/real_backend_diagnostics_execution_plan.md` | Completed execution plan | Independent real-backend probes and cross-mode stale-Session recovery. |
| `implementation/real_backend_diagnostics_progress.md` | Completed progress record | Gate status and verification evidence for the real-backend diagnostic slice. |
| `implementation/layered_result_diagnostics_execution_plan.md` | Completed execution plan | Layered metric semantics and durable Task result analysis. |
| `implementation/layered_result_diagnostics_progress.md` | Completed progress record | Gate status and verification evidence for the layered-result test slice. |
| `implementation/bounded_node_exporter_query_parameters_execution_plan.md` | Completed execution plan | Bounded query range/resolution, CPU windows, view-only Agent and truthful summaries. |
| `implementation/bounded_node_exporter_query_parameters_progress.md` | Completed progress record | Gate status and verification evidence for the bounded-query slice. |
| `implementation/real_backend_test_matrix.md` | Current runbook | Code-agent command order, expected result shapes and layered failure localization. |
| `implementation/basic_mock_skeleton_execution_plan.md` | Completed, Historical | Record of the finished deterministic Mock slice; not an active task entry point. |
| `implementation/basic_mock_remediation_plan.md` | Completed evidence | Historical remediation and verification record. |
| `implementation/chart_trio_ui_fit_plan.md` | Completed evidence | Historical chart-layout improvement record. |
| `implementation/current_codebase_overview.md`, `implementation/current_code_tree.md`, `implementation/basic_mock_progress.md` | Active snapshots / completed evidence | What is implemented now and the verification trail. See `implementation/README.md`. |
| `adr/` | Active | Architecture decisions and their review state. |

## Documentation maintenance rules

- **Evolution records are a completion requirement.** Update the relevant
  documents in the same focused commit as the code or contract change; do not
  leave a known code-to-document mismatch for a later cleanup.
- Use the following record chain to keep the repository explainable:

  ```text
  design/ explains why and the intended boundary
      -> implementation/code_skeleton_design.md defines the current buildable design
      -> implementation/*_plan.md records the active slice and its acceptance
      -> implementation/*_progress.md records gate evidence
      -> implementation/current_codebase_overview.md and current_code_tree.md
         describe what actually exists
      -> adr/ records decisions that change the boundary
  ```

- For every completed implementation slice, explicitly assess and update as
  applicable: OpenAPI/Schema/fixtures/generated clients, the active plan and
  its progress evidence, current-code overview, code tree, README/runbook,
  and ADR index. Pure refactors still update the code tree when ownership or
  layout changes; runtime behavior changes always update the current-code
  overview and verification commands/results.
- If a planned change invalidates a blueprint or an existing plan, revise the
  document before or together with the code. If it invalidates a stable design
  decision, create or supersede an ADR instead of silently drifting from it.
- Keep `design/` deliberately stable. It explains why the system exists and
  the long-term design direction; do not turn it into a running implementation
  diary.
- Update `implementation/code_skeleton_design.md` when a planned
  code structure, Port, contract boundary, or acceptance model changes. It is
  expected to evolve with implementation.
- Create a new self-contained execution plan directly in `implementation/`
  before starting a multi-step implementation phase. Its header must say
  `status: active`; change that status to `completed` after its acceptance gate
  passes, then link its evidence from this file. Do not move it merely to mark
  completion.
- Update `implementation/current_codebase_overview.md` and
  `implementation/current_code_tree.md` whenever a completed slice
  materially changes runtime behavior, module ownership, or the code tree.
- Keep completed plans and progress files as immutable historical evidence;
  add a status/banner instead of rewriting their technical history.
- Record decisions affecting interfaces, service ownership, security,
  permissions, or persistence through an ADR. Update `adr/README.md` when a
  new ADR is added or superseded.
- Prefer links to authoritative documents over copying their content. A
  document that repeats a rule should explain its local application, not own a
  competing version of the rule.
