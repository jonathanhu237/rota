package repository

import (
	"context"
	"database/sql"
	"errors"
	"sort"
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
	PublicationID             int64
	Type                      model.ShiftChangeType
	RequesterUserID           int64
	RequesterAssignmentID     int64
	OccurrenceDate            time.Time
	CounterpartUserID         *int64
	CounterpartAssignmentID   *int64
	CounterpartOccurrenceDate *time.Time
	LeaveID                   *int64
	ExpiresAt                 time.Time
	CreatedAt                 time.Time
	AfterCreateTx             func(ctx context.Context, tx *sql.Tx, req *model.ShiftChangeRequest) error
}

// Create inserts a new shift_change_request.
func (r *ShiftChangeRepository) Create(
	ctx context.Context,
	params CreateShiftChangeRequestParams,
) (*model.ShiftChangeRequest, error) {
	if params.AfterCreateTx != nil {
		tx, err := r.db.BeginTx(ctx, nil)
		if err != nil {
			return nil, err
		}
		defer tx.Rollback()

		req, err := createShiftChangeRequest(ctx, tx, params)
		if err != nil {
			return nil, err
		}
		if err := params.AfterCreateTx(ctx, tx, req); err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return req, nil
	}
	return createShiftChangeRequest(ctx, r.db, params)
}

func (r *ShiftChangeRepository) CreateTx(
	ctx context.Context,
	tx *sql.Tx,
	params CreateShiftChangeRequestParams,
) (*model.ShiftChangeRequest, error) {
	return createShiftChangeRequest(ctx, tx, params)
}

