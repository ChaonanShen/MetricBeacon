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
[`current_codebase_overview.md`](current_codebase_overview.md) and
[`current_code_tree.md`](current_code_tree.md).

The next agreed implementation direction is:

```text
documentation governance
  -> persistent multi-turn Session/Message workbench
  -> real Prometheus + node_exporter adapter
  -> static node_exporter Agent profile document
  -> minimal Eino-backed Agent
  -> real end-to-end demonstration
```

Skill, Playbook, Dashboard write, and alert integration remain outside this
next slice. A dedicated execution plan must be added before implementation of
the multi-turn/real-Agent slice begins.

## Reading and conflict rules

| Question | Read first | Authority when documents disagree |
|---|---|---|
| Product scope and milestone | `product_design.md` | Product design |
| Existing code, runnable behavior, and test evidence | `current_codebase_overview.md`, `current_code_tree.md`, then code/tests | Actual code and verification evidence |
| Module boundaries, ports, data ownership, contracts, and acceptance design | `code_skeleton_design.md` | Code skeleton design |
| A decision explicitly marked as decided | relevant section of `arch_design_detail.md` or an ADR | ADR first; otherwise the explicit detailed decision |
| Grafana delegation grant | `adr/ADR-017-grafana-delegation-grant.md` | ADR-017; it is still Provisional |
| A finished historical implementation | its completed plan plus `development/basic_mock_progress.md` | Historical record only; do not reactivate it |

If the disagreement changes product scope, module ownership, permissions, or an
irreversible data structure, do not silently merge the two interpretations.
Surface the conflict and obtain a decision or add an ADR.

## Document map

| Document | Status | Use |
|---|---|---|
| `original_task.md` | Reference | Project origin and long-term intent; not current implementation scope. |
| `product_design.md` | Active, Draft | Product scope, milestones, and explicit non-goals. |
| `arch_design_draft.md` | Reference, Draft | Long-term six-layer architecture context. |
| `arch_design_detail.md` | Reference, Draft | Detailed proposals; use only its explicitly decided parts without expanding milestones. |
| `code_skeleton_design.md` | Active, Implementation Blueprint | Structural authority for code, contracts, Ports/Adapters, ownership, and verification. |
| `basic_mock_skeleton_execution_plan.md` | Completed, Historical | Record of the finished deterministic Mock slice; not an active task entry point. |
| `current_codebase_overview.md` | Active snapshot | What currently works, boundaries, commands, and known unimplemented scope. |
| `current_code_tree.md` | Active snapshot | Human-readable code ownership map. |
| `development/basic_mock_progress.md` | Completed evidence | Gate-by-gate verification evidence for the Mock slice. |
| `development/basic_mock_remediation_plan.md` | Completed evidence | Historical remediation and verification record. |
| `development/chart_trio_ui_fit_plan.md` | Completed evidence | Historical chart-layout improvement record. |
| `adr/` | Active | Architecture decisions and their review state. |

## Documentation maintenance rules

- Update `current_codebase_overview.md` and `current_code_tree.md` whenever a
  completed implementation slice materially changes runtime behavior, module
  ownership, or the code tree.
- Create a new self-contained execution plan under `docs/development/` before
  starting a new multi-step implementation phase. Mark the plan `completed`
  when its acceptance gate passes, then link its evidence from this file.
- Keep completed plans and progress files as immutable historical evidence;
  add a status/banner instead of rewriting their technical history.
- Record decisions affecting interfaces, service ownership, security,
  permissions, or persistence through an ADR. Update `adr/README.md` when a
  new ADR is added or superseded.
- Prefer links to authoritative documents over copying their content. A
  document that repeats a rule should explain its local application, not own a
  competing version of the rule.
