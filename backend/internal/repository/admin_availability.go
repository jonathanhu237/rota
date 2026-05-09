package repository

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/lib/pq"
)

type ListAdminAvailabilityEmployeesParams struct {
	PublicationID int64
	Offset        int
	Limit         int
	Search        string
}

type AdminAvailabilityEmployee struct {
	UserID         int64
	Name           string
	Email          string
	Positions      []*model.Position
	SubmittedCount int
}

type ReplaceAdminAvailabilitySubmissionsParams struct {
	PublicationID int64
	UserID        int64
	Submissions   []model.SlotRef
	CreatedAt     time.Time
	Now           time.Time
}

type ReplaceAdminAvailabilitySubmissionsResult struct {
	Added   []model.SlotRef
	Removed []model.SlotRef
}

var ErrInvalidAvailabilityCell = errors.New("invalid availability cell")

func (r *PublicationRepository) ListAdminAvailabilityEmployees(
	ctx context.Context,
	params ListAdminAvailabilityEmployeesParams,
) ([]*AdminAvailabilityEmployee, int, error) {
	exists, err := publicationExists(ctx, r.db, params.PublicationID)
	if err != nil {
		return nil, 0, err
	}
	if !exists {
		return nil, 0, ErrPublicationNotFound
	}

	search := strings.TrimSpace(params.Search)
	const countQuery = `
		WITH template_positions AS (
			SELECT DISTINCT tsp.position_id
			FROM publications pub
			INNER JOIN template_slots ts ON ts.template_id = pub.template_id
			INNER JOIN template_slot_positions tsp ON tsp.slot_id = ts.id
			WHERE pub.id = $1
		),
		eligible_users AS (
			SELECT u.id
			FROM users u
			INNER JOIN user_positions up ON up.user_id = u.id
			INNER JOIN template_positions tp ON tp.position_id = up.position_id
			WHERE u.status = 'active'
				AND u.is_admin = false
				AND ($2 = '' OR u.name ILIKE '%' || $2 || '%' OR u.email ILIKE '%' || $2 || '%')
			GROUP BY u.id
		)
		SELECT COUNT(*)
		FROM eligible_users;
	`

	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, params.PublicationID, search).Scan(&total); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*AdminAvailabilityEmployee{}, 0, nil
	}

	const query = `
		WITH template_positions AS (
			SELECT DISTINCT tsp.position_id
			FROM publications pub
			INNER JOIN template_slots ts ON ts.template_id = pub.template_id
			INNER JOIN template_slot_positions tsp ON tsp.slot_id = ts.id
			WHERE pub.id = $1
		),
		eligible_users AS (
			SELECT
				u.id,
				u.name,
				u.email,
				COUNT(DISTINCT asub.id)::int AS submitted_count
			FROM users u
			INNER JOIN user_positions up ON up.user_id = u.id
			INNER JOIN template_positions tp ON tp.position_id = up.position_id
			LEFT JOIN availability_submissions asub
				ON asub.publication_id = $1
				AND asub.user_id = u.id
			WHERE u.status = 'active'
				AND u.is_admin = false
				AND ($2 = '' OR u.name ILIKE '%' || $2 || '%' OR u.email ILIKE '%' || $2 || '%')
			GROUP BY u.id, u.name, u.email
			ORDER BY u.id ASC
			LIMIT $3 OFFSET $4
		)
		SELECT
			eu.id,
			eu.name,
			eu.email,
			eu.submitted_count,
			pos.id,
			pos.name,
			pos.description,
			pos.created_at,
			pos.updated_at
		FROM eligible_users eu
		INNER JOIN user_positions up ON up.user_id = eu.id
		INNER JOIN template_positions tp ON tp.position_id = up.position_id
		INNER JOIN positions pos ON pos.id = up.position_id
		ORDER BY eu.id ASC, pos.id ASC;
	`

	rows, err := r.db.QueryContext(ctx, query, params.PublicationID, search, params.Limit, params.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	employees := make([]*AdminAvailabilityEmployee, 0)
	employeeByID := make(map[int64]*AdminAvailabilityEmployee)
	for rows.Next() {
		var (
			userID         int64
			name           string
			email          string
			submittedCount int
			position       model.Position
		)
		if err := rows.Scan(
			&userID,
			&name,
			&email,
			&submittedCount,
			&position.ID,
			&position.Name,
			&position.Description,
			&position.CreatedAt,
			&position.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}

		employee := employeeByID[userID]
		if employee == nil {
			employee = &AdminAvailabilityEmployee{
				UserID:         userID,
				Name:           name,
				Email:          email,
				SubmittedCount: submittedCount,
				Positions:      make([]*model.Position, 0, 1),
			}
			employeeByID[userID] = employee
			employees = append(employees, employee)
		}
		positionCopy := position
		employee.Positions = append(employee.Positions, &positionCopy)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return employees, total, nil
}

func (r *PublicationRepository) GetAdminAvailabilityUser(
	ctx context.Context,
	publicationID, userID int64,
) (*model.User, []*model.Position, error) {
	exists, err := publicationExists(ctx, r.db, publicationID)
	if err != nil {
		return nil, nil, err
	}
	if !exists {
		return nil, nil, ErrPublicationNotFound
	}

	user, positions, err := r.getAdminAvailabilityUser(ctx, r.db, publicationID, userID, false)
	if err != nil {
		return nil, nil, err
	}

	return user, positions, nil
}

func (r *PublicationRepository) ReplaceAdminAvailabilitySubmissions(
	ctx context.Context,
	params ReplaceAdminAvailabilitySubmissionsParams,
) (*ReplaceAdminAvailabilitySubmissionsResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, mapSchedulingRetryableError(err)
	}
	defer tx.Rollback()

	now := params.Now
	if now.IsZero() {
		now = params.CreatedAt
	}

	if err := LockAndCheckUserStatus(ctx, tx, params.PublicationID, params.UserID); err != nil {
		return nil, err
	}

	publication, err := getPublicationByIDForUpdate(ctx, tx, params.PublicationID)
	if err != nil {
		return nil, err
	}
	effectiveState := model.ResolvePublicationState(publication, now)
	if effectiveState != model.PublicationStateCollecting && effectiveState != model.PublicationStateAssigning {
		return nil, model.ErrPublicationNotMutable
	}
	if publication.State != effectiveState {
		if err := updatePublicationStateIfNeeded(ctx, tx, params.PublicationID, &effectiveState, now); err != nil {
			return nil, err
		}
	}

	if _, _, err := r.getAdminAvailabilityUser(ctx, tx, params.PublicationID, params.UserID, true); err != nil {
		return nil, err
	}
	if err := validateAdminAvailabilityTarget(ctx, tx, params.PublicationID, params.UserID, params.Submissions); err != nil {
		return nil, err
	}

	const currentQuery = `
		SELECT slot_id, weekday
		FROM availability_submissions
		WHERE publication_id = $1 AND user_id = $2
		FOR UPDATE;
	`

	rows, err := tx.QueryContext(ctx, currentQuery, params.PublicationID, params.UserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	current := make(map[model.SlotRef]struct{})
	for rows.Next() {
		var slot model.SlotRef
		if err := rows.Scan(&slot.SlotID, &slot.Weekday); err != nil {
			return nil, err
		}
		current[slot] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	target := make(map[model.SlotRef]struct{}, len(params.Submissions))
	for _, slot := range params.Submissions {
		target[slot] = struct{}{}
	}

	removed := make([]model.SlotRef, 0)
	for slot := range current {
		if _, ok := target[slot]; !ok {
			removed = append(removed, slot)
		}
	}
	added := make([]model.SlotRef, 0)
	for slot := range target {
		if _, ok := current[slot]; !ok {
			added = append(added, slot)
		}
	}
	sortSlotRefs(removed)
	sortSlotRefs(added)

	const deleteQuery = `
		DELETE FROM availability_submissions
		WHERE publication_id = $1 AND user_id = $2 AND slot_id = $3 AND weekday = $4;
	`
	for _, slot := range removed {
		if _, err := tx.ExecContext(ctx, deleteQuery, params.PublicationID, params.UserID, slot.SlotID, slot.Weekday); err != nil {
			return nil, err
		}
	}

	const insertQuery = `
		INSERT INTO availability_submissions (
			publication_id,
			user_id,
			slot_id,
			weekday,
			created_at
		)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (publication_id, user_id, slot_id, weekday) DO NOTHING;
	`
	for _, slot := range added {
		if _, err := tx.ExecContext(ctx, insertQuery, params.PublicationID, params.UserID, slot.SlotID, slot.Weekday, params.CreatedAt); err != nil {
			return nil, mapAvailabilitySubmissionWriteError(err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, mapSchedulingRetryableError(err)
	}

	return &ReplaceAdminAvailabilitySubmissionsResult{
		Added:   added,
		Removed: removed,
	}, nil
}

func (r *PublicationRepository) getAdminAvailabilityUser(
	ctx context.Context,
	db dbtx,
	publicationID, userID int64,
	lockPositions bool,
) (*model.User, []*model.Position, error) {
	const userQuery = `
		SELECT id, email, password_hash, name, is_admin, status, version, language_preference, theme_preference
		FROM users
		WHERE id = $1;
	`

	user := &model.User{}
	err := scanUser(db.QueryRowContext(ctx, userQuery, userID), user)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, ErrUserNotFound
	}
	if err != nil {
		return nil, nil, err
	}
	if user.IsAdmin {
		return nil, nil, ErrUserNotFound
	}
	if user.Status == model.UserStatusDisabled {
		return nil, nil, ErrUserDisabled
	}
	if user.Status != model.UserStatusActive {
		return nil, nil, ErrUserNotFound
	}

	positions, err := r.listRelevantUserPositions(ctx, db, publicationID, userID, lockPositions)
	if err != nil {
		return nil, nil, err
	}
	if len(positions) == 0 {
		return nil, nil, ErrUserNotFound
	}

	return user, positions, nil
}

func (r *PublicationRepository) listRelevantUserPositions(
	ctx context.Context,
	db dbtx,
	publicationID, userID int64,
	lockPositions bool,
) ([]*model.Position, error) {
	query := `
		WITH template_positions AS (
			SELECT DISTINCT tsp.position_id
			FROM publications pub
			INNER JOIN template_slots ts ON ts.template_id = pub.template_id
			INNER JOIN template_slot_positions tsp ON tsp.slot_id = ts.id
			WHERE pub.id = $1
		)
		SELECT p.id, p.name, p.description, p.created_at, p.updated_at
		FROM user_positions up
		INNER JOIN template_positions tp ON tp.position_id = up.position_id
		INNER JOIN positions p ON p.id = up.position_id
		WHERE up.user_id = $2
		ORDER BY p.id ASC
	`
	if lockPositions {
		query += `
		FOR UPDATE OF up;
	`
	} else {
		query += `;`
	}

	rows, err := db.QueryContext(ctx, query, publicationID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	positions := make([]*model.Position, 0)
	for rows.Next() {
		position := &model.Position{}
		if err := rows.Scan(
			&position.ID,
			&position.Name,
			&position.Description,
			&position.CreatedAt,
			&position.UpdatedAt,
		); err != nil {
			return nil, err
		}
		positions = append(positions, position)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return positions, nil
}

func getPublicationByIDForUpdate(ctx context.Context, tx *sql.Tx, id int64) (*model.Publication, error) {
	const query = `
		SELECT
			p.id,
			p.template_id,
			t.name,
			p.name,
			p.description,
			p.state,
			p.submission_start_at,
			p.submission_end_at,
			p.planned_active_from,
			p.planned_active_until,
			p.overtime_entry_window_hours,
			p.activated_at,
			p.created_at,
			p.updated_at
		FROM publications p
		INNER JOIN templates t ON t.id = p.template_id
		WHERE p.id = $1
		FOR UPDATE OF p;
	`

	publication, err := scanPublication(tx.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrPublicationNotFound
	}
	if err != nil {
		return nil, err
	}

	return publication, nil
}

func validateAdminAvailabilityTarget(
	ctx context.Context,
	tx *sql.Tx,
	publicationID, userID int64,
	submissions []model.SlotRef,
) error {
	if len(submissions) == 0 {
		return nil
	}

	slotIDs := make([]int64, 0, len(submissions))
	weekdays := make([]int, 0, len(submissions))
	for _, submission := range submissions {
		slotIDs = append(slotIDs, submission.SlotID)
		weekdays = append(weekdays, submission.Weekday)
	}

	const query = `
		WITH target(slot_id, weekday) AS (
			SELECT *
			FROM UNNEST($3::BIGINT[], $4::INT[])
		)
		SELECT
			target.slot_id,
			target.weekday,
			EXISTS (
				SELECT 1
				FROM publications pub
				INNER JOIN template_slots ts
					ON ts.template_id = pub.template_id
					AND ts.id = target.slot_id
				INNER JOIN template_slot_weekdays tsw
					ON tsw.slot_id = ts.id
					AND tsw.weekday = target.weekday
				WHERE pub.id = $1
			) AS in_template,
			EXISTS (
				SELECT 1
				FROM publications pub
				INNER JOIN template_slots ts
					ON ts.template_id = pub.template_id
					AND ts.id = target.slot_id
				INNER JOIN template_slot_weekdays tsw
					ON tsw.slot_id = ts.id
					AND tsw.weekday = target.weekday
				INNER JOIN template_slot_positions tsp ON tsp.slot_id = ts.id
				INNER JOIN user_positions up
					ON up.user_id = $2
					AND up.position_id = tsp.position_id
				WHERE pub.id = $1
			) AS eligible
		FROM target;
	`

	rows, err := tx.QueryContext(ctx, query, publicationID, userID, pq.Array(slotIDs), pq.Array(weekdays))
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			slotID     int64
			weekday    int
			inTemplate bool
			eligible   bool
		)
		if err := rows.Scan(&slotID, &weekday, &inTemplate, &eligible); err != nil {
			return err
		}
		if !inTemplate {
			return ErrInvalidAvailabilityCell
		}
		if !eligible {
			return model.ErrNotQualified
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	return nil
}

func mapAvailabilitySubmissionWriteError(err error) error {
	var pqErr *pq.Error
	if !errors.As(err, &pqErr) {
		return err
	}
	if pqErr.Code == "23503" {
		return ErrTemplateSlotNotFound
	}
	return err
}

func sortSlotRefs(slots []model.SlotRef) {
	sort.Slice(slots, func(i, j int) bool {
		if slots[i].Weekday != slots[j].Weekday {
			return slots[i].Weekday < slots[j].Weekday
		}
		return slots[i].SlotID < slots[j].SlotID
	})
}
