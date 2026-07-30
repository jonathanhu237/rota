# Component Guidelines

## Component Shape

Use named function components. Keep route components focused on data and page
composition; extract independently testable dialogs, forms, tables, and pure
helpers into the matching feature directory.

Props are explicit TypeScript object types. A short one-off prop object may be
typed inline, as in `LeaderRosterRow` in
`src/routes/_authenticated/attendance.tsx`; reusable props get a named type
near the component.

Prefer composition with existing shadcn primitives over duplicating buttons,
dialogs, inputs, cards, tables, or toast behavior.

## Forms

Forms use React Hook Form plus `zodResolver`. Build translated schemas with a
factory and memoize them when they depend on `t`, following
`components/settings/profile-form.tsx` and `settings-schemas.ts`.

- Keep form values in React Hook Form, not parallel `useState`.
- Reset form values when server-owned defaults change.
- Disable submit actions while a mutation is pending.
- Trim/normalize at the schema boundary or immediately before the API call,
  consistently with the existing feature.
- Render validation messages from translated schema messages.

## Styling

Use Tailwind utility classes and the existing design tokens (`background`,
`muted`, `destructive`, and so on). Use `cn` from `src/lib/utils.ts` for
conditional class merging. Modify a shared `components/ui` primitive only when
the behavior truly applies to all consumers.

## Internationalization

Call `useTranslation()` and use translation keys for headings, labels, empty
states, table cells, badges, aria labels, toasts, and errors. Add matching keys
to both `src/i18n/locales/en.json` and `zh.json`.

Dynamic status labels use namespaced keys such as
`t(\`attendance.status.${row.status}\`)`. Do not use an English backend message
as the normal visible label; it is only an error fallback.

## Accessibility

- Associate labels with inputs through `htmlFor`/`id`.
- Give icon-only controls an accessible name.
- Use semantic `button`, `form`, headings, tables, and dialog primitives.
- Preserve keyboard and focus behavior supplied by shadcn/Base UI.
- Express loading and disabled states without relying on color alone.

## Common Mistakes

- Hardcoded user-visible strings.
- Form fields mirrored in local state.
- One large route component containing reusable feature logic.
- Reimplementing a shadcn primitive with ad hoc HTML.
- Updating only one locale.
