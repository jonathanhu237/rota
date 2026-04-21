package service

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/audit"
	"github.com/jonathanhu237/rota/backend/internal/audit/audittest"
	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/jonathanhu237/rota/backend/internal/repository"
)

type fixedClock struct {
	now time.Time
}

func (c fixedClock) Now() time.Time {
	return c.now
}

type publicationRepositoryStatefulMock struct {
	nextPublicationID     int64
	nextSubmissionID      int64
	nextAssignmentID      int64
	deletePublicationFunc func(ctx context.Context, params repository.DeletePublicationParams) error
	publications          map[int64]*model.Publication
	templates             map[int64]*model.Template
	templateShifts        map[int64]*model.TemplateShift
	users                 map[int64]*model.User
	submissions           map[string]*model.AvailabilitySubmission
	assignments           map[string]*model.Assignment
	qualifiedByUser       map[int64]map[int64]struct{}
}

func newPublicationRepositoryStatefulMock() *publicationRepositoryStatefulMock {
	return &publicationRepositoryStatefulMock{
		nextPublicationID: 2,
		nextSubmissionID:  2,
		nextAssignmentID:  2,
		publications: map[int64]*model.Publication{
			1: {},
		},
		templates: map[int64]*model.Template{
			1: {
				ID:   1,
				Name: "Core Week",
			},
		},
		templateShifts: map[int64]*model.TemplateShift{
			11: {
				ID:                11,
				TemplateID:        1,
				Weekday:           1,
				StartTime:         "09:00",
				EndTime:           "12:00",
				PositionID:        101,
				RequiredHeadcount: 2,
			},
			12: {
				ID:                12,
				TemplateID:        1,
				Weekday:           3,
				StartTime:         "13:00",
				EndTime:           "17:00",
				PositionID:        102,
				RequiredHeadcount: 1,
			},
		},
		users: map[int64]*model.User{
			7: {
				ID:     7,
				Email:  "alice@example.com",
				Name:   "Alice",
				Status: model.UserStatusActive,
			},
			8: {
				ID:     8,
				Email:  "bob@example.com",
				Name:   "Bob",
				Status: model.UserStatusActive,
			},
			9: {
				ID:     9,
				Email:  "cora@example.com",
				Name:   "Cora",
				Status: model.UserStatusDisabled,
			},
		},
		submissions: make(map[string]*model.AvailabilitySubmission),
		assignments: make(map[string]*model.Assignment),
		qualifiedByUser: map[int64]map[int64]struct{}{
			7: {
				101: {},
			},
		},
	}
}

func (m *publicationRepositoryStatefulMock) ListPaginated(
	ctx context.Context,
	params repository.ListPublicationsParams,
) ([]*model.Publication, int, error) {
	publications := make([]*model.Publication, 0, len(m.publications))
	for _, publication := range m.publications {
		publications = append(publications, clonePublication(publication))
	}

	sort.Slice(publications, func(i, j int) bool {
		if !publications[i].CreatedAt.Equal(publications[j].CreatedAt) {
			return publications[i].CreatedAt.After(publications[j].CreatedAt)
		}

		return publications[i].ID > publications[j].ID
	})

	start := params.Offset
	if start > len(publications) {
		start = len(publications)
	}

	end := start + params.Limit
	if end > len(publications) {
		end = len(publications)
	}

	return publications[start:end], len(publications), nil
}

func (m *publicationRepositoryStatefulMock) GetByID(
	ctx context.Context,
	id int64,
) (*model.Publication, error) {
	publication, ok := m.publications[id]
	if !ok {
		return nil, repository.ErrPublicationNotFound
	}

	return clonePublication(publication), nil
}

func (m *publicationRepositoryStatefulMock) GetCurrent(
	ctx context.Context,
) (*model.Publication, error) {
	publications := make([]*model.Publication, 0, len(m.publications))
	for _, publication := range m.publications {
		if publication.State == model.PublicationStateEnded {
			continue
		}
		publications = append(publications, publication)
	}

	if len(publications) == 0 {
		return nil, nil
	}

	sort.Slice(publications, func(i, j int) bool {
		return publications[i].ID < publications[j].ID
	})

	return clonePublication(publications[0]), nil
}

func (m *publicationRepositoryStatefulMock) GetUserByID(
	ctx context.Context,
	id int64,
) (*model.User, error) {
	user, ok := m.users[id]
	if !ok {
		return nil, repository.ErrUserNotFound
	}

	cloned := *user
	return &cloned, nil
}

