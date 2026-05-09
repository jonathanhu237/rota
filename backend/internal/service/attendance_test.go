package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/audit"
	"github.com/jonathanhu237/rota/backend/internal/audit/audittest"
	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/jonathanhu237/rota/backend/internal/repository"
)

type attendanceRepositoryMock struct {
	leaderRefs []model.AttendanceShiftRef
	adminRefs  []model.AttendanceShiftRef
	roster     []*model.AttendanceRosterRow
	orphans    []*model.AttendanceRecord
	arrivals   map[attendanceRecordKey]*model.AttendanceRecord
	overtime   []*model.AttendanceOvertimeRecord
	nextID     int64
}

type attendanceRecordKey struct {
	assignmentID   int64
	occurrenceDate string
	userID         int64
}

func newAttendanceRepositoryMock(ref model.AttendanceShiftRef, roster []*model.AttendanceRosterRow) *attendanceRepositoryMock {
	return &attendanceRepositoryMock{
		leaderRefs: []model.AttendanceShiftRef{ref},
		adminRefs:  []model.AttendanceShiftRef{ref},
		roster:     roster,
		arrivals:   make(map[attendanceRecordKey]*model.AttendanceRecord),
		overtime:   make([]*model.AttendanceOvertimeRecord, 0),
		nextID:     1,
	}
}

func (m *attendanceRepositoryMock) ListLeaderCandidateShifts(
	ctx context.Context,
	publicationID, userID int64,
	fromDate, toDate time.Time,
) ([]model.AttendanceShiftRef, error) {
	return append([]model.AttendanceShiftRef(nil), m.leaderRefs...), nil
}

func (m *attendanceRepositoryMock) ListPublicationShiftRefsForDate(
	ctx context.Context,
	publicationID int64,
	occurrenceDate time.Time,
) ([]model.AttendanceShiftRef, error) {
	return append([]model.AttendanceShiftRef(nil), m.adminRefs...), nil
}

func (m *attendanceRepositoryMock) ListShiftRoster(
	ctx context.Context,
	publicationID, slotID int64,
	weekday int,
	occurrenceDate time.Time,
) ([]*model.AttendanceRosterRow, error) {
	rows := make([]*model.AttendanceRosterRow, 0, len(m.roster))
	for _, row := range m.roster {
		copied := *row
		if record := m.arrivals[keyForAttendance(row.AssignmentID, occurrenceDate, row.UserID)]; record != nil {
			recordCopy := *record
			copied.Record = &recordCopy
		} else if row.Record != nil {
			recordCopy := *row.Record
			copied.Record = &recordCopy
		} else {
			copied.Record = nil
		}
		rows = append(rows, &copied)
	}
	return rows, nil
}

func (m *attendanceRepositoryMock) ListOrphanArrivalRecords(
	ctx context.Context,
	publicationID, slotID int64,
	weekday int,
	occurrenceDate time.Time,
) ([]*model.AttendanceRecord, error) {
	records := make([]*model.AttendanceRecord, 0, len(m.orphans))
	for _, record := range m.orphans {
		copied := *record
		records = append(records, &copied)
	}
	return records, nil
}

func (m *attendanceRepositoryMock) ListOvertimeRecords(
	ctx context.Context,
	publicationID, slotID int64,
	weekday int,
	occurrenceDate time.Time,
) ([]*model.AttendanceOvertimeRecord, error) {
	records := make([]*model.AttendanceOvertimeRecord, 0, len(m.overtime))
	for _, record := range m.overtime {
		copied := *record
		records = append(records, &copied)
	}
	return records, nil
}

func (m *attendanceRepositoryMock) InsertLeaderArrival(
	ctx context.Context,
	params repository.UpsertAttendanceArrivalParams,
) (*model.AttendanceRecord, error) {
	key := keyForAttendance(params.AssignmentID, params.OccurrenceDate, params.UserID)
	if _, ok := m.arrivals[key]; ok {
		return nil, repository.ErrAttendanceAlreadyRecorded
	}
	if !m.isCurrentRosterUser(params.AssignmentID, params.UserID) {
		return nil, repository.ErrAttendanceRosterStale
	}
	record := m.newArrivalRecord(params)
	m.arrivals[key] = record
	return cloneAttendanceRecord(record), nil
}

func (m *attendanceRepositoryMock) UpsertAdminArrival(
	ctx context.Context,
	params repository.UpsertAttendanceArrivalParams,
) (*model.AttendanceRecord, error) {
	if !m.isCurrentRosterUser(params.AssignmentID, params.UserID) {
		return nil, repository.ErrAttendanceRosterStale
	}
	key := keyForAttendance(params.AssignmentID, params.OccurrenceDate, params.UserID)
	record := m.arrivals[key]
	if record == nil {
		record = m.newArrivalRecord(params)
		m.arrivals[key] = record
	} else {
		record.ArrivedAt = params.ArrivedAt
		record.UpdatedByUserID = int64Ptr(params.RecordedByUserID)
		record.UpdatedAt = params.RecordedAt
	}
	return cloneAttendanceRecord(record), nil
}

func (m *attendanceRepositoryMock) DeleteArrival(
	ctx context.Context,
	publicationID, recordID int64,
) (*model.AttendanceRecord, error) {
	for key, record := range m.arrivals {
		if record.ID == recordID && record.PublicationID == publicationID {
			delete(m.arrivals, key)
			return cloneAttendanceRecord(record), nil
		}
	}
	return nil, repository.ErrAttendanceRecordNotFound
}

