package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/model"
)

var ErrLeaveNotFound = model.ErrLeaveNotFound

type LeaveWithRequest struct {
	Leave           *model.Leave
	Request         *model.ShiftChangeRequest
	RequesterName   string
	CounterpartName *string
	SubstituteName  *string
	Shift           *LeaveShiftContext
}

type LeaveShiftContext struct {
	AssignmentID    int64
	SlotID          int64
	Weekday         int
	StartTime       string
	EndTime         string
	PositionID      int64
	PositionName    string
	OccurrenceStart time.Time
	OccurrenceEnd   time.Time
}

type ListLeavePoolParams struct {
	ViewerUserID  int64
	ViewerIsAdmin bool
	State         model.LeavePoolStateFilter
	Now           time.Time
	Offset        int
	Limit         int
}

type InsertLeaveParams struct {
	UserID               int64
	PublicationID        int64
	ShiftChangeRequestID int64
	Category             model.LeaveCategory
	Reason               string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type LeaveRepository struct {
	db *sql.DB
}

func NewLeaveRepository(db *sql.DB) *LeaveRepository {
	return &LeaveRepository{db: db}
}

func (r *LeaveRepository) WithTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := fn(tx); err != nil {
		return err
	}

	return tx.Commit()
}

func (r *LeaveRepository) Insert(
	ctx context.Context,
	tx *sql.Tx,
	params InsertLeaveParams,
) (*model.Leave, error) {
	const query = `
		INSERT INTO leaves (
			user_id,
			publication_id,
			shift_change_request_id,
			category,
			reason,
			created_at,
			updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING
			id,
			user_id,
			publication_id,
			shift_change_request_id,
			category,
			reason,
			created_at,
			updated_at;
	`

	return scanLeave(tx.QueryRowContext(
		ctx,
		query,
		params.UserID,
		params.PublicationID,
		params.ShiftChangeRequestID,
		params.Category,
		params.Reason,
		params.CreatedAt,
		params.UpdatedAt,
	))
}

