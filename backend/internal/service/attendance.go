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

const maxOvertimeNoteLength = 500

var (
	ErrAttendanceRecordNotFound  = model.ErrAttendanceRecordNotFound
	ErrAttendanceNotLeader       = model.ErrAttendanceNotLeader
	ErrAttendanceWindowClosed    = model.ErrAttendanceWindowClosed
	ErrAttendanceAlreadyRecorded = model.ErrAttendanceAlreadyRecorded
	ErrAttendanceRosterStale     = model.ErrAttendanceRosterStale
)

type attendanceRepository interface {
	ListLeaderCandidateShifts(ctx context.Context, publicationID, userID int64, fromDate, toDate time.Time) ([]model.AttendanceShiftRef, error)
	ListPublicationShiftRefsForDate(ctx context.Context, publicationID int64, occurrenceDate time.Time) ([]model.AttendanceShiftRef, error)
	ListShiftRoster(ctx context.Context, publicationID, slotID int64, weekday int, occurrenceDate time.Time) ([]*model.AttendanceRosterRow, error)
	ListOrphanArrivalRecords(ctx context.Context, publicationID, slotID int64, weekday int, occurrenceDate time.Time) ([]*model.AttendanceRecord, error)
	ListOvertimeRecords(ctx context.Context, publicationID, slotID int64, weekday int, occurrenceDate time.Time) ([]*model.AttendanceOvertimeRecord, error)
	InsertLeaderArrival(ctx context.Context, params repository.UpsertAttendanceArrivalParams) (*model.AttendanceRecord, error)
	UpsertAdminArrival(ctx context.Context, params repository.UpsertAttendanceArrivalParams) (*model.AttendanceRecord, error)
	DeleteArrival(ctx context.Context, publicationID, recordID int64) (*model.AttendanceRecord, error)
	CreateOvertime(ctx context.Context, params repository.CreateOvertimeRecordParams) (*model.AttendanceOvertimeRecord, error)
	GetOvertime(ctx context.Context, publicationID, recordID int64) (*model.AttendanceOvertimeRecord, error)
	UpdateOvertime(ctx context.Context, params repository.UpdateOvertimeRecordParams) (*model.AttendanceOvertimeRecord, error)
	DeleteOvertime(ctx context.Context, publicationID, recordID int64) (*model.AttendanceOvertimeRecord, error)
}

type attendancePublicationRepository interface {
	GetByID(ctx context.Context, id int64) (*model.Publication, error)
	GetCurrent(ctx context.Context) (*model.Publication, error)
	GetSlot(ctx context.Context, templateID, slotID int64) (*model.TemplateSlot, error)
	ListSlotPositions(ctx context.Context, slotID int64) ([]*model.TemplateSlotPosition, error)
	GetUserByID(ctx context.Context, id int64) (*model.User, error)
	UpdatePublicationFields(ctx context.Context, params repository.UpdatePublicationFieldsParams) (*model.Publication, error)
}

type AttendanceService struct {
	attendanceRepo  attendanceRepository
	publicationRepo attendancePublicationRepository
	clock           Clock
}

type LeaderAttendanceResult struct {
	Publication *model.Publication
	Shifts      []*AttendanceShiftDetail
}

type AdminAttendanceDayResult struct {
	Publication *model.Publication
	Date        time.Time
	Shifts      []*AttendanceShiftSummary
}

type AttendanceShiftSummary struct {
	SlotID         int64
	Weekday        int
	OccurrenceDate time.Time
	ScheduledStart time.Time
	ScheduledEnd   time.Time
	RosterCount    int
	PendingCount   int
	PresentCount   int
	LateCount      int
	AbsentCount    int
	OrphanCount    int
	OvertimeCount  int
}

type AttendanceShiftDetail struct {
	Publication        *model.Publication
	SlotID             int64
	Weekday            int
	StartTime          string
	EndTime            string
	OccurrenceDate     time.Time
	ScheduledStart     time.Time
	ScheduledEnd       time.Time
	ArrivalWindowOpen  bool
	OvertimeWindowOpen bool
	Roster             []*AttendanceRosterEntry
	OrphanArrivals     []*AttendanceArrivalEntry
	OvertimeRecords    []*model.AttendanceOvertimeRecord
}

type AttendanceRosterEntry struct {
	AssignmentID          int64
	PositionID            int64
	PositionName          string
	AttendanceResponsible bool
	UserID                int64
	UserName              string
	UserEmail             string
	Status                model.AttendanceStatus
	Record                *model.AttendanceRecord
}