func (m *publicationRepositoryStatefulMock) CreatePublication(
	ctx context.Context,
	params repository.CreatePublicationParams,
) (*model.Publication, error) {
	template, ok := m.templates[params.TemplateID]
	if !ok {
		return nil, repository.ErrTemplateNotFound
	}

	if !params.SubmissionStartAt.Before(params.SubmissionEndAt) ||
		params.PlannedActiveFrom.Before(params.SubmissionEndAt) {
		return nil, repository.ErrInvalidPublicationWindow
	}

	for _, publication := range m.publications {
		if publication.State != model.PublicationStateEnded {
			return nil, repository.ErrPublicationAlreadyExists
		}
	}

	now := params.CreatedAt
	publication := &model.Publication{
		ID:                m.nextPublicationID,
		TemplateID:        params.TemplateID,
		TemplateName:      template.Name,
		Name:              params.Name,
		State:             params.State,
		SubmissionStartAt: params.SubmissionStartAt,
		SubmissionEndAt:   params.SubmissionEndAt,
		PlannedActiveFrom: params.PlannedActiveFrom,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	template.IsLocked = true
	m.publications[publication.ID] = clonePublication(publication)
	m.nextPublicationID++

	return clonePublication(publication), nil
}

func (m *publicationRepositoryStatefulMock) DeletePublication(
	ctx context.Context,
	params repository.DeletePublicationParams,
) error {
	if m.deletePublicationFunc != nil {
		return m.deletePublicationFunc(ctx, params)
	}
	publication, ok := m.publications[params.ID]
	if !ok {
		return repository.ErrPublicationNotFound
	}
	if publication.State != model.PublicationStateDraft || !publication.SubmissionStartAt.After(params.Now) {
		return repository.ErrPublicationNotFound
	}

	delete(m.publications, params.ID)
	for key, submission := range m.submissions {
		if submission.PublicationID == params.ID {
			delete(m.submissions, key)
		}
	}

	return nil
}

func (m *publicationRepositoryStatefulMock) ListSubmissionShiftIDs(
	ctx context.Context,
	publicationID int64,
	userID int64,
) ([]int64, error) {
	if _, ok := m.publications[publicationID]; !ok {
		return nil, repository.ErrPublicationNotFound
	}

	shiftIDs := make([]int64, 0)
	for _, submission := range m.submissions {
		if submission.PublicationID != publicationID || submission.UserID != userID {
			continue
		}
		shiftIDs = append(shiftIDs, submission.TemplateShiftID)
	}

	sort.Slice(shiftIDs, func(i, j int) bool {
		return shiftIDs[i] < shiftIDs[j]
	})

	return shiftIDs, nil
}

func (m *publicationRepositoryStatefulMock) UpsertSubmission(
	ctx context.Context,
	params repository.UpsertAvailabilitySubmissionParams,
) (*model.AvailabilitySubmission, error) {
	publication, ok := m.publications[params.PublicationID]
	if !ok {
		return nil, repository.ErrPublicationNotFound
	}

	if params.PublicationState != nil {
		publication.State = *params.PublicationState
		publication.UpdatedAt = params.Now
	}

	key := submissionKey(params.PublicationID, params.UserID, params.TemplateShiftID)
	if existing, ok := m.submissions[key]; ok {
		return cloneSubmission(existing), nil
	}

	submission := &model.AvailabilitySubmission{
		ID:              m.nextSubmissionID,
		PublicationID:   params.PublicationID,
		UserID:          params.UserID,
		TemplateShiftID: params.TemplateShiftID,
		CreatedAt:       params.Now,
	}

	m.submissions[key] = cloneSubmission(submission)
	m.nextSubmissionID++

	return cloneSubmission(submission), nil
}

func (m *publicationRepositoryStatefulMock) DeleteSubmission(
	ctx context.Context,
	params repository.DeleteAvailabilitySubmissionParams,
) error {
	publication, ok := m.publications[params.PublicationID]
	if !ok {
		return repository.ErrPublicationNotFound
	}

	if params.PublicationState != nil {
		publication.State = *params.PublicationState
		publication.UpdatedAt = params.Now
	}

	delete(m.submissions, submissionKey(params.PublicationID, params.UserID, params.TemplateShiftID))
	return nil
}

func (m *publicationRepositoryStatefulMock) GetTemplateShift(
	ctx context.Context,
	templateID int64,
	shiftID int64,
) (*model.TemplateShift, error) {
	shift, ok := m.templateShifts[shiftID]
	if !ok || shift.TemplateID != templateID {
		return nil, repository.ErrTemplateShiftNotFound
	}

	return cloneTemplateShift(shift), nil
}

func (m *publicationRepositoryStatefulMock) IsUserQualifiedForPosition(
	ctx context.Context,
	userID int64,
	positionID int64,
) (bool, error) {
	positions, ok := m.qualifiedByUser[userID]
	if !ok {
		return false, nil
	}

	_, qualified := positions[positionID]
	return qualified, nil
}

func (m *publicationRepositoryStatefulMock) ListQualifiedShifts(
	ctx context.Context,
	publicationID int64,
	userID int64,
) ([]*model.TemplateShift, error) {
	publication, ok := m.publications[publicationID]
	if !ok {
		return nil, repository.ErrPublicationNotFound
	}

	shifts := make([]*model.TemplateShift, 0)
	for _, shift := range m.templateShifts {
		if shift.TemplateID != publication.TemplateID {
			continue
		}
		if qualified, _ := m.IsUserQualifiedForPosition(ctx, userID, shift.PositionID); !qualified {
			continue
		}
		shifts = append(shifts, cloneTemplateShift(shift))
	}

	sort.Slice(shifts, func(i, j int) bool {
		if shifts[i].Weekday != shifts[j].Weekday {
			return shifts[i].Weekday < shifts[j].Weekday
		}
		if shifts[i].StartTime != shifts[j].StartTime {
			return shifts[i].StartTime < shifts[j].StartTime
		}
		return shifts[i].ID < shifts[j].ID
	})

	return shifts, nil
}

func (m *publicationRepositoryStatefulMock) CreateAssignment(
	ctx context.Context,
	params repository.CreateAssignmentParams,
) (*model.Assignment, error) {
	if _, ok := m.publications[params.PublicationID]; !ok {
		return nil, repository.ErrPublicationNotFound
	}

	key := assignmentKey(params.PublicationID, params.UserID, params.TemplateShiftID)
	if existing, ok := m.assignments[key]; ok {
		return cloneAssignment(existing), nil
	}

	assignment := &model.Assignment{
		ID:              m.nextAssignmentID,
		PublicationID:   params.PublicationID,
		UserID:          params.UserID,
		TemplateShiftID: params.TemplateShiftID,
		CreatedAt:       params.CreatedAt,
	}
	m.assignments[key] = cloneAssignment(assignment)
	m.nextAssignmentID++

	return cloneAssignment(assignment), nil
}

func (m *publicationRepositoryStatefulMock) DeleteAssignment(
	ctx context.Context,
	params repository.DeleteAssignmentParams,
) error {
	for key, assignment := range m.assignments {
		if assignment.PublicationID == params.PublicationID && assignment.ID == params.AssignmentID {
			delete(m.assignments, key)
			break
		}
	}

	return nil
}

func (m *publicationRepositoryStatefulMock) ReplaceAssignments(
	ctx context.Context,
	params repository.ReplaceAssignmentsParams,
) error {
	if _, ok := m.publications[params.PublicationID]; !ok {
		return repository.ErrPublicationNotFound
	}

	for key, assignment := range m.assignments {
		if assignment.PublicationID == params.PublicationID {
			delete(m.assignments, key)
		}
	}

	for _, input := range params.Assignments {
		key := assignmentKey(params.PublicationID, input.UserID, input.TemplateShiftID)
		m.assignments[key] = &model.Assignment{
			ID:              m.nextAssignmentID,
			PublicationID:   params.PublicationID,
			UserID:          input.UserID,
			TemplateShiftID: input.TemplateShiftID,
			CreatedAt:       params.CreatedAt,
		}
		m.nextAssignmentID++
	}

	return nil
}

func (m *publicationRepositoryStatefulMock) ActivatePublication(
	ctx context.Context,
	params repository.ActivatePublicationParams,
) (*model.Publication, error) {
	publication, ok := m.publications[params.ID]
	if !ok {
		return nil, repository.ErrPublicationNotFound
	}
	if publication.State == model.PublicationStateActive || publication.State == model.PublicationStateEnded {
		return nil, sql.ErrNoRows
	}

	publication.State = model.PublicationStateActive
	publication.ActivatedAt = &params.Now
	publication.UpdatedAt = params.Now

	return clonePublication(publication), nil
}

func (m *publicationRepositoryStatefulMock) EndPublication(
	ctx context.Context,
	params repository.EndPublicationParams,
) (*model.Publication, error) {
	publication, ok := m.publications[params.ID]
	if !ok {
		return nil, repository.ErrPublicationNotFound
	}
	if publication.State != model.PublicationStateActive {
		return nil, sql.ErrNoRows
	}

	publication.State = model.PublicationStateEnded
	publication.EndedAt = &params.Now
	publication.UpdatedAt = params.Now

	return clonePublication(publication), nil
}

func (m *publicationRepositoryStatefulMock) ListPublicationShifts(
	ctx context.Context,
	publicationID int64,
) ([]*model.PublicationShift, error) {
	publication, ok := m.publications[publicationID]
	if !ok {
		return nil, repository.ErrPublicationNotFound
	}

	shifts := make([]*model.PublicationShift, 0)
	for _, shift := range m.templateShifts {
		if shift.TemplateID != publication.TemplateID {
			continue
		}

		positionName := "Unknown"
		switch shift.PositionID {
		case 101:
			positionName = "Front Desk"
		case 102:
			positionName = "Cashier"
		case 103:
			positionName = "Pharmacist"
		}

		shifts = append(shifts, &model.PublicationShift{
			ID:                shift.ID,
			TemplateID:        shift.TemplateID,
			Weekday:           shift.Weekday,
			StartTime:         shift.StartTime,
			EndTime:           shift.EndTime,
			PositionID:        shift.PositionID,
			PositionName:      positionName,
			RequiredHeadcount: shift.RequiredHeadcount,
			CreatedAt:         shift.CreatedAt,
			UpdatedAt:         shift.UpdatedAt,
		})
	}

	sort.Slice(shifts, func(i, j int) bool {
		if shifts[i].Weekday != shifts[j].Weekday {
			return shifts[i].Weekday < shifts[j].Weekday
		}
		if shifts[i].StartTime != shifts[j].StartTime {
			return shifts[i].StartTime < shifts[j].StartTime
		}
		return shifts[i].ID < shifts[j].ID
	})

	return shifts, nil
}

func (m *publicationRepositoryStatefulMock) ListAssignmentCandidates(
	ctx context.Context,
	publicationID int64,
) ([]*model.AssignmentCandidate, error) {
	if _, ok := m.publications[publicationID]; !ok {
		return nil, repository.ErrPublicationNotFound
	}

	candidates := make([]*model.AssignmentCandidate, 0)
	for _, submission := range m.submissions {
		user, ok := m.users[submission.UserID]
		if !ok {
			continue
		}
		candidates = append(candidates, &model.AssignmentCandidate{
			TemplateShiftID: submission.TemplateShiftID,
			UserID:          user.ID,
			Name:            user.Name,
			Email:           user.Email,
		})
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].TemplateShiftID != candidates[j].TemplateShiftID {
			return candidates[i].TemplateShiftID < candidates[j].TemplateShiftID
		}
		return candidates[i].UserID < candidates[j].UserID
	})

	return candidates, nil
}

