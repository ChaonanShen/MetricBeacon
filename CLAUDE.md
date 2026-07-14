# Claude entry point

This file intentionally does not duplicate repository instructions.

1. Read [`AGENTS.md`](AGENTS.md) for the authoritative engineering rules and
   active delivery boundary.
2. Read [`docs/CLAUDE.md`](docs/CLAUDE.md) before planning or changing
   documentation; it owns the document map, statuses, and conflict-routing
   rules.
3. Read the documents routed from `docs/CLAUDE.md` for the task at hand, then
   inspect the actual code before making changes.
4. Treat the matching evolution record as part of the change: implementation,
   plan/progress, current-code snapshot, contract, and ADR must stay aligned as
   required by `AGENTS.md` and `docs/CLAUDE.md`.

If this file and `AGENTS.md` ever disagree, `AGENTS.md` wins. If a task is
inside a directory that later gains its own `AGENTS.md`, the nearest applicable
file wins for that directory.