func createShiftChangeRequest(
	ctx context.Context,
	db dbtx,
	params CreateShiftChangeRequestParams,
) (*model.ShiftChangeRequest, error) {
	const query = `
		INSERT INTO shift_change_requests (
			publication_id,
			type,
			requester_user_id,
			requester_assignment_id,
			occurrence_date,
			counterpart_user_id,
			counterpart_assignment_id,
			counterpart_occurrence_date,
			leave_id,
			state,
			created_at,
			expires_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'pending', $10, $11)
		RETURNING
			id,
			publication_id,
			type,
			requester_user_id,
			requester_assignment_id,
			occurrence_date,
			counterpart_user_id,
			counterpart_assignment_id,
			counterpart_occurrence_date,
			state,
			leave_id,
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
	var counterpartOccurrence sql.NullTime
	if params.CounterpartOccurrenceDate != nil {
		counterpartOccurrence = sql.NullTime{Time: model.NormalizeOccurrenceDate(*params.CounterpartOccurrenceDate), Valid: true}
	}
	var leaveID sql.NullInt64
	if params.LeaveID != nil {
		leaveID = sql.NullInt64{Int64: *params.LeaveID, Valid: true}
	}

	return scanShiftChangeRequest(db.QueryRowContext(
		ctx,
		query,
		params.PublicationID,
		params.Type,
		params.RequesterUserID,
		params.RequesterAssignmentID,
		model.NormalizeOccurrenceDate(params.OccurrenceDate),
		counterpartUser,
		counterpartAssignment,
		counterpartOccurrence,
		leaveID,
		params.CreatedAt,
		params.ExpiresAt,
	))
}

// GetByID returns a single request by id, or ErrShiftChangeNotFound.
func (r *ShiftChangeRepository) GetByID(
	ctx context.Context,
	id int64,
) (*model.ShiftChangeRequest, error) {
	return getShiftChangeRequestByID(ctx, r.db, id)
}

func getShiftChangeRequestByID(
	ctx context.Context,
	db dbtx,
	id int64,
) (*model.ShiftChangeRequest, error) {
	const query = `
		SELECT
			id,
			publication_id,
			type,
			requester_user_id,
			requester_assignment_id,
			occurrence_date,
			counterpart_user_id,
			counterpart_assignment_id,
			counterpart_occurrence_date,
			state,
			leave_id,
			decided_by_user_id,
			created_at,
			decided_at,
			expires_at
		FROM shift_change_requests
		WHERE id = $1;
	`

	req, err := scanShiftChangeRequest(db.QueryRowContext(ctx, query, id))
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
				occurrence_date,
				counterpart_user_id,
				counterpart_assignment_id,
				counterpart_occurrence_date,
				state,
				leave_id,
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
				occurrence_date,
				counterpart_user_id,
				counterpart_assignment_id,
				counterpart_occurrence_date,
				state,
				leave_id,
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

func (r *ShiftChangeRepository) SetLeaveIDTx(
	ctx context.Context,
	tx *sql.Tx,
	requestID int64,
	leaveID int64,
) (*model.ShiftChangeRequest, error) {
	const query = `
		UPDATE shift_change_requests
		SET leave_id = $2
		WHERE id = $1
		RETURNING
			id,
			publication_id,
			type,
			requester_user_id,
			requester_assignment_id,
			occurrence_date,
			counterpart_user_id,
			counterpart_assignment_id,
			counterpart_occurrence_date,
			state,
			leave_id,
			decided_by_user_id,
			created_at,
			decided_at,
			expires_at;
	`

	req, err := scanShiftChangeRequest(tx.QueryRowContext(ctx, query, requestID, leaveID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrShiftChangeNotFound
	}
	return req, err
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
	AfterUpdateTx   func(ctx context.Context, tx *sql.Tx) error
}

// UpdateState transitions a request's state, only if it is currently in
// CurrentState. Returns ErrShiftChangeNotPending if the row is not in the
// expected state. This does NOT touch assignments — callers that need atomic
// approval/claim use the dedicated Approve / Claim helpers.
func (r *ShiftChangeRepository) UpdateState(
	ctx context.Context,
	params UpdateStateParams,
) error {
	if params.AfterUpdateTx != nil {
		tx, err := r.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()

		if err := updateShiftChangeState(ctx, tx, params); err != nil {
			return err
		}
		if err := params.AfterUpdateTx(ctx, tx); err != nil {
			return err
		}
		return tx.Commit()
	}

	return updateShiftChangeState(ctx, r.db, params)
}

func updateShiftChangeState(ctx context.Context, db dbtx, params UpdateStateParams) error {
	const query = `
		UPDATE shift_change_requests
		SET state = $2, decided_by_user_id = $3, decided_at = $4
		WHERE id = $1 AND state = $5;
	`

	var decidedBy sql.NullInt64
	if params.DecidedByUserID != nil {
		decidedBy = sql.NullInt64{Int64: *params.DecidedByUserID, Valid: true}
	}

	result, err := db.ExecContext(
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
	ID            int64
	PublicationID int64
	UserID        int64
	SlotID        int64
	Weekday       int
	PositionID    int64
	Exists        bool
}

// ApplySwapParams applies an approved swap. Callers pass the two
// assignment IDs; the repo verifies each still exists with the expected
// user, writes occurrence overrides, and flips the request to approved.
type ApplySwapParams struct {
	RequestID                 int64
	PublicationID             int64
	RequesterAssignmentID     int64
	RequesterUserID           int64
	OccurrenceDate            time.Time
	CounterpartAssignmentID   int64
	CounterpartUserID         int64
	CounterpartOccurrenceDate time.Time
	DecidedByUserID           int64
	Now                       time.Time
	AfterApplyTx              func(ctx context.Context, tx *sql.Tx) error
}

// ApplyGiveParams applies an approved give (direct or pool claim). The
// receiver is either the request's counterpart (direct) or the claimer
// (pool). Either way, the baseline assignment stays unchanged and the
// concrete occurrence is assigned through assignment_overrides.
type ApplyGiveParams struct {
	RequestID             int64
	PublicationID         int64
	RequesterAssignmentID int64
	RequesterUserID       int64
	OccurrenceDate        time.Time
	ReceiverUserID        int64
	DecidedByUserID       int64
	Now                   time.Time
	AfterApplyTx          func(ctx context.Context, tx *sql.Tx) error
}

// ApproveResult surfaces the final assignment snapshots after a successful
// swap or give, so audit metadata can be assembled without another round
// trip.
type ApproveResult struct {
	RequesterAssignment   *model.Assignment
	CounterpartAssignment *model.Assignment
}

// ApplySwap atomically verifies the two assignments and the request state,
// then writes assignment_overrides for both occurrences and marks the request
// approved. Returns one of the domain sentinels on validation failure.
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
	if requesterSnap.PublicationID != params.PublicationID ||
		counterpartSnap.PublicationID != params.PublicationID {
		return nil, ErrShiftChangeAssignmentMiss
	}

	if err := LockAndCheckUserStatus(ctx, tx, requesterSnap.PublicationID, params.RequesterUserID); err != nil {
		return nil, err
	}
	if err := LockAndCheckUserStatus(ctx, tx, requesterSnap.PublicationID, params.CounterpartUserID); err != nil {
		return nil, err
	}

	if err := insertAssignmentOverrideTx(ctx, tx, InsertAssignmentOverrideParams{
		AssignmentID:   params.RequesterAssignmentID,
		OccurrenceDate: params.OccurrenceDate,
		UserID:         params.CounterpartUserID,
		CreatedAt:      params.Now,
	}); err != nil {
		return nil, mapSchedulingRetryableError(err)
	}
	if err := insertAssignmentOverrideTx(ctx, tx, InsertAssignmentOverrideParams{
		AssignmentID:   params.CounterpartAssignmentID,
		OccurrenceDate: params.CounterpartOccurrenceDate,
		UserID:         params.RequesterUserID,
		CreatedAt:      params.Now,
	}); err != nil {
		return nil, mapSchedulingRetryableError(err)
	}

	if err := markApproved(ctx, tx, params.RequestID, params.DecidedByUserID, params.Now); err != nil {
		return nil, mapSchedulingRetryableError(err)
	}

	requester, err := loadAssignment(ctx, tx, params.RequesterAssignmentID)
	if err != nil {
		return nil, err
	}
	counterpart, err := loadAssignment(ctx, tx, params.CounterpartAssignmentID)
	if err != nil {
		return nil, err
	}

	if params.AfterApplyTx != nil {
		if err := params.AfterApplyTx(ctx, tx); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, mapSchedulingRetryableError(err)
	}
	return &ApproveResult{
		RequesterAssignment:   requester,
		CounterpartAssignment: counterpart,
	}, nil
}

// ApplyGive atomically verifies the requester's assignment and the request
// state, then writes one occurrence override for the receiver.
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
	if snap.PublicationID != params.PublicationID {
		return nil, ErrShiftChangeAssignmentMiss
	}

	if err := LockAndCheckUserStatus(ctx, tx, snap.PublicationID, params.ReceiverUserID); err != nil {
		return nil, err
	}

	if err := insertAssignmentOverrideTx(ctx, tx, InsertAssignmentOverrideParams{
		AssignmentID:   params.RequesterAssignmentID,
		OccurrenceDate: params.OccurrenceDate,
		UserID:         params.ReceiverUserID,
		CreatedAt:      params.Now,
	}); err != nil {
		return nil, mapSchedulingRetryableError(err)
	}
	if err := markApproved(ctx, tx, params.RequestID, params.DecidedByUserID, params.Now); err != nil {
		return nil, mapSchedulingRetryableError(err)
	}

	requester, err := loadAssignment(ctx, tx, params.RequesterAssignmentID)
	if err != nil {
		return nil, err
	}

	if params.AfterApplyTx != nil {
		if err := params.AfterApplyTx(ctx, tx); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, mapSchedulingRetryableError(err)
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

// InvalidateRequestsForAssignment transitions pending requests that
// reference the given assignment to invalidated and returns the affected ids.
func (r *ShiftChangeRepository) InvalidateRequestsForAssignment(
	ctx context.Context,
	assignmentID int64,
	now time.Time,
) ([]int64, error) {
	requests, err := invalidateRequestsForAssignment(ctx, r.db, assignmentID, now)
	if err != nil {
		return nil, err
	}

	ids := make([]int64, 0, len(requests))
	for _, req := range requests {
		ids = append(ids, req.ID)
	}
	return ids, nil
}

func (r *ShiftChangeRepository) InvalidateRequestsForAssignmentTx(
	ctx context.Context,
	tx *sql.Tx,
	assignmentID int64,
	now time.Time,
) ([]*model.ShiftChangeRequest, error) {
	return invalidateRequestsForAssignment(ctx, tx, assignmentID, now)
}

func invalidateRequestsForAssignment(
	ctx context.Context,
	db dbtx,
	assignmentID int64,
	now time.Time,
) ([]*model.ShiftChangeRequest, error) {
	const query = `
		UPDATE shift_change_requests
		SET state = 'invalidated', decided_at = $2
		WHERE state = 'pending'
			AND (
				requester_assignment_id = $1
				OR counterpart_assignment_id = $1
			)
		RETURNING id;
	`

	rows, err := db.QueryContext(ctx, query, assignmentID, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	requests := make([]*model.ShiftChangeRequest, 0, len(ids))
	for _, id := range ids {
		req, err := getShiftChangeRequestByID(ctx, db, id)
		if err != nil {
			return nil, err
		}
		requests = append(requests, req)
	}
	return requests, nil
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
		return mapSchedulingRetryableError(err)
	case state != model.ShiftChangeStatePending:
		return ErrShiftChangeNotPending
	default:
		return nil
	}
}

func lockAssignment(ctx context.Context, tx *sql.Tx, id int64) (AssignmentSnapshot, error) {
	const query = `
		SELECT
			a.publication_id,
			a.user_id,
			a.slot_id,
			a.weekday,
			a.position_id
		FROM assignments a
		WHERE a.id = $1
		FOR UPDATE;
	`
	snap := AssignmentSnapshot{ID: id}
	switch err := tx.QueryRowContext(ctx, query, id).Scan(
		&snap.PublicationID,
		&snap.UserID,
		&snap.SlotID,
		&snap.Weekday,
		&snap.PositionID,
	); {
	case errors.Is(err, sql.ErrNoRows):
		return AssignmentSnapshot{ID: id, Exists: false}, nil
	case err != nil:
		return AssignmentSnapshot{}, mapSchedulingRetryableError(err)
	}
	snap.Exists = true
	return snap, nil
}

func loadAssignment(ctx context.Context, tx *sql.Tx, id int64) (*model.Assignment, error) {
	const query = `
		SELECT id, publication_id, user_id, slot_id, weekday, position_id, created_at
		FROM assignments
		WHERE id = $1;
	`
	assignment := &model.Assignment{}
	if err := tx.QueryRowContext(ctx, query, id).Scan(
		&assignment.ID,
		&assignment.PublicationID,
		&assignment.UserID,
		&assignment.SlotID,
		&assignment.Weekday,
		&assignment.PositionID,
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
		req                   model.ShiftChangeRequest
		counterpartUser       sql.NullInt64
		counterpartAssignment sql.NullInt64
		counterpartOccurrence sql.NullTime
		leaveID               sql.NullInt64
		decidedBy             sql.NullInt64
		decidedAt             sql.NullTime
	)
	err := row.Scan(
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
	req.OccurrenceDate = model.NormalizeOccurrenceDate(req.OccurrenceDate)
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
	return &req, nil
}