func (m *publicationRepositoryStatefulMock) ListPublicationAssignments(
	ctx context.Context,
	publicationID int64,
) ([]*model.AssignmentParticipant, error) {
	if _, ok := m.publications[publicationID]; !ok {
		return nil, repository.ErrPublicationNotFound
	}

	assignments := make([]*model.AssignmentParticipant, 0)
	for _, assignment := range m.assignments {
		user, ok := m.users[assignment.UserID]
		if !ok {
			continue
		}
		assignments = append(assignments, &model.AssignmentParticipant{
			AssignmentID:    assignment.ID,
			TemplateShiftID: assignment.TemplateShiftID,
			UserID:          user.ID,
			Name:            user.Name,
			Email:           user.Email,
			CreatedAt:       assignment.CreatedAt,
		})
	}

	sort.Slice(assignments, func(i, j int) bool {
		if assignments[i].TemplateShiftID != assignments[j].TemplateShiftID {
			return assignments[i].TemplateShiftID < assignments[j].TemplateShiftID
		}
		return assignments[i].UserID < assignments[j].UserID
	})

	return assignments, nil
}

func TestPublicationServiceCreatePublication(t *testing.T) {
	t.Run("creates a publication and locks the template", func(t *testing.T) {
		t.Parallel()

		repo := newPublicationRepositoryStatefulMock()
		delete(repo.publications, 1)
		now := time.Date(2026, 4, 16, 9, 0, 0, 0, time.UTC)
		service := NewPublicationService(repo, fixedClock{now: now})

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())

		publication, err := service.CreatePublication(ctx, CreatePublicationInput{
			TemplateID:        1,
			Name:              "May availability",
			SubmissionStartAt: now.Add(2 * time.Hour),
			SubmissionEndAt:   now.Add(26 * time.Hour),
			PlannedActiveFrom: now.Add(48 * time.Hour),
		})
		if err != nil {
			t.Fatalf("CreatePublication returned error: %v", err)
		}

		if publication.State != model.PublicationStateDraft {
			t.Fatalf("expected draft state, got %s", publication.State)
		}
		if !repo.templates[1].IsLocked {
			t.Fatal("expected template to be locked")
		}

		event := stub.FindByAction(audit.ActionPublicationCreate)
		if event == nil {
			t.Fatalf("expected %q audit event, got %v", audit.ActionPublicationCreate, stub.Actions())
		}
		if event.TargetType != audit.TargetTypePublication {
			t.Fatalf("unexpected target type: %q", event.TargetType)
		}
		if event.TargetID == nil || *event.TargetID != publication.ID {
			t.Fatalf("unexpected target id: %v", event.TargetID)
		}
		if event.Metadata["template_id"] != int64(1) {
			t.Fatalf("expected template_id=1 in metadata, got %+v", event.Metadata)
		}
		if event.Metadata["name"] != "May availability" {
			t.Fatalf("expected name in metadata, got %+v", event.Metadata)
		}
		for _, key := range []string{"submission_start_at", "submission_end_at", "planned_active_from"} {
			if _, ok := event.Metadata[key]; !ok {
				t.Fatalf("expected %q in metadata, got %+v", key, event.Metadata)
			}
		}
	})

	t.Run("allows create when template is already locked by an ended publication", func(t *testing.T) {
		t.Parallel()

		repo := newPublicationRepositoryStatefulMock()
		now := time.Date(2026, 4, 16, 9, 0, 0, 0, time.UTC)
		repo.templates[1].IsLocked = true
		repo.publications[1] = &model.Publication{
			ID:                1,
			TemplateID:        1,
			TemplateName:      "Core Week",
			Name:              "Past run",
			State:             model.PublicationStateEnded,
			SubmissionStartAt: now.Add(-72 * time.Hour),
			SubmissionEndAt:   now.Add(-48 * time.Hour),
			PlannedActiveFrom: now.Add(-24 * time.Hour),
			CreatedAt:         now.Add(-96 * time.Hour),
			UpdatedAt:         now.Add(-24 * time.Hour),
		}
		service := NewPublicationService(repo, fixedClock{now: now})

		_, err := service.CreatePublication(context.Background(), CreatePublicationInput{
			TemplateID:        1,
			Name:              "Next run",
			SubmissionStartAt: now.Add(2 * time.Hour),
			SubmissionEndAt:   now.Add(26 * time.Hour),
			PlannedActiveFrom: now.Add(48 * time.Hour),
		})
		if err != nil {
			t.Fatalf("CreatePublication returned error: %v", err)
		}
		if !repo.templates[1].IsLocked {
			t.Fatal("expected template to stay locked")
		}
	})

	t.Run("rejects when a non-ended publication already exists", func(t *testing.T) {
		t.Parallel()

		repo := newPublicationRepositoryStatefulMock()
		now := time.Date(2026, 4, 20, 9, 0, 0, 0, time.UTC)
		repo.publications[1] = &model.Publication{
			ID:                1,
			TemplateID:        1,
			TemplateName:      "Core Week",
			Name:              "Existing",
			State:             model.PublicationStateDraft,
			SubmissionStartAt: now.Add(-72 * time.Hour),
			SubmissionEndAt:   now.Add(-48 * time.Hour),
			PlannedActiveFrom: now.Add(-24 * time.Hour),
			CreatedAt:         now.Add(-96 * time.Hour),
			UpdatedAt:         now.Add(-96 * time.Hour),
		}
		service := NewPublicationService(repo, fixedClock{now: now})

		_, err := service.CreatePublication(context.Background(), CreatePublicationInput{
			TemplateID:        1,
			Name:              "Blocked",
			SubmissionStartAt: now.Add(2 * time.Hour),
			SubmissionEndAt:   now.Add(26 * time.Hour),
			PlannedActiveFrom: now.Add(48 * time.Hour),
		})
		if !errors.Is(err, ErrPublicationAlreadyExists) {
			t.Fatalf("expected ErrPublicationAlreadyExists, got %v", err)
		}
	})

	t.Run("rejects invalid publication window", func(t *testing.T) {
		t.Parallel()

		repo := newPublicationRepositoryStatefulMock()
		delete(repo.publications, 1)
		now := time.Date(2026, 4, 16, 9, 0, 0, 0, time.UTC)
		service := NewPublicationService(repo, fixedClock{now: now})

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())

		_, err := service.CreatePublication(ctx, CreatePublicationInput{
			TemplateID:        1,
			Name:              "Invalid",
			SubmissionStartAt: now.Add(3 * time.Hour),
			SubmissionEndAt:   now.Add(2 * time.Hour),
			PlannedActiveFrom: now.Add(4 * time.Hour),
		})
		if !errors.Is(err, ErrInvalidPublicationWindow) {
			t.Fatalf("expected ErrInvalidPublicationWindow, got %v", err)
		}
		if len(stub.Events()) != 0 {
			t.Fatalf("expected no audit events, got %+v", stub.Events())
		}
	})

	t.Run("rejects missing template", func(t *testing.T) {
		t.Parallel()

		repo := newPublicationRepositoryStatefulMock()
		delete(repo.publications, 1)
		now := time.Date(2026, 4, 16, 9, 0, 0, 0, time.UTC)
		service := NewPublicationService(repo, fixedClock{now: now})

		_, err := service.CreatePublication(context.Background(), CreatePublicationInput{
			TemplateID:        999,
			Name:              "Invalid",
			SubmissionStartAt: now.Add(2 * time.Hour),
			SubmissionEndAt:   now.Add(3 * time.Hour),
			PlannedActiveFrom: now.Add(4 * time.Hour),
		})
		if !errors.Is(err, ErrTemplateNotFound) {
			t.Fatalf("expected ErrTemplateNotFound, got %v", err)
		}
	})

	t.Run("rejections leave template lock unchanged", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name     string
			template *model.Template
			input    CreatePublicationInput
		}{
			{
				name: "existing publication",
				template: &model.Template{
					ID:       1,
					Name:     "Core Week",
					IsLocked: false,
				},
				input: CreatePublicationInput{
					TemplateID:        1,
					Name:              "Blocked",
					SubmissionStartAt: time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC),
					SubmissionEndAt:   time.Date(2026, 4, 21, 12, 0, 0, 0, time.UTC),
					PlannedActiveFrom: time.Date(2026, 4, 22, 12, 0, 0, 0, time.UTC),
				},
			},
			{
				name: "invalid window",
				template: &model.Template{
					ID:       1,
					Name:     "Core Week",
					IsLocked: false,
				},
				input: CreatePublicationInput{
					TemplateID:        1,
					Name:              "Invalid",
					SubmissionStartAt: time.Date(2026, 4, 20, 14, 0, 0, 0, time.UTC),
					SubmissionEndAt:   time.Date(2026, 4, 20, 13, 0, 0, 0, time.UTC),
					PlannedActiveFrom: time.Date(2026, 4, 20, 15, 0, 0, 0, time.UTC),
				},
			},
			{
				name: "missing template",
				template: &model.Template{
					ID:       1,
					Name:     "Core Week",
					IsLocked: true,
				},
				input: CreatePublicationInput{
					TemplateID:        999,
					Name:              "Missing",
					SubmissionStartAt: time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC),
					SubmissionEndAt:   time.Date(2026, 4, 21, 12, 0, 0, 0, time.UTC),
					PlannedActiveFrom: time.Date(2026, 4, 22, 12, 0, 0, 0, time.UTC),
				},
			},
		}

		for _, tc := range tests {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				repo := newPublicationRepositoryStatefulMock()
				repo.templates[1] = &model.Template{
					ID:       tc.template.ID,
					Name:     tc.template.Name,
					IsLocked: tc.template.IsLocked,
				}
				service := NewPublicationService(repo, fixedClock{
					now: time.Date(2026, 4, 20, 9, 0, 0, 0, time.UTC),
				})

				_, _ = service.CreatePublication(context.Background(), tc.input)

				if repo.templates[1].IsLocked != tc.template.IsLocked {
					t.Fatalf("expected template lock to remain %v, got %v", tc.template.IsLocked, repo.templates[1].IsLocked)
				}
			})
		}
	})
}

