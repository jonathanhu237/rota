package service

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
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
	ErrInvalidPublicationWindow    = model.ErrInvalidPublicationWindow
	ErrInvalidOccurrenceDate       = model.ErrInvalidOccurrenceDate
	ErrPublicationAlreadyExists    = model.ErrPublicationAlreadyExists
	ErrPublicationNotFound         = model.ErrPublicationNotFound
	ErrPublicationNotDeletable     = model.ErrPublicationNotDeletable
	ErrPublicationNotCollecting    = model.ErrPublicationNotCollecting
	ErrPublicationNotMutable       = model.ErrPublicationNotMutable
	ErrPublicationNotAssigning     = model.ErrPublicationNotAssigning
	ErrPublicationNotPublished     = model.ErrPublicationNotPublished
	ErrPublicationNotActive        = model.ErrPublicationNotActive
	ErrAssignmentUserAlreadyInSlot = model.ErrAssignmentUserAlreadyInSlot
	ErrSchedulingRetryable         = model.ErrSchedulingRetryable
	ErrNotQualified                = model.ErrNotQualified
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
	UpdatePublicationFields(ctx context.Context, params repository.UpdatePublicationFieldsParams) (*model.Publication, error)
	DeletePublication(ctx context.Context, params repository.DeletePublicationParams) error
	ListSubmissionSlots(ctx context.Context, publicationID, userID int64) ([]model.SlotRef, error)
	UpsertSubmission(ctx context.Context, params repository.UpsertAvailabilitySubmissionParams) (*model.AvailabilitySubmission, error)
	DeleteSubmission(ctx context.Context, params repository.DeleteAvailabilitySubmissionParams) error
	GetSlot(ctx context.Context, templateID, slotID int64) (*model.TemplateSlot, error)
	ListSlotPositions(ctx context.Context, slotID int64) ([]*model.TemplateSlotPosition, error)
	IsUserQualifiedForPosition(ctx context.Context, userID, positionID int64) (bool, error)
	ListQualifiedPublicationSlotPositions(ctx context.Context, publicationID, userID int64) ([]*model.QualifiedShift, error)
	CreateAssignment(ctx context.Context, params repository.CreateAssignmentParams) (*model.Assignment, error)
	DeleteAssignment(ctx context.Context, params repository.DeleteAssignmentParams) error
	CountAssignmentOverridesByAssignment(ctx context.Context, assignmentID int64) (int64, error)
	GetAssignment(ctx context.Context, id int64) (*model.Assignment, error)
	ReplaceAssignments(ctx context.Context, params repository.ReplaceAssignmentsParams) error
	ActivatePublication(ctx context.Context, params repository.ActivatePublicationParams) (*repository.ActivatePublicationResult, error)
	PublishPublication(ctx context.Context, params repository.PublishPublicationParams) (*model.Publication, error)
	GetAssignmentBoardView(ctx context.Context, publicationID int64) (map[repository.AssignmentBoardSlotKey]*repository.AssignmentBoardSlotView, error)
	ListAssignmentBoardEmployees(ctx context.Context, publicationID int64) ([]*model.AssignmentBoardEmployee, error)
	ListPublicationShifts(ctx context.Context, publicationID int64) ([]*model.PublicationShift, error)
	ListAssignmentCandidates(ctx context.Context, publicationID int64) ([]*model.AssignmentCandidate, error)
	ListQualifiedUsersForPositions(ctx context.Context, positionIDs []int64) (map[int64][]*model.AssignmentCandidate, error)
	ListPublicationAssignments(ctx context.Context, publicationID int64) ([]*model.AssignmentParticipant, error)
	ListPublicationAssignmentsForWeek(ctx context.Context, publicationID int64, weekStart time.Time) ([]*model.AssignmentParticipant, error)
}

