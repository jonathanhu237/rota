# Navigation translation regression

Branch: `temvia-rota-rebuild`; starting commit: `c1e2476`.

## User-reported defect

The live demo sidebar rendered i18next's `key 'dashboard (zh-CN)' returned an object instead of string` diagnostics instead of six Rota navigation labels. Both languages were affected: merging business resource objects into `common` replaced same-named template strings (`dashboard`, `availability`, `roster`, `requests`, `leaves`, `attendance`). A shell-level `as any` bypassed translation key type checking. Existing English regular-expression assertions matched the diagnostic's key name, so they falsely passed.

## Minimal repair

- Put the six bilingual labels under `common:rotaNavigation.*`, leaving the existing business page resource objects intact.
- Use those keys for both visible menu labels and tooltips; remove the shell's `as any` cast.
- Assert exact accessible link names in both languages against the rendered shell. Harden the existing logout test's navigation assertions from regular expressions to exact string names.
- Add exact English/Chinese visible-navigation assertions to the real business browser suite. No permissions, routes, scheduling rules, layout or user data changed.

## Verification

All substantive checks ran on Centaurus in `/home/jonathanhu237/rota-temvia-review-final` using mise Node 24.20.0:

- Before the fix, the hardened shell suite failed **3/3 tests**, reproducing the actual defect.
- After the fix: `pnpm check`, `pnpm test` (**35 files / 188 tests**), `pnpm lint` (**8 warnings / 0 errors**) and `pnpm build` passed. The existing large-chunk advisory remains.
- `scripts/test-business-browser.sh` on a new disposable project with ports 27873/27880/27832/27825 passed **4/4 tests in 37.4s**, with no skips and exact bilingual menu assertions. The runner cleaned up only its own resources.
- Synced the fixed admin source to the independent live demo and rebuilt/recreated **only its admin container**. No database reset or API restart. A separate Chromium check logged into the deployed demo, verified all six exact Chinese labels, and captured `/tmp/rota-nav-demo.png`; the screenshot was visually inspected locally.
- `git diff --check` passed.

Logs: `/tmp/rota-nav-red.log`, `/tmp/rota-nav-check.log`, `/tmp/rota-nav-browser.log`, `/tmp/rota-nav-demo-build.log` on Centaurus. Demo remains at http://127.0.0.1:27773 via the existing SSH tunnel; refreshing loads the corrected assets.