func TestPublicationServiceGetListAndCurrent(t *testing.T) {
	t.Run("get by id returns effective state", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC)
		repo := newPublicationRepositoryStatefulMock()
		repo.publications[1] = &model.Publication{
			ID:                1,
			TemplateID:        1,
			TemplateName:      "Core Week",
			Name:              "Current",
			State:             model.PublicationStateDraft,
			SubmissionStartAt: now.Add(-2 * time.Hour),
			SubmissionEndAt:   now.Add(24 * time.Hour),
			PlannedActiveFrom: now.Add(48 * time.Hour),
			CreatedAt:         now.Add(-24 * time.Hour),
			UpdatedAt:         now.Add(-24 * time.Hour),
		}
		service := NewPublicationService(repo, fixedClock{now: now})

		publication, err := service.GetPublicationByID(context.Background(), 1)
		if err != nil {
			t.Fatalf("GetPublicationByID returned error: %v", err)
		}
		if publication.State != model.PublicationStateCollecting {
			t.Fatalf("expected collecting state, got %s", publication.State)
		}
	})

	t.Run("list orders by created_at descending", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC)
		repo := newPublicationRepositoryStatefulMock()
		repo.publications[1] = &model.Publication{
			ID:                1,
			TemplateID:        1,
			TemplateName:      "Core Week",
			Name:              "Older",
			State:             model.PublicationStateEnded,
			SubmissionStartAt: now.Add(-72 * time.Hour),
			SubmissionEndAt:   now.Add(-48 * time.Hour),
			PlannedActiveFrom: now.Add(-24 * time.Hour),
			CreatedAt:         now.Add(-72 * time.Hour),
			UpdatedAt:         now.Add(-24 * time.Hour),
		}
		repo.publications[2] = &model.Publication{
			ID:                2,
			TemplateID:        1,
			TemplateName:      "Core Week",
			Name:              "Newer",
			State:             model.PublicationStateDraft,
			SubmissionStartAt: now.Add(24 * time.Hour),
			SubmissionEndAt:   now.Add(48 * time.Hour),
			PlannedActiveFrom: now.Add(72 * time.Hour),
			CreatedAt:         now.Add(-2 * time.Hour),
			UpdatedAt:         now.Add(-2 * time.Hour),
		}
		service := NewPublicationService(repo, fixedClock{now: now})

		result, err := service.ListPublications(context.Background(), ListPublicationsInput{
			Page:     1,
			PageSize: 10,
		})
		if err != nil {
			t.Fatalf("ListPublications returned error: %v", err)
		}
		if len(result.Publications) != 2 {
			t.Fatalf("expected 2 publications, got %d", len(result.Publications))
		}
		if result.Publications[0].ID != 2 || result.Publications[1].ID != 1 {
			t.Fatalf("unexpected order: %d then %d", result.Publications[0].ID, result.Publications[1].ID)
		}
	})

	t.Run("current returns active non-ended publication or nil", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC)
		repo := newPublicationRepositoryStatefulMock()
		repo.publications[1] = &model.Publication{
			ID:                1,
			TemplateID:        1,
			TemplateName:      "Core Week",
			Name:              "Current",
			State:             model.PublicationStateDraft,
			SubmissionStartAt: now.Add(24 * time.Hour),
			SubmissionEndAt:   now.Add(48 * time.Hour),
			PlannedActiveFrom: now.Add(72 * time.Hour),
			CreatedAt:         now.Add(-24 * time.Hour),
			UpdatedAt:         now.Add(-24 * time.Hour),
		}
		service := NewPublicationService(repo, fixedClock{now: now})

		publication, err := service.GetCurrentPublication(context.Background())
		if err != nil {
			t.Fatalf("GetCurrentPublication returned error: %v", err)
		}
		if publication == nil || publication.ID != 1 {
			t.Fatalf("expected current publication 1, got %+v", publication)
		}

		repo.publications[1].State = model.PublicationStateEnded
		publication, err = service.GetCurrentPublication(context.Background())
		if err != nil {
			t.Fatalf("GetCurrentPublication returned error: %v", err)
		}
		if publication != nil {
			t.Fatalf("expected nil current publication, got %+v", publication)
		}
	})
}

