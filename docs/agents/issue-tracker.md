# Issue tracker: Local Markdown

Issues and specs live as Markdown files in `.scratch/`.

## Conventions

- One feature per directory: `.scratch/<feature-slug>/`.
- Spec: `.scratch/<feature-slug>/spec.md`.
- Implementation tickets: `issues/<NN>-<slug>.md` within that directory,
  numbered from `01`; never combine all tickets into one file.
- Record each ticket's state in a `Status:` line near the top.
- Append comments and conversation history under `## Comments`.

## Publishing and fetching

When a skill says "publish to the issue tracker", create the appropriate
spec or ticket file, creating directories as needed.

When a skill says "fetch the relevant ticket", read the referenced file.
Resolve a ticket number within its feature directory; ask if ambiguous.

## Wayfinding operations

Used by `/wayfinder`.

- Map: `.scratch/<effort>/map.md`, containing Notes, Decisions-so-far,
  and Fog.
- Child ticket: `.scratch/<effort>/issues/<NN>-<slug>.md`.
- Type: `research`, `prototype`, `grilling`, or `task`.
- Status: `open`, `claimed`, or `resolved`.
- Blocking: record `Blocked by: NN, NN` near the top.
  A ticket is unblocked when all referenced tickets are resolved.
- Frontier: select the first open, unblocked, unclaimed ticket by number.
- Claim: save `Status: claimed` before starting work.
- Resolve: append the answer under `## Answer`, set `Status: resolved`,
  then add a summary and relative link to the map's Decisions-so-far.
