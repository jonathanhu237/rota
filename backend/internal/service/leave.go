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
	GetWithRequestByID(ctx context.Context, id int64) (*repository.LeaveWithRequest, error)
	ListForUser(ctx context.Context, userID int64, page int, pageSize int) ([]*repository.LeaveWithRequest, error)
	ListForPublication(ctx context.Context, publicationID int64, page int, pageSize int) ([]*repository.LeaveWithRequest, error)
	ListPool(ctx context.Context, params repository.ListLeavePoolParams) ([]*repository.LeaveWithRequest, int, error)
}

type leaveShiftChangeRepository interface {
	CreateTx(ctx context.Context, tx *sql.Tx, params repository.CreateShiftChangeRequestParams) (*model.ShiftChangeRequest, error)
	SetLeaveIDTx(ctx context.Context, tx *sql.Tx, requestID int64, leaveID int64) (*model.ShiftChangeRequest, error)
}

type leavePublicationRepository interface {
	GetCurrent(ctx context.Context) (*model.Publication, error)
	ListPublicationAssignments(ctx context.Context, publicationID int64) ([]*model.AssignmentParticipant, error)
	ListPublicationShifts(ctx context.Context, publicationID int64) ([]*model.PublicationShift, error)
	ListAssignmentBoardEmployees(ctx context.Context, publicationID int64) ([]*model.AssignmentBoardEmployee, error)
	IsUserQualifiedForPosition(ctx context.Context, userID, positionID int64) (bool, error)
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
	Leave           *model.Leave
	Request         *model.ShiftChangeRequest
	State           model.LeaveState
	RequesterName   string
	CounterpartName *string
	SubstituteName  *string
	Shift           *LeaveShiftContext
	Urgency         *LeaveUrgency
	Actions         LeaveActions
}

type LeaveShiftContext struct {
	AssignmentID    int64
	SlotID          int64
	Weekday         int
	StartTime       string
	EndTime         string
	PositionID      int64
	PositionName    string
	OccurrenceStart time.Time
	OccurrenceEnd   time.Time
}

type LeaveUrgency struct {
	OccurrenceStart     time.Time
	SecondsUntilStart   int64
	StartsWithin24Hours bool
}

type LeaveActions struct {
	CanClaim       bool
	CanApprove     bool
	CanReject      bool
	CanCancel      bool
	DisabledReason model.LeaveActionDisabledReason
}

type ListLeavePoolInput struct {
	State    string
	Page     int
	PageSize int
}

type LeavePoolResult struct {
	Leaves     []*LeaveDetail
	Page       int
	PageSize   int
	TotalCount int
}

type ListLeavesInput struct {
	Page     int
	PageSize int
}

type OccurrencePreview struct {
	AssignmentID     int64
	OccurrenceDate   time.Time
	Slot             *model.TemplateSlot
	Position         *model.Position
	OccurrenceStart  time.Time
	OccurrenceEnd    time.Time
	DirectCandidates []LeaveDirectCandidate
}

