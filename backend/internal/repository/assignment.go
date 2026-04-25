package repository

import (
	"context"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/lib/pq"
)

var (
	ErrAssignmentNotFound          = errors.New("assignment not found")
	ErrAssignmentUserAlreadyInSlot = model.ErrAssignmentUserAlreadyInSlot
	ErrUserDisabled                = errors.New("user disabled")
	ErrSchedulingRetryable         = model.ErrSchedulingRetryable
)

type CreateAssignmentParams struct {
	PublicationID   int64
	UserID          int64
	SlotID          int64
	PositionID      int64
	TemplateShiftID int64
	CreatedAt       time.Time
}

type DeleteAssignmentParams struct {
	PublicationID int64
	AssignmentID  int64
}

type ReplaceAssignmentParams struct {
	UserID          int64
	SlotID          int64
	PositionID      int64
	TemplateShiftID int64
}

type ReplaceAssignmentsParams struct {
	PublicationID int64
	Assignments   []ReplaceAssignmentParams
	CreatedAt     time.Time
}

type AssignmentBoardSlotView struct {
	Slot      *model.TemplateSlot
	Positions map[int64]*AssignmentBoardPositionView
}

type AssignmentBoardPositionView struct {
	Position              *model.Position
	RequiredHeadcount     int
	Candidates            []*model.AssignmentCandidate
	NonCandidateQualified []*model.AssignmentCandidate
	Assignments           []*model.AssignmentParticipant
}

type ActivatePublicationParams struct {
	ID  int64
	Now time.Time
}

type PublishPublicationParams struct {
	ID  int64
	Now time.Time
}

type EndPublicationParams struct {
	ID  int64
	Now time.Time
}

func (r *PublicationRepository) GetSlot(
	ctx context.Context,
	templateID, slotID int64,
) (*model.TemplateSlot, error) {
	return getTemplateSlotByID(ctx, r.db, templateID, slotID)
}

func (r *PublicationRepository) ListSlotPositions(
	ctx context.Context,
	slotID int64,
) ([]*model.TemplateSlotPosition, error) {
	return listTemplateSlotPositions(ctx, r.db, slotID)
}

func (r *PublicationRepository) GetUserByID(ctx context.Context, id int64) (*model.User, error) {
	const query = `
		SELECT id, email, password_hash, name, is_admin, status, version
		FROM users
		WHERE id = $1;
	`

	user := &model.User{}
	err := scanUser(r.db.QueryRowContext(ctx, query, id), user)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (r *PublicationRepository) CreateAssignment(
	ctx context.Context,
	params CreateAssignmentParams,
) (*model.Assignment, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, mapSchedulingRetryableError(err)
	}
	defer tx.Rollback()

	assignment, err := createAssignmentWithScheduleCheck(ctx, tx, params)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, mapSchedulingRetryableError(err)
	}

	return assignment, nil
}

func createAssignmentWithScheduleCheck(
	ctx context.Context,
	tx *sql.Tx,
	params CreateAssignmentParams,
) (*model.Assignment, error) {
	slotID, positionID, _, err := resolveAssignmentRef(
		ctx,
		tx,
		params.SlotID,
		params.PositionID,
		params.TemplateShiftID,
	)
	if err != nil {
		return nil, err
	}

	if exists, err := assignmentExistsForUserSlot(ctx, tx, params.PublicationID, params.UserID, slotID); err != nil {
		return nil, err
	} else if exists {
		return nil, ErrAssignmentUserAlreadyInSlot
	}

	if err := LockAndCheckUserStatus(ctx, tx, params.PublicationID, params.UserID); err != nil {
		return nil, err
	}

	const query = `
		INSERT INTO assignments (
			publication_id,
			user_id,
			slot_id,
			position_id,
			created_at
		)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING
			id,
			publication_id,
			user_id,
			slot_id,
			position_id,
			created_at;
	`

	assignment, err := scanAssignment(tx.QueryRowContext(
		ctx,
		query,
		params.PublicationID,
		params.UserID,
		slotID,
		positionID,
		params.CreatedAt,
	))
	if err != nil {
		return nil, mapAssignmentWriteError(err)
	}

	return assignment, nil
}