type AttendanceArrivalEntry struct {
	Record *model.AttendanceRecord
	Status model.AttendanceStatus
}

type RecordLeaderArrivalInput struct {
	ActorUserID    int64
	PublicationID  int64
	SlotID         int64
	AssignmentID   int64
	OccurrenceDate time.Time
	UserID         int64
	ArrivedAt      *time.Time
}

type RecordOvertimeInput struct {
	ActorUserID    int64
	PublicationID  int64
	SlotID         int64
	OccurrenceDate time.Time
	UserID         int64
	Hours          float64
	Note           string
}

type ListAdminAttendanceInput struct {
	PublicationID  int64
	OccurrenceDate time.Time
}

type GetAdminShiftAttendanceInput struct {
	PublicationID  int64
	SlotID         int64
	OccurrenceDate time.Time
}

type AdminUpsertArrivalInput struct {
	ActorUserID    int64
	PublicationID  int64
	SlotID         int64
	AssignmentID   int64
	OccurrenceDate time.Time
	UserID         int64
	ArrivedAt      time.Time
}

type AdminClearArrivalInput struct {
	ActorUserID   int64
	PublicationID int64
	RecordID      int64
}

type AdminUpdateOvertimeInput struct {
	ActorUserID   int64
	PublicationID int64
	RecordID      int64
	Hours         float64
	Note          string
}

type UpdateAttendanceSettingsInput struct {
	ActorUserID              int64
	PublicationID            int64
	OvertimeEntryWindowHours float64
}

type shiftContext struct {
	publication    *model.Publication
	slot           *model.TemplateSlot
	weekday        int
	occurrenceDate time.Time
	scheduledStart time.Time
	scheduledEnd   time.Time
}

func NewAttendanceService(
	attendanceRepo attendanceRepository,
	publicationRepo attendancePublicationRepository,
	clock Clock,
) *AttendanceService {
	if clock == nil {
		clock = realClock{}
	}

	return &AttendanceService{
		attendanceRepo:  attendanceRepo,
		publicationRepo: publicationRepo,
		clock:           clock,
	}
}

func (s *AttendanceService) ListCurrentAttendance(
	ctx context.Context,
	userID int64,
) (*LeaderAttendanceResult, error) {
	if userID <= 0 {
		return nil, ErrInvalidInput
	}

	publication, err := s.publicationRepo.GetCurrent(ctx)
	if err != nil {
		return nil, mapAttendanceRepositoryError(err)
	}
	if publication == nil {
		return &LeaderAttendanceResult{Shifts: make([]*AttendanceShiftDetail, 0)}, nil
	}

	now := s.clock.Now()
	effective := publicationWithEffectiveState(publication, now)
	if effective.State != model.PublicationStateActive {
		return &LeaderAttendanceResult{
			Publication: effective,
			Shifts:      make([]*AttendanceShiftDetail, 0),
		}, nil
	}

	windowHours := effective.OvertimeEntryWindowHours
	if windowHours < 0 {
		windowHours = 0
	}
	fromDate := now.Add(-time.Duration(windowHours+24) * time.Hour)
	refs, err := s.attendanceRepo.ListLeaderCandidateShifts(ctx, effective.ID, userID, fromDate, now)
	if err != nil {
		return nil, mapAttendanceRepositoryError(err)
	}

	shifts := make([]*AttendanceShiftDetail, 0, len(refs))
	for _, ref := range refs {
		detail, err := s.buildShiftDetail(ctx, effective, ref.SlotID, ref.OccurrenceDate, now)
		if err != nil {
			if errors.Is(err, ErrInvalidOccurrenceDate) ||
				errors.Is(err, ErrTemplateSlotNotFound) ||
				errors.Is(err, ErrAttendanceResponsibleRequired) {
				continue
			}
			return nil, err
		}
		if !callerIsResponsible(detail.Roster, userID) {
			continue
		}
		if !detail.ArrivalWindowOpen && !detail.OvertimeWindowOpen {
			continue
		}
		shifts = append(shifts, detail)
	}

	return &LeaderAttendanceResult{
		Publication: effective,
		Shifts:      shifts,
	}, nil
}