func (m *attendanceRepositoryMock) CreateOvertime(
	ctx context.Context,
	params repository.CreateOvertimeRecordParams,
) (*model.AttendanceOvertimeRecord, error) {
	record := &model.AttendanceOvertimeRecord{
		ID:               m.nextRecordID(),
		PublicationID:    params.PublicationID,
		SlotID:           params.SlotID,
		Weekday:          params.Weekday,
		OccurrenceDate:   model.NormalizeOccurrenceDate(params.OccurrenceDate),
		UserID:           params.UserID,
		Hours:            params.Hours,
		Note:             params.Note,
		RecordedByUserID: int64Ptr(params.RecordedByUserID),
		RecordedAt:       params.RecordedAt,
		UpdatedByUserID:  int64Ptr(params.RecordedByUserID),
		UpdatedAt:        params.RecordedAt,
		UserName:         "User",
		UserEmail:        "user@example.com",
	}
	m.overtime = append(m.overtime, record)
	return cloneOvertimeRecord(record), nil
}

func (m *attendanceRepositoryMock) GetOvertime(
	ctx context.Context,
	publicationID, recordID int64,
) (*model.AttendanceOvertimeRecord, error) {
	for _, record := range m.overtime {
		if record.ID == recordID && record.PublicationID == publicationID {
			return cloneOvertimeRecord(record), nil
		}
	}
	return nil, repository.ErrAttendanceRecordNotFound
}

func (m *attendanceRepositoryMock) UpdateOvertime(
	ctx context.Context,
	params repository.UpdateOvertimeRecordParams,
) (*model.AttendanceOvertimeRecord, error) {
	for _, record := range m.overtime {
		if record.ID == params.RecordID && record.PublicationID == params.PublicationID {
			record.Hours = params.Hours
			record.Note = params.Note
			record.UpdatedByUserID = int64Ptr(params.UpdatedByUserID)
			record.UpdatedAt = params.UpdatedAt
			return cloneOvertimeRecord(record), nil
		}
	}
	return nil, repository.ErrAttendanceRecordNotFound
}

func (m *attendanceRepositoryMock) DeleteOvertime(
	ctx context.Context,
	publicationID, recordID int64,
) (*model.AttendanceOvertimeRecord, error) {
	for i, record := range m.overtime {
		if record.ID == recordID && record.PublicationID == publicationID {
			m.overtime = append(m.overtime[:i], m.overtime[i+1:]...)
			return cloneOvertimeRecord(record), nil
		}
	}
	return nil, repository.ErrAttendanceRecordNotFound
}

func (m *attendanceRepositoryMock) isCurrentRosterUser(assignmentID, userID int64) bool {
	for _, row := range m.roster {
		if row.AssignmentID == assignmentID && row.UserID == userID {
			return true
		}
	}
	return false
}

func (m *attendanceRepositoryMock) newArrivalRecord(params repository.UpsertAttendanceArrivalParams) *model.AttendanceRecord {
	record := &model.AttendanceRecord{
		ID:               m.nextRecordID(),
		PublicationID:    params.PublicationID,
		AssignmentID:     params.AssignmentID,
		OccurrenceDate:   model.NormalizeOccurrenceDate(params.OccurrenceDate),
		UserID:           params.UserID,
		ArrivedAt:        params.ArrivedAt,
		RecordedByUserID: int64Ptr(params.RecordedByUserID),
		RecordedAt:       params.RecordedAt,
		UpdatedByUserID:  int64Ptr(params.RecordedByUserID),
		UpdatedAt:        params.RecordedAt,
		UserName:         "User",
		UserEmail:        "user@example.com",
	}
	for _, row := range m.roster {
		if row.AssignmentID == params.AssignmentID && row.UserID == params.UserID {
			record.UserName = row.UserName
			record.UserEmail = row.UserEmail
			break
		}
	}
	return record
}

func (m *attendanceRepositoryMock) nextRecordID() int64 {
	id := m.nextID
	m.nextID++
	return id
}

type attendancePublicationRepositoryMock struct {
	publication *model.Publication
	slot        *model.TemplateSlot
	positions   []*model.TemplateSlotPosition
	users       map[int64]*model.User
}

func (m *attendancePublicationRepositoryMock) GetByID(ctx context.Context, id int64) (*model.Publication, error) {
	if m.publication == nil || m.publication.ID != id {
		return nil, repository.ErrPublicationNotFound
	}
	copied := *m.publication
	return &copied, nil
}

func (m *attendancePublicationRepositoryMock) GetCurrent(ctx context.Context) (*model.Publication, error) {
	if m.publication == nil {
		return nil, repository.ErrPublicationNotFound
	}
	copied := *m.publication
	return &copied, nil
}

func (m *attendancePublicationRepositoryMock) GetSlot(
	ctx context.Context,
	templateID, slotID int64,
) (*model.TemplateSlot, error) {
	if m.slot == nil || m.slot.TemplateID != templateID || m.slot.ID != slotID {
		return nil, repository.ErrTemplateSlotNotFound
	}
	copied := *m.slot
	copied.Weekdays = append([]int(nil), m.slot.Weekdays...)
	return &copied, nil
}

func (m *attendancePublicationRepositoryMock) ListSlotPositions(
	ctx context.Context,
	slotID int64,
) ([]*model.TemplateSlotPosition, error) {
	positions := make([]*model.TemplateSlotPosition, 0, len(m.positions))
	for _, position := range m.positions {
		if position.SlotID != slotID {
			continue
		}
		copied := *position
		positions = append(positions, &copied)
	}
	return positions, nil
}

