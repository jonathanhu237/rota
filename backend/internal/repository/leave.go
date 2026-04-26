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
	Leave   *model.Leave
	Request *model.ShiftChangeRequest
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
	const query = leaveWithRequestSelect + `
		WHERE l.id = $1;
	`

	row, err := scanLeaveWithRequest(r.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, ErrLeaveNotFound
	}
	if err != nil {
		return nil, nil, err
	}
	return row.Leave, row.Request, nil
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
		s.expires_at
	FROM leaves l
	INNER JOIN shift_change_requests s ON s.id = l.shift_change_request_id
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

	return &LeaveWithRequest{
		Leave:   &leave,
		Request: &req,
	}, nil
}

func offsetForPage(page, pageSize int) int {
	return (page - 1) * pageSize
}
