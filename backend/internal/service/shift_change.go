package service

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"sort"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/audit"
	"github.com/jonathanhu237/rota/backend/internal/email"
	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/jonathanhu237/rota/backend/internal/repository"
)

var (
	ErrShiftChangeNotFound       = model.ErrShiftChangeNotFound
	ErrShiftChangeInvalidType    = model.ErrShiftChangeInvalidType
	ErrShiftChangeNotOwner       = model.ErrShiftChangeNotOwner
	ErrShiftChangeNotQualified   = model.ErrShiftChangeNotQualified
	ErrShiftChangeNotPending     = model.ErrShiftChangeNotPending
	ErrShiftChangeExpired        = model.ErrShiftChangeExpired
	ErrShiftChangeInvalidated    = model.ErrShiftChangeInvalidated
	ErrShiftChangeSelf           = model.ErrShiftChangeSelf
	ErrShiftChangeAssignmentMiss = model.ErrShiftChangeAssignmentMiss
)

// shiftChangeRepository is the subset of repository operations the service
// needs. Kept narrow so tests can supply a stateful mock.
type shiftChangeRepository interface {
	Create(ctx context.Context, params repository.CreateShiftChangeRequestParams) (*model.ShiftChangeRequest, error)
	GetByID(ctx context.Context, id int64) (*model.ShiftChangeRequest, error)
	ListForPublication(ctx context.Context, params repository.ListForPublicationParams) ([]*model.ShiftChangeRequest, error)
	CountPendingForCounterpart(ctx context.Context, userID int64, now time.Time) (int, error)
	UpdateState(ctx context.Context, params repository.UpdateStateParams) error
	ApplySwap(ctx context.Context, params repository.ApplySwapParams) (*repository.ApproveResult, error)
	ApplyGive(ctx context.Context, params repository.ApplyGiveParams) (*repository.ApproveResult, error)
	MarkInvalidated(ctx context.Context, id int64, now time.Time) error
	MarkExpired(ctx context.Context, id int64, now time.Time) error
}

// shiftChangeDeps groups the other repositories the service depends on.
// The publication service already owns a rich repo with most of what we
// need (Publication, slot-position assignment rows, user qualification) —
// we bind that same repository interface here for consistency.
type shiftChangeDeps interface {
	GetByID(ctx context.Context, id int64) (*model.Publication, error)
	GetUserByID(ctx context.Context, id int64) (*model.User, error)
	GetAssignment(ctx context.Context, id int64) (*model.Assignment, error)
	IsUserQualifiedForPosition(ctx context.Context, userID, positionID int64) (bool, error)
	ListPublicationShifts(ctx context.Context, publicationID int64) ([]*model.PublicationShift, error)
	ListPublicationAssignments(ctx context.Context, publicationID int64) ([]*model.AssignmentParticipant, error)
}

// ShiftChangeService orchestrates the swap / give / pool lifecycle.
type ShiftChangeService struct {
	shiftChangeRepo shiftChangeRepository
	publicationRepo shiftChangeDeps
	outboxRepo      setupOutboxRepository
	logger          *slog.Logger
	clock           Clock
	appBaseURL      string
}

