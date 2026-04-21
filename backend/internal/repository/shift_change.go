package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/model"
)

var (
	ErrShiftChangeNotFound       = model.ErrShiftChangeNotFound
	ErrShiftChangeNotPending     = model.ErrShiftChangeNotPending
	ErrShiftChangeAssignmentMiss = model.ErrShiftChangeAssignmentMiss
)

// ShiftChangeRepository persists shift_change_requests rows.
type ShiftChangeRepository struct {
	db *sql.DB
}

// NewShiftChangeRepository constructs a repository over the given DB handle.
func NewShiftChangeRepository(db *sql.DB) *ShiftChangeRepository {
	return &ShiftChangeRepository{db: db}
}

// CreateShiftChangeRequestParams describes a new request.
type CreateShiftChangeRequestParams struct {
	PublicationID           int64
	Type                    model.ShiftChangeType
	RequesterUserID         int64
	RequesterAssignmentID   int64
	CounterpartUserID       *int64
	CounterpartAssignmentID *int64
	ExpiresAt               time.Time
	CreatedAt               time.Time
}

// Create inserts a new shift_change_request.
func (r *ShiftChangeRepository) Create(
	ctx context.Context,
	params CreateShiftChangeRequestParams,
) (*model.ShiftChangeRequest, error) {
	const query = `
		INSERT INTO shift_change_requests (
			publication_id,
			type,
			requester_user_id,
			requester_assignment_id,
			counterpart_user_id,
			counterpart_assignment_id,
			state,
			created_at,
			expires_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, 'pending', $7, $8)
		RETURNING
			id,
			publication_id,
			type,
			requester_user_id,
			requester_assignment_id,
			counterpart_user_id,
			counterpart_assignment_id,
			state,
			decided_by_user_id,
			created_at,
			decided_at,
			expires_at;
	`

	var counterpartUser sql.NullInt64
	if params.CounterpartUserID != nil {
		counterpartUser = sql.NullInt64{Int64: *params.CounterpartUserID, Valid: true}
	}
	var counterpartAssignment sql.NullInt64
	if params.CounterpartAssignmentID != nil {
		counterpartAssignment = sql.NullInt64{Int64: *params.CounterpartAssignmentID, Valid: true}
	}

	return scanShiftChangeRequest(r.db.QueryRowContext(
		ctx,
		query,
		params.PublicationID,
		params.Type,
		params.RequesterUserID,
		params.RequesterAssignmentID,
		counterpartUser,
		counterpartAssignment,
		params.CreatedAt,
		params.ExpiresAt,
	))
}

// GetByID returns a single request by id, or ErrShiftChangeNotFound.
func (r *ShiftChangeRepository) GetByID(
	ctx context.Context,
	id int64,
) (*model.ShiftChangeRequest, error) {
	const query = `
		SELECT
			id,
			publication_id,
			type,
			requester_user_id,
			requester_assignment_id,
			counterpart_user_id,
			counterpart_assignment_id,
			state,
			decided_by_user_id,
			created_at,
			decided_at,
			expires_at
		FROM shift_change_requests
		WHERE id = $1;
	`

	req, err := scanShiftChangeRequest(r.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrShiftChangeNotFound
	}
	return req, err
}

// ListForPublicationParams describes filters for the per-publication listing.
type ListForPublicationParams struct {
	PublicationID int64
	Audience      ShiftChangeAudience
}

// ShiftChangeAudience controls which rows the caller sees.
type ShiftChangeAudience struct {
	Admin           bool  // admin sees everything
	ViewerUserID    int64 // non-admin: rows where viewer is requester, counterpart, or (for pool) qualified
	QualifiedPoolID bool  // reserved for future filtering if needed
}