func (m *attendancePublicationRepositoryMock) GetUserByID(ctx context.Context, id int64) (*model.User, error) {
	user := m.users[id]
	if user == nil {
		return nil, repository.ErrUserNotFound
	}
	copied := *user
	return &copied, nil
}

func (m *attendancePublicationRepositoryMock) UpdatePublicationFields(
	ctx context.Context,
	params repository.UpdatePublicationFieldsParams,
) (*model.Publication, error) {
	if m.publication == nil || m.publication.ID != params.ID {
		return nil, repository.ErrPublicationNotFound
	}
	if params.OvertimeEntryWindowHours != nil {
		m.publication.OvertimeEntryWindowHours = *params.OvertimeEntryWindowHours
	}
	m.publication.UpdatedAt = params.UpdatedAt
	copied := *m.publication
	return &copied, nil
}

func TestAttendanceServiceListCurrentAttendance(t *testing.T) {
	t.Run("current shift appears at scheduled start", func(t *testing.T) {
		t.Parallel()

		fixture := newAttendanceServiceFixture(time.Date(2026, 5, 11, 9, 0, 0, 0, time.UTC))

		result, err := fixture.service.ListCurrentAttendance(context.Background(), fixture.leaderUserID)
		if err != nil {
			t.Fatalf("ListCurrentAttendance returned error: %v", err)
		}
		if len(result.Shifts) != 1 {
			t.Fatalf("expected one current shift, got %d", len(result.Shifts))
		}
		if !result.Shifts[0].ArrivalWindowOpen || !result.Shifts[0].OvertimeWindowOpen {
			t.Fatalf("expected both windows open at shift start: %+v", result.Shifts[0])
		}
	})

	t.Run("future shift does not appear early", func(t *testing.T) {
		t.Parallel()

		fixture := newAttendanceServiceFixture(time.Date(2026, 5, 11, 8, 59, 59, 0, time.UTC))

		result, err := fixture.service.ListCurrentAttendance(context.Background(), fixture.leaderUserID)
		if err != nil {
			t.Fatalf("ListCurrentAttendance returned error: %v", err)
		}
		if len(result.Shifts) != 0 {
			t.Fatalf("expected no early shift, got %d", len(result.Shifts))
		}
	})

	t.Run("shift remains visible for overtime window after end", func(t *testing.T) {
		t.Parallel()

		fixture := newAttendanceServiceFixture(time.Date(2026, 5, 12, 11, 0, 0, 0, time.UTC))

		result, err := fixture.service.ListCurrentAttendance(context.Background(), fixture.leaderUserID)
		if err != nil {
			t.Fatalf("ListCurrentAttendance returned error: %v", err)
		}
		if len(result.Shifts) != 1 {
			t.Fatalf("expected one overtime-window shift, got %d", len(result.Shifts))
		}
		shift := result.Shifts[0]
		if shift.ArrivalWindowOpen || !shift.OvertimeWindowOpen {
			t.Fatalf("expected only overtime window open, got arrival=%v overtime=%v", shift.ArrivalWindowOpen, shift.OvertimeWindowOpen)
		}
		if shift.Roster[1].Status != model.AttendanceStatusAbsent {
			t.Fatalf("expected absent status after shift end, got %s", shift.Roster[1].Status)
		}
	})

	t.Run("admin detail includes current roster and orphan arrivals", func(t *testing.T) {
		t.Parallel()

		fixture := newAttendanceServiceFixture(time.Date(2026, 5, 11, 13, 0, 0, 0, time.UTC))
		oldUserRecord := &model.AttendanceRecord{
			ID:             55,
			PublicationID:  fixture.publication.ID,
			AssignmentID:   fixture.workerAssignmentID,
			OccurrenceDate: fixture.occurrenceDate,
			UserID:         9,
			UserName:       "Alice",
			UserEmail:      "alice@example.com",
			ArrivedAt:      fixture.scheduledStart,
			RecordedAt:     fixture.scheduledStart,
			UpdatedAt:      fixture.scheduledStart,
		}
		fixture.attendanceRepo.orphans = []*model.AttendanceRecord{oldUserRecord}
		fixture.attendanceRepo.roster[1].UserID = 2
		fixture.attendanceRepo.roster[1].UserName = "Bob"

		shift, err := fixture.service.GetAdminShiftAttendance(context.Background(), GetAdminShiftAttendanceInput{
			PublicationID:  fixture.publication.ID,
			SlotID:         fixture.slot.ID,
			OccurrenceDate: fixture.occurrenceDate,
		})
		if err != nil {
			t.Fatalf("GetAdminShiftAttendance returned error: %v", err)
		}
		if shift.Roster[1].UserName != "Bob" {
			t.Fatalf("expected current roster user Bob, got %+v", shift.Roster[1])
		}
		if len(shift.OrphanArrivals) != 1 || shift.OrphanArrivals[0].Record.UserName != "Alice" {
			t.Fatalf("expected orphan Alice arrival, got %+v", shift.OrphanArrivals)
		}
	})
}