func (r *PublicationRepository) DeleteAssignment(ctx context.Context, params DeleteAssignmentParams) error {
	const query = `
		DELETE FROM assignments
		WHERE publication_id = $1 AND id = $2;
	`

	result, err := r.db.ExecContext(ctx, query, params.PublicationID, params.AssignmentID)
	if err != nil {
		return err
	}

	// Idempotent: deleting a non-existent assignment is not an error.
	_, err = result.RowsAffected()
	return err
}

func (r *PublicationRepository) ReplaceAssignments(
	ctx context.Context,
	params ReplaceAssignmentsParams,
) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	const deleteQuery = `
		DELETE FROM assignments
		WHERE publication_id = $1;
	`

	if _, err := tx.ExecContext(ctx, deleteQuery, params.PublicationID); err != nil {
		return err
	}

	const insertQuery = `
		INSERT INTO assignments (
			publication_id,
			user_id,
			slot_id,
			position_id,
			created_at
		)
		VALUES ($1, $2, $3, $4, $5);
	`

	for _, assignment := range params.Assignments {
		slotID, positionID, _, err := resolveAssignmentRef(
			ctx,
			tx,
			assignment.SlotID,
			assignment.PositionID,
			assignment.TemplateShiftID,
		)
		if err != nil {
			return err
		}

		if _, err := tx.ExecContext(
			ctx,
			insertQuery,
			params.PublicationID,
			assignment.UserID,
			slotID,
			positionID,
			params.CreatedAt,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// ActivatePublicationResult captures the updated publication plus the number
// of pending shift-change requests that were bulk-expired in the same
// transaction as the state flip.
type ActivatePublicationResult struct {
	Publication       *model.Publication
	ExpiredRequestIDs []int64
}

func (r *PublicationRepository) ActivatePublication(
	ctx context.Context,
	params ActivatePublicationParams,
) (*ActivatePublicationResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	const updateQuery = `
		UPDATE publications p
		SET state = 'ACTIVE', activated_at = $2, updated_at = $2
		FROM templates t
		WHERE p.id = $1
			AND p.template_id = t.id
			AND p.state = 'PUBLISHED'
		RETURNING
			p.id,
			p.template_id,
			t.name,
			p.name,
			p.state,
			p.submission_start_at,
			p.submission_end_at,
			p.planned_active_from,
			p.activated_at,
			p.ended_at,
			p.created_at,
			p.updated_at;
	`

	publication, err := scanPublication(tx.QueryRowContext(ctx, updateQuery, params.ID, params.Now))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, sql.ErrNoRows
	}
	if err != nil {
		return nil, err
	}

	const expireQuery = `
		UPDATE shift_change_requests
		SET state = 'expired', decided_at = $2
		WHERE publication_id = $1 AND state = 'pending'
		RETURNING id;
	`

	rows, err := tx.QueryContext(ctx, expireQuery, params.ID, params.Now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	expiredIDs := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		expiredIDs = append(expiredIDs, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &ActivatePublicationResult{
		Publication:       publication,
		ExpiredRequestIDs: expiredIDs,
	}, nil
}

func (r *PublicationRepository) PublishPublication(
	ctx context.Context,
	params PublishPublicationParams,
) (*model.Publication, error) {
	const query = `
		UPDATE publications p
		SET state = 'PUBLISHED', updated_at = $2
		FROM templates t
		WHERE p.id = $1
			AND p.template_id = t.id
			AND p.state IN ('DRAFT', 'COLLECTING', 'ASSIGNING')
		RETURNING
			p.id,
			p.template_id,
			t.name,
			p.name,
			p.state,
			p.submission_start_at,
			p.submission_end_at,
			p.planned_active_from,
			p.activated_at,
			p.ended_at,
			p.created_at,
			p.updated_at;
	`

	publication, err := scanPublication(r.db.QueryRowContext(ctx, query, params.ID, params.Now))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, sql.ErrNoRows
	}
	if err != nil {
		return nil, err
	}

	return publication, nil
}

func (r *PublicationRepository) EndPublication(
	ctx context.Context,
	params EndPublicationParams,
) (*model.Publication, error) {
	const query = `
		UPDATE publications p
		SET state = 'ENDED', ended_at = $2, updated_at = $2
		FROM templates t
		WHERE p.id = $1
			AND p.template_id = t.id
			AND p.state = 'ACTIVE'
		RETURNING
			p.id,
			p.template_id,
			t.name,
			p.name,
			p.state,
			p.submission_start_at,
			p.submission_end_at,
			p.planned_active_from,
			p.activated_at,
			p.ended_at,
			p.created_at,
			p.updated_at;
	`

	publication, err := scanPublication(r.db.QueryRowContext(ctx, query, params.ID, params.Now))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, sql.ErrNoRows
	}
	if err != nil {
		return nil, err
	}

	return publication, nil
}

func (r *PublicationRepository) ListPublicationShifts(
	ctx context.Context,
	publicationID int64,
) ([]*model.PublicationShift, error) {
	const query = `
		SELECT
			tsp.id,
			ts.id,
			ts.template_id,
			ts.weekday,
			TO_CHAR(ts.start_time, 'HH24:MI'),
			TO_CHAR(ts.end_time, 'HH24:MI'),
			tsp.position_id,
			pos.name,
			tsp.required_headcount,
			ts.created_at,
			ts.updated_at
		FROM publications p
		INNER JOIN template_slots ts ON ts.template_id = p.template_id
		INNER JOIN template_slot_positions tsp ON tsp.slot_id = ts.id
		INNER JOIN positions pos ON pos.id = tsp.position_id
		WHERE p.id = $1
		ORDER BY ts.weekday ASC, ts.start_time ASC, ts.id ASC, tsp.position_id ASC;
	`

	rows, err := r.db.QueryContext(ctx, query, publicationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	shifts := make([]*model.PublicationShift, 0)
	for rows.Next() {
		shift := &model.PublicationShift{}
		if err := rows.Scan(
			&shift.ID,
			&shift.SlotID,
			&shift.TemplateID,
			&shift.Weekday,
			&shift.StartTime,
			&shift.EndTime,
			&shift.PositionID,
			&shift.PositionName,
			&shift.RequiredHeadcount,
			&shift.CreatedAt,
			&shift.UpdatedAt,
		); err != nil {
			return nil, err
		}
		shifts = append(shifts, shift)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return shifts, nil
}

func (r *PublicationRepository) ListAssignmentCandidates(
	ctx context.Context,
	publicationID int64,
) ([]*model.AssignmentCandidate, error) {
	const query = `
		SELECT
			asub.slot_id,
			asub.position_id,
			u.id,
			u.name,
			u.email
		FROM availability_submissions asub
		INNER JOIN users u ON u.id = asub.user_id
		INNER JOIN user_positions up
			ON up.user_id = asub.user_id
			AND up.position_id = asub.position_id
		WHERE asub.publication_id = $1
			AND u.status = 'active'
		ORDER BY asub.slot_id ASC, asub.position_id ASC, u.id ASC;
	`

	rows, err := r.db.QueryContext(ctx, query, publicationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	candidates := make([]*model.AssignmentCandidate, 0)
	for rows.Next() {
		candidate := &model.AssignmentCandidate{}
		if err := rows.Scan(
			&candidate.SlotID,
			&candidate.PositionID,
			&candidate.UserID,
			&candidate.Name,
			&candidate.Email,
		); err != nil {
			return nil, err
		}
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return candidates, nil
}

// LockAndCheckUserStatus serializes assignment mutations for one publication/user
// and re-checks users.status inside the caller's transaction. The users row lock
// can still surface a Postgres deadlock, so callers preserve ErrSchedulingRetryable.
func LockAndCheckUserStatus(
	ctx context.Context,
	tx *sql.Tx,
	publicationID, userID int64,
) error {
	if tx == nil {
		return errors.New("nil transaction")
	}

	if err := lockUserSchedule(ctx, tx, publicationID, userID); err != nil {
		return err
	}

	status, err := getUserStatus(ctx, tx, userID)
	if err != nil {
		return err
	}
	if status != model.UserStatusActive {
		return ErrUserDisabled
	}

	return nil
}

func lockUserSchedule(ctx context.Context, tx *sql.Tx, publicationID, userID int64) error {
	var locked bool
	err := tx.QueryRowContext(ctx, `SELECT pg_advisory_xact_lock($1) IS NOT NULL;`, scheduleAdvisoryLockKey(publicationID, userID)).Scan(&locked)
	return mapSchedulingRetryableError(err)
}

func scheduleAdvisoryLockKey(publicationID, userID int64) int64 {
	hash := fnv.New64a()
	var buf [16]byte
	binary.BigEndian.PutUint64(buf[:8], uint64(publicationID))
	binary.BigEndian.PutUint64(buf[8:], uint64(userID))
	_, _ = hash.Write(buf[:])
	return int64(hash.Sum64())
}

func getUserStatus(ctx context.Context, db dbtx, userID int64) (model.UserStatus, error) {
	const query = `
		SELECT status
		FROM users
		WHERE id = $1
		FOR UPDATE;
	`

	var status model.UserStatus
	err := db.QueryRowContext(ctx, query, userID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrUserNotFound
	}
	if err != nil {
		return "", mapSchedulingRetryableError(err)
	}
	return status, nil
}

func assignmentExistsForUserSlot(
	ctx context.Context,
	db dbtx,
	publicationID, userID, slotID int64,
) (bool, error) {
	const query = `
		SELECT 1
		FROM assignments
		WHERE publication_id = $1
			AND user_id = $2
			AND slot_id = $3;
	`

	var one int
	err := db.QueryRowContext(ctx, query, publicationID, userID, slotID).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, mapSchedulingRetryableError(err)
	}
	return true, nil
}

func (r *PublicationRepository) GetAssignmentBoardView(
	ctx context.Context,
	publicationID int64,
) (map[int64]*AssignmentBoardSlotView, error) {
	const query = `
		SELECT
			ts.id,
			ts.template_id,
			ts.weekday,
			TO_CHAR(ts.start_time, 'HH24:MI'),
			TO_CHAR(ts.end_time, 'HH24:MI'),
			tsp.position_id,
			p.name,
			tsp.required_headcount,
			COALESCE(candidates.users, '[]'::jsonb),
			COALESCE(non_candidates.users, '[]'::jsonb),
			COALESCE(assignments.users, '[]'::jsonb)
		FROM publications pub
		INNER JOIN template_slots ts ON ts.template_id = pub.template_id
		INNER JOIN template_slot_positions tsp ON tsp.slot_id = ts.id
		INNER JOIN positions p ON p.id = tsp.position_id
		LEFT JOIN LATERAL (
			SELECT jsonb_agg(
				jsonb_build_object(
					'user_id', u.id,
					'name', u.name,
					'email', u.email
				)
				ORDER BY u.id
			) AS users
			FROM availability_submissions asub
			INNER JOIN users u ON u.id = asub.user_id
			WHERE asub.publication_id = pub.id
				AND asub.slot_id = ts.id
				AND asub.position_id = tsp.position_id
		) candidates ON true
		LEFT JOIN LATERAL (
			SELECT jsonb_agg(
				jsonb_build_object(
					'user_id', u.id,
					'name', u.name,
					'email', u.email
				)
				ORDER BY u.id
			) AS users
			FROM user_positions up
			INNER JOIN users u ON u.id = up.user_id
			WHERE up.position_id = tsp.position_id
				AND u.status = 'active'
				AND NOT EXISTS (
					SELECT 1
					FROM availability_submissions asub
					WHERE asub.publication_id = pub.id
						AND asub.slot_id = ts.id
						AND asub.position_id = tsp.position_id
						AND asub.user_id = u.id
				)
				AND NOT EXISTS (
					SELECT 1
					FROM assignments a
					WHERE a.publication_id = pub.id
						AND a.slot_id = ts.id
						AND a.position_id = tsp.position_id
						AND a.user_id = u.id
				)
		) non_candidates ON true
		LEFT JOIN LATERAL (
			SELECT jsonb_agg(
				jsonb_build_object(
					'assignment_id', a.id,
					'user_id', u.id,
					'name', u.name,
					'email', u.email,
					'created_at', TO_CHAR(a.created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
				)
				ORDER BY u.id
			) AS users
			FROM assignments a
			INNER JOIN users u ON u.id = a.user_id
			WHERE a.publication_id = pub.id
				AND a.slot_id = ts.id
				AND a.position_id = tsp.position_id
		) assignments ON true
		WHERE pub.id = $1
		ORDER BY ts.weekday ASC, ts.start_time ASC, ts.id ASC, tsp.position_id ASC;
	`

	rows, err := r.db.QueryContext(ctx, query, publicationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	board := make(map[int64]*AssignmentBoardSlotView)
	for rows.Next() {
		var (
			slotID            int64
			templateID        int64
			weekday           int
			startTime         string
			endTime           string
			positionID        int64
			positionName      string
			requiredHeadcount int
			candidatesJSON    []byte
			nonCandidatesJSON []byte
			assignmentsJSON   []byte
		)
		if err := rows.Scan(
			&slotID,
			&templateID,
			&weekday,
			&startTime,
			&endTime,
			&positionID,
			&positionName,
			&requiredHeadcount,
			&candidatesJSON,
			&nonCandidatesJSON,
			&assignmentsJSON,
		); err != nil {
			return nil, err
		}

		slotView := board[slotID]
		if slotView == nil {
			slotView = &AssignmentBoardSlotView{
				Slot: &model.TemplateSlot{
					ID:         slotID,
					TemplateID: templateID,
					Weekday:    weekday,
					StartTime:  startTime,
					EndTime:    endTime,
				},
				Positions: make(map[int64]*AssignmentBoardPositionView),
			}
			board[slotID] = slotView
		}

		candidates, err := decodeAssignmentBoardCandidates(candidatesJSON, slotID, positionID)
		if err != nil {
			return nil, fmt.Errorf("decode candidates for slot %d position %d: %w", slotID, positionID, err)
		}
		nonCandidates, err := decodeAssignmentBoardCandidates(nonCandidatesJSON, slotID, positionID)
		if err != nil {
			return nil, fmt.Errorf("decode non-candidates for slot %d position %d: %w", slotID, positionID, err)
		}
		assignments, err := decodeAssignmentBoardAssignments(assignmentsJSON, slotID, positionID)
		if err != nil {
			return nil, fmt.Errorf("decode assignments for slot %d position %d: %w", slotID, positionID, err)
		}

		slotView.Positions[positionID] = &AssignmentBoardPositionView{
			Position: &model.Position{
				ID:   positionID,
				Name: positionName,
			},
			RequiredHeadcount:     requiredHeadcount,
			Candidates:            candidates,
			NonCandidateQualified: nonCandidates,
			Assignments:           assignments,
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return board, nil
}

func (r *PublicationRepository) ListQualifiedUsersForPositions(
	ctx context.Context,
	positionIDs []int64,
) (map[int64][]*model.AssignmentCandidate, error) {
	uniquePositionIDs := uniqueSortedPositionIDs(positionIDs)
	if len(uniquePositionIDs) == 0 {
		return make(map[int64][]*model.AssignmentCandidate), nil
	}

	const query = `
		SELECT
			up.position_id,
			u.id,
			u.name,
			u.email
		FROM user_positions up
		INNER JOIN users u ON u.id = up.user_id
		WHERE up.position_id = ANY($1)
			AND u.status = 'active'
		ORDER BY up.position_id ASC, u.id ASC;
	`

	rows, err := r.db.QueryContext(ctx, query, pq.Array(uniquePositionIDs))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	qualifiedByPosition := make(map[int64][]*model.AssignmentCandidate, len(uniquePositionIDs))
	for rows.Next() {
		var (
			positionID int64
			candidate  model.AssignmentCandidate
		)
		if err := rows.Scan(
			&positionID,
			&candidate.UserID,
			&candidate.Name,
			&candidate.Email,
		); err != nil {
			return nil, err
		}
		qualifiedByPosition[positionID] = append(
			qualifiedByPosition[positionID],
			&model.AssignmentCandidate{
				UserID: candidate.UserID,
				Name:   candidate.Name,
				Email:  candidate.Email,
			},
		)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return qualifiedByPosition, nil
}

// GetAssignment loads one assignment by id, returning ErrAssignmentNotFound
// when the row no longer exists.
func (r *PublicationRepository) GetAssignment(
	ctx context.Context,
	id int64,
) (*model.Assignment, error) {
	const query = `
		SELECT
			a.id,
			a.publication_id,
			a.user_id,
			a.slot_id,
			a.position_id,
			a.created_at
		FROM assignments a
		WHERE id = $1;
	`

	assignment := &model.Assignment{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&assignment.ID,
		&assignment.PublicationID,
		&assignment.UserID,
		&assignment.SlotID,
		&assignment.PositionID,
		&assignment.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrAssignmentNotFound
	}
	if err != nil {
		return nil, err
	}
	return assignment, nil
}

func (r *PublicationRepository) ListPublicationAssignments(
	ctx context.Context,
	publicationID int64,
) ([]*model.AssignmentParticipant, error) {
	const query = `
		SELECT
			a.id,
			a.slot_id,
			a.position_id,
			u.id,
			u.name,
			u.email,
			a.created_at
		FROM assignments a
		INNER JOIN users u ON u.id = a.user_id
		WHERE a.publication_id = $1
		ORDER BY a.slot_id ASC, a.position_id ASC, u.id ASC;
	`

	rows, err := r.db.QueryContext(ctx, query, publicationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	assignments := make([]*model.AssignmentParticipant, 0)
	for rows.Next() {
		assignment := &model.AssignmentParticipant{}
		if err := rows.Scan(
			&assignment.AssignmentID,
			&assignment.SlotID,
			&assignment.PositionID,
			&assignment.UserID,
			&assignment.Name,
			&assignment.Email,
			&assignment.CreatedAt,
		); err != nil {
			return nil, err
		}
		assignments = append(assignments, assignment)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return assignments, nil
}

type assignmentBoardCandidateJSON struct {
	UserID int64  `json:"user_id"`
	Name   string `json:"name"`
	Email  string `json:"email"`
}

type assignmentBoardAssignmentJSON struct {
	AssignmentID int64  `json:"assignment_id"`
	UserID       int64  `json:"user_id"`
	Name         string `json:"name"`
	Email        string `json:"email"`
	CreatedAt    string `json:"created_at"`
}

func decodeAssignmentBoardCandidates(
	raw []byte,
	slotID, positionID int64,
) ([]*model.AssignmentCandidate, error) {
	var decoded []assignmentBoardCandidateJSON
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, err
	}

	candidates := make([]*model.AssignmentCandidate, 0, len(decoded))
	for _, candidate := range decoded {
		candidates = append(candidates, &model.AssignmentCandidate{
			SlotID:     slotID,
			PositionID: positionID,
			UserID:     candidate.UserID,
			Name:       candidate.Name,
			Email:      candidate.Email,
		})
	}

	return candidates, nil
}

func decodeAssignmentBoardAssignments(
	raw []byte,
	slotID, positionID int64,
) ([]*model.AssignmentParticipant, error) {
	var decoded []assignmentBoardAssignmentJSON
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, err
	}

	assignments := make([]*model.AssignmentParticipant, 0, len(decoded))
	for _, assignment := range decoded {
		createdAt, err := time.Parse(time.RFC3339, assignment.CreatedAt)
		if err != nil {
			return nil, err
		}
		assignments = append(assignments, &model.AssignmentParticipant{
			AssignmentID: assignment.AssignmentID,
			SlotID:       slotID,
			PositionID:   positionID,
			UserID:       assignment.UserID,
			Name:         assignment.Name,
			Email:        assignment.Email,
			CreatedAt:    createdAt,
		})
	}

	return assignments, nil
}

func scanAssignment(row scanner) (*model.Assignment, error) {
	assignment := &model.Assignment{}
	err := row.Scan(
		&assignment.ID,
		&assignment.PublicationID,
		&assignment.UserID,
		&assignment.SlotID,
		&assignment.PositionID,
		&assignment.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return assignment, nil
}

func getAssignmentByKey(
	ctx context.Context,
	db dbtx,
	publicationID, userID, slotID int64,
) (*model.Assignment, error) {
	const query = `
		SELECT
			a.id,
			a.publication_id,
			a.user_id,
			a.slot_id,
			a.position_id,
			a.created_at
		FROM assignments a
		WHERE a.publication_id = $1 AND a.user_id = $2 AND a.slot_id = $3;
	`

	return scanAssignment(db.QueryRowContext(ctx, query, publicationID, userID, slotID))
}

func resolveAssignmentRef(
	ctx context.Context,
	db dbtx,
	slotID, positionID, templateShiftID int64,
) (int64, int64, int64, error) {
	switch {
	case slotID > 0 && positionID > 0:
		entryID, err := getTemplateSlotPositionEntryID(ctx, db, slotID, positionID)
		if err != nil {
			return 0, 0, 0, err
		}
		return slotID, positionID, entryID, nil
	case templateShiftID > 0:
		resolvedSlotID, resolvedPositionID, err := getTemplateSlotPositionPairByEntryID(ctx, db, templateShiftID)
		if err != nil {
			return 0, 0, 0, err
		}
		return resolvedSlotID, resolvedPositionID, templateShiftID, nil
	default:
		return 0, 0, 0, ErrTemplateSlotPositionNotFound
	}
}

func getTemplateSlotPositionEntryID(
	ctx context.Context,
	db dbtx,
	slotID, positionID int64,
) (int64, error) {
	const query = `
		SELECT id
		FROM template_slot_positions
		WHERE slot_id = $1 AND position_id = $2;
	`

	var entryID int64
	err := db.QueryRowContext(ctx, query, slotID, positionID).Scan(&entryID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrTemplateSlotPositionNotFound
	}
	if err != nil {
		return 0, err
	}

	return entryID, nil
}

func getTemplateSlotPositionPairByEntryID(
	ctx context.Context,
	db dbtx,
	entryID int64,
) (int64, int64, error) {
	const query = `
		SELECT slot_id, position_id
		FROM template_slot_positions
		WHERE id = $1;
	`

	var (
		slotID     int64
		positionID int64
	)
	err := db.QueryRowContext(ctx, query, entryID).Scan(&slotID, &positionID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, 0, ErrTemplateShiftNotFound
	}
	if err != nil {
		return 0, 0, err
	}

	return slotID, positionID, nil
}

func mapAssignmentWriteError(err error) error {
	err = mapSchedulingRetryableError(err)
	var pqErr *pq.Error
	if !errors.As(err, &pqErr) {
		return err
	}

	if pqErr.Code == "23505" && pqErr.Constraint == "assignments_publication_user_slot_key" {
		return ErrAssignmentUserAlreadyInSlot
	}

	return err
}

func mapSchedulingRetryableError(err error) error {
	if err == nil {
		return nil
	}

	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == "40P01" {
		return ErrSchedulingRetryable
	}

	return err
}