func (s *AttendanceService) RecordLeaderArrival(
	ctx context.Context,
	input RecordLeaderArrivalInput,
) (*AttendanceShiftDetail, error) {
	if input.ActorUserID <= 0 ||
		input.PublicationID <= 0 ||
		input.SlotID <= 0 ||
		input.AssignmentID <= 0 ||
		input.UserID <= 0 ||
		input.OccurrenceDate.IsZero() {
		return nil, ErrInvalidInput
	}

	now := s.clock.Now()
	detail, err := s.getLeaderShiftDetail(ctx, input.PublicationID, input.SlotID, input.OccurrenceDate, input.ActorUserID, now)
	if err != nil {
		return nil, err
	}
	if !detail.ArrivalWindowOpen {
		return nil, ErrAttendanceWindowClosed
	}

	target := findRosterEntry(detail.Roster, input.AssignmentID, input.UserID)
	if target == nil {
		return nil, ErrAttendanceRosterStale
	}
	if target.Record != nil {
		return nil, ErrAttendanceAlreadyRecorded
	}

	arrivedAt := detail.ScheduledStart
	if input.ArrivedAt != nil {
		arrivedAt = input.ArrivedAt.UTC()
	}
	if arrivedAt.Before(detail.ScheduledStart) || arrivedAt.After(now) {
		return nil, ErrInvalidInput
	}

	record, err := s.attendanceRepo.InsertLeaderArrival(ctx, repository.UpsertAttendanceArrivalParams{
		PublicationID:    input.PublicationID,
		AssignmentID:     input.AssignmentID,
		OccurrenceDate:   input.OccurrenceDate,
		UserID:           input.UserID,
		ArrivedAt:        arrivedAt,
		RecordedByUserID: input.ActorUserID,
		RecordedAt:       now,
	})
	if err != nil {
		return nil, mapAttendanceRepositoryError(err)
	}

	targetID := record.ID
	audit.Record(ctx, audit.Event{
		Action:     audit.ActionAttendanceArrivalRecord,
		TargetType: audit.TargetTypeAttendanceRecord,
		TargetID:   &targetID,
		Metadata:   attendanceArrivalMetadata(record, nil),
	})

	return s.buildShiftDetail(ctx, detail.Publication, input.SlotID, input.OccurrenceDate, now)
}

func (s *AttendanceService) RecordLeaderOvertime(
	ctx context.Context,
	input RecordOvertimeInput,
) (*model.AttendanceOvertimeRecord, error) {
	if input.ActorUserID <= 0 ||
		input.PublicationID <= 0 ||
		input.SlotID <= 0 ||
		input.UserID <= 0 ||
		input.OccurrenceDate.IsZero() {
		return nil, ErrInvalidInput
	}

	note, err := normalizeOvertimeInput(input.Hours, input.Note)
	if err != nil {
		return nil, err
	}

	now := s.clock.Now()
	detail, err := s.getLeaderShiftDetail(ctx, input.PublicationID, input.SlotID, input.OccurrenceDate, input.ActorUserID, now)
	if err != nil {
		return nil, err
	}
	if !detail.OvertimeWindowOpen {
		return nil, ErrAttendanceWindowClosed
	}
	if err := s.ensureActiveUser(ctx, input.UserID); err != nil {
		return nil, err
	}

	record, err := s.attendanceRepo.CreateOvertime(ctx, repository.CreateOvertimeRecordParams{
		PublicationID:    input.PublicationID,
		SlotID:           input.SlotID,
		Weekday:          detail.Weekday,
		OccurrenceDate:   input.OccurrenceDate,
		UserID:           input.UserID,
		Hours:            input.Hours,
		Note:             note,
		RecordedByUserID: input.ActorUserID,
		RecordedAt:       now,
	})
	if err != nil {
		return nil, mapAttendanceRepositoryError(err)
	}

	targetID := record.ID
	audit.Record(ctx, audit.Event{
		Action:     audit.ActionAttendanceOvertimeRecord,
		TargetType: audit.TargetTypeAttendanceOvertime,
		TargetID:   &targetID,
		Metadata:   overtimeMetadata(record, nil),
	})

	return record, nil
}