func TestAttendanceServiceRecordLeaderArrival(t *testing.T) {
	t.Run("records default scheduled-start arrival", func(t *testing.T) {
		t.Parallel()

		fixture := newAttendanceServiceFixture(time.Date(2026, 5, 11, 9, 20, 0, 0, time.UTC))
		stub := audittest.New()

		shift, err := fixture.service.RecordLeaderArrival(stub.ContextWith(context.Background()), RecordLeaderArrivalInput{
			ActorUserID:    fixture.leaderUserID,
			PublicationID:  fixture.publication.ID,
			SlotID:         fixture.slot.ID,
			AssignmentID:   fixture.workerAssignmentID,
			OccurrenceDate: fixture.occurrenceDate,
			UserID:         fixture.workerUserID,
		})
		if err != nil {
			t.Fatalf("RecordLeaderArrival returned error: %v", err)
		}
		worker := findTestRosterEntry(t, shift, fixture.workerAssignmentID, fixture.workerUserID)
		if worker.Record == nil || !worker.Record.ArrivedAt.Equal(fixture.scheduledStart) {
			t.Fatalf("expected scheduled-start arrival, got %+v", worker.Record)
		}
		if worker.Status != model.AttendanceStatusPresent {
			t.Fatalf("expected present status, got %s", worker.Status)
		}
		event := assertAttendanceAudit(t, stub, audit.ActionAttendanceArrivalRecord, audit.TargetTypeAttendanceRecord)
		if event.Metadata["arrived_at"] == nil {
			t.Fatalf("expected arrived_at in audit metadata, got %+v", event.Metadata)
		}
	})

	t.Run("records late arrival", func(t *testing.T) {
		t.Parallel()

		fixture := newAttendanceServiceFixture(time.Date(2026, 5, 11, 9, 20, 0, 0, time.UTC))
		arrivedAt := time.Date(2026, 5, 11, 9, 15, 0, 0, time.UTC)

		shift, err := fixture.service.RecordLeaderArrival(context.Background(), RecordLeaderArrivalInput{
			ActorUserID:    fixture.leaderUserID,
			PublicationID:  fixture.publication.ID,
			SlotID:         fixture.slot.ID,
			AssignmentID:   fixture.workerAssignmentID,
			OccurrenceDate: fixture.occurrenceDate,
			UserID:         fixture.workerUserID,
			ArrivedAt:      &arrivedAt,
		})
		if err != nil {
			t.Fatalf("RecordLeaderArrival returned error: %v", err)
		}
		worker := findTestRosterEntry(t, shift, fixture.workerAssignmentID, fixture.workerUserID)
		if worker.Status != model.AttendanceStatusLate {
			t.Fatalf("expected late status, got %s", worker.Status)
		}
	})

	t.Run("rejects duplicate locked arrival", func(t *testing.T) {
		t.Parallel()

		fixture := newAttendanceServiceFixture(time.Date(2026, 5, 11, 9, 20, 0, 0, time.UTC))
		fixture.seedArrival(fixture.workerAssignmentID, fixture.workerUserID, fixture.scheduledStart)
		stub := audittest.New()

		_, err := fixture.service.RecordLeaderArrival(stub.ContextWith(context.Background()), RecordLeaderArrivalInput{
			ActorUserID:    fixture.leaderUserID,
			PublicationID:  fixture.publication.ID,
			SlotID:         fixture.slot.ID,
			AssignmentID:   fixture.workerAssignmentID,
			OccurrenceDate: fixture.occurrenceDate,
			UserID:         fixture.workerUserID,
		})
		if !errors.Is(err, ErrAttendanceAlreadyRecorded) {
			t.Fatalf("expected ErrAttendanceAlreadyRecorded, got %v", err)
		}
		if len(stub.Events()) != 0 {
			t.Fatalf("expected no audit events on duplicate, got %v", stub.Actions())
		}
	})

	t.Run("rejects non-leader", func(t *testing.T) {
		t.Parallel()

		fixture := newAttendanceServiceFixture(time.Date(2026, 5, 11, 9, 20, 0, 0, time.UTC))

		_, err := fixture.service.RecordLeaderArrival(context.Background(), RecordLeaderArrivalInput{
			ActorUserID:    fixture.workerUserID,
			PublicationID:  fixture.publication.ID,
			SlotID:         fixture.slot.ID,
			AssignmentID:   fixture.workerAssignmentID,
			OccurrenceDate: fixture.occurrenceDate,
			UserID:         fixture.workerUserID,
		})
		if !errors.Is(err, ErrAttendanceNotLeader) {
			t.Fatalf("expected ErrAttendanceNotLeader, got %v", err)
		}
	})

	t.Run("rejects after shift end", func(t *testing.T) {
		t.Parallel()

		fixture := newAttendanceServiceFixture(time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC))
		stub := audittest.New()

		_, err := fixture.service.RecordLeaderArrival(stub.ContextWith(context.Background()), RecordLeaderArrivalInput{
			ActorUserID:    fixture.leaderUserID,
			PublicationID:  fixture.publication.ID,
			SlotID:         fixture.slot.ID,
			AssignmentID:   fixture.workerAssignmentID,
			OccurrenceDate: fixture.occurrenceDate,
			UserID:         fixture.workerUserID,
		})
		if !errors.Is(err, ErrAttendanceWindowClosed) {
			t.Fatalf("expected ErrAttendanceWindowClosed, got %v", err)
		}
		if len(stub.Events()) != 0 {
			t.Fatalf("expected no audit events on closed window, got %v", stub.Actions())
		}
	})

	t.Run("rejects future arrival time", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 5, 11, 9, 20, 0, 0, time.UTC)
		fixture := newAttendanceServiceFixture(now)
		arrivedAt := now.Add(time.Minute)

		_, err := fixture.service.RecordLeaderArrival(context.Background(), RecordLeaderArrivalInput{
			ActorUserID:    fixture.leaderUserID,
			PublicationID:  fixture.publication.ID,
			SlotID:         fixture.slot.ID,
			AssignmentID:   fixture.workerAssignmentID,
			OccurrenceDate: fixture.occurrenceDate,
			UserID:         fixture.workerUserID,
			ArrivedAt:      &arrivedAt,
		})
		if !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("expected ErrInvalidInput, got %v", err)
		}
	})

	t.Run("rejects stale roster user", func(t *testing.T) {
		t.Parallel()

		fixture := newAttendanceServiceFixture(time.Date(2026, 5, 11, 9, 20, 0, 0, time.UTC))

		_, err := fixture.service.RecordLeaderArrival(context.Background(), RecordLeaderArrivalInput{
			ActorUserID:    fixture.leaderUserID,
			PublicationID:  fixture.publication.ID,
			SlotID:         fixture.slot.ID,
			AssignmentID:   fixture.workerAssignmentID,
			OccurrenceDate: fixture.occurrenceDate,
			UserID:         999,
		})
		if !errors.Is(err, ErrAttendanceRosterStale) {
			t.Fatalf("expected ErrAttendanceRosterStale, got %v", err)
		}
	})
}

