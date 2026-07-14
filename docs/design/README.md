# Stable design documents

This directory contains the documents that explain the product's intent and
long-term architecture. They are reference material, not a task queue.

Reading relationship:

```text
original_task -> product_design -> arch_design_draft -> arch_design_detail
```

- `original_task.md`: original problem and long-term ambition.
- `product_design.md`: current product scope and milestones; it wins for
  milestone scope.
- `arch_design_draft.md`: long-term architecture proposal.
- `arch_design_detail.md`: detailed proposals; only explicitly decided parts
  should constrain implementation.

Changes here should be deliberate and explain a product or architectural
decision. Day-to-day code plans and implementation evidence belong in
[`../implementation/README.md`](../implementation/README.md).
