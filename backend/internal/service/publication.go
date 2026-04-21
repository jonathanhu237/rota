package service

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jonathanhu237/rota/backend/internal/audit"
	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/jonathanhu237/rota/backend/internal/repository"
)

const maxPublicationNameLength = 100

var (
	ErrInvalidPublicationWindow = model.ErrInvalidPublicationWindow
	ErrPublicationAlreadyExists = model.ErrPublicationAlreadyExists
	ErrPublicationNotFound      = model.ErrPublicationNotFound
	ErrPublicationNotDeletable  = model.ErrPublicationNotDeletable
	ErrPublicationNotCollecting = model.ErrPublicationNotCollecting
	ErrPublicationNotAssigning  = model.ErrPublicationNotAssigning
	ErrPublicationNotPublished  = model.ErrPublicationNotPublished
	ErrPublicationNotActive     = model.ErrPublicationNotActive
	ErrNotQualified             = model.ErrNotQualified
)

type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time {
	return time.Now()
}

type publicationRepository interface {
	ListPaginated(ctx context.Context, params repository.ListPublicationsParams) ([]*model.Publication, int, error)
	GetByID(ctx context.Context, id int64) (*model.Publication, error)
	GetCurrent(ctx context.Context) (*model.Publication, error)
	GetUserByID(ctx context.Context, id int64) (*model.User, error)
	CreatePublication(ctx context.Context, params repository.CreatePublicationParams) (*model.Publication, error)
	DeletePublication(ctx context.Context, params repository.DeletePublicationParams) error
	ListSubmissionShiftIDs(ctx context.Context, publicationID, userID int64) ([]int64, error)
	UpsertSubmission(ctx context.Context, params repository.UpsertAvailabilitySubmissionParams) (*model.AvailabilitySubmission, error)
	DeleteSubmission(ctx context.Context, params repository.DeleteAvailabilitySubmissionParams) error
	GetTemplateShift(ctx context.Context, templateID, shiftID int64) (*model.TemplateShift, error)
	IsUserQualifiedForPosition(ctx context.Context, userID, positionID int64) (bool, error)
	ListQualifiedShifts(ctx context.Context, publicationID, userID int64) ([]*model.TemplateShift, error)
	CreateAssignment(ctx context.Context, params repository.CreateAssignmentParams) (*model.Assignment, error)
	DeleteAssignment(ctx context.Context, params repository.DeleteAssignmentParams) error
	ReplaceAssignments(ctx context.Context, params repository.ReplaceAssignmentsParams) error
	ActivatePublication(ctx context.Context, params repository.ActivatePublicationParams) (*repository.ActivatePublicationResult, error)
	PublishPublication(ctx context.Context, params repository.PublishPublicationParams) (*model.Publication, error)
	EndPublication(ctx context.Context, params repository.EndPublicationParams) (*model.Publication, error)
	ListPublicationShifts(ctx context.Context, publicationID int64) ([]*model.PublicationShift, error)
	ListAssignmentCandidates(ctx context.Context, publicationID int64) ([]*model.AssignmentCandidate, error)
	ListPublicationAssignments(ctx context.Context, publicationID int64) ([]*model.AssignmentParticipant, error)
}

type PublicationService struct {
	publicationRepo publicationRepository
	clock           Clock
}

type ListPublicationsInput struct {
	Page     int
	PageSize int
}

type ListPublicationsResult struct {
	Publications []*model.Publication
	Page         int
	PageSize     int
	Total        int
	TotalPages   int
}

type CreatePublicationInput struct {
	TemplateID        int64
	Name              string
	SubmissionStartAt time.Time
	SubmissionEndAt   time.Time
	PlannedActiveFrom time.Time
}

type CreateAvailabilitySubmissionInput struct {
	PublicationID   int64
	UserID          int64
	TemplateShiftID int64
}

type DeleteAvailabilitySubmissionInput struct {
	PublicationID   int64
	UserID          int64
	TemplateShiftID int64
}