func TestAttendanceServiceRecordLeaderOvertime(t *testing.T) {
	t.Run("records overtime during shift and omits note from audit metadata", func(t *testing.T) {
		t.Parallel()

		fixture := newAttendanceServiceFixture(time.Date(2026, 5, 11, 9, 30, 0, 0, time.UTC))
		stub := audittest.New()

		record, err := fixture.service.RecordLeaderOvertime(stub.ContextWith(context.Background()), RecordOvertimeInput{
			ActorUserID:    fixture.leaderUserID,
			PublicationID:  fixture.publication.ID,
			SlotID:         fixture.slot.ID,
			OccurrenceDate: fixture.occurrenceDate,
			UserID:         fixture.workerUserID,
			Hours:          1.5,
			Note:           " stayed late ",
		})
		if err != nil {
			t.Fatalf("RecordLeaderOvertime returned error: %v", err)
		}
		if record.Note != "stayed late" || record.Hours != 1.5 {
			t.Fatalf("unexpected overtime record: %+v", record)
		}
		event := assertAttendanceAudit(t, stub, audit.ActionAttendanceOvertimeRecord, audit.TargetTypeAttendanceOvertime)
		if _, exists := event.Metadata["note"]; exists {
			t.Fatalf("expected no note text in audit metadata, got %+v", event.Metadata)
		}
		if event.Metadata["hours"] != 1.5 {
			t.Fatalf("expected hours metadata, got %+v", event.Metadata)
		}
	})

	t.Run("records overtime after shift within window", func(t *testing.T) {
		t.Parallel()

		fixture := newAttendanceServiceFixture(time.Date(2026, 5, 12, 11, 0, 0, 0, time.UTC))

		_, err := fixture.service.RecordLeaderOvertime(context.Background(), RecordOvertimeInput{
			ActorUserID:    fixture.leaderUserID,
			PublicationID:  fixture.publication.ID,
			SlotID:         fixture.slot.ID,
			OccurrenceDate: fixture.occurrenceDate,
			UserID:         fixture.workerUserID,
			Hours:          2,
			Note:           "cleanup",
		})
		if err != nil {
			t.Fatalf("RecordLeaderOvertime returned error: %v", err)
		}
	})

	t.Run("rejects expired overtime window", func(t *testing.T) {
		t.Parallel()

		fixture := newAttendanceServiceFixture(time.Date(2026, 5, 12, 12, 0, 1, 0, time.UTC))

		_, err := fixture.service.RecordLeaderOvertime(context.Background(), RecordOvertimeInput{
			ActorUserID:    fixture.leaderUserID,
			PublicationID:  fixture.publication.ID,
			SlotID:         fixture.slot.ID,
			OccurrenceDate: fixture.occurrenceDate,
			UserID:         fixture.workerUserID,
			Hours:          1,
			Note:           "late task",
		})
		if !errors.Is(err, ErrAttendanceWindowClosed) {
			t.Fatalf("expected ErrAttendanceWindowClosed, got %v", err)
		}
	})

	t.Run("allows active non-roster user", func(t *testing.T) {
		t.Parallel()

		fixture := newAttendanceServiceFixture(time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC))

		_, err := fixture.service.RecordLeaderOvertime(context.Background(), RecordOvertimeInput{
			ActorUserID:    fixture.leaderUserID,
			PublicationID:  fixture.publication.ID,
			SlotID:         fixture.slot.ID,
			OccurrenceDate: fixture.occurrenceDate,
			UserID:         fixture.extraUserID,
			Hours:          1.25,
			Note:           "emergency cover",
		})
		if err != nil {
			t.Fatalf("RecordLeaderOvertime returned error: %v", err)
		}
	})

	t.Run("authorizes leader before target user lookup", func(t *testing.T) {
		t.Parallel()

		fixture := newAttendanceServiceFixture(time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC))

		_, err := fixture.service.RecordLeaderOvertime(context.Background(), RecordOvertimeInput{
			ActorUserID:    fixture.workerUserID,
			PublicationID:  fixture.publication.ID,
			SlotID:         fixture.slot.ID,
			OccurrenceDate: fixture.occurrenceDate,
			UserID:         999,
			Hours:          1,
			Note:           "coverage",
		})
		if !errors.Is(err, ErrAttendanceNotLeader) {
			t.Fatalf("expected ErrAttendanceNotLeader, got %v", err)
		}
	})

	t.Run("rejects blank note and excessive hours", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name  string
			hours float64
			note  string
		}{
			{name: "blank note", hours: 1, note: "   "},
			{name: "hours over twenty four", hours: 24.01, note: "too much"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				fixture := newAttendanceServiceFixture(time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC))

				_, err := fixture.service.RecordLeaderOvertime(context.Background(), RecordOvertimeInput{
					ActorUserID:    fixture.leaderUserID,
					PublicationID:  fixture.publication.ID,
					SlotID:         fixture.slot.ID,
					OccurrenceDate: fixture.occurrenceDate,
					UserID:         fixture.workerUserID,
					Hours:          tc.hours,
					Note:           tc.note,
				})
				if !errors.Is(err, ErrInvalidInput) {
					t.Fatalf("expected ErrInvalidInput, got %v", err)
				}
			})
		}
	})
}