type publicationShiftChangeRepository interface {
	GetByID(ctx context.Context, id int64) (*model.ShiftChangeRequest, error)
	InvalidateRequestsForAssignment(ctx context.Context, assignmentID int64, now time.Time) ([]int64, error)
	InvalidateRequestsForAssignmentTx(
		ctx context.Context,
		tx *sql.Tx,
		assignmentID int64,
		now time.Time,
	) ([]*model.ShiftChangeRequest, error)
}

type PublicationService struct {
	publicationRepo  publicationRepository
	shiftChangeRepo  publicationShiftChangeRepository
	outboxRepo       setupOutboxRepository
	logger           *slog.Logger
	clock            Clock
	appBaseURL       string
	brandingProvider emailBrandingProvider
}

type PublicationServiceOption func(*PublicationService)

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
	TemplateID         int64
	Name               string
	Description        string
	SubmissionStartAt  time.Time
	SubmissionEndAt    time.Time
	PlannedActiveFrom  time.Time
	PlannedActiveUntil time.Time
}

type UpdatePublicationInput struct {
	ID                 int64
	Name               *string
	Description        *string
	PlannedActiveUntil *time.Time
}

type CreateAvailabilitySubmissionInput struct {
	PublicationID int64
	UserID        int64
	SlotID        int64
	Weekday       int
}

type DeleteAvailabilitySubmissionInput struct {
	PublicationID int64
	UserID        int64
	SlotID        int64
	Weekday       int
}

type CreateAssignmentInput struct {
	PublicationID int64
	UserID        int64
	SlotID        int64
	Weekday       int
	PositionID    int64
}

type DeleteAssignmentInput struct {
	PublicationID int64
	AssignmentID  int64
}

func WithPublicationShiftChangeNotifications(
	shiftChangeRepo publicationShiftChangeRepository,
	outboxRepo setupOutboxRepository,
	appBaseURL string,
	logger *slog.Logger,
) PublicationServiceOption {
	return func(service *PublicationService) {
		service.shiftChangeRepo = shiftChangeRepo
		service.outboxRepo = outboxRepo
		service.appBaseURL = appBaseURL
		if logger != nil {
			service.logger = logger
		}
	}
}

func WithPublicationBrandingProvider(provider emailBrandingProvider) PublicationServiceOption {
	return func(service *PublicationService) {
		service.brandingProvider = provider
	}
}

func NewPublicationService(
	publicationRepo publicationRepository,
	clock Clock,
	opts ...PublicationServiceOption,
) *PublicationService {
	if clock == nil {
		clock = realClock{}
	}

	service := &PublicationService{
		publicationRepo: publicationRepo,
		clock:           clock,
		logger:          slog.Default(),
	}

	for _, opt := range opts {
		opt(service)
	}

	return service
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
		input.PlannedActiveUntil,
	); err != nil {
		return nil, err
	}

	now := s.clock.Now()
	publication, err := s.publicationRepo.CreatePublication(ctx, repository.CreatePublicationParams{
		TemplateID:         input.TemplateID,
		Name:               name,
		Description:        strings.TrimSpace(input.Description),
		State:              model.PublicationStateDraft,
		SubmissionStartAt:  input.SubmissionStartAt,
		SubmissionEndAt:    input.SubmissionEndAt,
		PlannedActiveFrom:  input.PlannedActiveFrom,
		PlannedActiveUntil: input.PlannedActiveUntil,
		CreatedAt:          now,
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
			"template_id":          publication.TemplateID,
			"name":                 publication.Name,
			"description":          publication.Description,
			"submission_start_at":  publication.SubmissionStartAt.Format(time.RFC3339),
			"submission_end_at":    publication.SubmissionEndAt.Format(time.RFC3339),
			"planned_active_from":  publication.PlannedActiveFrom.Format(time.RFC3339),
			"planned_active_until": publication.PlannedActiveUntil.Format(time.RFC3339),
		},
	})

	return publicationWithEffectiveState(publication, now), nil
}