func (s *AttendanceService) ListAdminAttendance(
	ctx context.Context,
	input ListAdminAttendanceInput,
) (*AdminAttendanceDayResult, error) {
	if input.PublicationID <= 0 || input.OccurrenceDate.IsZero() {
		return nil, ErrInvalidInput
	}

	publication, err := s.publicationRepo.GetByID(ctx, input.PublicationID)
	if err != nil {
		return nil, mapAttendanceRepositoryError(err)
	}

	refs, err := s.attendanceRepo.ListPublicationShiftRefsForDate(ctx, input.PublicationID, input.OccurrenceDate)
	if err != nil {
		return nil, mapAttendanceRepositoryError(err)
	}

	now := s.clock.Now()
	summaries := make([]*AttendanceShiftSummary, 0, len(refs))
	for _, ref := range refs {
		detail, err := s.buildShiftDetail(ctx, publication, ref.SlotID, input.OccurrenceDate, now)
		if err != nil {
			if errors.Is(err, ErrInvalidOccurrenceDate) {
				continue
			}
			return nil, err
		}
		summaries = append(summaries, summarizeAttendanceShift(detail))
	}

	return &AdminAttendanceDayResult{
		Publication: publicationWithEffectiveState(publication, now),
		Date:        model.NormalizeOccurrenceDate(input.OccurrenceDate),
		Shifts:      summaries,
	}, nil
}

func (s *AttendanceService) GetAdminShiftAttendance(
	ctx context.Context,
	input GetAdminShiftAttendanceInput,
) (*AttendanceShiftDetail, error) {
	if input.PublicationID <= 0 || input.SlotID <= 0 || input.OccurrenceDate.IsZero() {
		return nil, ErrInvalidInput
	}

	publication, err := s.publicationRepo.GetByID(ctx, input.PublicationID)
	if err != nil {
		return nil, mapAttendanceRepositoryError(err)
	}

	return s.buildShiftDetail(ctx, publicationWithEffectiveState(publication, s.clock.Now()), input.SlotID, input.OccurrenceDate, s.clock.Now())
}

func (s *AttendanceService) AdminUpsertArrival(
	ctx context.Context,
	input AdminUpsertArrivalInput,
) (*AttendanceShiftDetail, error) {
	if input.ActorUserID <= 0 ||
		input.PublicationID <= 0 ||
		input.SlotID <= 0 ||
		input.AssignmentID <= 0 ||
		input.UserID <= 0 ||
		input.OccurrenceDate.IsZero() ||
		input.ArrivedAt.IsZero() {
		return nil, ErrInvalidInput
	}

	now := s.clock.Now()
	publication, err := s.publicationRepo.GetByID(ctx, input.PublicationID)
	if err != nil {
		return nil, mapAttendanceRepositoryError(err)
	}
	detail, err := s.buildShiftDetail(ctx, publicationWithEffectiveState(publication, now), input.SlotID, input.OccurrenceDate, now)
	if err != nil {
		return nil, err
	}
	if findRosterEntry(detail.Roster, input.AssignmentID, input.UserID) == nil {
		return nil, ErrAttendanceRosterStale
	}
	arrivedAt := input.ArrivedAt.UTC()

	var previous *model.AttendanceRecord
	if target := findRosterEntry(detail.Roster, input.AssignmentID, input.UserID); target != nil {
		previous = target.Record
	}
	record, err := s.attendanceRepo.UpsertAdminArrival(ctx, repository.UpsertAttendanceArrivalParams{
		PublicationID:    input.PublicationID,
		AssignmentID:     input.AssignmentID,
		OccurrenceDate:   input.OccurrenceDate,
		UserID:           input.UserID,
		ArrivedAt:        arrivedAt,
		RecordedByUserID: input.ActorUserID,
		RecordedAt:       now,
	})
	if err != nil {
		return nil, mapAttendanceRepositoryError(err)
	}

	targetID := record.ID
	audit.Record(ctx, audit.Event{
		Action:     audit.ActionAttendanceArrivalAdminAdjust,
		TargetType: audit.TargetTypeAttendanceRecord,
		TargetID:   &targetID,
		Metadata:   attendanceArrivalMetadata(record, previous),
	})

	return s.buildShiftDetail(ctx, detail.Publication, input.SlotID, input.OccurrenceDate, now)
}

func (s *AttendanceService) AdminClearArrival(ctx context.Context, input AdminClearArrivalInput) error {
	if input.ActorUserID <= 0 || input.PublicationID <= 0 || input.RecordID <= 0 {
		return ErrInvalidInput
	}

	record, err := s.attendanceRepo.DeleteArrival(ctx, input.PublicationID, input.RecordID)
	if err != nil {
		return mapAttendanceRepositoryError(err)
	}

	targetID := record.ID
	audit.Record(ctx, audit.Event{
		Action:     audit.ActionAttendanceArrivalAdminClear,
		TargetType: audit.TargetTypeAttendanceRecord,
		TargetID:   &targetID,
		Metadata:   attendanceArrivalMetadata(nil, record),
	})
	return nil
}

