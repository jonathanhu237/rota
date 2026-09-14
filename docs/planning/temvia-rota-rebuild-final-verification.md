# Direct verification after corrective pass 2

Status: VERIFIED — employee-role adjustment approved and implemented; final cumulative review passed. The initial blocked checkpoint below is retained as history; the final checkpoint at the end supersedes it.
Fixed baseline: `290be51287caa3831a3b8a14e196664964a2802d`.
Original scope remains the confirmed discussion and ADR 0004.

## Environment and safe execution

Main agent inspected the cumulative source directly, then synchronized source one-way to the newly created mode-700 Centaurus directory `/home/jonathanhu237/rota-temvia-review-final`. `.env*`, `.git`, dependency/build/test caches, `.scratch`, and `processed.local.csv` were excluded. No existing user environment, data, or external project was changed.

Added reproducible current-source acceptance work:

- `scripts/test-business-browser.sh`: creates a unique disposable Compose project, isolated PostgreSQL volume, first administrator through real setup, and Mailpit; installs no persistent credentials and tears down its own containers/volume/network on exit.
- `scripts/fixtures/business-browser.sql`: test-only employee and business fixtures in that fresh database. **Employee role fixture is currently deliberately blocked/invalid under the template's nonempty-permission requirement; it is not an accepted product role design.**
- `admin/e2e/rota-business.spec.ts`: actual organization settings save/reload/localization/conflict tests, ordinary-employee navigation/read-write denials, real leave/shift mail destinations, and account-switch private-cache acceptance. Business cases remain blocked, not passed.

The application administrator test credential follows the unchanged template composition policy; the isolated database service password remains the project development default. No template password rule was weakened.

## Vulnerability check — resolved on Centaurus

Direct Centaurus HTTPS to `vuln.go.dev` and `proxy.golang.org` timed out. Local HTTPS to both succeeded. A temporary loopback-only HTTPS CONNECT relay allowed only the official public hosts `proxy.golang.org`, `sum.golang.org`, `vuln.go.dev`, and `storage.googleapis.com` on port 443; SSH reverse forwarding made it available only on remote loopback. TLS and Go checksum verification were retained, not disabled. No project source or secrets were sent to the relay targets beyond ordinary dependency/security queries.

Command on Centaurus:

```sh
cd /home/jonathanhu237/rota-temvia-review-final
HTTPS_PROXY=http://127.0.0.1:27791 timeout 300 make vuln-check
```

Result: exit 0 using mise Go 1.27.0 and pinned govulncheck v1.2.0.

```text
=== Symbol Results ===
No vulnerabilities found.
Your code is affected by 0 vulnerabilities.
This scan also found 0 vulnerabilities in packages you import and 4
vulnerabilities in modules you require, but your code doesn't appear to call
these vulnerabilities.
```

This is a real remote result, not reliance on the earlier unauthorized local execution. The four uncalled module findings are not represented as nonexistent. Local evidence copy: `/tmp/rota-review-vuln-final.log`. The temporary relay and SSH forwards were closed after verification.

## Current browser verification and discovered blocker

On Centaurus, dependencies were installed with `mise exec --raw node@24.20.0 -- pnpm install --frozen-lockfile`, then:

```sh
mise exec --raw node@24.20.0 -- bash scripts/test-business-browser.sh
```

Every attempt builds the current API/admin/migration images, applies migrations 1–14 and uses a fresh disposable database. Fixture harness mistakes (secret encoding, initial password composition, Playwright option placement, and an incorrect save-button disabled assumption) were corrected without changing product behavior.

Final observed result:

- Organization identity browser test: **PASS**, covering nonempty organization save/reload, public projection, stale revision rejection, length rejection and Chinese label/value preservation. SMTP settings were saved through the real API.
- Ordinary employee test: **BLOCKED / FAIL at login (503 dependency unavailable)**.
- Subsequent account-switch test: **not run**, because the serial business prerequisite failed.
- Therefore mail link follow-through, actual employee workflows and account-switch browser acceptance are not claimed complete.

### Evidence and decision required

`api/internal/auth/application/access.go:1182–1204` rejects a custom role with zero permissions and a principal with zero roles. Both a roleless employee fixture and an employee assigned an empty custom role reproduce login failure. This is the template's intentional integrity rule, not a transient dependency outage.

`api/internal/auth/domain/access.go` currently exposes only `rota.read` and `rota.manage` for the business extension; both expose administrative read models or management writes. Assigning either, or an unrelated common administrative permission, merely to make an ordinary employee authenticate would expand the baseline employee's authority. Weakening the template's nonempty-role invariant would instead contradict the chosen template ownership unless explicitly approved.

Recommended decision (NOT IMPLEMENTED): add a catalog entry such as `rota.self` for a valid self-service-only employee role, with no administrative grants. Preserve the existing authenticated self-service ownership/qualification rules and do not silently require this new key from existing valid roles. This implements an employee role without granting access to management read models. The user must confirm this mapping before product authorization changes.