func (s *PublicationService) UpdatePublication(
	ctx context.Context,
	input UpdatePublicationInput,
) (*model.Publication, error) {
	if input.ID <= 0 {
		return nil, ErrInvalidInput
	}
	if input.Name == nil && input.Description == nil && input.PlannedActiveUntil == nil {
		publication, err := s.publicationRepo.GetByID(ctx, input.ID)
		if err != nil {
			return nil, mapPublicationRepositoryError(err)
		}
		return publicationWithEffectiveState(publication, s.clock.Now()), nil
	}

	current, err := s.publicationRepo.GetByID(ctx, input.ID)
	if err != nil {
		return nil, mapPublicationRepositoryError(err)
	}

	var normalizedName *string
	if input.Name != nil {
		name, err := normalizePublicationName(*input.Name)
		if err != nil {
			return nil, err
		}
		normalizedName = &name
	}

	var normalizedDescription *string
	if input.Description != nil {
		description := strings.TrimSpace(*input.Description)
		normalizedDescription = &description
	}

	newUntil := current.PlannedActiveUntil
	if input.PlannedActiveUntil != nil {
		newUntil = *input.PlannedActiveUntil
	}
	if !current.PlannedActiveFrom.Before(newUntil) {
		return nil, ErrInvalidPublicationWindow
	}

	now := s.clock.Now()
	updated, err := s.publicationRepo.UpdatePublicationFields(ctx, repository.UpdatePublicationFieldsParams{
		ID:                 input.ID,
		Name:               normalizedName,
		Description:        normalizedDescription,
		PlannedActiveUntil: input.PlannedActiveUntil,
		UpdatedAt:          now,
	})
	if err != nil {
		return nil, mapPublicationRepositoryError(err)
	}

	targetID := updated.ID
	audit.Record(ctx, audit.Event{
		Action:     audit.ActionPublicationUpdate,
		TargetType: audit.TargetTypePublication,
		TargetID:   &targetID,
		Metadata:   publicationUpdateMetadata(current, updated, input),
	})

	return publicationWithEffectiveState(updated, now), nil
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

func (s *PublicationService) ListAvailabilitySubmissionSlots(
	ctx context.Context,
	publicationID, userID int64,
) ([]model.SlotRef, error) {
	if publicationID <= 0 || userID <= 0 {
		return nil, ErrInvalidInput
	}

	slots, err := s.publicationRepo.ListSubmissionSlots(ctx, publicationID, userID)
	if err != nil {
		return nil, mapPublicationRepositoryError(err)
	}

	return slots, nil
}

func (s *PublicationService) CreateAvailabilitySubmission(
	ctx context.Context,
	input CreateAvailabilitySubmissionInput,
) (*model.AvailabilitySubmission, error) {
	if input.PublicationID <= 0 || input.UserID <= 0 || input.SlotID <= 0 || input.Weekday < 1 || input.Weekday > 7 {
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

	slot, err := s.publicationRepo.GetSlot(ctx, publication.TemplateID, input.SlotID)
	if err != nil {
		return nil, mapPublicationRepositoryError(err)
	}
	if !slotHasWeekday(slot, input.Weekday) {
		return nil, ErrInvalidInput
	}

	slotPositions, err := s.publicationRepo.ListSlotPositions(ctx, input.SlotID)
	if err != nil {
		return nil, mapPublicationRepositoryError(err)
	}
	qualified, err := s.userQualifiedForAnySlotPosition(ctx, input.UserID, slotPositions)
	if err != nil {
		return nil, err
	}
	if !qualified {
		return nil, ErrNotQualified
	}

	submission, err := s.publicationRepo.UpsertSubmission(ctx, repository.UpsertAvailabilitySubmissionParams{
		PublicationID:    input.PublicationID,
		UserID:           input.UserID,
		SlotID:           input.SlotID,
		Weekday:          input.Weekday,
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
			"publication_id": submission.PublicationID,
			"slot_id":        submission.SlotID,
			"weekday":        submission.Weekday,
		},
	})

	return submission, nil
}

func (s *PublicationService) DeleteAvailabilitySubmission(
	ctx context.Context,
	input DeleteAvailabilitySubmissionInput,
) error {
	if input.PublicationID <= 0 || input.UserID <= 0 || input.SlotID <= 0 || input.Weekday < 1 || input.Weekday > 7 {
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
		SlotID:           input.SlotID,
		Weekday:          input.Weekday,
		PublicationState: stalePublicationState(publication.State, effectiveState),
		Now:              now,
	}); err != nil {
		return mapPublicationRepositoryError(err)
	}

	audit.Record(ctx, audit.Event{
		Action:     audit.ActionSubmissionDelete,
		TargetType: audit.TargetTypeAvailabilitySubmission,
		Metadata: map[string]any{
			"publication_id": input.PublicationID,
			"slot_id":        input.SlotID,
			"weekday":        input.Weekday,
		},
	})

	return nil
}