func (s *AttendanceService) AdminCreateOvertime(
	ctx context.Context,
	input RecordOvertimeInput,
) (*model.AttendanceOvertimeRecord, error) {
	if input.ActorUserID <= 0 ||
		input.PublicationID <= 0 ||
		input.SlotID <= 0 ||
		input.UserID <= 0 ||
		input.OccurrenceDate.IsZero() {
		return nil, ErrInvalidInput
	}

	note, err := normalizeOvertimeInput(input.Hours, input.Note)
	if err != nil {
		return nil, err
	}
	if err := s.ensureActiveUser(ctx, input.UserID); err != nil {
		return nil, err
	}

	now := s.clock.Now()
	publication, err := s.publicationRepo.GetByID(ctx, input.PublicationID)
	if err != nil {
		return nil, mapAttendanceRepositoryError(err)
	}
	shift, err := s.loadShiftContext(ctx, publicationWithEffectiveState(publication, now), input.SlotID, input.OccurrenceDate)
	if err != nil {
		return nil, err
	}

	record, err := s.attendanceRepo.CreateOvertime(ctx, repository.CreateOvertimeRecordParams{
		PublicationID:    input.PublicationID,
		SlotID:           input.SlotID,
		Weekday:          shift.weekday,
		OccurrenceDate:   input.OccurrenceDate,
		UserID:           input.UserID,
		Hours:            input.Hours,
		Note:             note,
		RecordedByUserID: input.ActorUserID,
		RecordedAt:       now,
	})
	if err != nil {
		return nil, mapAttendanceRepositoryError(err)
	}

	targetID := record.ID
	audit.Record(ctx, audit.Event{
		Action:     audit.ActionAttendanceOvertimeAdminCreate,
		TargetType: audit.TargetTypeAttendanceOvertime,
		TargetID:   &targetID,
		Metadata:   overtimeMetadata(record, nil),
	})

	return record, nil
}

func (s *AttendanceService) AdminUpdateOvertime(
	ctx context.Context,
	input AdminUpdateOvertimeInput,
) (*model.AttendanceOvertimeRecord, error) {
	if input.ActorUserID <= 0 || input.PublicationID <= 0 || input.RecordID <= 0 {
		return nil, ErrInvalidInput
	}
	note, err := normalizeOvertimeInput(input.Hours, input.Note)
	if err != nil {
		return nil, err
	}

	now := s.clock.Now()
	previous, err := s.attendanceRepo.GetOvertime(ctx, input.PublicationID, input.RecordID)
	if err != nil {
		return nil, mapAttendanceRepositoryError(err)
	}
	record, err := s.attendanceRepo.UpdateOvertime(ctx, repository.UpdateOvertimeRecordParams{
		PublicationID:   input.PublicationID,
		RecordID:        input.RecordID,
		Hours:           input.Hours,
		Note:            note,
		UpdatedByUserID: input.ActorUserID,
		UpdatedAt:       now,
	})
	if err != nil {
		return nil, mapAttendanceRepositoryError(err)
	}

	targetID := record.ID
	audit.Record(ctx, audit.Event{
		Action:     audit.ActionAttendanceOvertimeAdminAdjust,
		TargetType: audit.TargetTypeAttendanceOvertime,
		TargetID:   &targetID,
		Metadata:   overtimeMetadata(record, previous),
	})

	return record, nil
}

func (s *AttendanceService) AdminDeleteOvertime(
	ctx context.Context,
	input AdminClearArrivalInput,
) error {
	if input.ActorUserID <= 0 || input.PublicationID <= 0 || input.RecordID <= 0 {
		return ErrInvalidInput
	}

	record, err := s.attendanceRepo.DeleteOvertime(ctx, input.PublicationID, input.RecordID)
	if err != nil {
		return mapAttendanceRepositoryError(err)
	}

	targetID := record.ID
	audit.Record(ctx, audit.Event{
		Action:     audit.ActionAttendanceOvertimeAdminDelete,
		TargetType: audit.TargetTypeAttendanceOvertime,
		TargetID:   &targetID,
		Metadata:   overtimeMetadata(nil, record),
	})
	return nil
}

