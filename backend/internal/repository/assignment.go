package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/lib/pq"
)

var ErrAssignmentNotFound = errors.New("assignment not found")

type CreateAssignmentParams struct {
	PublicationID   int64
	UserID          int64
	TemplateShiftID int64
	CreatedAt       time.Time
}

type DeleteAssignmentParams struct {
	PublicationID int64
	AssignmentID  int64
}

type ReplaceAssignmentParams struct {
	UserID          int64
	TemplateShiftID int64
}

type ReplaceAssignmentsParams struct {
	PublicationID int64
	Assignments   []ReplaceAssignmentParams
	CreatedAt     time.Time
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

func (r *PublicationRepository) GetUserByID(ctx context.Context, id int64) (*model.User, error) {
	const query = `
		SELECT id, email, password_hash, name, is_admin, status, version
		FROM users
		WHERE id = $1;
	`

	user := &model.User{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.Name,
		&user.IsAdmin,
		&user.Status,
		&user.Version,
	)
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
	const query = `
		INSERT INTO assignments (
			publication_id,
			user_id,
			template_shift_id,
			created_at
		)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (publication_id, user_id, template_shift_id) DO NOTHING
		RETURNING id, publication_id, user_id, template_shift_id, created_at;
	`

	assignment, err := scanAssignment(r.db.QueryRowContext(
		ctx,
		query,
		params.PublicationID,
		params.UserID,
		params.TemplateShiftID,
		params.CreatedAt,
	))
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return getAssignmentByKey(ctx, r.db, params.PublicationID, params.UserID, params.TemplateShiftID)
	case err != nil:
		return nil, err
	default:
		return assignment, nil
	}
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
			template_shift_id,
			created_at
		)
		VALUES ($1, $2, $3, $4);
	`

	for _, assignment := range params.Assignments {
		if _, err := tx.ExecContext(
			ctx,
			insertQuery,
			params.PublicationID,
			assignment.UserID,
			assignment.TemplateShiftID,
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
			ts.id,
			ts.template_id,
			ts.weekday,
			TO_CHAR(ts.start_time, 'HH24:MI'),
			TO_CHAR(ts.end_time, 'HH24:MI'),
			ts.position_id,
			pos.name,
			ts.required_headcount,
			ts.created_at,
			ts.updated_at
		FROM publications p
		INNER JOIN template_shifts ts ON ts.template_id = p.template_id
		INNER JOIN positions pos ON pos.id = ts.position_id
		WHERE p.id = $1
		ORDER BY ts.weekday ASC, ts.start_time ASC, ts.id ASC;
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
			asub.template_shift_id,
			u.id,
			u.name,
			u.email
		FROM availability_submissions asub
		INNER JOIN users u ON u.id = asub.user_id
		WHERE asub.publication_id = $1
		ORDER BY asub.template_shift_id ASC, u.id ASC;
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
			&candidate.TemplateShiftID,
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
		SELECT id, publication_id, user_id, template_shift_id, created_at
		FROM assignments
		WHERE id = $1;
	`

	assignment := &model.Assignment{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&assignment.ID,
		&assignment.PublicationID,
		&assignment.UserID,
		&assignment.TemplateShiftID,
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
			a.template_shift_id,
			u.id,
			u.name,
			u.email,
			a.created_at
		FROM assignments a
		INNER JOIN users u ON u.id = a.user_id
		WHERE a.publication_id = $1
		ORDER BY a.template_shift_id ASC, u.id ASC;
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
			&assignment.TemplateShiftID,
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

func scanAssignment(row scanner) (*model.Assignment, error) {
	assignment := &model.Assignment{}
	err := row.Scan(
		&assignment.ID,
		&assignment.PublicationID,
		&assignment.UserID,
		&assignment.TemplateShiftID,
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
	publicationID, userID, templateShiftID int64,
) (*model.Assignment, error) {
	const query = `
		SELECT id, publication_id, user_id, template_shift_id, created_at
		FROM assignments
		WHERE publication_id = $1 AND user_id = $2 AND template_shift_id = $3;
	`

	return scanAssignment(db.QueryRowContext(ctx, query, publicationID, userID, templateShiftID))
}
