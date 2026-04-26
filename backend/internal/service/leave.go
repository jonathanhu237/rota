package service

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/audit"
	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/jonathanhu237/rota/backend/internal/repository"
)

var (
	ErrLeaveNotFound = model.ErrLeaveNotFound
	ErrLeaveNotOwner = model.ErrLeaveNotOwner
)

type leaveRepository interface {
	WithTx(ctx context.Context, fn func(tx *sql.Tx) error) error
	Insert(ctx context.Context, tx *sql.Tx, params repository.InsertLeaveParams) (*model.Leave, error)
	GetByID(ctx context.Context, id int64) (*model.Leave, *model.ShiftChangeRequest, error)
	ListForUser(ctx context.Context, userID int64, page int, pageSize int) ([]*repository.LeaveWithRequest, error)
	ListForPublication(ctx context.Context, publicationID int64, page int, pageSize int) ([]*repository.LeaveWithRequest, error)
}

type leaveShiftChangeRepository interface {
	CreateTx(ctx context.Context, tx *sql.Tx, params repository.CreateShiftChangeRequestParams) (*model.ShiftChangeRequest, error)
	SetLeaveIDTx(ctx context.Context, tx *sql.Tx, requestID int64, leaveID int64) (*model.ShiftChangeRequest, error)
}

type leavePublicationRepository interface {
	GetCurrent(ctx context.Context) (*model.Publication, error)
	ListPublicationAssignments(ctx context.Context, publicationID int64) ([]*model.AssignmentParticipant, error)
	ListPublicationShifts(ctx context.Context, publicationID int64) ([]*model.PublicationShift, error)
}

type LeaveService struct {
	leaveRepo          leaveRepository
	shiftChangeRepo    leaveShiftChangeRepository
	shiftChangeService *ShiftChangeService
	publicationRepo    leavePublicationRepository
	clock              Clock
}

type CreateLeaveInput struct {
	UserID            int64
	AssignmentID      int64
	OccurrenceDate    time.Time
	Type              model.ShiftChangeType
	CounterpartUserID *int64
	Category          model.LeaveCategory
	Reason            string
}

type LeaveDetail struct {
	Leave   *model.Leave
	Request *model.ShiftChangeRequest
	State   model.LeaveState
}

type ListLeavesInput struct {
	Page     int
	PageSize int
}

type OccurrencePreview struct {
	AssignmentID    int64
	OccurrenceDate  time.Time
	Slot            *model.TemplateSlot
	Position        *model.Position
	OccurrenceStart time.Time
	OccurrenceEnd   time.Time
}

func NewLeaveService(
	leaveRepo leaveRepository,
	shiftChangeRepo leaveShiftChangeRepository,
	shiftChangeService *ShiftChangeService,
	publicationRepo leavePublicationRepository,
	clock Clock,
) *LeaveService {
	if clock == nil {
		clock = realClock{}
	}
	return &LeaveService{
		leaveRepo:          leaveRepo,
		shiftChangeRepo:    shiftChangeRepo,
		shiftChangeService: shiftChangeService,
		publicationRepo:    publicationRepo,
		clock:              clock,
	}
}