func TestPublicationServiceDeletePublication(t *testing.T) {
	t.Run("deletes a draft publication", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC)
		repo := newPublicationRepositoryStatefulMock()
		repo.publications[1] = &model.Publication{
			ID:                1,
			TemplateID:        1,
			TemplateName:      "Core Week",
			Name:              "Draft",
			State:             model.PublicationStateDraft,
			SubmissionStartAt: now.Add(24 * time.Hour),
			SubmissionEndAt:   now.Add(48 * time.Hour),
			PlannedActiveFrom: now.Add(72 * time.Hour),
			CreatedAt:         now.Add(-24 * time.Hour),
			UpdatedAt:         now.Add(-24 * time.Hour),
		}
		repo.submissions[submissionKey(1, 7, 11)] = &model.AvailabilitySubmission{
			ID:              1,
			PublicationID:   1,
			UserID:          7,
			TemplateShiftID: 11,
			CreatedAt:       now.Add(-2 * time.Hour),
		}
		service := NewPublicationService(repo, fixedClock{now: now})

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())

		if err := service.DeletePublication(ctx, 1); err != nil {
			t.Fatalf("DeletePublication returned error: %v", err)
		}
		if _, ok := repo.publications[1]; ok {
			t.Fatal("expected publication to be deleted")
		}
		if len(repo.submissions) != 0 {
			t.Fatalf("expected submissions to be cascaded, got %d", len(repo.submissions))
		}

		event := stub.FindByAction(audit.ActionPublicationDelete)
		if event == nil {
			t.Fatalf("expected %q audit event, got %v", audit.ActionPublicationDelete, stub.Actions())
		}
		if event.TargetType != audit.TargetTypePublication {
			t.Fatalf("unexpected target type: %q", event.TargetType)
		}
		if event.TargetID == nil || *event.TargetID != 1 {
			t.Fatalf("unexpected target id: %v", event.TargetID)
		}
		if event.Metadata["name"] != "Draft" {
			t.Fatalf("expected name=Draft in metadata, got %+v", event.Metadata)
		}
	})

	t.Run("rejects delete when effective state is past draft", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC)
		repo := newPublicationRepositoryStatefulMock()
		repo.publications[1] = &model.Publication{
			ID:                1,
			TemplateID:        1,
			TemplateName:      "Core Week",
			Name:              "Collecting",
			State:             model.PublicationStateDraft,
			SubmissionStartAt: now.Add(-2 * time.Hour),
			SubmissionEndAt:   now.Add(24 * time.Hour),
			PlannedActiveFrom: now.Add(48 * time.Hour),
			CreatedAt:         now.Add(-24 * time.Hour),
			UpdatedAt:         now.Add(-24 * time.Hour),
		}
		service := NewPublicationService(repo, fixedClock{now: now})

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())

		err := service.DeletePublication(ctx, 1)
		if !errors.Is(err, ErrPublicationNotDeletable) {
			t.Fatalf("expected ErrPublicationNotDeletable, got %v", err)
		}
		if len(stub.Events()) != 0 {
			t.Fatalf("expected no audit events, got %+v", stub.Events())
		}
	})

	t.Run("rejects delete when stored state is draft but clock is past submission start", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC)
		repo := newPublicationRepositoryStatefulMock()
		repo.publications[1] = &model.Publication{
			ID:                1,
			TemplateID:        1,
			TemplateName:      "Core Week",
			Name:              "Clock Race",
			State:             model.PublicationStateDraft,
			SubmissionStartAt: now.Add(-1 * time.Minute),
			SubmissionEndAt:   now.Add(24 * time.Hour),
			PlannedActiveFrom: now.Add(48 * time.Hour),
			CreatedAt:         now.Add(-24 * time.Hour),
			UpdatedAt:         now.Add(-24 * time.Hour),
		}
		service := NewPublicationService(repo, fixedClock{now: now})

		err := service.DeletePublication(context.Background(), 1)
		if !errors.Is(err, ErrPublicationNotDeletable) {
			t.Fatalf("expected ErrPublicationNotDeletable, got %v", err)
		}
		if _, ok := repo.publications[1]; !ok {
			t.Fatal("expected publication to remain when delete is rejected")
		}
	})

	t.Run("returns not deletable when atomic delete condition no longer matches", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC)
		repo := newPublicationRepositoryStatefulMock()
		repo.publications[1] = &model.Publication{
			ID:                1,
			TemplateID:        1,
			TemplateName:      "Core Week",
			Name:              "Delete Race",
			State:             model.PublicationStateDraft,
			SubmissionStartAt: now.Add(1 * time.Minute),
			SubmissionEndAt:   now.Add(24 * time.Hour),
			PlannedActiveFrom: now.Add(48 * time.Hour),
			CreatedAt:         now.Add(-24 * time.Hour),
			UpdatedAt:         now.Add(-24 * time.Hour),
		}
		repo.deletePublicationFunc = func(ctx context.Context, params repository.DeletePublicationParams) error {
			return repository.ErrPublicationNotFound
		}
		service := NewPublicationService(repo, fixedClock{now: now})

		err := service.DeletePublication(context.Background(), 1)
		if !errors.Is(err, ErrPublicationNotDeletable) {
			t.Fatalf("expected ErrPublicationNotDeletable, got %v", err)
		}
	})
}

