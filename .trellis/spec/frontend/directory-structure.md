# Frontend Directory Structure

## Layout

```text
frontend/src/
├── components/
│   ├── ui/                 # shadcn primitives
│   └── <feature>/          # feature components, schemas, helpers, tests
├── hooks/                  # genuinely shared React hooks
├── i18n/
│   ├── index.ts
│   └── locales/{en,zh}.json
├── lib/
│   ├── axios.ts            # configured API client
│   ├── queries.ts          # shared API functions and query options
│   ├── types.ts            # shared API/domain DTO types
│   └── api-error.ts        # API error decoding and translation
├── routes/                 # TanStack Router file-based routes
├── test-utils/             # shared test providers/mocks
├── main.tsx
└── routeTree.gen.ts        # generated; do not hand-edit
```

## Placement Rules

- Route files own routing, loaders/guards, and page-level composition.
- Reusable feature UI lives under `components/<feature>/`.
- Keep a feature's schemas, pure helpers, and tests beside its components, as
  demonstrated by `components/settings/`, `components/templates/`, and
  `components/assignments/`.
- Shared API calls and query option factories stay in `lib/queries.ts` until a
  deliberate module split is warranted. Shared response/domain types stay in
  `lib/types.ts`.
- Low-level shadcn components stay in `components/ui/`; product behavior does
  not belong there.
- Translation keys are added to both locale JSON files in the same change.

## Naming

- Files use lower-case kebab case: `publication-state-badge.tsx`.
- React components and exported types use PascalCase.
- Hooks start with `use`.
- Tests use `*.test.ts` or `*.test.tsx` beside the source.
- TanStack route support files prefixed with `-` are not routes; keep the
  existing router naming convention.

## Examples

- Authenticated layout and route guard:
  `src/routes/_authenticated.tsx`.
- Form feature module: `src/components/settings/`.
- Complex stateful feature broken into pure helpers and components:
  `src/components/assignments/`.
- Provider-aware test rendering: `src/test-utils/render.tsx`.

## Avoid

- Editing `routeTree.gen.ts` by hand.
- Moving product-specific logic into `components/ui`.
- A new global `utils` file when the helper has a clear feature owner.
- Defining API response types independently inside multiple route components.