func TestAttendanceServiceAdminAttendance(t *testing.T) {
	t.Run("admin upserts and clears arrival outside leader window", func(t *testing.T) {
		t.Parallel()

		fixture := newAttendanceServiceFixture(time.Date(2026, 5, 11, 13, 0, 0, 0, time.UTC))
		stub := audittest.New()
		arrivedAt := time.Date(2026, 5, 11, 9, 30, 0, 0, time.UTC)

		shift, err := fixture.service.AdminUpsertArrival(stub.ContextWith(context.Background()), AdminUpsertArrivalInput{
			ActorUserID:    99,
			PublicationID:  fixture.publication.ID,
			SlotID:         fixture.slot.ID,
			AssignmentID:   fixture.workerAssignmentID,
			OccurrenceDate: fixture.occurrenceDate,
			UserID:         fixture.workerUserID,
			ArrivedAt:      arrivedAt,
		})
		if err != nil {
			t.Fatalf("AdminUpsertArrival returned error: %v", err)
		}
		worker := findTestRosterEntry(t, shift, fixture.workerAssignmentID, fixture.workerUserID)
		if worker.Status != model.AttendanceStatusLate {
			t.Fatalf("expected late status after admin upsert, got %s", worker.Status)
		}
		assertAttendanceAudit(t, stub, audit.ActionAttendanceArrivalAdminAdjust, audit.TargetTypeAttendanceRecord)

		err = fixture.service.AdminClearArrival(stub.ContextWith(context.Background()), AdminClearArrivalInput{
			ActorUserID:   99,
			PublicationID: fixture.publication.ID,
			RecordID:      worker.Record.ID,
		})
		if err != nil {
			t.Fatalf("AdminClearArrival returned error: %v", err)
		}
		if len(fixture.attendanceRepo.arrivals) != 0 {
			t.Fatalf("expected arrival to be removed, got %d", len(fixture.attendanceRepo.arrivals))
		}
		assertAttendanceAudit(t, stub, audit.ActionAttendanceArrivalAdminClear, audit.TargetTypeAttendanceRecord)
	})

	t.Run("admin can record an early arrival", func(t *testing.T) {
		t.Parallel()

		fixture := newAttendanceServiceFixture(time.Date(2026, 5, 11, 13, 0, 0, 0, time.UTC))
		arrivedAt := fixture.scheduledStart.Add(-10 * time.Minute)

		shift, err := fixture.service.AdminUpsertArrival(context.Background(), AdminUpsertArrivalInput{
			ActorUserID:    99,
			PublicationID:  fixture.publication.ID,
			SlotID:         fixture.slot.ID,
			AssignmentID:   fixture.workerAssignmentID,
			OccurrenceDate: fixture.occurrenceDate,
			UserID:         fixture.workerUserID,
			ArrivedAt:      arrivedAt,
		})
		if err != nil {
			t.Fatalf("AdminUpsertArrival returned error: %v", err)
		}
		worker := findTestRosterEntry(t, shift, fixture.workerAssignmentID, fixture.workerUserID)
		if worker.Record == nil || !worker.Record.ArrivedAt.Equal(arrivedAt) {
			t.Fatalf("expected early arrival to be recorded, got %+v", worker.Record)
		}
		if worker.Status != model.AttendanceStatusPresent {
			t.Fatalf("expected present status for early arrival, got %s", worker.Status)
		}
	})

	t.Run("admin arrival correction must target current roster", func(t *testing.T) {
		t.Parallel()

		fixture := newAttendanceServiceFixture(time.Date(2026, 5, 11, 13, 0, 0, 0, time.UTC))

		_, err := fixture.service.AdminUpsertArrival(context.Background(), AdminUpsertArrivalInput{
			ActorUserID:    99,
			PublicationID:  fixture.publication.ID,
			SlotID:         fixture.slot.ID,
			AssignmentID:   fixture.workerAssignmentID,
			OccurrenceDate: fixture.occurrenceDate,
			UserID:         fixture.extraUserID,
			ArrivedAt:      fixture.scheduledStart,
		})
		if !errors.Is(err, ErrAttendanceRosterStale) {
			t.Fatalf("expected ErrAttendanceRosterStale, got %v", err)
		}
	})

	t.Run("admin manages overtime outside leader window", func(t *testing.T) {
		t.Parallel()

		fixture := newAttendanceServiceFixture(time.Date(2026, 5, 13, 9, 0, 0, 0, time.UTC))
		stub := audittest.New()

		record, err := fixture.service.AdminCreateOvertime(stub.ContextWith(context.Background()), RecordOvertimeInput{
			ActorUserID:    99,
			PublicationID:  fixture.publication.ID,
			SlotID:         fixture.slot.ID,
			OccurrenceDate: fixture.occurrenceDate,
			UserID:         fixture.workerUserID,
			Hours:          1,
			Note:           "back office task",
		})
		if err != nil {
			t.Fatalf("AdminCreateOvertime returned error: %v", err)
		}
		assertAttendanceAudit(t, stub, audit.ActionAttendanceOvertimeAdminCreate, audit.TargetTypeAttendanceOvertime)

		updated, err := fixture.service.AdminUpdateOvertime(stub.ContextWith(context.Background()), AdminUpdateOvertimeInput{
			ActorUserID:   99,
			PublicationID: fixture.publication.ID,
			RecordID:      record.ID,
			Hours:         1.5,
			Note:          "updated note",
		})
		if err != nil {
			t.Fatalf("AdminUpdateOvertime returned error: %v", err)
		}
		if updated.Hours != 1.5 || updated.Note != "updated note" {
			t.Fatalf("unexpected updated overtime: %+v", updated)
		}
		event := assertAttendanceAudit(t, stub, audit.ActionAttendanceOvertimeAdminAdjust, audit.TargetTypeAttendanceOvertime)
		if _, exists := event.Metadata["note"]; exists {
			t.Fatalf("expected no overtime note in audit metadata, got %+v", event.Metadata)
		}
		if event.Metadata["previous_hours"] != 1.0 || event.Metadata["hours"] != 1.5 {
			t.Fatalf("expected old and new hours in audit metadata, got %+v", event.Metadata)
		}

		if err := fixture.service.AdminDeleteOvertime(stub.ContextWith(context.Background()), AdminClearArrivalInput{
			ActorUserID:   99,
			PublicationID: fixture.publication.ID,
			RecordID:      record.ID,
		}); err != nil {
			t.Fatalf("AdminDeleteOvertime returned error: %v", err)
		}
		if len(fixture.attendanceRepo.overtime) != 0 {
			t.Fatalf("expected overtime record to be deleted, got %d", len(fixture.attendanceRepo.overtime))
		}
		assertAttendanceAudit(t, stub, audit.ActionAttendanceOvertimeAdminDelete, audit.TargetTypeAttendanceOvertime)
	})
}

