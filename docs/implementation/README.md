# Implementation documents

This directory contains the mutable documents used to build and verify code.
They may change substantially as the repository evolves.

| Document kind | Role |
|---|---|
| `code_skeleton_design.md` | Current structural blueprint: contracts, Ports, ownership, and acceptance model. |
| `*_plan.md` | A self-contained implementation plan. The header states whether it is `active` or `completed`; completed plans remain as evidence. |
| `*_progress.md` | Gate-by-gate verification record for a plan. |
| `current_codebase_overview.md`, `current_code_tree.md` | Current factual snapshot of code and runnable behavior. |

Before beginning a new multi-step slice, create one `*_plan.md` here with a
scope, non-goals, sequence, and acceptance checks. Keep it in this directory
after completion and change its status; paths stay stable for links and
history.

The related stable product and architecture material is in
[`../design/README.md`](../design/README.md); ADRs remain in [`../adr/`](../adr/).
