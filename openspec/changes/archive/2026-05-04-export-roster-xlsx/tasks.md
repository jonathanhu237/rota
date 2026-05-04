## 1. Backend Export

- [x] 1.1 Add the XLSX writer dependency and a schedule export renderer that creates one localized workbook sheet with metadata rows, weekday headers, sorted time rows, wrapped multi-line cells, required headcount labels, vacancy lines, and no user emails. Verification: `cd backend && go test ./internal/service`
- [x] 1.2 Add renderer/content tests that open the generated workbook and assert sheet count/name, localized headers, position headcount text, one name per line, vacancy count, and email omission. Verification: `cd backend && go test ./internal/service`
- [x] 1.3 Add a publication service export method that resolves export language, enforces role/state visibility, reads the current baseline assignment snapshot, and excludes occurrence-level overrides. Verification: `cd backend && go test ./internal/service`
- [x] 1.4 Add service tests for admin `ASSIGNING` success, employee `PUBLISHED` success, employee `ASSIGNING` rejection, `DRAFT` rejection, unsupported language rejection, and baseline assignment export despite an occurrence override. Verification: `cd backend && go test ./internal/service`
- [x] 1.5 Add the authenticated `GET /publications/{id}/schedule.xlsx` route and handler logic for path parsing, `lang` parsing, workbook response headers, and existing scheduling error mapping. Verification: `cd backend && go test ./internal/handler`
- [x] 1.6 Add handler tests for successful XLSX response content type/body, invalid language mapping to `INVALID_REQUEST`, and export-invisible publication mapping to `PUBLICATION_NOT_ACTIVE`. Verification: `cd backend && go test ./internal/handler`

## 2. Frontend Download

- [x] 2.1 Add a frontend schedule download helper that requests `/publications/{id}/schedule.xlsx` as a blob with the current UI language. Verification: `cd frontend && pnpm test`
- [x] 2.2 Add a filename helper and tests for localized roster label, client-local `YYYYMMDD-HHmm` timestamp formatting, and replacement of filename-unsafe characters with `-`. Verification: `cd frontend && pnpm test`
- [x] 2.3 Add the admin assignment-board download button with the download icon, pending disabled state, localized text, and destructive toast on failure. Verification: `cd frontend && pnpm test`
- [x] 2.4 Add assignment-board UI tests proving the download control is visible in `ASSIGNING` and calls the helper with the publication id and current language. Verification: `cd frontend && pnpm test`
- [x] 2.5 Add the roster-page download button for visible roster publications, using the same helper and filename behavior. Verification: `cd frontend && pnpm test`
- [x] 2.6 Add roster UI tests proving the download control appears when a roster publication is present and is absent from the empty-roster state. Verification: `cd frontend && pnpm test`

## 3. Validation

- [x] 3.1 Run backend checks cleanly. Verification: `cd backend && go build ./... && go vet ./... && go test ./...`
- [x] 3.2 Run frontend checks cleanly. Verification: `cd frontend && pnpm lint && pnpm test && pnpm build`
- [x] 3.3 Confirm the OpenSpec change is apply-ready and all task boxes are ready for `/opsx:apply`. Verification: `openspec status --change export-roster-xlsx`
