package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/lib/pq"
)

var (
	ErrAttendanceRecordNotFound  = model.ErrAttendanceRecordNotFound
	ErrAttendanceAlreadyRecorded = model.ErrAttendanceAlreadyRecorded
	ErrAttendanceRosterStale     = model.ErrAttendanceRosterStale
)

const attendanceRecordsUniqueKey = "attendance_records_assignment_id_occurrence_date_user_id_key"

type AttendanceRepository struct {
	db *sql.DB
}

type UpsertAttendanceArrivalParams struct {
	PublicationID    int64
	AssignmentID     int64
	OccurrenceDate   time.Time
	UserID           int64
	ArrivedAt        time.Time
	RecordedByUserID int64
	RecordedAt       time.Time
}

type CreateOvertimeRecordParams struct {
	PublicationID    int64
	SlotID           int64
	Weekday          int
	OccurrenceDate   time.Time
	UserID           int64
	Hours            float64
	Note             string
	RecordedByUserID int64
	RecordedAt       time.Time
}

type UpdateOvertimeRecordParams struct {
	PublicationID   int64
	RecordID        int64
	Hours           float64
	Note            string
	UpdatedByUserID int64
	UpdatedAt       time.Time
}

func NewAttendanceRepository(db *sql.DB) *AttendanceRepository {
	return &AttendanceRepository{db: db}
}