func TestPublicationServiceListAvailabilitySubmissionShiftIDs(t *testing.T) {
	t.Run("returns submitted shift IDs", func(t *testing.T) {
		t.Parallel()

		repo := newPublicationRepositoryStatefulMock()
		repo.submissions[submissionKey(1, 7, 11)] = &model.AvailabilitySubmission{
			ID:              1,
			PublicationID:   1,
			UserID:          7,
			TemplateShiftID: 11,
		}
		repo.submissions[submissionKey(1, 7, 12)] = &model.AvailabilitySubmission{
			ID:              2,
			PublicationID:   1,
			UserID:          7,
			TemplateShiftID: 12,
		}
		service := NewPublicationService(repo, fixedClock{})

		shiftIDs, err := service.ListAvailabilitySubmissionShiftIDs(context.Background(), 1, 7)
		if err != nil {
			t.Fatalf("ListAvailabilitySubmissionShiftIDs returned error: %v", err)
		}
		if len(shiftIDs) != 2 {
			t.Fatalf("expected 2 shift ids, got %v", shiftIDs)
		}
		if shiftIDs[0] != 11 || shiftIDs[1] != 12 {
			t.Fatalf("expected [11 12], got %v", shiftIDs)
		}
	})

	t.Run("returns empty slice when user has no submissions", func(t *testing.T) {
		t.Parallel()

		repo := newPublicationRepositoryStatefulMock()
		service := NewPublicationService(repo, fixedClock{})

		shiftIDs, err := service.ListAvailabilitySubmissionShiftIDs(context.Background(), 1, 8)
		if err != nil {
			t.Fatalf("ListAvailabilitySubmissionShiftIDs returned error: %v", err)
		}
		if shiftIDs == nil {
			t.Fatal("expected empty slice, got nil")
		}
		if len(shiftIDs) != 0 {
			t.Fatalf("expected no shift ids, got %v", shiftIDs)
		}
	})

	t.Run("returns ErrPublicationNotFound for missing publication", func(t *testing.T) {
		t.Parallel()

		repo := newPublicationRepositoryStatefulMock()
		service := NewPublicationService(repo, fixedClock{})

		_, err := service.ListAvailabilitySubmissionShiftIDs(context.Background(), 999, 7)
		if !errors.Is(err, ErrPublicationNotFound) {
			t.Fatalf("expected ErrPublicationNotFound, got %v", err)
		}
	})
}