func TestAttendanceServiceUpdateAttendanceSettings(t *testing.T) {
	t.Run("updates overtime window and audits old and new values", func(t *testing.T) {
		t.Parallel()

		fixture := newAttendanceServiceFixture(time.Date(2026, 5, 11, 9, 0, 0, 0, time.UTC))
		stub := audittest.New()

		publication, err := fixture.service.UpdateAttendanceSettings(stub.ContextWith(context.Background()), UpdateAttendanceSettingsInput{
			ActorUserID:              99,
			PublicationID:            fixture.publication.ID,
			OvertimeEntryWindowHours: 12.5,
		})
		if err != nil {
			t.Fatalf("UpdateAttendanceSettings returned error: %v", err)
		}
		if publication.OvertimeEntryWindowHours != 12.5 {
			t.Fatalf("expected overtime window 12.5, got %v", publication.OvertimeEntryWindowHours)
		}
		event := assertAttendanceAudit(t, stub, audit.ActionAttendanceSettingsUpdate, audit.TargetTypePublication)
		if event.Metadata["overtime_entry_window_hours"] == nil {
			t.Fatalf("expected overtime window metadata, got %+v", event.Metadata)
		}
	})

	t.Run("rejects invalid overtime window without audit", func(t *testing.T) {
		t.Parallel()

		fixture := newAttendanceServiceFixture(time.Date(2026, 5, 11, 9, 0, 0, 0, time.UTC))
		stub := audittest.New()

		_, err := fixture.service.UpdateAttendanceSettings(stub.ContextWith(context.Background()), UpdateAttendanceSettingsInput{
			ActorUserID:              99,
			PublicationID:            fixture.publication.ID,
			OvertimeEntryWindowHours: -1,
		})
		if !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("expected ErrInvalidInput, got %v", err)
		}
		if len(stub.Events()) != 0 {
			t.Fatalf("expected no audit events, got %v", stub.Actions())
		}
	})
}

type attendanceServiceFixture struct {
	service            *AttendanceService
	attendanceRepo     *attendanceRepositoryMock
	publicationRepo    *attendancePublicationRepositoryMock
	publication        *model.Publication
	slot               *model.TemplateSlot
	occurrenceDate     time.Time
	scheduledStart     time.Time
	leaderUserID       int64
	workerUserID       int64
	extraUserID        int64
	leaderAssignmentID int64
	workerAssignmentID int64
}

