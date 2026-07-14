# Implementation documents

This directory contains the mutable documents used to build and verify code.
They may change substantially as the repository evolves.

| Document kind | Role |
|---|---|
| `code_skeleton_design.md` | Current structural blueprint: contracts, Ports, ownership, and acceptance model. |
| `*_plan.md` | A roadmap or self-contained implementation plan. A `draft-review` roadmap does not authorize code changes; an executable plan is `active`, and completed plans remain as evidence. |
| `*_progress.md` | Gate-by-gate verification record for a plan. |
| `current_codebase_overview.md`, `current_code_tree.md` | Current factual snapshot of code and runnable behavior. |
| `real_backend_test_matrix.md` | Current code-agent runbook for layered backend tests, expected result shapes, and failure localization. |

Before beginning a new multi-step slice, create or promote one `*_plan.md`
here with a scope, non-goals, sequence, and acceptance checks, and mark it
`active`. Keep it in this directory after completion and change its status;
paths stay stable for links and history.

The completed detailed plan is
[`node_exporter_real_analysis_execution_plan.md`](node_exporter_real_analysis_execution_plan.md);
its completion verification record is
[`node_exporter_real_analysis_progress.md`](node_exporter_real_analysis_progress.md).

The completed read-only UI slice is
[`three_pane_workbench_execution_plan.md`](three_pane_workbench_execution_plan.md).
Its completion evidence is in
[`three_pane_workbench_progress.md`](three_pane_workbench_progress.md).

The completed layered-result diagnostic slice is
[`layered_result_diagnostics_execution_plan.md`](layered_result_diagnostics_execution_plan.md),
with evidence in
[`layered_result_diagnostics_progress.md`](layered_result_diagnostics_progress.md).

The completed bounded-query slice is
[`bounded_node_exporter_query_parameters_execution_plan.md`](bounded_node_exporter_query_parameters_execution_plan.md),
with gate evidence in
[`bounded_node_exporter_query_parameters_progress.md`](bounded_node_exporter_query_parameters_progress.md).

The completed natural-language input refinement is
[`natural_language_query_input_execution_plan.md`](natural_language_query_input_execution_plan.md),
with gate evidence in
[`natural_language_query_input_progress.md`](natural_language_query_input_progress.md).

The completed grouped chart-canvas refinement is
[`grouped_chart_canvas_execution_plan.md`](grouped_chart_canvas_execution_plan.md),
with future gate evidence in
[`grouped_chart_canvas_progress.md`](grouped_chart_canvas_progress.md).

The related stable product and architecture material is in
[`../design/README.md`](../design/README.md); ADRs remain in [`../adr/`](../adr/).
