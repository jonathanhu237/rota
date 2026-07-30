# Shared Development Guides

These guides cover decisions that span the backend and frontend layer indexes.

| Guide | Read when |
|---|---|
| [Cross-Layer Change Guide](./cross-layer-thinking-guide.md) | A field, state, error, permission, date, or workflow crosses layer boundaries |
| [Code Reuse Guide](./code-reuse-thinking-guide.md) | Adding a helper, component, contract, constant, query key, or repeated behavior |

## Pre-Development Checklist

- Search for the current behavior and all consumers with `rg`.
- Load the matching domain contract under `../domain/` for product behavior.
- Map changed fields and errors from persistence through the UI.
- Decide the single owner of every new rule.
- Identify success and rejection/error tests before implementation.

After debugging reveals a reusable invariant or recurring failure mode, update
the appropriate project spec instead of leaving the lesson only in a chat.

## Quality Check

- Confirm the full cross-layer flow was reviewed for any contract change.
- Search for old/new literals, shared helpers, error codes, query keys, and
  translation keys with `rg`.
- Verify each new rule has one clear owner and consumers do not reimplement it.
- Validate every review finding against the actual source and domain contract
  before requiring a fix.