func newAttendanceServiceFixture(now time.Time) attendanceServiceFixture {
	occurrenceDate := time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC)
	scheduledStart := time.Date(2026, 5, 11, 9, 0, 0, 0, time.UTC)
	publication := &model.Publication{
		ID:                       10,
		TemplateID:               20,
		TemplateName:             "Core Week",
		Name:                     "May roster",
		State:                    model.PublicationStateActive,
		SubmissionStartAt:        time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC),
		SubmissionEndAt:          time.Date(2026, 4, 15, 9, 0, 0, 0, time.UTC),
		PlannedActiveFrom:        time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		PlannedActiveUntil:       time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		OvertimeEntryWindowHours: 24,
		CreatedAt:                time.Date(2026, 4, 1, 8, 0, 0, 0, time.UTC),
		UpdatedAt:                time.Date(2026, 4, 1, 8, 0, 0, 0, time.UTC),
	}
	slot := &model.TemplateSlot{
		ID:         30,
		TemplateID: publication.TemplateID,
		Weekdays:   []int{1},
		StartTime:  "09:00",
		EndTime:    "12:00",
	}
	ref := model.AttendanceShiftRef{
		SlotID:         slot.ID,
		Weekday:        1,
		StartTime:      slot.StartTime,
		EndTime:        slot.EndTime,
		OccurrenceDate: occurrenceDate,
	}
	const (
		leaderUserID       = int64(1)
		workerUserID       = int64(2)
		extraUserID        = int64(3)
		leaderAssignmentID = int64(1001)
		workerAssignmentID = int64(1002)
	)
	roster := []*model.AttendanceRosterRow{
		{
			AssignmentID:          leaderAssignmentID,
			SlotID:                slot.ID,
			Weekday:               1,
			PositionID:            41,
			PositionName:          "负责人",
			AttendanceResponsible: true,
			UserID:                leaderUserID,
			UserName:              "Leader",
			UserEmail:             "leader@example.com",
		},
		{
			AssignmentID: workerAssignmentID,
			SlotID:       slot.ID,
			Weekday:      1,
			PositionID:   42,
			PositionName: "Front Desk",
			UserID:       workerUserID,
			UserName:     "Worker",
			UserEmail:    "worker@example.com",
		},
	}
	attendanceRepo := newAttendanceRepositoryMock(ref, roster)
	publicationRepo := &attendancePublicationRepositoryMock{
		publication: publication,
		slot:        slot,
		positions: []*model.TemplateSlotPosition{
			{ID: 501, SlotID: slot.ID, PositionID: 41, RequiredHeadcount: 1, AttendanceResponsible: true},
			{ID: 502, SlotID: slot.ID, PositionID: 42, RequiredHeadcount: 1},
		},
		users: map[int64]*model.User{
			leaderUserID: {ID: leaderUserID, Name: "Leader", Email: "leader@example.com", Status: model.UserStatusActive},
			workerUserID: {ID: workerUserID, Name: "Worker", Email: "worker@example.com", Status: model.UserStatusActive},
			extraUserID:  {ID: extraUserID, Name: "Extra", Email: "extra@example.com", Status: model.UserStatusActive},
		},
	}

	return attendanceServiceFixture{
		service:            NewAttendanceService(attendanceRepo, publicationRepo, fixedClock{now: now}),
		attendanceRepo:     attendanceRepo,
		publicationRepo:    publicationRepo,
		publication:        publication,
		slot:               slot,
		occurrenceDate:     occurrenceDate,
		scheduledStart:     scheduledStart,
		leaderUserID:       leaderUserID,
		workerUserID:       workerUserID,
		extraUserID:        extraUserID,
		leaderAssignmentID: leaderAssignmentID,
		workerAssignmentID: workerAssignmentID,
	}
}

func (f attendanceServiceFixture) seedArrival(assignmentID, userID int64, arrivedAt time.Time) *model.AttendanceRecord {
	record := f.attendanceRepo.newArrivalRecord(repository.UpsertAttendanceArrivalParams{
		PublicationID:    f.publication.ID,
		AssignmentID:     assignmentID,
		OccurrenceDate:   f.occurrenceDate,
		UserID:           userID,
		ArrivedAt:        arrivedAt,
		RecordedByUserID: f.leaderUserID,
		RecordedAt:       arrivedAt,
	})
	f.attendanceRepo.arrivals[keyForAttendance(assignmentID, f.occurrenceDate, userID)] = record
	return record
}

func keyForAttendance(assignmentID int64, occurrenceDate time.Time, userID int64) attendanceRecordKey {
	return attendanceRecordKey{
		assignmentID:   assignmentID,
		occurrenceDate: model.NormalizeOccurrenceDate(occurrenceDate).Format("2006-01-02"),
		userID:         userID,
	}
}

func cloneAttendanceRecord(record *model.AttendanceRecord) *model.AttendanceRecord {
	if record == nil {
		return nil
	}
	copied := *record
	return &copied
}

func cloneOvertimeRecord(record *model.AttendanceOvertimeRecord) *model.AttendanceOvertimeRecord {
	if record == nil {
		return nil
	}
	copied := *record
	return &copied
}

func int64Ptr(value int64) *int64 {
	return &value
}

func findTestRosterEntry(
	t testing.TB,
	shift *AttendanceShiftDetail,
	assignmentID int64,
	userID int64,
) *AttendanceRosterEntry {
	t.Helper()

	for _, row := range shift.Roster {
		if row.AssignmentID == assignmentID && row.UserID == userID {
			return row
		}
	}
	t.Fatalf("roster entry assignment=%d user=%d not found in %+v", assignmentID, userID, shift.Roster)
	return nil
}

func assertAttendanceAudit(
	t testing.TB,
	stub *audittest.Stub,
	action string,
	targetType string,
) audit.RecordedEvent {
	t.Helper()

	event := stub.FindByAction(action)
	if event == nil {
		t.Fatalf("expected %q audit event, got %v", action, stub.Actions())
	}
	if event.TargetType != targetType {
		t.Fatalf("expected target type %q, got %q", targetType, event.TargetType)
	}
	return *event
}