type LeaveDirectCandidate struct {
	UserID int64
	Name   string
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
		if err != nil {
			return err
		}
		if shouldNotifyShiftChangeCreate(request.Type) {
			return s.shiftChangeService.enqueueRequestCreatedTx(ctx, tx, request, prepared.requesterShift, prepared.counterpartShift)
		}
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

func (s *LeaveService) GetByID(
	ctx context.Context,
	leaveID int64,
	viewerUserID int64,
	viewerIsAdmin bool,
) (*LeaveDetail, error) {
	if leaveID <= 0 {
		return nil, ErrInvalidInput
	}
	row, err := s.leaveRepo.GetWithRequestByID(ctx, leaveID)
	if err != nil {
		return nil, mapLeaveRepositoryError(err)
	}
	return s.newLeaveDetailFromRow(ctx, row, viewerUserID, viewerIsAdmin), nil
}

func (s *LeaveService) ListPool(
	ctx context.Context,
	viewerUserID int64,
	viewerIsAdmin bool,
	input ListLeavePoolInput,
) (*LeavePoolResult, error) {
	if viewerUserID <= 0 {
		return nil, ErrInvalidInput
	}
	state, err := normalizeLeavePoolState(input.State)
	if err != nil {
		return nil, err
	}
	page, pageSize, err := normalizeLeavePoolPagination(input.Page, input.PageSize)
	if err != nil {
		return nil, err
	}
	rows, total, err := s.leaveRepo.ListPool(ctx, repository.ListLeavePoolParams{
		ViewerUserID:  viewerUserID,
		ViewerIsAdmin: viewerIsAdmin,
		State:         state,
		Now:           s.clock.Now(),
		Offset:        (page - 1) * pageSize,
		Limit:         pageSize,
	})
	if err != nil {
		return nil, err
	}
	out := make([]*LeaveDetail, 0, len(rows))
	for _, row := range rows {
		out = append(out, s.newLeaveDetailFromRow(ctx, row, viewerUserID, viewerIsAdmin))
	}
	return &LeavePoolResult{
		Leaves:     out,
		Page:       page,
		PageSize:   pageSize,
		TotalCount: total,
	}, nil
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
	employees, err := s.publicationRepo.ListAssignmentBoardEmployees(ctx, publication.ID)
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
		assignmentOccurrence := &model.Assignment{Weekday: assignment.Weekday}
		for date := from; !date.After(to); date = date.AddDate(0, 0, 1) {
			if weekdayToSlotValue(date.Weekday()) != assignment.Weekday {
				continue
			}
			if err := model.IsValidOccurrenceForAssignment(publication, slot, assignmentOccurrence, date, now); err != nil {
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
				AssignmentID:     assignment.AssignmentID,
				OccurrenceDate:   model.NormalizeOccurrenceDate(date),
				Slot:             slot,
				Position:         position,
				OccurrenceStart:  start,
				OccurrenceEnd:    end,
				DirectCandidates: directCandidatesForPosition(employees, userID, position.ID),
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
		out = append(out, s.newLeaveDetailFromRow(ctx, row, 0, false))
	}
	return out
}

func newLeaveDetail(leave *model.Leave, req *model.ShiftChangeRequest) *LeaveDetail {
	return &LeaveDetail{
		Leave:         leave,
		Request:       req,
		State:         model.LeaveStateFromSCRT(req.State),
		RequesterName: "",
	}
}

func (s *LeaveService) newLeaveDetailFromRow(
	ctx context.Context,
	row *repository.LeaveWithRequest,
	viewerUserID int64,
	viewerIsAdmin bool,
) *LeaveDetail {
	req := s.shiftChangeService.expireOnReadIfStale(ctx, row.Request)
	detail := newLeaveDetail(row.Leave, req)
	detail.RequesterName = row.RequesterName
	detail.CounterpartName = row.CounterpartName
	if model.LeaveStateFromSCRT(req.State) == model.LeaveStateCompleted {
		detail.SubstituteName = row.SubstituteName
	}
	if row.Shift != nil {
		detail.Shift = &LeaveShiftContext{
			AssignmentID:    row.Shift.AssignmentID,
			SlotID:          row.Shift.SlotID,
			Weekday:         row.Shift.Weekday,
			StartTime:       row.Shift.StartTime,
			EndTime:         row.Shift.EndTime,
			PositionID:      row.Shift.PositionID,
			PositionName:    row.Shift.PositionName,
			OccurrenceStart: row.Shift.OccurrenceStart,
			OccurrenceEnd:   row.Shift.OccurrenceEnd,
		}
		detail.Urgency = s.leaveUrgency(detail)
	}
	detail.Actions = s.leaveActions(ctx, detail, viewerUserID, viewerIsAdmin)
	return detail
}

func (s *LeaveService) leaveUrgency(detail *LeaveDetail) *LeaveUrgency {
	if detail == nil || detail.Shift == nil || detail.State != model.LeaveStatePending {
		return nil
	}
	seconds := int64(detail.Shift.OccurrenceStart.Sub(s.clock.Now()).Seconds())
	if seconds < 0 {
		seconds = 0
	}
	return &LeaveUrgency{
		OccurrenceStart:     detail.Shift.OccurrenceStart,
		SecondsUntilStart:   seconds,
		StartsWithin24Hours: seconds <= int64((24 * time.Hour).Seconds()),
	}
}

func (s *LeaveService) leaveActions(
	ctx context.Context,
	detail *LeaveDetail,
	viewerUserID int64,
	viewerIsAdmin bool,
) LeaveActions {
	if detail == nil || detail.Request == nil || viewerUserID <= 0 {
		return LeaveActions{}
	}
	if viewerIsAdmin {
		return LeaveActions{DisabledReason: model.LeaveActionDisabledAdminViewOnly}
	}
	if detail.State != model.LeaveStatePending {
		return LeaveActions{}
	}
	if detail.Leave.UserID == viewerUserID {
		return LeaveActions{CanCancel: true}
	}
	req := detail.Request
	switch req.Type {
	case model.ShiftChangeTypeGiveDirect:
		if req.CounterpartUserID != nil && *req.CounterpartUserID == viewerUserID {
			return LeaveActions{CanApprove: true, CanReject: true}
		}
	case model.ShiftChangeTypeGivePool:
		if detail.Shift == nil || detail.Shift.PositionID <= 0 {
			return LeaveActions{DisabledReason: model.LeaveActionDisabledNotQualified}
		}
		qualified, err := s.publicationRepo.IsUserQualifiedForPosition(ctx, viewerUserID, detail.Shift.PositionID)
		if err != nil || !qualified {
			return LeaveActions{DisabledReason: model.LeaveActionDisabledNotQualified}
		}
		return LeaveActions{CanClaim: true}
	}
	return LeaveActions{}
}

func normalizeLeavePoolState(value string) (model.LeavePoolStateFilter, error) {
	if value == "" {
		return model.LeavePoolStatePending, nil
	}
	switch model.LeavePoolStateFilter(value) {
	case model.LeavePoolStatePending,
		model.LeavePoolStateCompleted,
		model.LeavePoolStateFailed,
		model.LeavePoolStateCancelled,
		model.LeavePoolStateAll:
		return model.LeavePoolStateFilter(value), nil
	default:
		return "", ErrInvalidInput
	}
}

func normalizeLeavePoolPagination(page, pageSize int) (int, int, error) {
	if page == 0 {
		page = 1
	}
	if pageSize == 0 {
		pageSize = 20
	}
	if page < 1 || pageSize < 1 || pageSize > 100 {
		return 0, 0, ErrInvalidInput
	}
	return page, pageSize, nil
}

func directCandidatesForPosition(
	employees []*model.AssignmentBoardEmployee,
	requesterID int64,
	positionID int64,
) []LeaveDirectCandidate {
	out := make([]LeaveDirectCandidate, 0)
	for _, employee := range employees {
		if employee.UserID == requesterID {
			continue
		}
		for _, candidatePositionID := range employee.PositionIDs {
			if candidatePositionID == positionID {
				out = append(out, LeaveDirectCandidate{UserID: employee.UserID, Name: employee.Name})
				break
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].UserID < out[j].UserID
	})
	return out
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