func (s *AttendanceService) UpdateAttendanceSettings(
	ctx context.Context,
	input UpdateAttendanceSettingsInput,
) (*model.Publication, error) {
	if input.ActorUserID <= 0 || input.PublicationID <= 0 {
		return nil, ErrInvalidInput
	}
	if !isValidOvertimeEntryWindowHours(input.OvertimeEntryWindowHours) {
		return nil, ErrInvalidInput
	}

	current, err := s.publicationRepo.GetByID(ctx, input.PublicationID)
	if err != nil {
		return nil, mapAttendanceRepositoryError(err)
	}
	now := s.clock.Now()
	updated, err := s.publicationRepo.UpdatePublicationFields(ctx, repository.UpdatePublicationFieldsParams{
		ID:                       input.PublicationID,
		OvertimeEntryWindowHours: &input.OvertimeEntryWindowHours,
		UpdatedAt:                now,
	})
	if err != nil {
		return nil, mapAttendanceRepositoryError(err)
	}

	targetID := updated.ID
	audit.Record(ctx, audit.Event{
		Action:     audit.ActionAttendanceSettingsUpdate,
		TargetType: audit.TargetTypePublication,
		TargetID:   &targetID,
		Metadata: map[string]any{
			"publication_id":              updated.ID,
			"overtime_entry_window_hours": map[string]any{"from": current.OvertimeEntryWindowHours, "to": updated.OvertimeEntryWindowHours},
		},
	})

	return publicationWithEffectiveState(updated, now), nil
}

func (s *AttendanceService) getLeaderShiftDetail(
	ctx context.Context,
	publicationID, slotID int64,
	occurrenceDate time.Time,
	actorUserID int64,
	now time.Time,
) (*AttendanceShiftDetail, error) {
	publication, err := s.publicationRepo.GetByID(ctx, publicationID)
	if err != nil {
		return nil, mapAttendanceRepositoryError(err)
	}
	effective := publicationWithEffectiveState(publication, now)
	if effective.State != model.PublicationStateActive {
		return nil, ErrPublicationNotActive
	}

	detail, err := s.buildShiftDetail(ctx, effective, slotID, occurrenceDate, now)
	if err != nil {
		return nil, err
	}
	if !callerIsResponsible(detail.Roster, actorUserID) {
		return nil, ErrAttendanceNotLeader
	}
	return detail, nil
}

func (s *AttendanceService) buildShiftDetail(
	ctx context.Context,
	publication *model.Publication,
	slotID int64,
	occurrenceDate time.Time,
	now time.Time,
) (*AttendanceShiftDetail, error) {
	shift, err := s.loadShiftContext(ctx, publication, slotID, occurrenceDate)
	if err != nil {
		return nil, err
	}

	rosterRows, err := s.attendanceRepo.ListShiftRoster(
		ctx,
		publication.ID,
		slotID,
		shift.weekday,
		shift.occurrenceDate,
	)
	if err != nil {
		return nil, mapAttendanceRepositoryError(err)
	}
	if err := s.ensureResponsiblePosition(ctx, slotID); err != nil {
		return nil, err
	}

	roster := make([]*AttendanceRosterEntry, 0, len(rosterRows))
	for _, row := range rosterRows {
		roster = append(roster, newAttendanceRosterEntry(row, shift.scheduledStart, shift.scheduledEnd, now))
	}

	orphanRecords, err := s.attendanceRepo.ListOrphanArrivalRecords(
		ctx,
		publication.ID,
		slotID,
		shift.weekday,
		shift.occurrenceDate,
	)
	if err != nil {
		return nil, mapAttendanceRepositoryError(err)
	}
	orphanArrivals := make([]*AttendanceArrivalEntry, 0, len(orphanRecords))
	for _, record := range orphanRecords {
		arrivedAt := record.ArrivedAt
		orphanArrivals = append(orphanArrivals, &AttendanceArrivalEntry{
			Record: record,
			Status: model.DeriveAttendanceStatus(&arrivedAt, shift.scheduledStart, shift.scheduledEnd, now),
		})
	}

	overtimeRecords, err := s.attendanceRepo.ListOvertimeRecords(
		ctx,
		publication.ID,
		slotID,
		shift.weekday,
		shift.occurrenceDate,
	)
	if err != nil {
		return nil, mapAttendanceRepositoryError(err)
	}

	return &AttendanceShiftDetail{
		Publication:        publication,
		SlotID:             slotID,
		Weekday:            shift.weekday,
		StartTime:          shift.slot.StartTime,
		EndTime:            shift.slot.EndTime,
		OccurrenceDate:     shift.occurrenceDate,
		ScheduledStart:     shift.scheduledStart,
		ScheduledEnd:       shift.scheduledEnd,
		ArrivalWindowOpen:  isArrivalWindowOpen(now, shift.scheduledStart, shift.scheduledEnd),
		OvertimeWindowOpen: isOvertimeWindowOpen(now, shift.scheduledStart, shift.scheduledEnd, publication.OvertimeEntryWindowHours),
		Roster:             roster,
		OrphanArrivals:     orphanArrivals,
		OvertimeRecords:    overtimeRecords,
	}, nil
}