func TestPublicationServiceCreateAvailabilitySubmission(t *testing.T) {
	t.Run("creates a submission during collecting", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC)
		repo := newPublicationRepositoryStatefulMock()
		repo.publications[1] = collectingPublication(now)
		service := NewPublicationService(repo, fixedClock{now: now})

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())

		submission, err := service.CreateAvailabilitySubmission(ctx, CreateAvailabilitySubmissionInput{
			PublicationID:   1,
			UserID:          7,
			TemplateShiftID: 11,
		})
		if err != nil {
			t.Fatalf("CreateAvailabilitySubmission returned error: %v", err)
		}
		if submission.PublicationID != 1 || submission.UserID != 7 || submission.TemplateShiftID != 11 {
			t.Fatalf("unexpected submission: %+v", submission)
		}

		event := stub.FindByAction(audit.ActionSubmissionCreate)
		if event == nil {
			t.Fatalf("expected %q audit event, got %v", audit.ActionSubmissionCreate, stub.Actions())
		}
		if event.TargetType != audit.TargetTypeAvailabilitySubmission {
			t.Fatalf("unexpected target type: %q", event.TargetType)
		}
		if event.TargetID == nil || *event.TargetID != submission.ID {
			t.Fatalf("unexpected target id: %v", event.TargetID)
		}
		if event.Metadata["publication_id"] != int64(1) {
			t.Fatalf("expected publication_id=1 in metadata, got %+v", event.Metadata)
		}
		if event.Metadata["template_shift_id"] != int64(11) {
			t.Fatalf("expected template_shift_id=11 in metadata, got %+v", event.Metadata)
		}
	})

	t.Run("lazy write-through updates stored draft state to collecting", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC)
		repo := newPublicationRepositoryStatefulMock()
		repo.publications[1] = collectingPublication(now)
		repo.publications[1].State = model.PublicationStateDraft
		service := NewPublicationService(repo, fixedClock{now: now})

		_, err := service.CreateAvailabilitySubmission(context.Background(), CreateAvailabilitySubmissionInput{
			PublicationID:   1,
			UserID:          7,
			TemplateShiftID: 11,
		})
		if err != nil {
			t.Fatalf("CreateAvailabilitySubmission returned error: %v", err)
		}
		if repo.publications[1].State != model.PublicationStateCollecting {
			t.Fatalf("expected stored state to advance to collecting, got %s", repo.publications[1].State)
		}
	})

	t.Run("duplicate submission is idempotent", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC)
		repo := newPublicationRepositoryStatefulMock()
		repo.publications[1] = collectingPublication(now)
		service := NewPublicationService(repo, fixedClock{now: now})

		first, err := service.CreateAvailabilitySubmission(context.Background(), CreateAvailabilitySubmissionInput{
			PublicationID:   1,
			UserID:          7,
			TemplateShiftID: 11,
		})
		if err != nil {
			t.Fatalf("first CreateAvailabilitySubmission returned error: %v", err)
		}
		second, err := service.CreateAvailabilitySubmission(context.Background(), CreateAvailabilitySubmissionInput{
			PublicationID:   1,
			UserID:          7,
			TemplateShiftID: 11,
		})
		if err != nil {
			t.Fatalf("second CreateAvailabilitySubmission returned error: %v", err)
		}
		if first.ID != second.ID {
			t.Fatalf("expected idempotent submission, got ids %d and %d", first.ID, second.ID)
		}
		if len(repo.submissions) != 1 {
			t.Fatalf("expected one submission, got %d", len(repo.submissions))
		}
	})

	t.Run("rejects draft effective state", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC)
		repo := newPublicationRepositoryStatefulMock()
		repo.publications[1] = &model.Publication{
			ID:                1,
			TemplateID:        1,
			TemplateName:      "Core Week",
			Name:              "Draft",
			State:             model.PublicationStateDraft,
			SubmissionStartAt: now.Add(2 * time.Hour),
			SubmissionEndAt:   now.Add(24 * time.Hour),
			PlannedActiveFrom: now.Add(48 * time.Hour),
			CreatedAt:         now.Add(-24 * time.Hour),
			UpdatedAt:         now.Add(-24 * time.Hour),
		}
		service := NewPublicationService(repo, fixedClock{now: now})

		_, err := service.CreateAvailabilitySubmission(context.Background(), CreateAvailabilitySubmissionInput{
			PublicationID:   1,
			UserID:          7,
			TemplateShiftID: 11,
		})
		if !errors.Is(err, ErrPublicationNotCollecting) {
			t.Fatalf("expected ErrPublicationNotCollecting, got %v", err)
		}
	})

	t.Run("rejects assigning effective state", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC)
		repo := newPublicationRepositoryStatefulMock()
		repo.publications[1] = &model.Publication{
			ID:                1,
			TemplateID:        1,
			TemplateName:      "Core Week",
			Name:              "Assigning",
			State:             model.PublicationStateDraft,
			SubmissionStartAt: now.Add(-24 * time.Hour),
			SubmissionEndAt:   now.Add(-2 * time.Hour),
			PlannedActiveFrom: now.Add(24 * time.Hour),
			CreatedAt:         now.Add(-48 * time.Hour),
			UpdatedAt:         now.Add(-48 * time.Hour),
		}
		service := NewPublicationService(repo, fixedClock{now: now})

		_, err := service.CreateAvailabilitySubmission(context.Background(), CreateAvailabilitySubmissionInput{
			PublicationID:   1,
			UserID:          7,
			TemplateShiftID: 11,
		})
		if !errors.Is(err, ErrPublicationNotCollecting) {
			t.Fatalf("expected ErrPublicationNotCollecting, got %v", err)
		}
	})

	t.Run("rejects shift outside publication template", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC)
		repo := newPublicationRepositoryStatefulMock()
		repo.publications[1] = collectingPublication(now)
		repo.templateShifts[99] = &model.TemplateShift{
			ID:                99,
			TemplateID:        999,
			Weekday:           5,
			StartTime:         "10:00",
			EndTime:           "12:00",
			PositionID:        101,
			RequiredHeadcount: 1,
		}
		service := NewPublicationService(repo, fixedClock{now: now})

		_, err := service.CreateAvailabilitySubmission(context.Background(), CreateAvailabilitySubmissionInput{
			PublicationID:   1,
			UserID:          7,
			TemplateShiftID: 99,
		})
		if !errors.Is(err, ErrTemplateShiftNotFound) {
			t.Fatalf("expected ErrTemplateShiftNotFound, got %v", err)
		}
	})

	t.Run("rejects not qualified user", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC)
		repo := newPublicationRepositoryStatefulMock()
		repo.publications[1] = collectingPublication(now)
		service := NewPublicationService(repo, fixedClock{now: now})

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())

		_, err := service.CreateAvailabilitySubmission(ctx, CreateAvailabilitySubmissionInput{
			PublicationID:   1,
			UserID:          7,
			TemplateShiftID: 12,
		})
		if !errors.Is(err, ErrNotQualified) {
			t.Fatalf("expected ErrNotQualified, got %v", err)
		}
		if len(stub.Events()) != 0 {
			t.Fatalf("expected no audit events, got %+v", stub.Events())
		}
	})

	t.Run("rejects missing publication", func(t *testing.T) {
		t.Parallel()

		repo := newPublicationRepositoryStatefulMock()
		delete(repo.publications, 1)
		service := NewPublicationService(repo, fixedClock{
			now: time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC),
		})

		_, err := service.CreateAvailabilitySubmission(context.Background(), CreateAvailabilitySubmissionInput{
			PublicationID:   1,
			UserID:          7,
			TemplateShiftID: 11,
		})
		if !errors.Is(err, ErrPublicationNotFound) {
			t.Fatalf("expected ErrPublicationNotFound, got %v", err)
		}
	})
}

func TestPublicationServiceDeleteAvailabilitySubmission(t *testing.T) {
	t.Run("deletes an existing submission", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC)
		repo := newPublicationRepositoryStatefulMock()
		repo.publications[1] = collectingPublication(now)
		repo.submissions[submissionKey(1, 7, 11)] = &model.AvailabilitySubmission{
			ID:              1,
			PublicationID:   1,
			UserID:          7,
			TemplateShiftID: 11,
			CreatedAt:       now.Add(-15 * time.Minute),
		}
		service := NewPublicationService(repo, fixedClock{now: now})

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())

		err := service.DeleteAvailabilitySubmission(ctx, DeleteAvailabilitySubmissionInput{
			PublicationID:   1,
			UserID:          7,
			TemplateShiftID: 11,
		})
		if err != nil {
			t.Fatalf("DeleteAvailabilitySubmission returned error: %v", err)
		}
		if len(repo.submissions) != 0 {
			t.Fatalf("expected submission to be deleted, got %d", len(repo.submissions))
		}

		event := stub.FindByAction(audit.ActionSubmissionDelete)
		if event == nil {
			t.Fatalf("expected %q audit event, got %v", audit.ActionSubmissionDelete, stub.Actions())
		}
		if event.TargetType != audit.TargetTypeAvailabilitySubmission {
			t.Fatalf("unexpected target type: %q", event.TargetType)
		}
		if event.TargetID == nil || *event.TargetID != 11 {
			t.Fatalf("unexpected target id: %v", event.TargetID)
		}
		if event.Metadata["publication_id"] != int64(1) {
			t.Fatalf("expected publication_id=1 in metadata, got %+v", event.Metadata)
		}
		if event.Metadata["template_shift_id"] != int64(11) {
			t.Fatalf("expected template_shift_id=11 in metadata, got %+v", event.Metadata)
		}
	})

	t.Run("delete is idempotent", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC)
		repo := newPublicationRepositoryStatefulMock()
		repo.publications[1] = collectingPublication(now)
		service := NewPublicationService(repo, fixedClock{now: now})

		err := service.DeleteAvailabilitySubmission(context.Background(), DeleteAvailabilitySubmissionInput{
			PublicationID:   1,
			UserID:          7,
			TemplateShiftID: 11,
		})
		if err != nil {
			t.Fatalf("DeleteAvailabilitySubmission returned error: %v", err)
		}
	})

	t.Run("allows delete after qualification is revoked", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC)
		repo := newPublicationRepositoryStatefulMock()
		repo.publications[1] = collectingPublication(now)
		repo.submissions[submissionKey(1, 7, 11)] = &model.AvailabilitySubmission{
			ID:              1,
			PublicationID:   1,
			UserID:          7,
			TemplateShiftID: 11,
			CreatedAt:       now.Add(-15 * time.Minute),
		}
		delete(repo.qualifiedByUser, 7)
		service := NewPublicationService(repo, fixedClock{now: now})

		err := service.DeleteAvailabilitySubmission(context.Background(), DeleteAvailabilitySubmissionInput{
			PublicationID:   1,
			UserID:          7,
			TemplateShiftID: 11,
		})
		if err != nil {
			t.Fatalf("DeleteAvailabilitySubmission returned error: %v", err)
		}
		if len(repo.submissions) != 0 {
			t.Fatalf("expected revoked-user submission to be deleted, got %d", len(repo.submissions))
		}
	})

	t.Run("rejects delete outside collecting", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC)
		repo := newPublicationRepositoryStatefulMock()
		repo.publications[1] = &model.Publication{
			ID:                1,
			TemplateID:        1,
			TemplateName:      "Core Week",
			Name:              "Assigning",
			State:             model.PublicationStateDraft,
			SubmissionStartAt: now.Add(-24 * time.Hour),
			SubmissionEndAt:   now.Add(-2 * time.Hour),
			PlannedActiveFrom: now.Add(24 * time.Hour),
			CreatedAt:         now.Add(-48 * time.Hour),
			UpdatedAt:         now.Add(-48 * time.Hour),
		}
		service := NewPublicationService(repo, fixedClock{now: now})

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())

		err := service.DeleteAvailabilitySubmission(ctx, DeleteAvailabilitySubmissionInput{
			PublicationID:   1,
			UserID:          7,
			TemplateShiftID: 11,
		})
		if !errors.Is(err, ErrPublicationNotCollecting) {
			t.Fatalf("expected ErrPublicationNotCollecting, got %v", err)
		}
		if len(stub.Events()) != 0 {
			t.Fatalf("expected no audit events, got %+v", stub.Events())
		}
	})
}