func (r *LeaveRepository) GetByID(
	ctx context.Context,
	id int64,
) (*model.Leave, *model.ShiftChangeRequest, error) {
	row, err := r.GetWithRequestByID(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	return row.Leave, row.Request, nil
}

func (r *LeaveRepository) GetWithRequestByID(
	ctx context.Context,
	id int64,
) (*LeaveWithRequest, error) {
	const query = leaveWithRequestSelect + `
		WHERE l.id = $1;
	`

	row, err := scanLeaveWithRequest(r.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrLeaveNotFound
	}
	if err != nil {
		return nil, err
	}
	return row, nil
}

func (r *LeaveRepository) ListForUser(
	ctx context.Context,
	userID int64,
	page int,
	pageSize int,
) ([]*LeaveWithRequest, error) {
	const query = leaveWithRequestSelect + `
		WHERE l.user_id = $1
		ORDER BY l.created_at DESC, l.id DESC
		LIMIT $2 OFFSET $3;
	`

	return r.list(ctx, query, userID, pageSize, offsetForPage(page, pageSize))
}

func (r *LeaveRepository) ListForPublication(
	ctx context.Context,
	publicationID int64,
	page int,
	pageSize int,
) ([]*LeaveWithRequest, error) {
	const query = leaveWithRequestSelect + `
		WHERE l.publication_id = $1
		ORDER BY l.created_at DESC, l.id DESC
		LIMIT $2 OFFSET $3;
	`

	return r.list(ctx, query, publicationID, pageSize, offsetForPage(page, pageSize))
}

func (r *LeaveRepository) ListPool(
	ctx context.Context,
	params ListLeavePoolParams,
) ([]*LeaveWithRequest, int, error) {
	countQuery := leavePoolCTE + `
		SELECT COUNT(*)
		FROM leave_rows
		WHERE ($3 = 'all' OR leave_state = $3);
	`
	var total int
	if err := r.db.QueryRowContext(
		ctx,
		countQuery,
		params.ViewerIsAdmin,
		params.ViewerUserID,
		params.State,
		params.Now,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	listQuery := leavePoolCTE + `
		SELECT
			leave_id,
			leave_user_id,
			leave_publication_id,
			leave_shift_change_request_id,
			leave_category,
			leave_reason,
			leave_created_at,
			leave_updated_at,
			request_id,
			request_publication_id,
			request_type,
			requester_user_id,
			requester_assignment_id,
			occurrence_date,
			counterpart_user_id,
			counterpart_assignment_id,
			counterpart_occurrence_date,
			request_state,
			request_leave_id,
			decided_by_user_id,
			request_created_at,
			decided_at,
			expires_at,
			requester_name,
			counterpart_name,
			substitute_name,
			shift_slot_id,
			shift_weekday,
			shift_start_time,
			shift_end_time,
			shift_position_id,
			shift_position_name
		FROM leave_rows
		WHERE ($3 = 'all' OR leave_state = $3)
		ORDER BY
			CASE WHEN $3 = 'all' AND leave_state = 'pending' THEN 0 WHEN $3 = 'all' THEN 1 ELSE 0 END ASC,
			CASE WHEN $3 = 'pending' OR ($3 = 'all' AND leave_state = 'pending') THEN expires_at END ASC NULLS LAST,
			CASE WHEN $3 <> 'pending' OR ($3 = 'all' AND leave_state <> 'pending') THEN leave_created_at END DESC NULLS LAST,
			leave_id DESC
		LIMIT $5 OFFSET $6;
	`
	rows, err := r.db.QueryContext(
		ctx,
		listQuery,
		params.ViewerIsAdmin,
		params.ViewerUserID,
		params.State,
		params.Now,
		params.Limit,
		params.Offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]*LeaveWithRequest, 0)
	for rows.Next() {
		row, err := scanLeaveWithRequest(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

func (r *LeaveRepository) list(
	ctx context.Context,
	query string,
	id int64,
	limit int,
	offset int,
) ([]*LeaveWithRequest, error) {
	rows, err := r.db.QueryContext(ctx, query, id, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]*LeaveWithRequest, 0)
	for rows.Next() {
		row, err := scanLeaveWithRequest(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

const leaveWithRequestSelect = `
	SELECT
		l.id,
		l.user_id,
		l.publication_id,
		l.shift_change_request_id,
		l.category,
		l.reason,
		l.created_at,
		l.updated_at,
		s.id,
		s.publication_id,
		s.type,
		s.requester_user_id,
		s.requester_assignment_id,
		s.occurrence_date,
		s.counterpart_user_id,
		s.counterpart_assignment_id,
		s.counterpart_occurrence_date,
		s.state,
		s.leave_id,
		s.decided_by_user_id,
		s.created_at,
		s.decided_at,
		s.expires_at,
		requester.name,
		counterpart.name,
		CASE WHEN s.state = 'approved' THEN substitute.name ELSE NULL END,
		a.slot_id,
		a.weekday,
		TO_CHAR(ts.start_time, 'HH24:MI'),
		TO_CHAR(ts.end_time, 'HH24:MI'),
		a.position_id,
		p.name
	FROM leaves l
	INNER JOIN shift_change_requests s ON s.id = l.shift_change_request_id
	INNER JOIN users requester ON requester.id = l.user_id
	LEFT JOIN users counterpart ON counterpart.id = s.counterpart_user_id
	LEFT JOIN users substitute ON substitute.id = s.decided_by_user_id
	LEFT JOIN assignments a ON a.id = s.requester_assignment_id
	LEFT JOIN template_slots ts ON ts.id = a.slot_id
	LEFT JOIN positions p ON p.id = a.position_id
`

const leavePoolCTE = `
	WITH leave_rows AS (
		SELECT
			l.id AS leave_id,
			l.user_id AS leave_user_id,
			l.publication_id AS leave_publication_id,
			l.shift_change_request_id AS leave_shift_change_request_id,
			l.category AS leave_category,
			l.reason AS leave_reason,
			l.created_at AS leave_created_at,
			l.updated_at AS leave_updated_at,
			s.id AS request_id,
			s.publication_id AS request_publication_id,
			s.type AS request_type,
			s.requester_user_id,
			s.requester_assignment_id,
			s.occurrence_date,
			s.counterpart_user_id,
			s.counterpart_assignment_id,
			s.counterpart_occurrence_date,
			s.state AS request_state,
			s.leave_id AS request_leave_id,
			s.decided_by_user_id,
			s.created_at AS request_created_at,
			s.decided_at,
			s.expires_at,
			CASE
				WHEN s.state = 'approved' THEN 'completed'
				WHEN s.state = 'pending' AND s.expires_at <= $4 THEN 'failed'
				WHEN s.state IN ('expired', 'rejected') THEN 'failed'
				WHEN s.state IN ('cancelled', 'invalidated') THEN 'cancelled'
				ELSE 'pending'
			END AS leave_state,
			requester.name AS requester_name,
			counterpart.name AS counterpart_name,
			CASE WHEN s.state = 'approved' THEN substitute.name ELSE NULL END AS substitute_name,
			a.slot_id AS shift_slot_id,
			a.weekday AS shift_weekday,
			TO_CHAR(ts.start_time, 'HH24:MI') AS shift_start_time,
			TO_CHAR(ts.end_time, 'HH24:MI') AS shift_end_time,
			a.position_id AS shift_position_id,
			p.name AS shift_position_name
		FROM leaves l
		INNER JOIN shift_change_requests s ON s.id = l.shift_change_request_id
		INNER JOIN users requester ON requester.id = l.user_id
		LEFT JOIN users counterpart ON counterpart.id = s.counterpart_user_id
		LEFT JOIN users substitute ON substitute.id = s.decided_by_user_id
		LEFT JOIN assignments a ON a.id = s.requester_assignment_id
		LEFT JOIN template_slots ts ON ts.id = a.slot_id
		LEFT JOIN positions p ON p.id = a.position_id
		WHERE ($1 OR s.type = 'give_pool' OR l.user_id = $2 OR s.counterpart_user_id = $2)
	)
`

func scanLeave(row scanner) (*model.Leave, error) {
	leave := &model.Leave{}
	if err := row.Scan(
		&leave.ID,
		&leave.UserID,
		&leave.PublicationID,
		&leave.ShiftChangeRequestID,
		&leave.Category,
		&leave.Reason,
		&leave.CreatedAt,
		&leave.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return leave, nil
}

func scanLeaveWithRequest(row scanner) (*LeaveWithRequest, error) {
	var (
		leave                 model.Leave
		req                   model.ShiftChangeRequest
		counterpartUser       sql.NullInt64
		counterpartAssignment sql.NullInt64
		counterpartOccurrence sql.NullTime
		leaveID               sql.NullInt64
		decidedBy             sql.NullInt64
		decidedAt             sql.NullTime
		requesterName         string
		counterpartName       sql.NullString
		substituteName        sql.NullString
		slotID                sql.NullInt64
		weekday               sql.NullInt64
		startTime             sql.NullString
		endTime               sql.NullString
		positionID            sql.NullInt64
		positionName          sql.NullString
	)

	if err := row.Scan(
		&leave.ID,
		&leave.UserID,
		&leave.PublicationID,
		&leave.ShiftChangeRequestID,
		&leave.Category,
		&leave.Reason,
		&leave.CreatedAt,
		&leave.UpdatedAt,
		&req.ID,
		&req.PublicationID,
		&req.Type,
		&req.RequesterUserID,
		&req.RequesterAssignmentID,
		&req.OccurrenceDate,
		&counterpartUser,
		&counterpartAssignment,
		&counterpartOccurrence,
		&req.State,
		&leaveID,
		&decidedBy,
		&req.CreatedAt,
		&decidedAt,
		&req.ExpiresAt,
		&requesterName,
		&counterpartName,
		&substituteName,
		&slotID,
		&weekday,
		&startTime,
		&endTime,
		&positionID,
		&positionName,
	); err != nil {
		return nil, err
	}

	req.OccurrenceDate = model.NormalizeOccurrenceDate(req.OccurrenceDate)
	if counterpartUser.Valid {
		id := counterpartUser.Int64
		req.CounterpartUserID = &id
	}
	if counterpartAssignment.Valid {
		id := counterpartAssignment.Int64
		req.CounterpartAssignmentID = &id
	}
	if counterpartOccurrence.Valid {
		date := model.NormalizeOccurrenceDate(counterpartOccurrence.Time)
		req.CounterpartOccurrenceDate = &date
	}
	if leaveID.Valid {
		id := leaveID.Int64
		req.LeaveID = &id
	}
	if decidedBy.Valid {
		id := decidedBy.Int64
		req.DecidedByUserID = &id
	}
	if decidedAt.Valid {
		t := decidedAt.Time
		req.DecidedAt = &t
	}

	var counterpartNamePtr *string
	if counterpartName.Valid {
		v := counterpartName.String
		counterpartNamePtr = &v
	}
	var substituteNamePtr *string
	if substituteName.Valid {
		v := substituteName.String
		substituteNamePtr = &v
	}
	var shift *LeaveShiftContext
	if slotID.Valid && weekday.Valid && startTime.Valid && endTime.Valid && positionID.Valid && positionName.Valid {
		shift = &LeaveShiftContext{
			AssignmentID:    req.RequesterAssignmentID,
			SlotID:          slotID.Int64,
			Weekday:         int(weekday.Int64),
			StartTime:       startTime.String,
			EndTime:         endTime.String,
			PositionID:      positionID.Int64,
			PositionName:    positionName.String,
			OccurrenceStart: req.ExpiresAt,
		}
		if end, err := occurrenceEndFromClock(endTime.String, req.OccurrenceDate); err == nil {
			shift.OccurrenceEnd = end
		}
	}

	return &LeaveWithRequest{
		Leave:           &leave,
		Request:         &req,
		RequesterName:   requesterName,
		CounterpartName: counterpartNamePtr,
		SubstituteName:  substituteNamePtr,
		Shift:           shift,
	}, nil
}

func occurrenceEndFromClock(clock string, occurrenceDate time.Time) (time.Time, error) {
	endClock, err := time.Parse("15:04", clock)
	if err != nil {
		return time.Time{}, err
	}
	date := model.NormalizeOccurrenceDate(occurrenceDate)
	return time.Date(
		date.Year(),
		date.Month(),
		date.Day(),
		endClock.Hour(),
		endClock.Minute(),
		0,
		0,
		time.UTC,
	), nil
}

func offsetForPage(page, pageSize int) int {
	return (page - 1) * pageSize
}