## Cleanup and next action

All `rota-business-*` projects created by this direct acceptance harness were torn down, including their newly created test volumes and networks; a final `docker ps` query found none running. Other existing Centaurus containers were not touched. The isolated source checkout and remote test logs remain for reproduction; no user data was imported. Git diff whitespace check passes. Work is uncommitted.

At this checkpoint the main agent awaited the employee mapping before further implementation. This block was subsequently resolved as recorded below.

## Final checkpoint — employee mapping approved; direct verification passed

The user authorized adjustment and explicitly confirmed that ordinary employees may submit their own availability. The implemented catalog key is `rota.self` (Employee self-service): it forms a valid nonempty template role without administrative permissions. Existing authenticated self-service/ownership/qualification rules remain unchanged; no existing valid role is newly denied self-service. The role checkbox has English and Chinese labels. Both READ and MANAGE capabilities remain independent.

Main-agent direct changes and evidence:

- `api/internal/auth/domain/access_test.go::TestEmployeeRoleHasSelfServiceWithoutAdministrativeGrants` first **failed** on Centaurus with the missing catalog entry, then **passed** after adding `PermissionRotaSelf`. Empty permissions remain rejected; no administrative grants are implied.
- `api/internal/rota/httpapi/router_test.go::TestRouterAllowsEmployeeSelfServiceButDeniesManagementReads` verifies own submission binding even when an extra user_id is supplied, and management-read rejection. `admin/src/features/rota/queries.test.ts` verifies `rota.self` is not mapped to administrative capabilities.
- The fixture now assigns a real nonempty `rota.self` role, rather than roleless/empty-role identities. Both employee accounts authenticate through the actual template password/session/RBAC path.
- `scripts/test-business-browser.sh` passed **4/4 Chromium tests, 0 skipped** on the final synchronized source. It builds API/admin/migration images and applies migrations 1–14 to a fresh disposable database. Covered: organization name save/reload/public projection/length and revision rejection/Chinese UI; employee navigation and management HTTP denial; active leave creation and duplicate rejection; Chinese Mailpit notification link opening the correct leave; logout/login B with failed refetch not rendering A's cached detail; own availability submit/read/delete; refusal to alter another employee's availability; real collection-to-assignment time transition; management assignment/publish followed by a valid pre-activation shift-change request; English Mailpit notification link opening `/rota/requests`. The active-publication direct-shift rejection is explicitly preserved, not changed to make the test pass.
- `scripts/test-integration.sh` now uses an explicit empty env file and isolated host settings rather than reading the user's application `.env`; defaults to a unique project; refuses existing project containers/volumes before any destructive fixtures or cleanup. A remote marker-volume rejection test passed and proved the marker was preserved; the marker was then removed by its creator.
- README files now describe Rota, the employee-role setup and deferred legacy data migration, and no longer instruct overwriting existing `.env`.

Final commands on Centaurus (`/home/jonathanhu237/rota-temvia-review-final`):

```sh
mise exec --raw go@1.27.0 node@24.20.0 -- make test
POSTGRES_HOST_PORT=27832 mise exec --raw go@1.27.0 -- bash scripts/test-integration.sh
HTTPS_PROXY=http://127.0.0.1:27791 timeout 180 make vuln-check
mise exec --raw node@24.20.0 -- bash scripts/test-business-browser.sh
```

Results:

- Go vet/build/unit: PASS.
- Serialized real PostgreSQL integration suite, migration 14: PASS, all packages.
- Admin lint: PASS (8 existing warnings, 0 errors); typecheck: PASS; unit tests: **35 files / 186 tests PASS**; production build: PASS (existing large-chunk advisory retained).
- Pinned govulncheck on Centaurus after the employee change: **PASS, 0 reachable vulnerabilities**, 0 vulnerable imported packages, 4 uncalled module vulnerabilities. Uses the same TLS-preserving, allowlisted temporary official-service relay described above; no checksum or security check disabled.
- Final current-source business browser suite: **4 passed in 37.8s**, no skips. The earlier r4 auth suite and unchanged-migration roundtrip remain separate evidence, not substitutes for these tests.
- Local `gofmt` on edited Go source, `bash -n` on both runners, `git diff --check`: PASS. No resource-intensive local checks were used by the main agent.

Remote log paths: `/tmp/rota-review-full.log`, `/tmp/rota-review-integration.log`, `/tmp/rota-review-vuln.log`, `/tmp/rota-review-business.log`, `/tmp/rota-review-isolation.log`. The business/API/Mailpit services were forwarded on loopback for validation. Every newly created acceptance/integration project was cleaned up by the owning runner; no existing user data or unrelated project was changed.

Third cumulative direct review conclusion: **Standards PASS (0 open findings), Spec PASS (0 open findings)** against the original fixed baseline, approved migration requirements and user-confirmed employee-role clarification. No delegated fourth review or third Luna fix attempt was used. Ready for the task-only Conventional Commit and user acceptance.