// NewShiftChangeService constructs a ShiftChangeService. If logger is nil,
// slog.Default is used. If clock is nil, a real clock is used.
func NewShiftChangeService(
	shiftChangeRepo shiftChangeRepository,
	publicationRepo shiftChangeDeps,
	outboxRepo setupOutboxRepository,
	appBaseURL string,
	clock Clock,
	logger *slog.Logger,
) *ShiftChangeService {
	if clock == nil {
		clock = realClock{}
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &ShiftChangeService{
		shiftChangeRepo: shiftChangeRepo,
		publicationRepo: publicationRepo,
		outboxRepo:      outboxRepo,
		logger:          logger,
		clock:           clock,
		appBaseURL:      appBaseURL,
	}
}

// CreateShiftChangeInput is the admin-authoritative input shape for the
// create request.
type CreateShiftChangeInput struct {
	PublicationID             int64
	RequesterUserID           int64
	Type                      model.ShiftChangeType
	RequesterAssignmentID     int64
	OccurrenceDate            time.Time
	CounterpartUserID         *int64
	CounterpartAssignmentID   *int64
	CounterpartOccurrenceDate *time.Time
	LeaveID                   *int64
}

// CreateShiftChangeRequest validates preconditions and persists a new row.
func (s *ShiftChangeService) CreateShiftChangeRequest(
	ctx context.Context,
	input CreateShiftChangeInput,
) (*model.ShiftChangeRequest, error) {
	if input.PublicationID <= 0 ||
		input.RequesterUserID <= 0 ||
		input.RequesterAssignmentID <= 0 ||
		input.OccurrenceDate.IsZero() {
		return nil, ErrInvalidInput
	}
	if input.LeaveID != nil {
		return nil, ErrInvalidInput
	}
	if !isValidShiftChangeType(input.Type) {
		return nil, ErrShiftChangeInvalidType
	}

	now := s.clock.Now()

	publication, err := s.publicationRepo.GetByID(ctx, input.PublicationID)
	if err != nil {
		return nil, mapPublicationRepositoryError(err)
	}
	if model.ResolvePublicationState(publication, now) != model.PublicationStatePublished {
		return nil, ErrPublicationNotPublished
	}

	prepared, err := s.prepareCreateShiftChangeRequest(ctx, input, publication, now)
	if err != nil {
		return nil, err
	}
	if shouldNotifyShiftChangeCreate(prepared.params.Type) && s.outboxRepo != nil {
		prepared.params.AfterCreateTx = func(
			ctx context.Context,
			tx *sql.Tx,
			created *model.ShiftChangeRequest,
		) error {
			return s.enqueueRequestCreatedTx(ctx, tx, created, prepared.requesterShift, prepared.counterpartShift)
		}
	}

	created, err := s.shiftChangeRepo.Create(ctx, prepared.params)
	if err != nil {
		return nil, err
	}

	s.recordShiftChangeCreated(ctx, created, prepared.requesterShift, prepared.counterpartShift)
	return created, nil
}

type preparedShiftChangeCreate struct {
	params           repository.CreateShiftChangeRequestParams
	requesterShift   *model.PublicationShift
	counterpartShift *model.PublicationShift
}

func (s *ShiftChangeService) prepareCreateShiftChangeRequest(
	ctx context.Context,
	input CreateShiftChangeInput,
	publication *model.Publication,
	now time.Time,
) (*preparedShiftChangeCreate, error) {
	if input.PublicationID <= 0 ||
		input.RequesterUserID <= 0 ||
		input.RequesterAssignmentID <= 0 ||
		input.OccurrenceDate.IsZero() {
		return nil, ErrInvalidInput
	}
	if !isValidShiftChangeType(input.Type) {
		return nil, ErrShiftChangeInvalidType
	}

	requesterAssignment, err := s.publicationRepo.GetAssignment(ctx, input.RequesterAssignmentID)
	if err != nil {
		if errors.Is(err, repository.ErrAssignmentNotFound) {
			return nil, ErrShiftChangeNotOwner
		}
		return nil, err
	}
	if requesterAssignment.PublicationID != input.PublicationID {
		return nil, ErrShiftChangeNotOwner
	}
	if requesterAssignment.UserID != input.RequesterUserID {
		return nil, ErrShiftChangeNotOwner
	}

	shiftIndex, err := s.loadShiftIndex(ctx, input.PublicationID)
	if err != nil {
		return nil, mapPublicationRepositoryError(err)
	}
	requesterShift := findPublicationShiftForAssignment(shiftIndex, requesterAssignment)
	if requesterShift == nil {
		return nil, ErrTemplateSlotPositionNotFound
	}
	if err := model.IsValidOccurrenceForAssignment(publication, publicationShiftSlot(requesterShift), requesterAssignment, input.OccurrenceDate, now); err != nil {
		return nil, ErrInvalidOccurrenceDate
	}
	expiresAt, err := model.OccurrenceStart(publicationShiftSlot(requesterShift), input.OccurrenceDate)
	if err != nil {
		return nil, err
	}

	var counterpartAssignment *model.Assignment
	var counterpartShift *model.PublicationShift

	switch input.Type {
	case model.ShiftChangeTypeSwap:
		if input.CounterpartUserID == nil ||
			input.CounterpartAssignmentID == nil ||
			input.CounterpartOccurrenceDate == nil ||
			input.CounterpartOccurrenceDate.IsZero() {
			return nil, ErrShiftChangeInvalidType
		}
		if *input.CounterpartUserID == input.RequesterUserID {
			return nil, ErrShiftChangeSelf
		}
		counterpartAssignment, err = s.publicationRepo.GetAssignment(ctx, *input.CounterpartAssignmentID)
		if err != nil {
			if errors.Is(err, repository.ErrAssignmentNotFound) {
				return nil, ErrShiftChangeNotFound
			}
			return nil, err
		}
		if counterpartAssignment.PublicationID != input.PublicationID {
			return nil, ErrShiftChangeNotFound
		}
		if counterpartAssignment.UserID != *input.CounterpartUserID {
			return nil, ErrShiftChangeNotFound
		}
		counterpartShift = findPublicationShiftForAssignment(shiftIndex, counterpartAssignment)
		if counterpartShift == nil {
			return nil, ErrTemplateSlotPositionNotFound
		}
		if err := model.IsValidOccurrenceForAssignment(publication, publicationShiftSlot(counterpartShift), counterpartAssignment, *input.CounterpartOccurrenceDate, now); err != nil {
			return nil, ErrInvalidOccurrenceDate
		}
		if err := s.mutuallyQualified(ctx, input.RequesterUserID, *input.CounterpartUserID, requesterAssignment.PositionID, counterpartAssignment.PositionID); err != nil {
			return nil, err
		}

	case model.ShiftChangeTypeGiveDirect:
		if input.CounterpartUserID == nil {
			return nil, ErrShiftChangeInvalidType
		}
		if input.CounterpartAssignmentID != nil {
			return nil, ErrShiftChangeInvalidType
		}
		if *input.CounterpartUserID == input.RequesterUserID {
			return nil, ErrShiftChangeSelf
		}
		qualified, err := s.publicationRepo.IsUserQualifiedForPosition(ctx, *input.CounterpartUserID, requesterAssignment.PositionID)
		if err != nil {
			return nil, err
		}
		if !qualified {
			return nil, ErrShiftChangeNotQualified
		}

	case model.ShiftChangeTypeGivePool:
		if input.CounterpartUserID != nil || input.CounterpartAssignmentID != nil {
			return nil, ErrShiftChangeInvalidType
		}
	default:
		return nil, ErrShiftChangeInvalidType
	}

	params := repository.CreateShiftChangeRequestParams{
		PublicationID:             input.PublicationID,
		Type:                      input.Type,
		RequesterUserID:           input.RequesterUserID,
		RequesterAssignmentID:     input.RequesterAssignmentID,
		OccurrenceDate:            input.OccurrenceDate,
		CounterpartUserID:         input.CounterpartUserID,
		CounterpartAssignmentID:   input.CounterpartAssignmentID,
		CounterpartOccurrenceDate: input.CounterpartOccurrenceDate,
		ExpiresAt:                 expiresAt,
		CreatedAt:                 now,
		LeaveID:                   input.LeaveID,
	}

	return &preparedShiftChangeCreate{
		params:           params,
		requesterShift:   requesterShift,
		counterpartShift: counterpartShift,
	}, nil
}

func (s *ShiftChangeService) recordShiftChangeCreated(
	ctx context.Context,
	created *model.ShiftChangeRequest,
	requesterShift *model.PublicationShift,
	counterpartShift *model.PublicationShift,
) {
	targetID := created.ID
	audit.Record(ctx, audit.Event{
		Action:     audit.ActionShiftChangeCreate,
		TargetType: audit.TargetTypeShiftChangeRequest,
		TargetID:   &targetID,
		Metadata:   shiftChangeCreateMetadata(created),
	})
}

// GetShiftChangeRequest returns a single request, enforcing that the viewer
// is admin, requester, counterpart, or (for give_pool) simply authenticated.
func (s *ShiftChangeService) GetShiftChangeRequest(
	ctx context.Context,
	requestID, viewerUserID int64,
	viewerIsAdmin bool,
) (*model.ShiftChangeRequest, error) {
	if requestID <= 0 {
		return nil, ErrInvalidInput
	}
	req, err := s.shiftChangeRepo.GetByID(ctx, requestID)
	if err != nil {
		return nil, err
	}
	if !s.canViewRequest(req, viewerUserID, viewerIsAdmin) {
		return nil, ErrShiftChangeNotFound
	}
	return s.expireOnReadIfStale(ctx, req), nil
}

// ListShiftChangeRequests returns requests for a publication, filtered by
// audience. Admin sees all; employees see sent / received / give_pool rows.
func (s *ShiftChangeService) ListShiftChangeRequests(
	ctx context.Context,
	publicationID, viewerUserID int64,
	viewerIsAdmin bool,
) ([]*model.ShiftChangeRequest, error) {
	if publicationID <= 0 {
		return nil, ErrInvalidInput
	}

	audience := repository.ShiftChangeAudience{Admin: viewerIsAdmin, ViewerUserID: viewerUserID}
	rows, err := s.shiftChangeRepo.ListForPublication(ctx, repository.ListForPublicationParams{
		PublicationID: publicationID,
		Audience:      audience,
	})
	if err != nil {
		return nil, err
	}

	result := make([]*model.ShiftChangeRequest, 0, len(rows))
	for _, r := range rows {
		result = append(result, s.expireOnReadIfStale(ctx, r))
	}
	return result, nil
}

// CountPendingForViewer returns the number of pending swap/give_direct
// requests where the viewer is counterpart. Pool requests are not counted
// because they have no specific recipient.
func (s *ShiftChangeService) CountPendingForViewer(
	ctx context.Context,
	viewerUserID int64,
) (int, error) {
	if viewerUserID <= 0 {
		return 0, ErrInvalidInput
	}
	return s.shiftChangeRepo.CountPendingForCounterpart(ctx, viewerUserID, s.clock.Now())
}

// CancelShiftChangeRequest marks a pending request cancelled. Only the
// requester can cancel.
func (s *ShiftChangeService) CancelShiftChangeRequest(
	ctx context.Context,
	requestID, viewerUserID int64,
) error {
	if requestID <= 0 || viewerUserID <= 0 {
		return ErrInvalidInput
	}
	req, err := s.shiftChangeRepo.GetByID(ctx, requestID)
	if err != nil {
		return err
	}
	if req.RequesterUserID != viewerUserID {
		return ErrShiftChangeNotOwner
	}
	now := s.clock.Now()
	effState := model.EffectiveShiftChangeState(req.State, req.ExpiresAt, now)
	if effState == model.ShiftChangeStateExpired {
		_ = s.shiftChangeRepo.MarkExpired(ctx, req.ID, now)
		return ErrShiftChangeExpired
	}
	if effState != model.ShiftChangeStatePending {
		return ErrShiftChangeNotPending
	}

	if err := s.shiftChangeRepo.UpdateState(ctx, repository.UpdateStateParams{
		ID:              req.ID,
		CurrentState:    model.ShiftChangeStatePending,
		NextState:       model.ShiftChangeStateCancelled,
		DecidedByUserID: &viewerUserID,
		Now:             now,
		AfterUpdateTx: func(ctx context.Context, tx *sql.Tx) error {
			return s.enqueueRequestResolvedTx(ctx, tx, req, email.ShiftChangeOutcomeCancelled, viewerUserID)
		},
	}); err != nil {
		return err
	}

	targetID := req.ID
	audit.Record(ctx, audit.Event{
		Action:     audit.ActionShiftChangeCancel,
		TargetType: audit.TargetTypeShiftChangeRequest,
		TargetID:   &targetID,
		Metadata: map[string]any{
			"canceller_id":   viewerUserID,
			"type":           req.Type,
			"publication_id": req.PublicationID,
		},
	})

	return nil
}

// RejectShiftChangeRequest marks a pending swap or give_direct rejected.
// Only the counterpart can reject. Pool requests cannot be rejected.
func (s *ShiftChangeService) RejectShiftChangeRequest(
	ctx context.Context,
	requestID, viewerUserID int64,
) error {
	if requestID <= 0 || viewerUserID <= 0 {
		return ErrInvalidInput
	}
	req, err := s.shiftChangeRepo.GetByID(ctx, requestID)
	if err != nil {
		return err
	}
	if req.Type == model.ShiftChangeTypeGivePool {
		return ErrShiftChangeInvalidType
	}
	if req.CounterpartUserID == nil || *req.CounterpartUserID != viewerUserID {
		return ErrShiftChangeNotOwner
	}

	now := s.clock.Now()
	effState := model.EffectiveShiftChangeState(req.State, req.ExpiresAt, now)
	if effState == model.ShiftChangeStateExpired {
		_ = s.shiftChangeRepo.MarkExpired(ctx, req.ID, now)
		return ErrShiftChangeExpired
	}
	if effState != model.ShiftChangeStatePending {
		return ErrShiftChangeNotPending
	}

	if err := s.shiftChangeRepo.UpdateState(ctx, repository.UpdateStateParams{
		ID:              req.ID,
		CurrentState:    model.ShiftChangeStatePending,
		NextState:       model.ShiftChangeStateRejected,
		DecidedByUserID: &viewerUserID,
		Now:             now,
		AfterUpdateTx: func(ctx context.Context, tx *sql.Tx) error {
			return s.enqueueRequestResolvedTx(ctx, tx, req, email.ShiftChangeOutcomeRejected, viewerUserID)
		},
	}); err != nil {
		return err
	}

	targetID := req.ID
	audit.Record(ctx, audit.Event{
		Action:     audit.ActionShiftChangeReject,
		TargetType: audit.TargetTypeShiftChangeRequest,
		TargetID:   &targetID,
		Metadata: map[string]any{
			"decider_id":     viewerUserID,
			"type":           req.Type,
			"publication_id": req.PublicationID,
		},
	})

	return nil
}

// ApproveShiftChangeRequest is the single entry point for "approve"
// (swap/give_direct) and "claim" (give_pool) actions. It validates
// preconditions, re-checks qualifications and time conflicts, applies the
// change atomically, and emits audit + email notifications.
func (s *ShiftChangeService) ApproveShiftChangeRequest(
	ctx context.Context,
	requestID, viewerUserID int64,
) error {
	if requestID <= 0 || viewerUserID <= 0 {
		return ErrInvalidInput
	}
	req, err := s.shiftChangeRepo.GetByID(ctx, requestID)
	if err != nil {
		return err
	}

	now := s.clock.Now()
	effState := model.EffectiveShiftChangeState(req.State, req.ExpiresAt, now)
	if effState == model.ShiftChangeStateExpired {
		_ = s.shiftChangeRepo.MarkExpired(ctx, req.ID, now)
		return ErrShiftChangeExpired
	}
	if effState != model.ShiftChangeStatePending {
		return ErrShiftChangeNotPending
	}

	publication, err := s.publicationRepo.GetByID(ctx, req.PublicationID)
	if err != nil {
		return mapPublicationRepositoryError(err)
	}
	publicationState := model.ResolvePublicationState(publication, now)
	if req.LeaveID != nil {
		if publicationState != model.PublicationStatePublished && publicationState != model.PublicationStateActive {
			return ErrPublicationNotPublished
		}
	} else if publicationState != model.PublicationStatePublished {
		return ErrPublicationNotPublished
	}

	// Authorisation check per request type.
	switch req.Type {
	case model.ShiftChangeTypeSwap, model.ShiftChangeTypeGiveDirect:
		if req.CounterpartUserID == nil || *req.CounterpartUserID != viewerUserID {
			return ErrShiftChangeNotOwner
		}
	case model.ShiftChangeTypeGivePool:
		if viewerUserID == req.RequesterUserID {
			return ErrShiftChangeSelf
		}
	}

	// Requester's shift and position — needed for qualification + conflict
	// checks.
	requesterAssignment, err := s.publicationRepo.GetAssignment(ctx, req.RequesterAssignmentID)
	if err != nil {
		if errors.Is(err, repository.ErrAssignmentNotFound) {
			_ = s.shiftChangeRepo.MarkInvalidated(ctx, req.ID, now)
			return ErrShiftChangeInvalidated
		}
		return err
	}
	if requesterAssignment.UserID != req.RequesterUserID {
		_ = s.shiftChangeRepo.MarkInvalidated(ctx, req.ID, now)
		return ErrShiftChangeInvalidated
	}

	var receiverUserID int64
	switch req.Type {
	case model.ShiftChangeTypeSwap:
		if req.CounterpartAssignmentID == nil || req.CounterpartOccurrenceDate == nil {
			return ErrShiftChangeInvalidated
		}
		counterpartAssignment, err := s.publicationRepo.GetAssignment(ctx, *req.CounterpartAssignmentID)
		if err != nil {
			if errors.Is(err, repository.ErrAssignmentNotFound) {
				_ = s.shiftChangeRepo.MarkInvalidated(ctx, req.ID, now)
				return ErrShiftChangeInvalidated
			}
			return err
		}
		if req.CounterpartUserID == nil || counterpartAssignment.UserID != *req.CounterpartUserID {
			_ = s.shiftChangeRepo.MarkInvalidated(ctx, req.ID, now)
			return ErrShiftChangeInvalidated
		}
		receiverUserID = *req.CounterpartUserID
		if err := s.mutuallyQualified(ctx, req.RequesterUserID, receiverUserID, requesterAssignment.PositionID, counterpartAssignment.PositionID); err != nil {
			return err
		}
	case model.ShiftChangeTypeGiveDirect:
		receiverUserID = viewerUserID
		qualified, err := s.publicationRepo.IsUserQualifiedForPosition(ctx, receiverUserID, requesterAssignment.PositionID)
		if err != nil {
			return err
		}
		if !qualified {
			return ErrShiftChangeNotQualified
		}
	case model.ShiftChangeTypeGivePool:
		receiverUserID = viewerUserID
		qualified, err := s.publicationRepo.IsUserQualifiedForPosition(ctx, receiverUserID, requesterAssignment.PositionID)
		if err != nil {
			return err
		}
		if !qualified {
			return ErrShiftChangeNotQualified
		}
	}

	switch req.Type {
	case model.ShiftChangeTypeSwap:
		_, err := s.shiftChangeRepo.ApplySwap(ctx, repository.ApplySwapParams{
			RequestID:                 req.ID,
			PublicationID:             req.PublicationID,
			RequesterAssignmentID:     req.RequesterAssignmentID,
			RequesterUserID:           req.RequesterUserID,
			OccurrenceDate:            req.OccurrenceDate,
			CounterpartAssignmentID:   *req.CounterpartAssignmentID,
			CounterpartUserID:         receiverUserID,
			CounterpartOccurrenceDate: *req.CounterpartOccurrenceDate,
			DecidedByUserID:           viewerUserID,
			Now:                       now,
			AfterApplyTx: func(ctx context.Context, tx *sql.Tx) error {
				return s.enqueueRequestResolvedTx(ctx, tx, req, email.ShiftChangeOutcomeApproved, viewerUserID)
			},
		})
		if err != nil {
			return s.mapApplyError(ctx, req, err, now)
		}
	default:
		_, err := s.shiftChangeRepo.ApplyGive(ctx, repository.ApplyGiveParams{
			RequestID:             req.ID,
			PublicationID:         req.PublicationID,
			RequesterAssignmentID: req.RequesterAssignmentID,
			RequesterUserID:       req.RequesterUserID,
			OccurrenceDate:        req.OccurrenceDate,
			ReceiverUserID:        receiverUserID,
			DecidedByUserID:       viewerUserID,
			Now:                   now,
			AfterApplyTx: func(ctx context.Context, tx *sql.Tx) error {
				outcome := email.ShiftChangeOutcomeApproved
				if req.Type == model.ShiftChangeTypeGivePool {
					outcome = email.ShiftChangeOutcomeClaimed
				}
				return s.enqueueRequestResolvedTx(ctx, tx, req, outcome, viewerUserID)
			},
		})
		if err != nil {
			return s.mapApplyError(ctx, req, err, now)
		}
	}

	targetID := req.ID
	audit.Record(ctx, audit.Event{
		Action:     audit.ActionShiftChangeApprove,
		TargetType: audit.TargetTypeShiftChangeRequest,
		TargetID:   &targetID,
		Metadata: map[string]any{
			"decider_id":       viewerUserID,
			"type":             req.Type,
			"publication_id":   req.PublicationID,
			"receiver_user_id": receiverUserID,
		},
	})

	return nil
}

// ListPublicationMembers returns a compact list of users assigned in a
// publication, for the frontend's "pick a counterpart" UX. No email.
type PublicationMember struct {
	UserID int64
	Name   string
}

func (s *ShiftChangeService) ListPublicationMembers(
	ctx context.Context,
	publicationID int64,
) ([]PublicationMember, error) {
	if publicationID <= 0 {
		return nil, ErrInvalidInput
	}
	assignments, err := s.publicationRepo.ListPublicationAssignments(ctx, publicationID)
	if err != nil {
		return nil, err
	}
	seen := make(map[int64]struct{}, len(assignments))
	members := make([]PublicationMember, 0, len(assignments))
	for _, a := range assignments {
		if _, ok := seen[a.UserID]; ok {
			continue
		}
		seen[a.UserID] = struct{}{}
		members = append(members, PublicationMember{UserID: a.UserID, Name: a.Name})
	}
	sort.Slice(members, func(i, j int) bool { return members[i].UserID < members[j].UserID })
	return members, nil
}

// ------------ internal helpers ------------

func isValidShiftChangeType(t model.ShiftChangeType) bool {
	switch t {
	case model.ShiftChangeTypeSwap, model.ShiftChangeTypeGiveDirect, model.ShiftChangeTypeGivePool:
		return true
	default:
		return false
	}
}

func (s *ShiftChangeService) canViewRequest(req *model.ShiftChangeRequest, viewerUserID int64, admin bool) bool {
	if admin {
		return true
	}
	if req.RequesterUserID == viewerUserID {
		return true
	}
	if req.CounterpartUserID != nil && *req.CounterpartUserID == viewerUserID {
		return true
	}
	if req.Type == model.ShiftChangeTypeGivePool {
		return true
	}
	return false
}

func (s *ShiftChangeService) expireOnReadIfStale(
	ctx context.Context,
	req *model.ShiftChangeRequest,
) *model.ShiftChangeRequest {
	now := s.clock.Now()
	effState := model.EffectiveShiftChangeState(req.State, req.ExpiresAt, now)
	if req.State == model.ShiftChangeStatePending && effState == model.ShiftChangeStateExpired {
		// Best-effort persistence on read.
		if err := s.shiftChangeRepo.MarkExpired(ctx, req.ID, now); err != nil {
			s.logger.Warn("shift_change: mark expired on read", "id", req.ID, "error", err)
		}
		cloned := *req
		cloned.State = model.ShiftChangeStateExpired
		cloned.DecidedAt = &now
		return &cloned
	}
	return req
}

func (s *ShiftChangeService) mutuallyQualified(
	ctx context.Context,
	userA, userB, positionA, positionB int64,
) error {
	okBforA, err := s.publicationRepo.IsUserQualifiedForPosition(ctx, userB, positionA)
	if err != nil {
		return err
	}
	if !okBforA {
		return ErrShiftChangeNotQualified
	}
	okAforB, err := s.publicationRepo.IsUserQualifiedForPosition(ctx, userA, positionB)
	if err != nil {
		return err
	}
	if !okAforB {
		return ErrShiftChangeNotQualified
	}
	return nil
}

func (s *ShiftChangeService) loadShiftIndex(
	ctx context.Context,
	publicationID int64,
) (map[slotPositionKey]*model.PublicationShift, error) {
	shifts, err := s.publicationRepo.ListPublicationShifts(ctx, publicationID)
	if err != nil {
		return nil, err
	}
	return buildPublicationShiftIndex(shifts), nil
}

// mapApplyError coerces repository-layer errors from ApplySwap/ApplyGive
// into domain errors the handler layer can translate to HTTP statuses.
func (s *ShiftChangeService) mapApplyError(
	ctx context.Context,
	req *model.ShiftChangeRequest,
	err error,
	now time.Time,
) error {
	switch {
	case errors.Is(err, repository.ErrShiftChangeAssignmentMiss):
		_ = s.shiftChangeRepo.MarkInvalidated(ctx, req.ID, now)
		return ErrShiftChangeInvalidated
	case errors.Is(err, repository.ErrShiftChangeNotPending):
		return ErrShiftChangeNotPending
	case errors.Is(err, repository.ErrShiftChangeNotFound):
		return ErrShiftChangeNotFound
	case errors.Is(err, repository.ErrUserDisabled):
		return ErrUserDisabled
	case errors.Is(err, repository.ErrSchedulingRetryable):
		return ErrSchedulingRetryable
	default:
		return err
	}
}

// ------------ email ------------

func shouldNotifyShiftChangeCreate(requestType model.ShiftChangeType) bool {
	return requestType == model.ShiftChangeTypeSwap || requestType == model.ShiftChangeTypeGiveDirect
}

func (s *ShiftChangeService) enqueueRequestCreatedTx(
	ctx context.Context,
	tx *sql.Tx,
	req *model.ShiftChangeRequest,
	requesterShift, counterpartShift *model.PublicationShift,
) error {
	if s.outboxRepo == nil || req.CounterpartUserID == nil {
		return nil
	}
	counterpart, err := s.publicationRepo.GetUserByID(ctx, *req.CounterpartUserID)
	if err != nil {
		return err
	}
	requester, err := s.publicationRepo.GetUserByID(ctx, req.RequesterUserID)
	if err != nil {
		return err
	}

	data := email.ShiftChangeRequestReceivedData{
		To:             counterpart.Email,
		RecipientName:  counterpart.Name,
		RequesterName:  requester.Name,
		Type:           email.ShiftChangeType(req.Type),
		RequesterShift: toShiftRefWithOccurrence(requesterShift, &req.OccurrenceDate),
		BaseURL:        s.appBaseURL,
		Language:       resolveRequestEmailLanguage(ctx, counterpart),
	}
	if counterpartShift != nil {
		ref := toShiftRefWithOccurrence(counterpartShift, req.CounterpartOccurrenceDate)
		data.CounterpartShift = &ref
	}
	msg := email.BuildShiftChangeRequestReceivedMessage(data)
	return s.outboxRepo.EnqueueTx(ctx, tx, msg, repository.WithOutboxUserID(counterpart.ID))
}

func (s *ShiftChangeService) enqueueRequestResolvedTx(
	ctx context.Context,
	tx *sql.Tx,
	req *model.ShiftChangeRequest,
	outcome email.ShiftChangeOutcome,
	responderID int64,
) error {
	if s.outboxRepo == nil {
		return nil
	}
	requester, err := s.publicationRepo.GetUserByID(ctx, req.RequesterUserID)
	if err != nil {
		return err
	}
	shiftIndex, err := s.loadShiftIndex(ctx, req.PublicationID)
	if err != nil {
		return err
	}
	responder, err := s.publicationRepo.GetUserByID(ctx, responderID)
	if err != nil {
		return err
	}

	requesterAssignment, err := s.publicationRepo.GetAssignment(ctx, req.RequesterAssignmentID)
	if err != nil {
		return err
	}
	requesterShift := findPublicationShiftForAssignment(shiftIndex, requesterAssignment)
	if requesterShift == nil {
		return ErrTemplateSlotPositionNotFound
	}

	data := email.ShiftChangeResolvedData{
		To:             requester.Email,
		RecipientName:  requester.Name,
		Outcome:        outcome,
		Type:           email.ShiftChangeType(req.Type),
		ResponderName:  responder.Name,
		RequesterShift: toShiftRefWithOccurrence(requesterShift, &req.OccurrenceDate),
		BaseURL:        s.appBaseURL,
		Language:       resolveRequestEmailLanguage(ctx, requester),
	}
	if req.CounterpartAssignmentID != nil {
		counterpartAssignment, err := s.publicationRepo.GetAssignment(ctx, *req.CounterpartAssignmentID)
		if err == nil {
			if counterpartShift := findPublicationShiftForAssignment(shiftIndex, counterpartAssignment); counterpartShift != nil {
				ref := toShiftRefWithOccurrence(counterpartShift, req.CounterpartOccurrenceDate)
				data.CounterpartShift = &ref
			}
		}
	}
	msg := email.BuildShiftChangeResolvedMessage(data)
	return s.outboxRepo.EnqueueTx(ctx, tx, msg, repository.WithOutboxUserID(requester.ID))
}

func weekdayLabel(weekday int) string {
	switch weekday {
	case 1:
		return "Mon"
	case 2:
		return "Tue"
	case 3:
		return "Wed"
	case 4:
		return "Thu"
	case 5:
		return "Fri"
	case 6:
		return "Sat"
	case 7:
		return "Sun"
	default:
		return ""
	}
}

func shiftChangeCreateMetadata(req *model.ShiftChangeRequest) map[string]any {
	metadata := map[string]any{
		"type":                    req.Type,
		"publication_id":          req.PublicationID,
		"requester_user_id":       req.RequesterUserID,
		"requester_assignment_id": req.RequesterAssignmentID,
		"occurrence_date":         req.OccurrenceDate.Format("2006-01-02"),
	}
	if req.CounterpartUserID != nil {
		metadata["counterpart_user_id"] = *req.CounterpartUserID
	}
	if req.CounterpartAssignmentID != nil {
		metadata["counterpart_assignment_id"] = *req.CounterpartAssignmentID
	}
	if req.CounterpartOccurrenceDate != nil {
		metadata["counterpart_occurrence_date"] = req.CounterpartOccurrenceDate.Format("2006-01-02")
	}
	if req.LeaveID != nil {
		metadata["leave_id"] = *req.LeaveID
	}
	return metadata
}
