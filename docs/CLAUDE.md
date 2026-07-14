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
| `implementation/basic_mock_skeleton_execution_plan.md` | Completed, Historical | Record of the finished deterministic Mock slice; not an active task entry point. |
| `implementation/basic_mock_remediation_plan.md` | Completed evidence | Historical remediation and verification record. |
| `implementation/chart_trio_ui_fit_plan.md` | Completed evidence | Historical chart-layout improvement record. |
| `implementation/current_codebase_overview.md`, `implementation/current_code_tree.md`, `implementation/basic_mock_progress.md` | Active snapshots / completed evidence | What is implemented now and the verification trail. See `implementation/README.md`. |
| `adr/` | Active | Architecture decisions and their review state. |

## Documentation maintenance rules

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
