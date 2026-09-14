# Rota navigation simplification

Status: implementation verified; main-agent Standards and Spec review round 1 passed. Ready for the task-only local commit and user acceptance.

Branch: `rota-navigation-simplification`.
Fixed implementation/review baseline: `fb81ada1c26182f4201a3fe56ef87bde742de0a2`.
Review scope: cumulative tracked and new task changes against this baseline; maximum three direct main-agent review rounds.

## Confirmed decisions

- Move Personal settings out of the main sidebar and into the bottom avatar dropdown alongside Logout. Preserve the existing settings page and its functionality.
- The user will adjust the upstream template separately; this effort does not modify or regenerate that template.
- Remove the Rota Dashboard page and its navigation entry, including its overview cards and management shortcuts. Do not merge those cards into Home or introduce replacement management navigation as part of this change: the user explicitly requested direct removal and said the management destinations can be accessed in other ways.
- Preserve the underlying business pages and capabilities, including positions, templates, publications, availability, rosters, requests, leave and attendance. Removing Dashboard does not authorize deleting the `/rota/*` route subtree or changing permissions.
- Keep Home as the sole home entry; no home-page redesign is requested.

## Observed current behavior

Read-only inspection found Home is a welcome-only page. Dashboard contains publication status/actions, unread-request and recent-leave summaries, and management shortcuts. Positions has no other incoming application navigation link identified; templates/publications have internal links but no sidebar entries. The user was informed of the management-entry concern and chose direct removal rather than adding replacement entries. This records a deliberate discoverability trade-off, not proof that equivalent menus already exist.

The retained `create-temvia@0.5.0` generated baseline also places Personal settings after Home and has a logout-only footer menu. Bottom placement is the user's desired adjustment, not a claim about that pinned template's existing behavior.

## Confirmed removed-address behavior

The user explicitly rejected compatibility work for the removed `/rota` dashboard. Delete its index page without adding a redirect or a replacement landing page. Keep any routing parent required by the surviving `/rota/*` business pages; removing the dashboard does not mean deleting that subtree.

## Acceptance criteria for subsequent implementation

- Personal settings is accessible from the avatar dropdown, absent from the main sidebar, and still opens the same functional page.
- Only Home remains as the home/dashboard navigation entry; Dashboard overview and shortcut UI is removed without adding substitute management menus.
- Business routes and effective permissions remain unchanged; no data is removed.
- The dashboard index page is removed, with no compatibility redirect or replacement landing page; routing infrastructure required by surviving business pages remains.
- Navigation tests cover exact English/Chinese labels, settings access, absence of Dashboard, and landing-route behavior.

## Implementation and verification checklist

- [x] Read current admin conventions; inventory dashboard-only code and shared dependencies before deletion.
- [x] Move Personal settings to the avatar dropdown, preserving keyboard access and existing settings route/functionality.
- [x] Remove Dashboard index/menu/cards and exclusively unused code; preserve shared business components, routes and permissions. Do not add management menus or alter Home.
- [x] Update generated routing through the project tooling; add exact bilingual navigation/dropdown tests and rejection/absence checks.
- [x] Synchronize local source one-way to an isolated Centaurus checkout, preserving `.env`, databases, current demo and unrelated services. Run required checks and real browser acceptance there; no resource-intensive local fallback.
- [x] Main agent reviews Standards and Spec directly against the fixed baseline; round 1 passed, no corrective attempts needed.
- [x] Record exact verification and review outcomes; only complete, verified task changes are included in the local Conventional Commit.

## Implementation verification record

Implementation completed locally on branch `rota-navigation-simplification` against baseline `fb81ada1c26182f4201a3fe56ef87bde742de0a2`. The dashboard index route, four dashboard-only cards, dashboard-only Rota translations, and Dashboard sidebar entry were removed. Personal settings now remains at `/personal-settings` and is exposed as a keyboard-operable link in the authenticated avatar dropdown; Home and all existing Rota business routes remain unchanged.