type CreateAssignmentInput struct {
	PublicationID   int64
	UserID          int64
	TemplateShiftID int64
}

type DeleteAssignmentInput struct {
	PublicationID int64
	AssignmentID  int64
}

func NewPublicationService(publicationRepo publicationRepository, clock Clock) *PublicationService {
	if clock == nil {
		clock = realClock{}
	}

	return &PublicationService{
		publicationRepo: publicationRepo,
		clock:           clock,
	}
}

func (s *PublicationService) ListPublications(
	ctx context.Context,
	input ListPublicationsInput,
) (*ListPublicationsResult, error) {
	page, pageSize, err := normalizePagination(input.Page, input.PageSize)
	if err != nil {
		return nil, err
	}

	offset := (page - 1) * pageSize
	publications, total, err := s.publicationRepo.ListPaginated(ctx, repository.ListPublicationsParams{
		Offset: offset,
		Limit:  pageSize,
	})
	if err != nil {
		return nil, mapPublicationRepositoryError(err)
	}

	now := s.clock.Now()
	result := make([]*model.Publication, 0, len(publications))
	for _, publication := range publications {
		result = append(result, publicationWithEffectiveState(publication, now))
	}

	totalPages := 0
	if total > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}

	return &ListPublicationsResult{
		Publications: result,
		Page:         page,
		PageSize:     pageSize,
		Total:        total,
		TotalPages:   totalPages,
	}, nil
}

func (s *PublicationService) CreatePublication(
	ctx context.Context,
	input CreatePublicationInput,
) (*model.Publication, error) {
	if input.TemplateID <= 0 {
		return nil, ErrInvalidInput
	}

	name, err := normalizePublicationName(input.Name)
	if err != nil {
		return nil, err
	}
	if err := validatePublicationWindow(
		input.SubmissionStartAt,
		input.SubmissionEndAt,
		input.PlannedActiveFrom,
	); err != nil {
		return nil, err
	}

	now := s.clock.Now()
	publication, err := s.publicationRepo.CreatePublication(ctx, repository.CreatePublicationParams{
		TemplateID:        input.TemplateID,
		Name:              name,
		State:             model.PublicationStateDraft,
		SubmissionStartAt: input.SubmissionStartAt,
		SubmissionEndAt:   input.SubmissionEndAt,
		PlannedActiveFrom: input.PlannedActiveFrom,
		CreatedAt:         now,
	})
	if err != nil {
		return nil, mapPublicationRepositoryError(err)
	}

	targetID := publication.ID
	audit.Record(ctx, audit.Event{
		Action:     audit.ActionPublicationCreate,
		TargetType: audit.TargetTypePublication,
		TargetID:   &targetID,
		Metadata: map[string]any{
			"template_id":         publication.TemplateID,
			"name":                publication.Name,
			"submission_start_at": publication.SubmissionStartAt.Format(time.RFC3339),
			"submission_end_at":   publication.SubmissionEndAt.Format(time.RFC3339),
			"planned_active_from": publication.PlannedActiveFrom.Format(time.RFC3339),
		},
	})

	return publicationWithEffectiveState(publication, now), nil
}

func (s *PublicationService) GetPublicationByID(
	ctx context.Context,
	id int64,
) (*model.Publication, error) {
	if id <= 0 {
		return nil, ErrInvalidInput
	}

	publication, err := s.publicationRepo.GetByID(ctx, id)
	if err != nil {
		return nil, mapPublicationRepositoryError(err)
	}

	now := s.clock.Now()
	return publicationWithEffectiveState(publication, now), nil
}

func (s *PublicationService) GetCurrentPublication(
	ctx context.Context,
) (*model.Publication, error) {
	publication, err := s.publicationRepo.GetCurrent(ctx)
	if err != nil {
		return nil, mapPublicationRepositoryError(err)
	}
	if publication == nil {
		return nil, nil
	}

	now := s.clock.Now()
	return publicationWithEffectiveState(publication, now), nil
}

