# Domain Docs

## Layout

This repo uses a single-context layout:

- `CONTEXT.md` at the repository root: domain vocabulary and context.
- `docs/adr/`: architectural decisions shared by backend and frontend.

## Before exploring

Read root `CONTEXT.md` and ADRs relevant to the area being changed.

If a document does not exist, proceed silently. Do not suggest creating
it upfront. `/domain-modeling`, also reached via `/grill-with-docs`
and `/improve-codebase-architecture`, creates domain documents lazily
when terms or decisions are resolved.

## Vocabulary

Use the terms defined in `CONTEXT.md` when naming domain concepts in
issues, proposals, hypotheses, and tests. Avoid rejected synonyms.

If a concept is missing, reconsider whether it belongs to the domain
or note the gap for `/domain-modeling`.

## ADR conflicts

Explicitly identify any proposal that contradicts an existing ADR,
including the ADR reference and the reason to reconsider its decision.