func (s *LeaveService) Create(ctx context.Context, input CreateLeaveInput) (*LeaveDetail, error) {
	if input.UserID <= 0 || input.AssignmentID <= 0 || input.OccurrenceDate.IsZero() {
		return nil, ErrInvalidInput
	}
	if !isValidLeaveCategory(input.Category) {
		return nil, ErrInvalidInput
	}
	if input.Type == model.ShiftChangeTypeSwap || !isValidShiftChangeType(input.Type) {
		return nil, ErrShiftChangeInvalidType
	}

	now := s.clock.Now()
	publication, err := s.publicationRepo.GetCurrent(ctx)
	if err != nil {
		return nil, mapPublicationRepositoryError(err)
	}
	if publication == nil || model.ResolvePublicationState(publication, now) != model.PublicationStateActive {
		return nil, ErrPublicationNotActive
	}

	prepared, err := s.shiftChangeService.prepareCreateShiftChangeRequest(ctx, CreateShiftChangeInput{
		PublicationID:         publication.ID,
		RequesterUserID:       input.UserID,
		Type:                  input.Type,
		RequesterAssignmentID: input.AssignmentID,
		OccurrenceDate:        input.OccurrenceDate,
		CounterpartUserID:     input.CounterpartUserID,
	}, publication, now)
	if err != nil {
		return nil, err
	}

	var leave *model.Leave
	var request *model.ShiftChangeRequest
	if err := s.leaveRepo.WithTx(ctx, func(tx *sql.Tx) error {
		request, err = s.shiftChangeRepo.CreateTx(ctx, tx, prepared.params)
		if err != nil {
			return err
		}

		leave, err = s.leaveRepo.Insert(ctx, tx, repository.InsertLeaveParams{
			UserID:               input.UserID,
			PublicationID:        publication.ID,
			ShiftChangeRequestID: request.ID,
			Category:             input.Category,
			Reason:               input.Reason,
			CreatedAt:            now,
			UpdatedAt:            now,
		})
		if err != nil {
			return err
		}

		request, err = s.shiftChangeRepo.SetLeaveIDTx(ctx, tx, request.ID, leave.ID)
		return err
	}); err != nil {
		return nil, err
	}

	s.shiftChangeService.recordShiftChangeCreated(ctx, request, prepared.requesterShift, prepared.counterpartShift)
	targetID := leave.ID
	audit.Record(ctx, audit.Event{
		Action:     audit.ActionLeaveCreate,
		TargetType: audit.TargetTypeLeave,
		TargetID:   &targetID,
		Metadata: map[string]any{
			"leave_id":                leave.ID,
			"user_id":                 leave.UserID,
			"publication_id":          leave.PublicationID,
			"shift_change_request_id": leave.ShiftChangeRequestID,
			"category":                leave.Category,
		},
	})

	return newLeaveDetail(leave, request), nil
}

func (s *LeaveService) Cancel(ctx context.Context, leaveID, userID int64) error {
	if leaveID <= 0 || userID <= 0 {
		return ErrInvalidInput
	}

	leave, req, err := s.leaveRepo.GetByID(ctx, leaveID)
	if err != nil {
		return mapLeaveRepositoryError(err)
	}
	if leave.UserID != userID {
		return ErrLeaveNotOwner
	}

	now := s.clock.Now()
	effState := model.EffectiveShiftChangeState(req.State, req.ExpiresAt, now)
	if req.State == model.ShiftChangeStatePending && effState == model.ShiftChangeStateExpired {
		_ = s.shiftChangeService.shiftChangeRepo.MarkExpired(ctx, req.ID, now)
		return nil
	}
	if effState != model.ShiftChangeStatePending {
		return nil
	}

	if err := s.shiftChangeService.CancelShiftChangeRequest(ctx, req.ID, userID); err != nil {
		return err
	}

	targetID := leave.ID
	audit.Record(ctx, audit.Event{
		Action:     audit.ActionLeaveCancel,
		TargetType: audit.TargetTypeLeave,
		TargetID:   &targetID,
		Metadata: map[string]any{
			"leave_id": leave.ID,
		},
	})
	return nil
}

func (s *LeaveService) GetByID(ctx context.Context, leaveID int64) (*LeaveDetail, error) {
	if leaveID <= 0 {
		return nil, ErrInvalidInput
	}
	leave, req, err := s.leaveRepo.GetByID(ctx, leaveID)
	if err != nil {
		return nil, mapLeaveRepositoryError(err)
	}
	req = s.shiftChangeService.expireOnReadIfStale(ctx, req)
	return newLeaveDetail(leave, req), nil
}

func (s *LeaveService) ListForUser(
	ctx context.Context,
	userID int64,
	input ListLeavesInput,
) ([]*LeaveDetail, error) {
	if userID <= 0 {
		return nil, ErrInvalidInput
	}
	page, pageSize, err := normalizePagination(input.Page, input.PageSize)
	if err != nil {
		return nil, err
	}
	rows, err := s.leaveRepo.ListForUser(ctx, userID, page, pageSize)
	if err != nil {
		return nil, err
	}
	return s.newLeaveDetails(ctx, rows), nil
}