func (s *PublicationService) DeletePublication(ctx context.Context, id int64) error {
	if id <= 0 {
		return ErrInvalidInput
	}

	now := s.clock.Now()
	existing, err := s.publicationRepo.GetByID(ctx, id)
	if err != nil {
		return mapPublicationRepositoryError(err)
	}

	err = s.publicationRepo.DeletePublication(ctx, repository.DeletePublicationParams{
		ID:  id,
		Now: now,
	})
	switch {
	case err == nil:
		targetID := id
		audit.Record(ctx, audit.Event{
			Action:     audit.ActionPublicationDelete,
			TargetType: audit.TargetTypePublication,
			TargetID:   &targetID,
			Metadata: map[string]any{
				"name": existing.Name,
			},
		})
		return nil
	case !errors.Is(err, repository.ErrPublicationNotFound):
		return mapPublicationRepositoryError(err)
	}

	publication, getErr := s.publicationRepo.GetByID(ctx, id)
	if errors.Is(getErr, repository.ErrPublicationNotFound) {
		return ErrPublicationNotFound
	}
	if getErr != nil {
		return mapPublicationRepositoryError(getErr)
	}
	if publication != nil {
		return ErrPublicationNotDeletable
	}

	return ErrPublicationNotFound
}

func (s *PublicationService) ListAvailabilitySubmissionShiftIDs(
	ctx context.Context,
	publicationID, userID int64,
) ([]int64, error) {
	if publicationID <= 0 || userID <= 0 {
		return nil, ErrInvalidInput
	}

	shiftIDs, err := s.publicationRepo.ListSubmissionShiftIDs(ctx, publicationID, userID)
	if err != nil {
		return nil, mapPublicationRepositoryError(err)
	}

	return shiftIDs, nil
}

func (s *PublicationService) CreateAvailabilitySubmission(
	ctx context.Context,
	input CreateAvailabilitySubmissionInput,
) (*model.AvailabilitySubmission, error) {
	if input.PublicationID <= 0 || input.UserID <= 0 || input.TemplateShiftID <= 0 {
		return nil, ErrInvalidInput
	}

	publication, err := s.publicationRepo.GetByID(ctx, input.PublicationID)
	if err != nil {
		return nil, mapPublicationRepositoryError(err)
	}

	now := s.clock.Now()
	effectiveState := model.ResolvePublicationState(publication, now)
	if effectiveState != model.PublicationStateCollecting {
		return nil, ErrPublicationNotCollecting
	}

	shift, err := s.publicationRepo.GetTemplateShift(ctx, publication.TemplateID, input.TemplateShiftID)
	if err != nil {
		return nil, mapPublicationRepositoryError(err)
	}

	qualified, err := s.publicationRepo.IsUserQualifiedForPosition(ctx, input.UserID, shift.PositionID)
	if err != nil {
		return nil, err
	}
	if !qualified {
		return nil, ErrNotQualified
	}

	submission, err := s.publicationRepo.UpsertSubmission(ctx, repository.UpsertAvailabilitySubmissionParams{
		PublicationID:    input.PublicationID,
		UserID:           input.UserID,
		TemplateShiftID:  input.TemplateShiftID,
		PublicationState: stalePublicationState(publication.State, effectiveState),
		Now:              now,
	})
	if err != nil {
		return nil, mapPublicationRepositoryError(err)
	}

	targetID := submission.ID
	audit.Record(ctx, audit.Event{
		Action:     audit.ActionSubmissionCreate,
		TargetType: audit.TargetTypeAvailabilitySubmission,
		TargetID:   &targetID,
		Metadata: map[string]any{
			"publication_id":    submission.PublicationID,
			"template_shift_id": submission.TemplateShiftID,
		},
	})

	return submission, nil
}