func TestPublicationServiceListQualifiedPublicationShifts(t *testing.T) {
	t.Run("returns qualified shifts during collecting", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC)
		repo := newPublicationRepositoryStatefulMock()
		repo.publications[1] = collectingPublication(now)
		service := NewPublicationService(repo, fixedClock{now: now})

		shifts, err := service.ListQualifiedPublicationShifts(context.Background(), 1, 7)
		if err != nil {
			t.Fatalf("ListQualifiedPublicationShifts returned error: %v", err)
		}
		if len(shifts) != 1 || shifts[0].ID != 11 {
			t.Fatalf("expected one qualified shift 11, got %+v", shifts)
		}
	})

	t.Run("rejects outside collecting", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC)
		repo := newPublicationRepositoryStatefulMock()
		repo.publications[1] = &model.Publication{
			ID:                1,
			TemplateID:        1,
			TemplateName:      "Core Week",
			Name:              "Draft",
			State:             model.PublicationStateDraft,
			SubmissionStartAt: now.Add(2 * time.Hour),
			SubmissionEndAt:   now.Add(3 * time.Hour),
			PlannedActiveFrom: now.Add(4 * time.Hour),
			CreatedAt:         now.Add(-24 * time.Hour),
			UpdatedAt:         now.Add(-24 * time.Hour),
		}
		service := NewPublicationService(repo, fixedClock{now: now})

		_, err := service.ListQualifiedPublicationShifts(context.Background(), 1, 7)
		if !errors.Is(err, ErrPublicationNotCollecting) {
			t.Fatalf("expected ErrPublicationNotCollecting, got %v", err)
		}
	})
}

func TestResolvePublicationState(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC)
	base := &model.Publication{
		State:             model.PublicationStateDraft,
		SubmissionStartAt: now.Add(2 * time.Hour),
		SubmissionEndAt:   now.Add(4 * time.Hour),
		PlannedActiveFrom: now.Add(6 * time.Hour),
	}

	tests := []struct {
		name        string
		publication *model.Publication
		now         time.Time
		want        model.PublicationState
	}{
		{
			name:        "draft before window",
			publication: clonePublication(base),
			now:         now,
			want:        model.PublicationStateDraft,
		},
		{
			name:        "collecting after submission start",
			publication: clonePublication(base),
			now:         base.SubmissionStartAt,
			want:        model.PublicationStateCollecting,
		},
		{
			name:        "assigning after submission end",
			publication: clonePublication(base),
			now:         base.SubmissionEndAt,
			want:        model.PublicationStateAssigning,
		},
		{
			name: "active stays active",
			publication: func() *model.Publication {
				publication := clonePublication(base)
				publication.State = model.PublicationStateActive
				return publication
			}(),
			now:  base.SubmissionEndAt.Add(24 * time.Hour),
			want: model.PublicationStateActive,
		},
		{
			name: "ended stays ended",
			publication: func() *model.Publication {
				publication := clonePublication(base)
				publication.State = model.PublicationStateEnded
				return publication
			}(),
			now:  base.SubmissionEndAt.Add(24 * time.Hour),
			want: model.PublicationStateEnded,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := model.ResolvePublicationState(tc.publication, tc.now)
			if got != tc.want {
				t.Fatalf("expected %s, got %s", tc.want, got)
			}
		})
	}
}

func collectingPublication(now time.Time) *model.Publication {
	return &model.Publication{
		ID:                1,
		TemplateID:        1,
		TemplateName:      "Core Week",
		Name:              "Current",
		State:             model.PublicationStateDraft,
		SubmissionStartAt: now.Add(-2 * time.Hour),
		SubmissionEndAt:   now.Add(24 * time.Hour),
		PlannedActiveFrom: now.Add(48 * time.Hour),
		CreatedAt:         now.Add(-24 * time.Hour),
		UpdatedAt:         now.Add(-24 * time.Hour),
	}
}

func submissionKey(publicationID, userID, shiftID int64) string {
	return strconv.FormatInt(publicationID, 10) +
		":" + strconv.FormatInt(userID, 10) +
		":" + strconv.FormatInt(shiftID, 10)
}

func assignmentKey(publicationID, userID, shiftID int64) string {
	return submissionKey(publicationID, userID, shiftID)
}

func clonePublication(publication *model.Publication) *model.Publication {
	if publication == nil {
		return nil
	}

	cloned := *publication
	return &cloned
}

func cloneSubmission(submission *model.AvailabilitySubmission) *model.AvailabilitySubmission {
	if submission == nil {
		return nil
	}

	cloned := *submission
	return &cloned
}

func cloneAssignment(assignment *model.Assignment) *model.Assignment {
	if assignment == nil {
		return nil
	}

	cloned := *assignment
	return &cloned
}

func cloneTemplateShift(shift *model.TemplateShift) *model.TemplateShift {
	if shift == nil {
		return nil
	}

	cloned := *shift
	return &cloned
}