func (s *LeaveService) ListForPublication(
	ctx context.Context,
	publicationID int64,
	input ListLeavesInput,
) ([]*LeaveDetail, error) {
	if publicationID <= 0 {
		return nil, ErrInvalidInput
	}
	page, pageSize, err := normalizePagination(input.Page, input.PageSize)
	if err != nil {
		return nil, err
	}
	rows, err := s.leaveRepo.ListForPublication(ctx, publicationID, page, pageSize)
	if err != nil {
		return nil, err
	}
	return s.newLeaveDetails(ctx, rows), nil
}

func (s *LeaveService) PreviewOccurrences(
	ctx context.Context,
	userID int64,
	from time.Time,
	to time.Time,
) ([]*OccurrencePreview, error) {
	if userID <= 0 || from.IsZero() || to.IsZero() {
		return nil, ErrInvalidInput
	}
	from = model.NormalizeOccurrenceDate(from)
	to = model.NormalizeOccurrenceDate(to)
	if from.After(to) {
		return nil, ErrInvalidInput
	}

	now := s.clock.Now()
	publication, err := s.publicationRepo.GetCurrent(ctx)
	if err != nil {
		return nil, mapPublicationRepositoryError(err)
	}
	if publication == nil || model.ResolvePublicationState(publication, now) != model.PublicationStateActive {
		return []*OccurrencePreview{}, nil
	}

	assignments, err := s.publicationRepo.ListPublicationAssignments(ctx, publication.ID)
	if err != nil {
		return nil, err
	}
	shifts, err := s.publicationRepo.ListPublicationShifts(ctx, publication.ID)
	if err != nil {
		return nil, err
	}
	shiftIndex := buildPublicationShiftIndex(shifts)

	out := make([]*OccurrencePreview, 0)
	for _, assignment := range assignments {
		if assignment.UserID != userID {
			continue
		}
		shift := findPublicationShiftForParticipant(shiftIndex, assignment)
		if shift == nil {
			continue
		}
		slot := publicationShiftSlot(shift)
		position := publicationShiftPosition(shift)
		for date := from; !date.After(to); date = date.AddDate(0, 0, 1) {
			if weekdayToSlotValue(date.Weekday()) != shift.Weekday {
				continue
			}
			if err := model.IsValidOccurrence(publication, slot, date, now); err != nil {
				continue
			}
			start, err := model.OccurrenceStart(slot, date)
			if err != nil {
				return nil, err
			}
			end, err := occurrenceEnd(slot, date)
			if err != nil {
				return nil, err
			}
			out = append(out, &OccurrencePreview{
				AssignmentID:    assignment.AssignmentID,
				OccurrenceDate:  model.NormalizeOccurrenceDate(date),
				Slot:            slot,
				Position:        position,
				OccurrenceStart: start,
				OccurrenceEnd:   end,
			})
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if !out[i].OccurrenceStart.Equal(out[j].OccurrenceStart) {
			return out[i].OccurrenceStart.Before(out[j].OccurrenceStart)
		}
		return out[i].AssignmentID < out[j].AssignmentID
	})
	return out, nil
}

func (s *LeaveService) newLeaveDetails(
	ctx context.Context,
	rows []*repository.LeaveWithRequest,
) []*LeaveDetail {
	out := make([]*LeaveDetail, 0, len(rows))
	for _, row := range rows {
		req := s.shiftChangeService.expireOnReadIfStale(ctx, row.Request)
		out = append(out, newLeaveDetail(row.Leave, req))
	}
	return out
}

func newLeaveDetail(leave *model.Leave, req *model.ShiftChangeRequest) *LeaveDetail {
	return &LeaveDetail{
		Leave:   leave,
		Request: req,
		State:   model.LeaveStateFromSCRT(req.State),
	}
}

func isValidLeaveCategory(category model.LeaveCategory) bool {
	switch category {
	case model.LeaveCategorySick, model.LeaveCategoryPersonal, model.LeaveCategoryBereavement:
		return true
	default:
		return false
	}
}

func occurrenceEnd(slot *model.TemplateSlot, occurrenceDate time.Time) (time.Time, error) {
	endClock, err := time.Parse("15:04", slot.EndTime)
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

func mapLeaveRepositoryError(err error) error {
	if errors.Is(err, repository.ErrLeaveNotFound) {
		return ErrLeaveNotFound
	}
	return err
}
