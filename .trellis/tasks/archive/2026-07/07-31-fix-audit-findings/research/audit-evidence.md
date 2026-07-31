# Audit evidence

## Test baseline

- Backend passed `go build ./...`, `go vet ./...`, `go test -count=1 ./...`,
  `go test -race -count=1 ./...`, and integration-tagged tests against isolated
  PostgreSQL with migrations 00001-00021.
- Frontend passed `pnpm lint`, 313 tests across 67 files, and `pnpm build`.
- Excel export returned a valid XLSX archive and correct MIME type.

## Reproduced correctness defects

### Schedule timezone drift

- Browser timezone: Asia/Shanghai.
- Assignment 1 / 2026-08-03 slot: 08:00-10:00.
- Leave UI: 16:00-18:00.
- Backend notification: 08:00-10:00.
- Root boundary:
  - `backend/internal/model/publication.go:96-136` creates occurrence values in
    UTC.
  - `frontend/src/routes/_authenticated/leaves/new.tsx:63-66,267-268`
    formats those values without `timeZone: "UTC"`.

### Duplicate active leaves

The isolated preview database accepted:

```text
leave_id  user_id  requester_assignment_id  occurrence_date  state
1         2        1                        2026-08-03       approved
2         2        1                        2026-08-03       cancelled
```

Both rows were initially pending after two successful UI submissions nine
seconds apart. No existing index owns active leave uniqueness.

### Attendance error masking

`GET /api/publications/4/attendance?date=2026-07-31` returned:

```text
HTTP 409
ATTENDANCE_RESPONSIBLE_REQUIRED
```

The administrator page retried, displayed literal `common.loading`, then
rendered the successful-empty copy. The final branch at
`frontend/src/routes/_authenticated/publications/$publicationId/attendance.tsx:448-454`
does not inspect `dayQuery.isError`.

### Responsive overflow

Measured document overflow (`scrollWidth - clientWidth`):

| Route | Viewport | Overflow |
|---|---:|---:|
| `/users` | 768px | 194px |
| `/templates` | 768px | 44px |
| `/publications` | 768px | 256px |
| `/publications` | 1024px | 256px |

`SidebarInset` at `frontend/src/components/ui/sidebar.tsx:303-313` is the common
flex child and lacks `min-w-0`.

### Localization drift

- Chinese UI retained `<html lang="en">`.
- DatePicker rendered `Jul 31, 2026`.
- Roster rendered `7/27/2026` under Chinese UI.
- Preference saved toast used the language active before the mutation.
- Sidebar developer snapshot exposed `Toggle Sidebar`.

## Vulnerability baseline

`govulncheck` v1.6.0 with the 2026-07-27 database found four reachable issues:

- GO-2026-5960 — Excelize 2.10.1; fixed in 2.11.0.
- GO-2026-5856 — Go `crypto/tls`; fixed in 1.26.5.
- GO-2026-5039 — Go `net/textproto`; fixed in 1.26.4.
- GO-2026-5037 — Go `crypto/x509`; fixed in 1.26.4.

## Scope decision

This task intentionally excludes the contradictory stress-seed requirement
(`ACTIVE` plus pending regular requests) and bundle splitting. Those are
independently reviewable follow-ups and are not required to close the approved
release-blocker/UI-remediation scope.