func (s *AttendanceService) loadShiftContext(
	ctx context.Context,
	publication *model.Publication,
	slotID int64,
	occurrenceDate time.Time,
) (*shiftContext, error) {
	if publication == nil || slotID <= 0 || occurrenceDate.IsZero() {
		return nil, ErrInvalidInput
	}

	slot, err := s.publicationRepo.GetSlot(ctx, publication.TemplateID, slotID)
	if err != nil {
		return nil, mapAttendanceRepositoryError(err)
	}

	date := model.NormalizeOccurrenceDate(occurrenceDate)
	weekday := model.SlotWeekdayValue(date.Weekday())
	if !slotIncludesWeekday(slot, weekday) {
		return nil, ErrInvalidOccurrenceDate
	}

	start, err := model.OccurrenceStart(slot, date)
	if err != nil {
		return nil, ErrInvalidOccurrenceDate
	}
	end, err := model.OccurrenceEnd(slot, date)
	if err != nil {
		return nil, ErrInvalidOccurrenceDate
	}
	if start.Before(publication.PlannedActiveFrom) || !start.Before(publication.PlannedActiveUntil) {
		return nil, ErrInvalidOccurrenceDate
	}

	return &shiftContext{
		publication:    publication,
		slot:           slot,
		weekday:        weekday,
		occurrenceDate: date,
		scheduledStart: start,
		scheduledEnd:   end,
	}, nil
}

func (s *AttendanceService) ensureResponsiblePosition(ctx context.Context, slotID int64) error {
	positions, err := s.publicationRepo.ListSlotPositions(ctx, slotID)
	if err != nil {
		return mapAttendanceRepositoryError(err)
	}
	count := 0
	for _, position := range positions {
		if position.AttendanceResponsible && position.RequiredHeadcount == 1 {
			count++
		}
	}
	if count != 1 {
		return ErrAttendanceResponsibleRequired
	}
	return nil
}

func (s *AttendanceService) ensureActiveUser(ctx context.Context, userID int64) error {
	user, err := s.publicationRepo.GetUserByID(ctx, userID)
	if err != nil {
		return mapAttendanceRepositoryError(err)
	}
	if user.Status != model.UserStatusActive {
		return ErrUserNotFound
	}
	return nil
}

func newAttendanceRosterEntry(
	row *model.AttendanceRosterRow,
	scheduledStart time.Time,
	scheduledEnd time.Time,
	now time.Time,
) *AttendanceRosterEntry {
	var arrivedAt *time.Time
	if row.Record != nil {
		value := row.Record.ArrivedAt
		arrivedAt = &value
	}
	return &AttendanceRosterEntry{
		AssignmentID:          row.AssignmentID,
		PositionID:            row.PositionID,
		PositionName:          row.PositionName,
		AttendanceResponsible: row.AttendanceResponsible,
		UserID:                row.UserID,
		UserName:              row.UserName,
		UserEmail:             row.UserEmail,
		Status:                model.DeriveAttendanceStatus(arrivedAt, scheduledStart, scheduledEnd, now),
		Record:                row.Record,
	}
}

func summarizeAttendanceShift(detail *AttendanceShiftDetail) *AttendanceShiftSummary {
	summary := &AttendanceShiftSummary{
		SlotID:         detail.SlotID,
		Weekday:        detail.Weekday,
		OccurrenceDate: detail.OccurrenceDate,
		ScheduledStart: detail.ScheduledStart,
		ScheduledEnd:   detail.ScheduledEnd,
		RosterCount:    len(detail.Roster),
		OrphanCount:    len(detail.OrphanArrivals),
		OvertimeCount:  len(detail.OvertimeRecords),
	}
	for _, row := range detail.Roster {
		switch row.Status {
		case model.AttendanceStatusPending:
			summary.PendingCount++
		case model.AttendanceStatusPresent:
			summary.PresentCount++
		case model.AttendanceStatusLate:
			summary.LateCount++
		case model.AttendanceStatusAbsent:
			summary.AbsentCount++
		}
	}
	return summary
}