func (s *PublicationService) ListQualifiedPublicationSlotPositions(
	ctx context.Context,
	publicationID, userID int64,
) ([]*model.QualifiedShift, error) {
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

	shifts, err := s.publicationRepo.ListQualifiedPublicationSlotPositions(ctx, publicationID, userID)
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
		if shifts[i].SlotID != shifts[j].SlotID {
			return shifts[i].SlotID < shifts[j].SlotID
		}
		return false
	})

	return shifts, nil
}

func (s *PublicationService) userQualifiedForAnySlotPosition(
	ctx context.Context,
	userID int64,
	slotPositions []*model.TemplateSlotPosition,
) (bool, error) {
	for _, slotPosition := range slotPositions {
		if slotPosition == nil {
			continue
		}
		qualified, err := s.publicationRepo.IsUserQualifiedForPosition(ctx, userID, slotPosition.PositionID)
		if err != nil {
			return false, err
		}
		if qualified {
			return true, nil
		}
	}

	return false, nil
}

func slotHasWeekday(slot *model.TemplateSlot, weekday int) bool {
	if slot == nil {
		return false
	}
	for _, candidate := range slot.Weekdays {
		if candidate == weekday {
			return true
		}
	}
	return false
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
	plannedActiveUntil time.Time,
) error {
	if !submissionStartAt.Before(submissionEndAt) ||
		plannedActiveFrom.Before(submissionEndAt) ||
		!plannedActiveFrom.Before(plannedActiveUntil) {
		return ErrInvalidPublicationWindow
	}

	return nil
}

func publicationUpdateMetadata(
	before *model.Publication,
	after *model.Publication,
	input UpdatePublicationInput,
) map[string]any {
	metadata := map[string]any{
		"publication_id": after.ID,
	}
	if input.Name != nil && before.Name != after.Name {
		metadata["name"] = map[string]any{"from": before.Name, "to": after.Name}
	}
	if input.Description != nil && before.Description != after.Description {
		metadata["description"] = map[string]any{"from": before.Description, "to": after.Description}
	}
	if input.PlannedActiveUntil != nil && !before.PlannedActiveUntil.Equal(after.PlannedActiveUntil) {
		metadata["planned_active_until"] = map[string]any{
			"from": before.PlannedActiveUntil.Format(time.RFC3339),
			"to":   after.PlannedActiveUntil.Format(time.RFC3339),
		}
	}
	return metadata
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
	case errors.Is(err, repository.ErrTemplateSlotNotFound):
		return ErrTemplateSlotNotFound
	case errors.Is(err, repository.ErrTemplateSlotPositionNotFound):
		return ErrTemplateSlotPositionNotFound
	case errors.Is(err, repository.ErrUserNotFound):
		return ErrUserNotFound
	case errors.Is(err, repository.ErrAssignmentUserAlreadyInSlot):
		return ErrAssignmentUserAlreadyInSlot
	case errors.Is(err, repository.ErrUserDisabled):
		return ErrUserDisabled
	case errors.Is(err, repository.ErrSchedulingRetryable):
		return ErrSchedulingRetryable
	default:
		return err
	}
}