func (s *PublicationService) DeleteAvailabilitySubmission(
	ctx context.Context,
	input DeleteAvailabilitySubmissionInput,
) error {
	if input.PublicationID <= 0 || input.UserID <= 0 || input.TemplateShiftID <= 0 {
		return ErrInvalidInput
	}

	publication, err := s.publicationRepo.GetByID(ctx, input.PublicationID)
	if err != nil {
		return mapPublicationRepositoryError(err)
	}

	now := s.clock.Now()
	effectiveState := model.ResolvePublicationState(publication, now)
	if effectiveState != model.PublicationStateCollecting {
		return ErrPublicationNotCollecting
	}

	if err := s.publicationRepo.DeleteSubmission(ctx, repository.DeleteAvailabilitySubmissionParams{
		PublicationID:    input.PublicationID,
		UserID:           input.UserID,
		TemplateShiftID:  input.TemplateShiftID,
		PublicationState: stalePublicationState(publication.State, effectiveState),
		Now:              now,
	}); err != nil {
		return mapPublicationRepositoryError(err)
	}

	targetID := input.TemplateShiftID
	audit.Record(ctx, audit.Event{
		Action:     audit.ActionSubmissionDelete,
		TargetType: audit.TargetTypeAvailabilitySubmission,
		TargetID:   &targetID,
		Metadata: map[string]any{
			"publication_id":    input.PublicationID,
			"template_shift_id": input.TemplateShiftID,
		},
	})

	return nil
}

func (s *PublicationService) ListQualifiedPublicationShifts(
	ctx context.Context,
	publicationID, userID int64,
) ([]*model.TemplateShift, error) {
	if publicationID <= 0 || userID <= 0 {
		return nil, ErrInvalidInput
	}

	publication, err := s.publicationRepo.GetByID(ctx, publicationID)
	if err != nil {
		return nil, mapPublicationRepositoryError(err)
	}
	now := s.clock.Now()
	if model.ResolvePublicationState(publication, now) != model.PublicationStateCollecting {
		return nil, ErrPublicationNotCollecting
	}

	shifts, err := s.publicationRepo.ListQualifiedShifts(ctx, publicationID, userID)
	if err != nil {
		return nil, mapPublicationRepositoryError(err)
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

func normalizePublicationName(name string) (string, error) {
	normalizedName := strings.TrimSpace(name)
	if normalizedName == "" || utf8.RuneCountInString(normalizedName) > maxPublicationNameLength {
		return "", ErrInvalidInput
	}

	return normalizedName, nil
}

func validatePublicationWindow(
	submissionStartAt time.Time,
	submissionEndAt time.Time,
	plannedActiveFrom time.Time,
) error {
	if !submissionStartAt.Before(submissionEndAt) || plannedActiveFrom.Before(submissionEndAt) {
		return ErrInvalidPublicationWindow
	}

	return nil
}

func publicationWithEffectiveState(
	publication *model.Publication,
	now time.Time,
) *model.Publication {
	if publication == nil {
		return nil
	}

	cloned := *publication
	cloned.State = model.ResolvePublicationState(publication, now)
	return &cloned
}

func stalePublicationState(
	storedState model.PublicationState,
	effectiveState model.PublicationState,
) *model.PublicationState {
	if storedState == effectiveState {
		return nil
	}

	nextState := effectiveState
	return &nextState
}

func mapPublicationRepositoryError(err error) error {
	switch {
	case errors.Is(err, repository.ErrInvalidPublicationWindow):
		return ErrInvalidPublicationWindow
	case errors.Is(err, repository.ErrPublicationAlreadyExists):
		return ErrPublicationAlreadyExists
	case errors.Is(err, repository.ErrPublicationNotFound):
		return ErrPublicationNotFound
	case errors.Is(err, repository.ErrTemplateNotFound):
		return ErrTemplateNotFound
	case errors.Is(err, repository.ErrTemplateShiftNotFound):
		return ErrTemplateShiftNotFound
	case errors.Is(err, repository.ErrUserNotFound):
		return ErrUserNotFound
	default:
		return err
	}
}