func callerIsResponsible(roster []*AttendanceRosterEntry, userID int64) bool {
	responsibleCount := 0
	callerResponsible := false
	for _, row := range roster {
		if !row.AttendanceResponsible {
			continue
		}
		responsibleCount++
		if row.UserID == userID {
			callerResponsible = true
		}
	}
	return responsibleCount == 1 && callerResponsible
}

func findRosterEntry(roster []*AttendanceRosterEntry, assignmentID, userID int64) *AttendanceRosterEntry {
	for _, row := range roster {
		if row.AssignmentID == assignmentID && row.UserID == userID {
			return row
		}
	}
	return nil
}

func isArrivalWindowOpen(now, scheduledStart, scheduledEnd time.Time) bool {
	return !now.Before(scheduledStart) && now.Before(scheduledEnd)
}

func isOvertimeWindowOpen(now, scheduledStart, scheduledEnd time.Time, windowHours float64) bool {
	windowEnd := scheduledEnd.Add(time.Duration(windowHours * float64(time.Hour)))
	return !now.Before(scheduledStart) && !now.After(windowEnd)
}

func normalizeOvertimeInput(hours float64, note string) (string, error) {
	normalizedNote := strings.TrimSpace(note)
	if hours <= 0 || hours > 24 || normalizedNote == "" || utf8.RuneCountInString(normalizedNote) > maxOvertimeNoteLength {
		return "", ErrInvalidInput
	}
	return normalizedNote, nil
}

func slotIncludesWeekday(slot *model.TemplateSlot, weekday int) bool {
	for _, candidate := range slot.Weekdays {
		if candidate == weekday {
			return true
		}
	}
	return false
}

func attendanceArrivalMetadata(current *model.AttendanceRecord, previous *model.AttendanceRecord) map[string]any {
	record := current
	if record == nil {
		record = previous
	}
	metadata := map[string]any{
		"publication_id":  record.PublicationID,
		"assignment_id":   record.AssignmentID,
		"occurrence_date": record.OccurrenceDate.Format("2006-01-02"),
		"user_id":         record.UserID,
	}
	if previous != nil {
		metadata["previous_arrived_at"] = previous.ArrivedAt.Format(time.RFC3339)
	}
	if current != nil {
		metadata["arrived_at"] = current.ArrivedAt.Format(time.RFC3339)
	}
	return metadata
}

func overtimeMetadata(current *model.AttendanceOvertimeRecord, previous *model.AttendanceOvertimeRecord) map[string]any {
	record := current
	if record == nil {
		record = previous
	}
	metadata := map[string]any{
		"publication_id":  record.PublicationID,
		"slot_id":         record.SlotID,
		"weekday":         record.Weekday,
		"occurrence_date": record.OccurrenceDate.Format("2006-01-02"),
		"user_id":         record.UserID,
	}
	if previous != nil {
		metadata["previous_hours"] = previous.Hours
	}
	if current != nil {
		metadata["hours"] = current.Hours
	}
	return metadata
}

func mapAttendanceRepositoryError(err error) error {
	switch {
	case errors.Is(err, repository.ErrPublicationNotFound):
		return ErrPublicationNotFound
	case errors.Is(err, repository.ErrTemplateSlotNotFound):
		return ErrTemplateSlotNotFound
	case errors.Is(err, repository.ErrUserNotFound):
		return ErrUserNotFound
	case errors.Is(err, repository.ErrAttendanceRecordNotFound):
		return ErrAttendanceRecordNotFound
	case errors.Is(err, repository.ErrAttendanceAlreadyRecorded):
		return ErrAttendanceAlreadyRecorded
	case errors.Is(err, repository.ErrAttendanceRosterStale):
		return ErrAttendanceRosterStale
	default:
		return err
	}
}

func sortAttendanceRoster(entries []*AttendanceRosterEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].AttendanceResponsible != entries[j].AttendanceResponsible {
			return entries[i].AttendanceResponsible
		}
		if entries[i].PositionName != entries[j].PositionName {
			return entries[i].PositionName < entries[j].PositionName
		}
		return entries[i].UserName < entries[j].UserName
	})
}