Source was synchronized one-way to the isolated Centaurus checkout `/home/jonathanhu237/rota-navigation-simplification-20260914-r1` with `.git`, `.env`, dependencies, and build outputs excluded. The live demo and unrelated projects were not touched.

- Generated routing: `mise exec --raw node@24.20.0 -- pnpm exec vite build` regenerated `admin/src/routeTree.gen.ts`; the generated tree retains the `/rota` parent for subroutes but no `/_authenticated/rota/` index route.
- Targeted admin tests: `pnpm exec vitest run src/features/auth/authenticated-shell.test.tsx src/routes/-rota.test.ts` — 2 files, 7 tests passed. Covers exact English/Chinese navigation labels, Dashboard/settings absence from the sidebar, keyboard settings access, retained Rota children, and absent dashboard index.
- Admin checks on Centaurus: `pnpm check` passed; `pnpm test` passed with 35 files and 190 tests; `pnpm lint` passed with the existing 8 warnings and 0 errors; `pnpm build` passed with the existing large-chunk warning.
- Isolated browser acceptance: `E2E_ADMIN_PORT=27873 E2E_API_PORT=27880 E2E_POSTGRES_PORT=27832 E2E_MAILPIT_PORT=27825 mise exec --raw node@24.20.0 -- bash scripts/test-business-browser.sh` — 4 Chromium tests passed, 0 skipped. The run retained employee authorization, availability ownership, mail-link, settings, account-switch, logout/cache, and business-route coverage, and verified exact bilingual navigation, Chinese avatar-dropdown keyboard navigation to the settings page, and that `/rota` remains on the removed parent address without a replacement landing page. The runner cleaned only its disposable Compose project, containers, network, and volume; no matching resources remain.
- API checks: not applicable; no API files, SQL, data, permissions, or environment configuration changed.

No unresolved implementation issues or verification blockers remained at implementation handoff. Standards/spec review was deferred to the main agent and completed below. No push or demo update was performed. Final verification logs are retained in the isolated checkout at `admin/verification/check-final.log`, `admin/verification/targeted-tests-final.log`, `admin/verification/admin-test-final.log`, `admin/verification/admin-lint-final.log`, `admin/verification/admin-build.log`, and `verification/business-browser.log`.

## Direct cumulative review — round 1

Baseline remains `fb81ada1c26182f4201a3fe56ef87bde742de0a2`. Since implementation is uncommitted, review used `git diff <baseline>` plus the new planning document, not an empty commit-only diff. Both review axes were performed directly by the main agent; no reviewer subagents were used.

### Standards — PASS, 0 findings

Checked root/admin conventions, changed runtime code, tests, generated routing, translation deletions and the documented code-smell baseline. The dropdown composes the existing Radix-backed item with a router Link, preserves translation typing and keyboard semantics, and introduces no duplicate navigation implementation. Deleted code is confined to the removed dashboard. No API, schema, configuration, cache-lifecycle or dependency changes. Tooling advisories remain pre-existing and nonblocking.

### Spec — PASS, 0 findings

Personal settings exists only in the avatar dropdown; the original page is unchanged. Dashboard index, menu label, cards and exclusive translations are removed. Home is unchanged, no replacement management menu is introduced, and no compatibility redirect is added. The `/rota` routing parent and business child destinations remain intact; the empty parent outlet is normal routing behavior, not a replacement dashboard. Effective permissions and business email destinations are untouched.

### Independent verification

The main agent checked retained remote logs for 190 passing admin tests, successful lint/build and 4 passing browser tests (37.6s). Checksum-based rsync dry runs found no differences between local and verified remote `admin/src/` or the changed business browser spec. Independently reran the focused navigation/route suites on Centaurus: 2 files, 7 tests passed. JSDOM reports its known unsupported document-navigation message for the mocked anchor; the real browser test verifies actual settings-page navigation. `git diff --check` passed. No fix attempt or further review round was needed.

Do not push or replace the running demo as part of this invocation unless the user requests it. The existing demo remains unchanged.
