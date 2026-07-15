# Stable design documents

This directory contains the documents that explain the product's intent and
long-term architecture. They are reference material, not a task queue.

Reading relationship:

```text
original_task -> product_design_final -> arch_design_draft -> arch_design_detail
```

- `original_task.md`: original problem and long-term ambition.
- `product_design_final.md`: current product baseline for scope, user value,
  feature boundaries, product behavior, and acceptance direction.
- `product_design.md`: superseded MS1-MS4 milestone design retained as
  historical decision evidence.
- `arch_design_draft.md`: long-term architecture proposal.
- `arch_design_detail.md`: detailed proposals; only explicitly decided parts
  should constrain implementation.

Product-level topology in the current baseline describes user-facing
responsibilities. ADRs, the implementation blueprint, and actual code remain
authoritative for implemented service boundaries, ownership, and contracts.

Changes here should be deliberate and explain a product or architectural
decision. Day-to-day code plans and implementation evidence belong in
[`../implementation/README.md`](../implementation/README.md).
