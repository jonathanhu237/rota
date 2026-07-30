# Type Safety

TypeScript is strict and the production build runs `tsc -b`. Keep the API
boundary explicit even though Axios does not perform runtime response
validation.

## Type Ownership

- Shared backend response/domain shapes live in `src/lib/types.ts`.
- Request and response wrapper types used by API calls live near those calls in
  `src/lib/queries.ts`.
- Component-only props and view models stay beside their component.
- Form values are inferred from Zod schemas where practical.
- Stable API error codes are enumerated in `src/lib/api-error.ts`.

The backend JSON contract uses snake-case fields; frontend API types match it
directly. Do not silently camel-case one endpoint while the rest of the API
remains snake case.

## Runtime Validation

Import Zod only through:

```ts
import { z } from "zod/v3"
```

Use translated schema factories for user input and `z.infer` for form value
types. Put non-trivial normalization in the schema or a shared boundary helper
and test Unicode, empty, limit, and invalid-enum cases as appropriate.

## Narrowing

Treat caught errors and untrusted input as `unknown`. Use library guards such
as `isAxiosError`, Zod parsing, or a purpose-built type guard before reading
fields. Prefer discriminated unions for states such as publication and
shift-change status.

## Avoid

- `any` when `unknown` plus narrowing is possible.
- Broad `as` assertions used to bypass an API mismatch.
- Importing from `zod` instead of `zod/v3`.
- Re-declaring the same response shape in multiple components.
- Adding a backend response field without updating the shared frontend type and
  the consumers/tests in the same change.