// ListForPublication returns requests for a publication, filtered by audience.
// Admin sees all; non-admin sees sent + received + (for pool) everything.
// The pool-qualification check is left to the service layer; the repo simply
// includes all give_pool rows when ViewerUserID is non-zero and lets the
// service filter further.
func (r *ShiftChangeRepository) ListForPublication(
	ctx context.Context,
	params ListForPublicationParams,
) ([]*model.ShiftChangeRequest, error) {
	var (
		query string
		args  []any
	)
	if params.Audience.Admin {
		query = `
			SELECT
				id,
				publication_id,
				type,
				requester_user_id,
				requester_assignment_id,
				counterpart_user_id,
				counterpart_assignment_id,
				state,
				decided_by_user_id,
				created_at,
				decided_at,
				expires_at
			FROM shift_change_requests
			WHERE publication_id = $1
			ORDER BY created_at DESC, id DESC;
		`
		args = []any{params.PublicationID}
	} else {
		query = `
			SELECT
				id,
				publication_id,
				type,
				requester_user_id,
				requester_assignment_id,
				counterpart_user_id,
				counterpart_assignment_id,
				state,
				decided_by_user_id,
				created_at,
				decided_at,
				expires_at
			FROM shift_change_requests
			WHERE publication_id = $1
				AND (
					requester_user_id = $2
					OR counterpart_user_id = $2
					OR type = 'give_pool'
				)
			ORDER BY created_at DESC, id DESC;
		`
		args = []any{params.PublicationID, params.Audience.ViewerUserID}
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]*model.ShiftChangeRequest, 0)
	for rows.Next() {
		req, err := scanShiftChangeRequest(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, req)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// CountPendingForCounterpart returns the number of pending rows where the
// given user is the counterpart (swap / give_direct) and the expires_at
// has not yet passed.
func (r *ShiftChangeRepository) CountPendingForCounterpart(
	ctx context.Context,
	userID int64,
	now time.Time,
) (int, error) {
	const query = `
		SELECT COUNT(*)
		FROM shift_change_requests
		WHERE counterpart_user_id = $1
			AND state = 'pending'
			AND expires_at > $2;
	`

	var count int
	if err := r.db.QueryRowContext(ctx, query, userID, now).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// UpdateStateParams captures a straightforward state transition for cancel /
// reject / expire (where there is no additional data to write).
type UpdateStateParams struct {
	ID              int64
	CurrentState    model.ShiftChangeState
	NextState       model.ShiftChangeState
	DecidedByUserID *int64
	Now             time.Time
}

// UpdateState transitions a request's state, only if it is currently in
// CurrentState. Returns ErrShiftChangeNotPending if the row is not in the
// expected state. This does NOT touch assignments — callers that need atomic
// approval/claim use the dedicated Approve / Claim helpers.
func (r *ShiftChangeRepository) UpdateState(
	ctx context.Context,
	params UpdateStateParams,
) error {
	const query = `
		UPDATE shift_change_requests
		SET state = $2, decided_by_user_id = $3, decided_at = $4
		WHERE id = $1 AND state = $5;
	`

	var decidedBy sql.NullInt64
	if params.DecidedByUserID != nil {
		decidedBy = sql.NullInt64{Int64: *params.DecidedByUserID, Valid: true}
	}

	result, err := r.db.ExecContext(
		ctx,
		query,
		params.ID,
		params.NextState,
		decidedBy,
		params.Now,
		params.CurrentState,
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrShiftChangeNotPending
	}
	return nil
}

// AssignmentSnapshot captures the minimum fields needed for the optimistic
// lock check at approval time.
type AssignmentSnapshot struct {
	ID     int64
	UserID int64
	Exists bool
}

// ApplySwapParams applies an approved swap. Callers pass the two
// assignment IDs; the repo verifies each still exists with the expected
// user, swaps the user_id, and flips the request to approved.
type ApplySwapParams struct {
	RequestID             int64
	RequesterAssignmentID int64
	RequesterUserID       int64
	CounterpartAssignmentID int64
	CounterpartUserID     int64
	DecidedByUserID       int64
	Now                   time.Time
}

// ApplyGiveParams applies an approved give (direct or pool claim). The
// receiver is either the request's counterpart (direct) or the claimer
// (pool). Either way, we transfer the requester's assignment to the
// receiver.
type ApplyGiveParams struct {
	RequestID             int64
	RequesterAssignmentID int64
	RequesterUserID       int64
	ReceiverUserID        int64
	DecidedByUserID       int64
	Now                   time.Time
}

// ApproveResult surfaces the final assignment snapshots after a successful
// swap or give, so audit metadata can be assembled without another round
// trip.
type ApproveResult struct {
	RequesterAssignment *model.Assignment
	CounterpartAssignment *model.Assignment
}

// ApplySwap atomically verifies the two assignments and the request state,
// then swaps user_ids and marks the request approved. Returns one of the
// domain sentinels on validation failure.
func (r *ShiftChangeRepository) ApplySwap(
	ctx context.Context,
	params ApplySwapParams,
) (*ApproveResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if err := lockPendingRequest(ctx, tx, params.RequestID); err != nil {
		return nil, err
	}

	requesterSnap, err := lockAssignment(ctx, tx, params.RequesterAssignmentID)
	if err != nil {
		return nil, err
	}
	counterpartSnap, err := lockAssignment(ctx, tx, params.CounterpartAssignmentID)
	if err != nil {
		return nil, err
	}
	if !requesterSnap.Exists || requesterSnap.UserID != params.RequesterUserID {
		return nil, ErrShiftChangeAssignmentMiss
	}
	if !counterpartSnap.Exists || counterpartSnap.UserID != params.CounterpartUserID {
		return nil, ErrShiftChangeAssignmentMiss
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE assignments SET user_id = $2 WHERE id = $1;`,
		params.RequesterAssignmentID, params.CounterpartUserID,
	); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE assignments SET user_id = $2 WHERE id = $1;`,
		params.CounterpartAssignmentID, params.RequesterUserID,
	); err != nil {
		return nil, err
	}

	if err := markApproved(ctx, tx, params.RequestID, params.DecidedByUserID, params.Now); err != nil {
		return nil, err
	}

	requester, err := loadAssignment(ctx, tx, params.RequesterAssignmentID)
	if err != nil {
		return nil, err
	}
	counterpart, err := loadAssignment(ctx, tx, params.CounterpartAssignmentID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &ApproveResult{
		RequesterAssignment:   requester,
		CounterpartAssignment: counterpart,
	}, nil
}

// ApplyGive atomically verifies the requester's assignment and the request
// state, then transfers user_id from requester to receiver.
func (r *ShiftChangeRepository) ApplyGive(
	ctx context.Context,
	params ApplyGiveParams,
) (*ApproveResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if err := lockPendingRequest(ctx, tx, params.RequestID); err != nil {
		return nil, err
	}
	snap, err := lockAssignment(ctx, tx, params.RequesterAssignmentID)
	if err != nil {
		return nil, err
	}
	if !snap.Exists || snap.UserID != params.RequesterUserID {
		return nil, ErrShiftChangeAssignmentMiss
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE assignments SET user_id = $2 WHERE id = $1;`,
		params.RequesterAssignmentID, params.ReceiverUserID,
	); err != nil {
		return nil, err
	}
	if err := markApproved(ctx, tx, params.RequestID, params.DecidedByUserID, params.Now); err != nil {
		return nil, err
	}

	requester, err := loadAssignment(ctx, tx, params.RequesterAssignmentID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &ApproveResult{
		RequesterAssignment: requester,
	}, nil
}

// MarkInvalidated transitions a pending request to invalidated; used when
// the optimistic lock discovers the underlying assignment changed.
func (r *ShiftChangeRepository) MarkInvalidated(
	ctx context.Context,
	id int64,
	now time.Time,
) error {
	const query = `
		UPDATE shift_change_requests
		SET state = 'invalidated', decided_at = $2
		WHERE id = $1 AND state = 'pending';
	`
	_, err := r.db.ExecContext(ctx, query, id, now)
	return err
}

// MarkExpired transitions a pending request to expired on observation.
func (r *ShiftChangeRepository) MarkExpired(
	ctx context.Context,
	id int64,
	now time.Time,
) error {
	const query = `
		UPDATE shift_change_requests
		SET state = 'expired', decided_at = $2
		WHERE id = $1 AND state = 'pending';
	`
	_, err := r.db.ExecContext(ctx, query, id, now)
	return err
}

func lockPendingRequest(ctx context.Context, tx *sql.Tx, id int64) error {
	const query = `
		SELECT state
		FROM shift_change_requests
		WHERE id = $1
		FOR UPDATE;
	`
	var state model.ShiftChangeState
	switch err := tx.QueryRowContext(ctx, query, id).Scan(&state); {
	case errors.Is(err, sql.ErrNoRows):
		return ErrShiftChangeNotFound
	case err != nil:
		return err
	case state != model.ShiftChangeStatePending:
		return ErrShiftChangeNotPending
	default:
		return nil
	}
}

func lockAssignment(ctx context.Context, tx *sql.Tx, id int64) (AssignmentSnapshot, error) {
	const query = `
		SELECT user_id
		FROM assignments
		WHERE id = $1
		FOR UPDATE;
	`
	var userID int64
	switch err := tx.QueryRowContext(ctx, query, id).Scan(&userID); {
	case errors.Is(err, sql.ErrNoRows):
		return AssignmentSnapshot{ID: id, Exists: false}, nil
	case err != nil:
		return AssignmentSnapshot{}, err
	}
	return AssignmentSnapshot{ID: id, UserID: userID, Exists: true}, nil
}

func loadAssignment(ctx context.Context, tx *sql.Tx, id int64) (*model.Assignment, error) {
	const query = `
		SELECT id, publication_id, user_id, template_shift_id, created_at
		FROM assignments
		WHERE id = $1;
	`
	assignment := &model.Assignment{}
	if err := tx.QueryRowContext(ctx, query, id).Scan(
		&assignment.ID,
		&assignment.PublicationID,
		&assignment.UserID,
		&assignment.TemplateShiftID,
		&assignment.CreatedAt,
	); err != nil {
		return nil, err
	}
	return assignment, nil
}

func markApproved(
	ctx context.Context,
	tx *sql.Tx,
	requestID int64,
	deciderID int64,
	now time.Time,
) error {
	const query = `
		UPDATE shift_change_requests
		SET state = 'approved', decided_by_user_id = $2, decided_at = $3
		WHERE id = $1;
	`
	_, err := tx.ExecContext(ctx, query, requestID, deciderID, now)
	return err
}

func scanShiftChangeRequest(row scanner) (*model.ShiftChangeRequest, error) {
	var (
		req                      model.ShiftChangeRequest
		counterpartUser          sql.NullInt64
		counterpartAssignment    sql.NullInt64
		decidedBy                sql.NullInt64
		decidedAt                sql.NullTime
	)
	err := row.Scan(
		&req.ID,
		&req.PublicationID,
		&req.Type,
		&req.RequesterUserID,
		&req.RequesterAssignmentID,
		&counterpartUser,
		&counterpartAssignment,
		&req.State,
		&decidedBy,
		&req.CreatedAt,
		&decidedAt,
		&req.ExpiresAt,
	)
	if err != nil {
		return nil, err
	}

	if counterpartUser.Valid {
		id := counterpartUser.Int64
		req.CounterpartUserID = &id
	}
	if counterpartAssignment.Valid {
		id := counterpartAssignment.Int64
		req.CounterpartAssignmentID = &id
	}
	if decidedBy.Valid {
		id := decidedBy.Int64
		req.DecidedByUserID = &id
	}
	if decidedAt.Valid {
		t := decidedAt.Time
		req.DecidedAt = &t
	}
	return &req, nil
}