func (r *AttendanceRepository) ListLeaderCandidateShifts(
	ctx context.Context,
	publicationID, userID int64,
	fromDate, toDate time.Time,
) ([]model.AttendanceShiftRef, error) {
	const query = `
		WITH occurrence_dates AS (
			SELECT generate_series($3::date, $4::date, INTERVAL '1 day')::date AS occurrence_date
		)
		SELECT DISTINCT
			ts.id,
			tsw.weekday,
			TO_CHAR(ts.start_time, 'HH24:MI'),
			TO_CHAR(ts.end_time, 'HH24:MI'),
			od.occurrence_date
		FROM publications p
		INNER JOIN template_slots ts ON ts.template_id = p.template_id
		INNER JOIN template_slot_weekdays tsw
			ON tsw.slot_id = ts.id
		INNER JOIN occurrence_dates od
			ON tsw.weekday = EXTRACT(ISODOW FROM od.occurrence_date)::integer
		INNER JOIN template_slot_positions tsp
			ON tsp.slot_id = ts.id
			AND tsp.attendance_responsible = TRUE
		INNER JOIN assignments a
			ON a.publication_id = p.id
			AND a.slot_id = ts.id
			AND a.weekday = tsw.weekday
			AND a.position_id = tsp.position_id
		LEFT JOIN assignment_overrides ao
			ON ao.assignment_id = a.id
			AND ao.occurrence_date = od.occurrence_date
		WHERE p.id = $1
			AND COALESCE(ao.user_id, a.user_id) = $2
		ORDER BY od.occurrence_date ASC, TO_CHAR(ts.start_time, 'HH24:MI') ASC, ts.id ASC;
	`

	rows, err := r.db.QueryContext(
		ctx,
		query,
		publicationID,
		userID,
		model.NormalizeOccurrenceDate(fromDate),
		model.NormalizeOccurrenceDate(toDate),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	refs := make([]model.AttendanceShiftRef, 0)
	for rows.Next() {
		var ref model.AttendanceShiftRef
		if err := rows.Scan(
			&ref.SlotID,
			&ref.Weekday,
			&ref.StartTime,
			&ref.EndTime,
			&ref.OccurrenceDate,
		); err != nil {
			return nil, err
		}
		ref.OccurrenceDate = model.NormalizeOccurrenceDate(ref.OccurrenceDate)
		refs = append(refs, ref)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return refs, nil
}

func (r *AttendanceRepository) ListPublicationShiftRefsForDate(
	ctx context.Context,
	publicationID int64,
	occurrenceDate time.Time,
) ([]model.AttendanceShiftRef, error) {
	const query = `
		SELECT
			ts.id,
			tsw.weekday,
			TO_CHAR(ts.start_time, 'HH24:MI'),
			TO_CHAR(ts.end_time, 'HH24:MI'),
			$2::date
		FROM publications p
		INNER JOIN template_slots ts ON ts.template_id = p.template_id
		INNER JOIN template_slot_weekdays tsw ON tsw.slot_id = ts.id
		WHERE p.id = $1
			AND tsw.weekday = EXTRACT(ISODOW FROM $2::date)::integer
		ORDER BY ts.start_time ASC, ts.id ASC;
	`

	rows, err := r.db.QueryContext(ctx, query, publicationID, model.NormalizeOccurrenceDate(occurrenceDate))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	refs := make([]model.AttendanceShiftRef, 0)
	for rows.Next() {
		var ref model.AttendanceShiftRef
		if err := rows.Scan(
			&ref.SlotID,
			&ref.Weekday,
			&ref.StartTime,
			&ref.EndTime,
			&ref.OccurrenceDate,
		); err != nil {
			return nil, err
		}
		ref.OccurrenceDate = model.NormalizeOccurrenceDate(ref.OccurrenceDate)
		refs = append(refs, ref)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return refs, nil
}

func (r *AttendanceRepository) ListShiftRoster(
	ctx context.Context,
	publicationID, slotID int64,
	weekday int,
	occurrenceDate time.Time,
) ([]*model.AttendanceRosterRow, error) {
	const query = `
		SELECT
			a.id,
			a.slot_id,
			a.weekday,
			a.position_id,
			pos.name,
			tsp.attendance_responsible,
			u.id,
			u.name,
			u.email,
			a.created_at,
			ar.id,
			ar.publication_id,
			ar.assignment_id,
			ar.occurrence_date,
			ar.user_id,
			ar.arrived_at,
			ar.recorded_by_user_id,
			ar.recorded_at,
			ar.updated_by_user_id,
			ar.updated_at
		FROM assignments a
		INNER JOIN template_slot_positions tsp
			ON tsp.slot_id = a.slot_id
			AND tsp.position_id = a.position_id
		INNER JOIN positions pos ON pos.id = a.position_id
		LEFT JOIN assignment_overrides ao
			ON ao.assignment_id = a.id
			AND ao.occurrence_date = $4::date
		INNER JOIN users u ON u.id = COALESCE(ao.user_id, a.user_id)
		LEFT JOIN attendance_records ar
			ON ar.assignment_id = a.id
			AND ar.occurrence_date = $4::date
			AND ar.user_id = u.id
		WHERE a.publication_id = $1
			AND a.slot_id = $2
			AND a.weekday = $3
		ORDER BY tsp.attendance_responsible DESC, pos.name ASC, u.name ASC, a.id ASC;
	`

	rows, err := r.db.QueryContext(
		ctx,
		query,
		publicationID,
		slotID,
		weekday,
		model.NormalizeOccurrenceDate(occurrenceDate),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	roster := make([]*model.AttendanceRosterRow, 0)
	for rows.Next() {
		row := &model.AttendanceRosterRow{}
		var record nullableAttendanceRecord
		if err := rows.Scan(
			&row.AssignmentID,
			&row.SlotID,
			&row.Weekday,
			&row.PositionID,
			&row.PositionName,
			&row.AttendanceResponsible,
			&row.UserID,
			&row.UserName,
			&row.UserEmail,
			&row.CreatedAt,
			&record.ID,
			&record.PublicationID,
			&record.AssignmentID,
			&record.OccurrenceDate,
			&record.UserID,
			&record.ArrivedAt,
			&record.RecordedByUserID,
			&record.RecordedAt,
			&record.UpdatedByUserID,
			&record.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if attendanceRecord := record.toModel(row.UserName, row.UserEmail); attendanceRecord != nil {
			row.Record = attendanceRecord
		}
		roster = append(roster, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return roster, nil
}

func (r *AttendanceRepository) ListOrphanArrivalRecords(
	ctx context.Context,
	publicationID, slotID int64,
	weekday int,
	occurrenceDate time.Time,
) ([]*model.AttendanceRecord, error) {
	const query = `
		SELECT
			ar.id,
			ar.publication_id,
			ar.assignment_id,
			ar.occurrence_date,
			ar.user_id,
			ar.arrived_at,
			ar.recorded_by_user_id,
			ar.recorded_at,
			ar.updated_by_user_id,
			ar.updated_at,
			u.name,
			u.email
		FROM attendance_records ar
		INNER JOIN assignments a ON a.id = ar.assignment_id
		INNER JOIN users u ON u.id = ar.user_id
		LEFT JOIN assignment_overrides ao
			ON ao.assignment_id = a.id
			AND ao.occurrence_date = ar.occurrence_date
		WHERE ar.publication_id = $1
			AND a.slot_id = $2
			AND a.weekday = $3
			AND ar.occurrence_date = $4::date
			AND COALESCE(ao.user_id, a.user_id) <> ar.user_id
		ORDER BY u.name ASC, ar.id ASC;
	`

	rows, err := r.db.QueryContext(
		ctx,
		query,
		publicationID,
		slotID,
		weekday,
		model.NormalizeOccurrenceDate(occurrenceDate),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanAttendanceRecords(rows)
}

func (r *AttendanceRepository) ListOvertimeRecords(
	ctx context.Context,
	publicationID, slotID int64,
	weekday int,
	occurrenceDate time.Time,
) ([]*model.AttendanceOvertimeRecord, error) {
	const query = `
		SELECT
			aor.id,
			aor.publication_id,
			aor.slot_id,
			aor.weekday,
			aor.occurrence_date,
			aor.user_id,
			aor.hours,
			aor.note,
			aor.recorded_by_user_id,
			aor.recorded_at,
			aor.updated_by_user_id,
			aor.updated_at,
			u.name,
			u.email
		FROM attendance_overtime_records aor
		INNER JOIN users u ON u.id = aor.user_id
		WHERE aor.publication_id = $1
			AND aor.slot_id = $2
			AND aor.weekday = $3
			AND aor.occurrence_date = $4::date
		ORDER BY aor.recorded_at ASC, aor.id ASC;
	`

	rows, err := r.db.QueryContext(
		ctx,
		query,
		publicationID,
		slotID,
		weekday,
		model.NormalizeOccurrenceDate(occurrenceDate),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanOvertimeRecords(rows)
}

func (r *AttendanceRepository) InsertLeaderArrival(
	ctx context.Context,
	params UpsertAttendanceArrivalParams,
) (*model.AttendanceRecord, error) {
	const query = `
		WITH inserted AS (
			INSERT INTO attendance_records (
				publication_id,
				assignment_id,
				occurrence_date,
				user_id,
				arrived_at,
				recorded_by_user_id,
				recorded_at,
				updated_by_user_id,
				updated_at
			)
			SELECT
				$1,
				a.id,
				$3::date,
				$4,
				$5,
				$6,
				$7,
				$6,
				$7
			FROM assignments a
			LEFT JOIN assignment_overrides ao
				ON ao.assignment_id = a.id
				AND ao.occurrence_date = $3::date
			WHERE a.publication_id = $1
				AND a.id = $2
				AND COALESCE(ao.user_id, a.user_id) = $4
			RETURNING *
		)
		SELECT
			inserted.id,
			inserted.publication_id,
			inserted.assignment_id,
			inserted.occurrence_date,
			inserted.user_id,
			inserted.arrived_at,
			inserted.recorded_by_user_id,
			inserted.recorded_at,
			inserted.updated_by_user_id,
			inserted.updated_at,
			u.name,
			u.email
		FROM inserted
		INNER JOIN users u ON u.id = inserted.user_id;
	`

	record, err := scanAttendanceRecord(r.db.QueryRowContext(
		ctx,
		query,
		params.PublicationID,
		params.AssignmentID,
		model.NormalizeOccurrenceDate(params.OccurrenceDate),
		params.UserID,
		params.ArrivedAt,
		params.RecordedByUserID,
		params.RecordedAt,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrAttendanceRosterStale
	}
	if err != nil {
		return nil, mapAttendanceWriteError(err)
	}

	return record, nil
}

func (r *AttendanceRepository) UpsertAdminArrival(
	ctx context.Context,
	params UpsertAttendanceArrivalParams,
) (*model.AttendanceRecord, error) {
	const query = `
		WITH upserted AS (
			INSERT INTO attendance_records (
				publication_id,
				assignment_id,
				occurrence_date,
				user_id,
				arrived_at,
				recorded_by_user_id,
				recorded_at,
				updated_by_user_id,
				updated_at
			)
			SELECT
				$1,
				a.id,
				$3::date,
				$4,
				$5,
				$6,
				$7,
				$6,
				$7
			FROM assignments a
			LEFT JOIN assignment_overrides ao
				ON ao.assignment_id = a.id
				AND ao.occurrence_date = $3::date
			WHERE a.publication_id = $1
				AND a.id = $2
				AND COALESCE(ao.user_id, a.user_id) = $4
			ON CONFLICT (assignment_id, occurrence_date, user_id) DO UPDATE
			SET
				arrived_at = EXCLUDED.arrived_at,
				updated_by_user_id = EXCLUDED.updated_by_user_id,
				updated_at = EXCLUDED.updated_at
			RETURNING *
		)
		SELECT
			upserted.id,
			upserted.publication_id,
			upserted.assignment_id,
			upserted.occurrence_date,
			upserted.user_id,
			upserted.arrived_at,
			upserted.recorded_by_user_id,
			upserted.recorded_at,
			upserted.updated_by_user_id,
			upserted.updated_at,
			u.name,
			u.email
		FROM upserted
		INNER JOIN users u ON u.id = upserted.user_id;
	`

	record, err := scanAttendanceRecord(r.db.QueryRowContext(
		ctx,
		query,
		params.PublicationID,
		params.AssignmentID,
		model.NormalizeOccurrenceDate(params.OccurrenceDate),
		params.UserID,
		params.ArrivedAt,
		params.RecordedByUserID,
		params.RecordedAt,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrAttendanceRosterStale
	}
	if err != nil {
		return nil, err
	}

	return record, nil
}

func (r *AttendanceRepository) DeleteArrival(
	ctx context.Context,
	publicationID, recordID int64,
) (*model.AttendanceRecord, error) {
	const query = `
		WITH deleted AS (
			DELETE FROM attendance_records
			WHERE publication_id = $1 AND id = $2
			RETURNING *
		)
		SELECT
			deleted.id,
			deleted.publication_id,
			deleted.assignment_id,
			deleted.occurrence_date,
			deleted.user_id,
			deleted.arrived_at,
			deleted.recorded_by_user_id,
			deleted.recorded_at,
			deleted.updated_by_user_id,
			deleted.updated_at,
			u.name,
			u.email
		FROM deleted
		INNER JOIN users u ON u.id = deleted.user_id;
	`

	record, err := scanAttendanceRecord(r.db.QueryRowContext(ctx, query, publicationID, recordID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrAttendanceRecordNotFound
	}
	if err != nil {
		return nil, err
	}

	return record, nil
}

func (r *AttendanceRepository) CreateOvertime(
	ctx context.Context,
	params CreateOvertimeRecordParams,
) (*model.AttendanceOvertimeRecord, error) {
	const query = `
		WITH inserted AS (
			INSERT INTO attendance_overtime_records (
				publication_id,
				slot_id,
				weekday,
				occurrence_date,
				user_id,
				hours,
				note,
				recorded_by_user_id,
				recorded_at,
				updated_by_user_id,
				updated_at
			)
			VALUES ($1, $2, $3, $4::date, $5, $6, $7, $8, $9, $8, $9)
			RETURNING *
		)
		SELECT
			inserted.id,
			inserted.publication_id,
			inserted.slot_id,
			inserted.weekday,
			inserted.occurrence_date,
			inserted.user_id,
			inserted.hours,
			inserted.note,
			inserted.recorded_by_user_id,
			inserted.recorded_at,
			inserted.updated_by_user_id,
			inserted.updated_at,
			u.name,
			u.email
		FROM inserted
		INNER JOIN users u ON u.id = inserted.user_id;
	`

	return scanOvertimeRecord(r.db.QueryRowContext(
		ctx,
		query,
		params.PublicationID,
		params.SlotID,
		params.Weekday,
		model.NormalizeOccurrenceDate(params.OccurrenceDate),
		params.UserID,
		params.Hours,
		params.Note,
		params.RecordedByUserID,
		params.RecordedAt,
	))
}

func (r *AttendanceRepository) GetOvertime(
	ctx context.Context,
	publicationID, recordID int64,
) (*model.AttendanceOvertimeRecord, error) {
	const query = `
		SELECT
			aor.id,
			aor.publication_id,
			aor.slot_id,
			aor.weekday,
			aor.occurrence_date,
			aor.user_id,
			aor.hours,
			aor.note,
			aor.recorded_by_user_id,
			aor.recorded_at,
			aor.updated_by_user_id,
			aor.updated_at,
			u.name,
			u.email
		FROM attendance_overtime_records aor
		INNER JOIN users u ON u.id = aor.user_id
		WHERE aor.publication_id = $1 AND aor.id = $2;
	`

	record, err := scanOvertimeRecord(r.db.QueryRowContext(ctx, query, publicationID, recordID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrAttendanceRecordNotFound
	}
	if err != nil {
		return nil, err
	}

	return record, nil
}

func (r *AttendanceRepository) UpdateOvertime(
	ctx context.Context,
	params UpdateOvertimeRecordParams,
) (*model.AttendanceOvertimeRecord, error) {
	const query = `
		WITH updated AS (
			UPDATE attendance_overtime_records
			SET
				hours = $3,
				note = $4,
				updated_by_user_id = $5,
				updated_at = $6
			WHERE publication_id = $1 AND id = $2
			RETURNING *
		)
		SELECT
			updated.id,
			updated.publication_id,
			updated.slot_id,
			updated.weekday,
			updated.occurrence_date,
			updated.user_id,
			updated.hours,
			updated.note,
			updated.recorded_by_user_id,
			updated.recorded_at,
			updated.updated_by_user_id,
			updated.updated_at,
			u.name,
			u.email
		FROM updated
		INNER JOIN users u ON u.id = updated.user_id;
	`

	record, err := scanOvertimeRecord(r.db.QueryRowContext(
		ctx,
		query,
		params.PublicationID,
		params.RecordID,
		params.Hours,
		params.Note,
		params.UpdatedByUserID,
		params.UpdatedAt,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrAttendanceRecordNotFound
	}
	if err != nil {
		return nil, err
	}

	return record, nil
}

func (r *AttendanceRepository) DeleteOvertime(
	ctx context.Context,
	publicationID, recordID int64,
) (*model.AttendanceOvertimeRecord, error) {
	const query = `
		WITH deleted AS (
			DELETE FROM attendance_overtime_records
			WHERE publication_id = $1 AND id = $2
			RETURNING *
		)
		SELECT
			deleted.id,
			deleted.publication_id,
			deleted.slot_id,
			deleted.weekday,
			deleted.occurrence_date,
			deleted.user_id,
			deleted.hours,
			deleted.note,
			deleted.recorded_by_user_id,
			deleted.recorded_at,
			deleted.updated_by_user_id,
			deleted.updated_at,
			u.name,
			u.email
		FROM deleted
		INNER JOIN users u ON u.id = deleted.user_id;
	`

	record, err := scanOvertimeRecord(r.db.QueryRowContext(ctx, query, publicationID, recordID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrAttendanceRecordNotFound
	}
	if err != nil {
		return nil, err
	}

	return record, nil
}

type nullableAttendanceRecord struct {
	ID               sql.NullInt64
	PublicationID    sql.NullInt64
	AssignmentID     sql.NullInt64
	OccurrenceDate   sql.NullTime
	UserID           sql.NullInt64
	ArrivedAt        sql.NullTime
	RecordedByUserID sql.NullInt64
	RecordedAt       sql.NullTime
	UpdatedByUserID  sql.NullInt64
	UpdatedAt        sql.NullTime
}

func (r nullableAttendanceRecord) toModel(userName, userEmail string) *model.AttendanceRecord {
	if !r.ID.Valid {
		return nil
	}

	record := &model.AttendanceRecord{
		ID:             r.ID.Int64,
		PublicationID:  r.PublicationID.Int64,
		AssignmentID:   r.AssignmentID.Int64,
		OccurrenceDate: model.NormalizeOccurrenceDate(r.OccurrenceDate.Time),
		UserID:         r.UserID.Int64,
		ArrivedAt:      r.ArrivedAt.Time,
		RecordedAt:     r.RecordedAt.Time,
		UpdatedAt:      r.UpdatedAt.Time,
		UserName:       userName,
		UserEmail:      userEmail,
	}
	if r.RecordedByUserID.Valid {
		record.RecordedByUserID = &r.RecordedByUserID.Int64
	}
	if r.UpdatedByUserID.Valid {
		record.UpdatedByUserID = &r.UpdatedByUserID.Int64
	}
	return record
}

func scanAttendanceRecords(rows *sql.Rows) ([]*model.AttendanceRecord, error) {
	records := make([]*model.AttendanceRecord, 0)
	for rows.Next() {
		record, err := scanAttendanceRecord(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return records, nil
}

func scanAttendanceRecord(row scanner) (*model.AttendanceRecord, error) {
	record := &model.AttendanceRecord{}
	var recordedBy sql.NullInt64
	var updatedBy sql.NullInt64
	if err := row.Scan(
		&record.ID,
		&record.PublicationID,
		&record.AssignmentID,
		&record.OccurrenceDate,
		&record.UserID,
		&record.ArrivedAt,
		&recordedBy,
		&record.RecordedAt,
		&updatedBy,
		&record.UpdatedAt,
		&record.UserName,
		&record.UserEmail,
	); err != nil {
		return nil, err
	}
	record.OccurrenceDate = model.NormalizeOccurrenceDate(record.OccurrenceDate)
	if recordedBy.Valid {
		record.RecordedByUserID = &recordedBy.Int64
	}
	if updatedBy.Valid {
		record.UpdatedByUserID = &updatedBy.Int64
	}
	return record, nil
}

func scanOvertimeRecords(rows *sql.Rows) ([]*model.AttendanceOvertimeRecord, error) {
	records := make([]*model.AttendanceOvertimeRecord, 0)
	for rows.Next() {
		record, err := scanOvertimeRecord(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return records, nil
}

func scanOvertimeRecord(row scanner) (*model.AttendanceOvertimeRecord, error) {
	record := &model.AttendanceOvertimeRecord{}
	var recordedBy sql.NullInt64
	var updatedBy sql.NullInt64
	if err := row.Scan(
		&record.ID,
		&record.PublicationID,
		&record.SlotID,
		&record.Weekday,
		&record.OccurrenceDate,
		&record.UserID,
		&record.Hours,
		&record.Note,
		&recordedBy,
		&record.RecordedAt,
		&updatedBy,
		&record.UpdatedAt,
		&record.UserName,
		&record.UserEmail,
	); err != nil {
		return nil, err
	}
	record.OccurrenceDate = model.NormalizeOccurrenceDate(record.OccurrenceDate)
	if recordedBy.Valid {
		record.RecordedByUserID = &recordedBy.Int64
	}
	if updatedBy.Valid {
		record.UpdatedByUserID = &updatedBy.Int64
	}
	return record, nil
}

func mapAttendanceWriteError(err error) error {
	var pqErr *pq.Error
	if !errors.As(err, &pqErr) {
		return err
	}

	if pqErr.Code == "23505" && pqErr.Constraint == attendanceRecordsUniqueKey {
		return ErrAttendanceAlreadyRecorded
	}
	return err
}
