package service

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/jonathanhu237/rota/api/internal/rota/model"
	"github.com/jonathanhu237/rota/api/internal/rota/repository"
)

const (
	serviceTestUserA = "019535d9-3df7-79fb-b466-fa907fa17f9e"
	serviceTestUserB = "019535d9-3df7-79fb-b466-fa907fa17f90"
)

type serviceTestClock time.Time

func (c serviceTestClock) Now() time.Time { return time.Time(c) }

func serviceTestPublication(state model.PublicationState) *model.Publication {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	return &model.Publication{
		ID:                       1,
		TemplateID:               2,
		TemplateName:             "Weekly",
		Name:                     "May rota",
		State:                    state,
		SubmissionStartAt:        now.Add(-24 * time.Hour),
		SubmissionEndAt:          now.Add(24 * time.Hour),
		PlannedActiveFrom:        now.Add(48 * time.Hour),
		PlannedActiveUntil:       now.Add(14 * 24 * time.Hour),
		OvertimeEntryWindowHours: 24,
		CreatedAt:                now.Add(-48 * time.Hour),
		UpdatedAt:                now,
	}
}

// Embedding the production interfaces keeps these fakes honest when a
// repository contract grows, while the methods below provide state for the
// paths under test.
type migratedPublicationRepo struct {
	publicationRepository
	publication                *model.Publication
	publications               []*model.Publication
	listTotal                  int
	getErr                     error
	listErr                    error
	submissions                []model.SlotRef
	qualifiedShifts            []*model.QualifiedShift
	slot                       *model.TemplateSlot
	slotPositions              []*model.TemplateSlotPosition
	qualified                  bool
	upserted                   *model.AvailabilitySubmission
	updatedPublication         *model.Publication
	adminEmployees             []*repository.AdminAvailabilityEmployee
	adminEmployeeTotal         int
	adminUser                  *model.User
	adminPositions             []*model.Position
	publicationShifts          []*model.PublicationShift
	publicationAssignments     []*model.AssignmentParticipant
	replaceAvailability        *repository.ReplaceAdminAvailabilitySubmissionsResult
	createdPublication         *model.Publication
	createPublicationErr       error
	deletePublicationErr       error
	createdAssignment          *model.Assignment
	createAssignmentErr        error
	deleteAssignmentErr        error
	assignment                 *model.Assignment
	assignmentOverrides        int64
	countOverridesErr          error
	replaceAssignmentsErr      error
	activatedResult            *repository.ActivatePublicationResult
	activateErr                error
	publishedPublication       *model.Publication
	publishErr                 error
	boardView                  map[repository.AssignmentBoardSlotKey]*repository.AssignmentBoardSlotView
	boardEmployees             []*model.AssignmentBoardEmployee
	assignmentCandidates       []*model.AssignmentCandidate
	qualifiedUsers             map[int64][]*model.AssignmentCandidate
	publicationAssignmentsWeek []*model.AssignmentParticipant
}

func (r *migratedPublicationRepo) ListPaginated(context.Context, repository.ListPublicationsParams) ([]*model.Publication, int, error) {
	return r.publications, r.listTotal, r.listErr
}
func (r *migratedPublicationRepo) GetByID(context.Context, int64) (*model.Publication, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}
	return r.publication, nil
}
func (r *migratedPublicationRepo) GetCurrent(context.Context) (*model.Publication, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}
	return r.publication, nil
}
func (r *migratedPublicationRepo) GetUserByID(context.Context, string) (*model.User, error) {
	if r.adminUser == nil {
		return nil, repository.ErrUserNotFound
	}
	return r.adminUser, nil
}
func (r *migratedPublicationRepo) ListSubmissionSlots(context.Context, int64, string) ([]model.SlotRef, error) {
	return append([]model.SlotRef(nil), r.submissions...), r.listErr
}
func (r *migratedPublicationRepo) UpsertSubmission(context.Context, repository.UpsertAvailabilitySubmissionParams) (*model.AvailabilitySubmission, error) {
	return r.upserted, r.listErr
}
func (r *migratedPublicationRepo) DeleteSubmission(context.Context, repository.DeleteAvailabilitySubmissionParams) error {
	return r.listErr
}
func (r *migratedPublicationRepo) GetSlot(context.Context, int64, int64) (*model.TemplateSlot, error) {
	if r.slot == nil {
		return nil, repository.ErrTemplateSlotNotFound
	}
	return r.slot, nil
}
func (r *migratedPublicationRepo) ListSlotPositions(context.Context, int64) ([]*model.TemplateSlotPosition, error) {
	return r.slotPositions, r.listErr
}
func (r *migratedPublicationRepo) IsUserQualifiedForPosition(context.Context, string, int64) (bool, error) {
	return r.qualified, r.listErr
}
func (r *migratedPublicationRepo) ListQualifiedPublicationSlotPositions(context.Context, int64, string) ([]*model.QualifiedShift, error) {
	return r.qualifiedShifts, r.listErr
}
func (r *migratedPublicationRepo) ListPublicationShifts(context.Context, int64) ([]*model.PublicationShift, error) {
	return r.publicationShifts, r.listErr
}
func (r *migratedPublicationRepo) ListPublicationAssignments(context.Context, int64) ([]*model.AssignmentParticipant, error) {
	return r.publicationAssignments, r.listErr
}
func (r *migratedPublicationRepo) ListAdminAvailabilityEmployees(context.Context, repository.ListAdminAvailabilityEmployeesParams) ([]*repository.AdminAvailabilityEmployee, int, error) {
	return r.adminEmployees, r.adminEmployeeTotal, r.listErr
}
func (r *migratedPublicationRepo) GetAdminAvailabilityUser(context.Context, int64, string) (*model.User, []*model.Position, error) {
	if r.adminUser == nil {
		return nil, nil, repository.ErrUserNotFound
	}
	return r.adminUser, r.adminPositions, nil
}
func (r *migratedPublicationRepo) ReplaceAdminAvailabilitySubmissions(context.Context, repository.ReplaceAdminAvailabilitySubmissionsParams) (*repository.ReplaceAdminAvailabilitySubmissionsResult, error) {
	return r.replaceAvailability, r.listErr
}
func (r *migratedPublicationRepo) UpdatePublicationFields(context.Context, repository.UpdatePublicationFieldsParams) (*model.Publication, error) {
	if r.updatedPublication != nil {
		return r.updatedPublication, r.listErr
	}
	return r.publication, r.listErr
}
func (r *migratedPublicationRepo) CreatePublication(_ context.Context, params repository.CreatePublicationParams) (*model.Publication, error) {
	if r.createPublicationErr != nil {
		return nil, r.createPublicationErr
	}
	if r.createdPublication != nil {
		return r.createdPublication, nil
	}
	return &model.Publication{
		ID:                       10,
		TemplateID:               params.TemplateID,
		Name:                     params.Name,
		Description:              params.Description,
		State:                    params.State,
		SubmissionStartAt:        params.SubmissionStartAt,
		SubmissionEndAt:          params.SubmissionEndAt,
		PlannedActiveFrom:        params.PlannedActiveFrom,
		PlannedActiveUntil:       params.PlannedActiveUntil,
		OvertimeEntryWindowHours: params.OvertimeEntryWindowHours,
		CreatedAt:                params.CreatedAt,
		UpdatedAt:                params.CreatedAt,
	}, nil
}
func (r *migratedPublicationRepo) DeletePublication(context.Context, repository.DeletePublicationParams) error {
	return r.deletePublicationErr
}
func (r *migratedPublicationRepo) CreateAssignment(_ context.Context, params repository.CreateAssignmentParams) (*model.Assignment, error) {
	if r.createAssignmentErr != nil {
		return nil, r.createAssignmentErr
	}
	if r.createdAssignment != nil {
		return r.createdAssignment, nil
	}
	return &model.Assignment{
		ID:            20,
		PublicationID: params.PublicationID,
		UserID:        params.UserID,
		SlotID:        params.SlotID,
		Weekday:       params.Weekday,
		PositionID:    params.PositionID,
		CreatedAt:     params.CreatedAt,
	}, nil
}
func (r *migratedPublicationRepo) DeleteAssignment(context.Context, repository.DeleteAssignmentParams) error {
	return r.deleteAssignmentErr
}
func (r *migratedPublicationRepo) CountAssignmentOverridesByAssignment(context.Context, int64) (int64, error) {
	return r.assignmentOverrides, r.countOverridesErr
}
func (r *migratedPublicationRepo) GetAssignment(context.Context, int64) (*model.Assignment, error) {
	if r.assignment == nil {
		return nil, repository.ErrAssignmentNotFound
	}
	return r.assignment, nil
}
func (r *migratedPublicationRepo) ReplaceAssignments(context.Context, repository.ReplaceAssignmentsParams) error {
	return r.replaceAssignmentsErr
}
func (r *migratedPublicationRepo) ActivatePublication(context.Context, repository.ActivatePublicationParams) (*repository.ActivatePublicationResult, error) {
	if r.activateErr != nil {
		return nil, r.activateErr
	}
	if r.activatedResult != nil {
		return r.activatedResult, nil
	}
	return &repository.ActivatePublicationResult{Publication: r.publication}, nil
}
func (r *migratedPublicationRepo) PublishPublication(context.Context, repository.PublishPublicationParams) (*model.Publication, error) {
	if r.publishErr != nil {
		return nil, r.publishErr
	}
	if r.publishedPublication != nil {
		return r.publishedPublication, nil
	}
	return r.publication, nil
}
func (r *migratedPublicationRepo) GetAssignmentBoardView(context.Context, int64) (map[repository.AssignmentBoardSlotKey]*repository.AssignmentBoardSlotView, error) {
	return r.boardView, r.listErr
}
func (r *migratedPublicationRepo) ListAssignmentBoardEmployees(context.Context, int64) ([]*model.AssignmentBoardEmployee, error) {
	return r.boardEmployees, r.listErr
}
func (r *migratedPublicationRepo) ListAssignmentCandidates(context.Context, int64) ([]*model.AssignmentCandidate, error) {
	return r.assignmentCandidates, r.listErr
}
func (r *migratedPublicationRepo) ListQualifiedUsersForPositions(context.Context, []int64) (map[int64][]*model.AssignmentCandidate, error) {
	return r.qualifiedUsers, r.listErr
}
func (r *migratedPublicationRepo) ListPublicationAssignmentsForWeek(context.Context, int64, time.Time) ([]*model.AssignmentParticipant, error) {
	if r.publicationAssignmentsWeek != nil {
		return r.publicationAssignmentsWeek, r.listErr
	}
	return r.publicationAssignments, r.listErr
}

func TestMigratedPublicationServiceListAvailabilityAndSubmissionPaths(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	pub := serviceTestPublication(model.PublicationStateCollecting)
	pub.SubmissionStartAt = now.Add(-time.Hour)
	pub.SubmissionEndAt = now.Add(time.Hour)
	repo := &migratedPublicationRepo{
		publication:     pub,
		publications:    []*model.Publication{pub},
		listTotal:       1,
		submissions:     []model.SlotRef{{SlotID: 4, Weekday: 1}},
		slot:            &model.TemplateSlot{ID: 4, TemplateID: 2, Weekdays: []int{1}, StartTime: "09:00", EndTime: "10:00"},
		slotPositions:   []*model.TemplateSlotPosition{{PositionID: 3}},
		qualified:       true,
		upserted:        &model.AvailabilitySubmission{ID: 9, PublicationID: 1, UserID: serviceTestUserA, SlotID: 4, Weekday: 1},
		qualifiedShifts: []*model.QualifiedShift{{SlotID: 4, Weekday: 1, StartTime: "09:00", EndTime: "10:00"}},
	}
	service := NewPublicationService(repo, serviceTestClock(now))

	list, err := service.ListPublications(context.Background(), ListPublicationsInput{})
	if err != nil || list.Total != 1 || list.Page != 1 || len(list.Publications) != 1 {
		t.Fatalf("ListPublications() = %#v, %v", list, err)
	}
	slots, err := service.ListAvailabilitySubmissionSlots(context.Background(), 1, serviceTestUserA)
	if err != nil || len(slots) != 1 || slots[0].SlotID != 4 {
		t.Fatalf("ListAvailabilitySubmissionSlots() = %#v, %v", slots, err)
	}
	created, err := service.CreateAvailabilitySubmission(context.Background(), CreateAvailabilitySubmissionInput{PublicationID: 1, UserID: serviceTestUserA, SlotID: 4, Weekday: 1})
	if err != nil || created.ID != 9 {
		t.Fatalf("CreateAvailabilitySubmission() = %#v, %v", created, err)
	}
	qualified, err := service.ListQualifiedPublicationSlotPositions(context.Background(), 1, serviceTestUserA)
	if err != nil || len(qualified) != 1 {
		t.Fatalf("ListQualifiedPublicationSlotPositions() = %#v, %v", qualified, err)
	}
	if err := service.DeleteAvailabilitySubmission(context.Background(), DeleteAvailabilitySubmissionInput{
		PublicationID: 1, UserID: serviceTestUserA, SlotID: 4, Weekday: 1,
	}); err != nil {
		t.Fatalf("DeleteAvailabilitySubmission() error = %v", err)
	}
	if _, err := service.ListAvailabilitySubmissionSlots(context.Background(), 1, "not-a-uuid"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid availability viewer error = %v, want ErrInvalidInput", err)
	}
	pub.State = model.PublicationStatePublished
	if _, err := service.CreateAvailabilitySubmission(context.Background(), CreateAvailabilitySubmissionInput{PublicationID: 1, UserID: serviceTestUserA, SlotID: 4, Weekday: 1}); !errors.Is(err, ErrPublicationNotCollecting) {
		t.Fatalf("non-collecting submission error = %v, want ErrPublicationNotCollecting", err)
	}
}

func TestMigratedPublicationServiceAdminAvailabilityAndAttendanceSettings(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	pub := serviceTestPublication(model.PublicationStateActive)
	repo := &migratedPublicationRepo{
		publication:        pub,
		adminEmployees:     []*repository.AdminAvailabilityEmployee{{UserID: serviceTestUserA, Name: "Alice"}},
		adminEmployeeTotal: 1,
		adminUser:          &model.User{ID: serviceTestUserA, Name: "Alice", Email: "alice@example.com", Status: model.UserStatusActive},
		adminPositions:     []*model.Position{{ID: 3, Name: "Nurse"}},
		updatedPublication: func() *model.Publication { p := *pub; p.OvertimeEntryWindowHours = 12; return &p }(),
	}
	service := NewPublicationService(repo, serviceTestClock(now))
	board, err := service.ListAdminAvailability(context.Background(), ListAdminAvailabilityInput{PublicationID: 1, Page: 1, PageSize: 10})
	if err != nil || board.Total != 1 || len(board.Employees) != 1 {
		t.Fatalf("ListAdminAvailability() = %#v, %v", board, err)
	}
	detail, err := service.GetAdminAvailabilityDetail(context.Background(), GetAdminAvailabilityDetailInput{PublicationID: 1, UserID: serviceTestUserA})
	if err != nil || detail.User.ID != serviceTestUserA {
		t.Fatalf("GetAdminAvailabilityDetail() = %#v, %v", detail, err)
	}
	if _, err := service.GetAdminAvailabilityDetail(context.Background(), GetAdminAvailabilityDetailInput{PublicationID: 1, UserID: "bad"}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid admin availability user error = %v", err)
	}
	updated, err := NewAttendanceService(nil, repo, serviceTestClock(now)).UpdateAttendanceSettings(context.Background(), UpdateAttendanceSettingsInput{ActorUserID: serviceTestUserA, PublicationID: 1, OvertimeEntryWindowHours: 12})
	if err != nil || updated.OvertimeEntryWindowHours != 12 {
		t.Fatalf("UpdateAttendanceSettings() = %#v, %v", updated, err)
	}
}

type migratedTemplateRepo struct {
	templateRepository
	template  *model.Template
	templates []*model.Template
	created   *model.Template
	updated   *model.Template
	slot      *model.TemplateSlot
	position  *model.TemplateSlotPosition
	listErr   error
}

func (r *migratedTemplateRepo) ListPaginated(context.Context, repository.ListTemplatesParams) ([]*model.Template, int, error) {
	return r.templates, len(r.templates), r.listErr
}
func (r *migratedTemplateRepo) GetByID(context.Context, int64) (*model.Template, error) {
	if r.template == nil {
		return nil, repository.ErrTemplateNotFound
	}
	return r.template, r.listErr
}
func (r *migratedTemplateRepo) Create(context.Context, repository.CreateTemplateParams) (*model.Template, error) {
	return r.created, r.listErr
}
func (r *migratedTemplateRepo) Update(context.Context, repository.UpdateTemplateParams) (*model.Template, error) {
	return r.updated, r.listErr
}
func (r *migratedTemplateRepo) Delete(context.Context, int64) error { return r.listErr }
func (r *migratedTemplateRepo) Clone(context.Context, int64, string) (*model.Template, error) {
	return r.created, r.listErr
}
func (r *migratedTemplateRepo) CreateSlot(context.Context, repository.CreateTemplateSlotParams) (*model.TemplateSlot, error) {
	return r.slot, r.listErr
}
func (r *migratedTemplateRepo) UpdateSlot(context.Context, repository.UpdateTemplateSlotParams) (*model.TemplateSlot, error) {
	return r.slot, r.listErr
}
func (r *migratedTemplateRepo) DeleteSlot(context.Context, int64, int64) error { return r.listErr }
func (r *migratedTemplateRepo) CreateSlotPosition(context.Context, repository.CreateTemplateSlotPositionParams) (*model.TemplateSlotPosition, error) {
	return r.position, r.listErr
}
func (r *migratedTemplateRepo) UpdateSlotPosition(context.Context, repository.UpdateTemplateSlotPositionParams) (*model.TemplateSlotPosition, error) {
	return r.position, r.listErr
}
func (r *migratedTemplateRepo) DeleteSlotPosition(context.Context, int64, int64, int64) error {
	return r.listErr
}
func (r *migratedTemplateRepo) GetByIDPosition(context.Context, int64) (*model.Position, error) {
	return &model.Position{ID: 3, Name: "Nurse"}, nil
}

type migratedPositionLookup struct{}

func (migratedPositionLookup) GetByID(context.Context, int64) (*model.Position, error) {
	return &model.Position{ID: 3, Name: "Nurse"}, nil
}

func TestMigratedPublicationServiceCRUDLifecycleAssignmentAndRoster(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	pub := serviceTestPublication(model.PublicationStateDraft)
	pub.PlannedActiveFrom = now.Add(-7 * 24 * time.Hour)
	pub.PlannedActiveUntil = now.Add(7 * 24 * time.Hour)
	created := &model.Publication{ID: 10, TemplateID: 2, Name: "May rota", State: model.PublicationStateDraft, PlannedActiveFrom: now.Add(time.Hour), PlannedActiveUntil: now.Add(48 * time.Hour)}
	updated := *pub
	updated.Name = "Updated rota"
	repo := &migratedPublicationRepo{
		publication:                pub,
		createdPublication:         created,
		updatedPublication:         &updated,
		adminUser:                  &model.User{ID: serviceTestUserA, Name: "Alice", Email: "alice@example.com", Status: model.UserStatusActive},
		slot:                       &model.TemplateSlot{ID: 4, TemplateID: 2, Weekdays: []int{5}, StartTime: "09:00", EndTime: "10:00"},
		slotPositions:              []*model.TemplateSlotPosition{{PositionID: 3, RequiredHeadcount: 1, AttendanceResponsible: true}},
		assignment:                 &model.Assignment{ID: 20, PublicationID: 1, UserID: serviceTestUserA, SlotID: 4, Weekday: 5, PositionID: 3},
		publicationShifts:          []*model.PublicationShift{{ID: 30, SlotID: 4, TemplateID: 2, Weekday: 5, StartTime: "09:00", EndTime: "10:00", PositionID: 3, PositionName: "Nurse", RequiredHeadcount: 1}},
		publicationAssignmentsWeek: []*model.AssignmentParticipant{{AssignmentID: 20, SlotID: 4, Weekday: 5, PositionID: 3, UserID: serviceTestUserA, Name: "Alice", Email: "alice@example.com"}},
	}
	service := NewPublicationService(repo, serviceTestClock(now))

	createdResult, err := service.CreatePublication(context.Background(), CreatePublicationInput{
		TemplateID:         2,
		Name:               " May rota ",
		Description:        " weekly ",
		SubmissionStartAt:  now.Add(-48 * time.Hour),
		SubmissionEndAt:    now.Add(-24 * time.Hour),
		PlannedActiveFrom:  now.Add(time.Hour),
		PlannedActiveUntil: now.Add(48 * time.Hour),
	})
	if err != nil || createdResult.ID != 10 {
		t.Fatalf("CreatePublication() = %#v, %v", createdResult, err)
	}
	if got, err := service.UpdatePublication(context.Background(), UpdatePublicationInput{ID: 1, Name: stringPtr(" Updated rota ")}); err != nil || got.Name != "Updated rota" {
		t.Fatalf("UpdatePublication() = %#v, %v", got, err)
	}
	if got, err := service.GetPublicationByID(context.Background(), 1); err != nil || got.ID != 1 {
		t.Fatalf("GetPublicationByID() = %#v, %v", got, err)
	}
	if got, err := service.GetCurrentPublication(context.Background()); err != nil || got.ID != 1 {
		t.Fatalf("GetCurrentPublication() = %#v, %v", got, err)
	}
	if err := service.DeletePublication(context.Background(), 1); err != nil {
		t.Fatalf("DeletePublication() error = %v", err)
	}

	pub.SubmissionEndAt = now.Add(-time.Hour)
	pub.State = model.PublicationStateAssigning
	repo.publishedPublication = func() *model.Publication { p := *pub; p.State = model.PublicationStatePublished; return &p }()
	if result, err := service.PublishPublication(context.Background(), 1); err != nil || result.State != model.PublicationStatePublished {
		t.Fatalf("PublishPublication() = %#v, %v", result, err)
	}
	pub.State = model.PublicationStatePublished
	repo.activatedResult = &repository.ActivatePublicationResult{Publication: func() *model.Publication { p := *pub; p.State = model.PublicationStateActive; return &p }()}
	if result, err := service.ActivatePublication(context.Background(), 1); err != nil || result.State != model.PublicationStateActive {
		t.Fatalf("ActivatePublication() = %#v, %v", result, err)
	}
	pub.State = model.PublicationStateActive
	ended := *pub
	ended.PlannedActiveUntil = now
	repo.updatedPublication = &ended
	if result, err := service.EndPublication(context.Background(), 1); err != nil || result.State != model.PublicationStateEnded {
		t.Fatalf("EndPublication() = %#v, %v", result, err)
	}

	pub.State = model.PublicationStateAssigning
	if assignment, err := service.CreateAssignment(context.Background(), CreateAssignmentInput{PublicationID: 1, UserID: serviceTestUserA, SlotID: 4, Weekday: 5, PositionID: 3}); err != nil || assignment.UserID != serviceTestUserA {
		t.Fatalf("CreateAssignment() = %#v, %v", assignment, err)
	}
	if err := service.DeleteAssignment(context.Background(), DeleteAssignmentInput{PublicationID: 1, AssignmentID: 20}); err != nil {
		t.Fatalf("DeleteAssignment() error = %v", err)
	}

	pub.State = model.PublicationStatePublished
	boardSlot := &repository.AssignmentBoardSlotView{
		Slot: &model.TemplateSlot{ID: 4, TemplateID: 2, Weekdays: []int{5}, StartTime: "09:00", EndTime: "10:00"},
		Positions: map[int64]*repository.AssignmentBoardPositionView{
			3: {Position: &model.Position{ID: 3, Name: "Nurse"}, RequiredHeadcount: 1},
		},
	}
	repo.boardView = map[repository.AssignmentBoardSlotKey]*repository.AssignmentBoardSlotView{{SlotID: 4, Weekday: 5}: boardSlot}
	repo.boardEmployees = []*model.AssignmentBoardEmployee{{UserID: serviceTestUserA, Name: "Alice", Email: "alice@example.com", PositionIDs: []int64{3}}}
	board, err := service.GetAssignmentBoard(context.Background(), 1)
	if err != nil || len(board.Slots) != 1 || len(board.Employees) != 1 {
		t.Fatalf("GetAssignmentBoard() = %#v, %v", board, err)
	}
	pub.State = model.PublicationStateAssigning
	repo.assignmentCandidates = []*model.AssignmentCandidate{{SlotID: 4, Weekday: 5, PositionID: 3, UserID: serviceTestUserA}}
	if autoBoard, err := service.AutoAssignPublication(context.Background(), 1); err != nil || len(autoBoard.Slots) != 1 {
		t.Fatalf("AutoAssignPublication() = %#v, %v", autoBoard, err)
	}
	pub.State = model.PublicationStatePublished
	viewer := &model.User{ID: serviceTestUserA, IsAdmin: true}
	if data, err := service.ExportScheduleXLSX(context.Background(), 1, viewer, ExportScheduleOptions{Language: "en"}); err != nil || len(data) == 0 {
		t.Fatalf("ExportScheduleXLSX() = %d bytes, %v", len(data), err)
	}
	if _, err := service.ExportScheduleXLSX(context.Background(), 1, viewer, ExportScheduleOptions{Language: "fr"}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid export language error = %v", err)
	}

	pub.State = model.PublicationStateActive
	roster, err := service.GetPublicationRoster(context.Background(), 1, func() *time.Time { d := now.AddDate(0, 0, -4); return &d }())
	if err != nil || len(roster.Weekdays) != 7 || len(roster.Weekdays[4].Slots) != 1 {
		t.Fatalf("GetPublicationRoster() = %#v, %v", roster, err)
	}
	currentRoster, err := service.GetCurrentRoster(context.Background())
	if err != nil || currentRoster.Publication == nil {
		t.Fatalf("GetCurrentRoster() = %#v, %v", currentRoster, err)
	}
}

func TestMigratedPublicationServiceRejectsInvalidWritesAndLifecycleStates(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	pub := serviceTestPublication(model.PublicationStateDraft)
	repo := &migratedPublicationRepo{publication: pub}
	service := NewPublicationService(repo, serviceTestClock(now))
	if _, err := service.CreatePublication(context.Background(), CreatePublicationInput{TemplateID: 0}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid publication create error = %v", err)
	}
	if _, err := service.CreatePublication(context.Background(), CreatePublicationInput{
		TemplateID: 2, Name: "bad window", SubmissionStartAt: now, SubmissionEndAt: now,
		PlannedActiveFrom: now.Add(time.Hour), PlannedActiveUntil: now.Add(2 * time.Hour),
	}); !errors.Is(err, ErrInvalidPublicationWindow) {
		t.Fatalf("invalid publication window error = %v", err)
	}
	if _, err := service.GetPublicationByID(context.Background(), 0); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid publication get error = %v", err)
	}
	if err := service.DeletePublication(context.Background(), 0); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid publication delete error = %v", err)
	}
	if _, err := service.PublishPublication(context.Background(), 1); !errors.Is(err, ErrPublicationNotAssigning) {
		t.Fatalf("invalid publish state error = %v", err)
	}
	if _, err := service.ActivatePublication(context.Background(), 1); !errors.Is(err, ErrPublicationNotPublished) {
		t.Fatalf("invalid activate state error = %v", err)
	}
	if _, err := service.GetAssignmentBoard(context.Background(), 1); !errors.Is(err, ErrPublicationNotAssigning) {
		t.Fatalf("invalid assignment board state error = %v", err)
	}
}

func stringPtr(value string) *string { return &value }

func TestMigratedTemplateServiceCRUDAndValidation(t *testing.T) {
	template := &model.Template{ID: 2, Name: "Weekly", Description: "Old"}
	repo := &migratedTemplateRepo{
		template: template, templates: []*model.Template{template},
		created: &model.Template{ID: 3, Name: "New"}, updated: &model.Template{ID: 2, Name: "Updated"},
		slot:     &model.TemplateSlot{ID: 4, TemplateID: 2, Weekdays: []int{1}, StartTime: "09:00", EndTime: "10:00"},
		position: &model.TemplateSlotPosition{ID: 5, SlotID: 4, PositionID: 3, RequiredHeadcount: 1},
	}
	service := NewTemplateService(repo, migratedPositionLookup{})
	if result, err := service.ListTemplates(context.Background(), ListTemplatesInput{}); err != nil || result.Total != 1 {
		t.Fatalf("ListTemplates() = %#v, %v", result, err)
	}
	if created, err := service.CreateTemplate(context.Background(), CreateTemplateInput{Name: " New ", Description: " desc "}); err != nil || created.ID != 3 {
		t.Fatalf("CreateTemplate() = %#v, %v", created, err)
	}
	if got, err := service.GetTemplateByID(context.Background(), 2); err != nil || got.ID != 2 {
		t.Fatalf("GetTemplateByID() = %#v, %v", got, err)
	}
	if _, err := service.CreateTemplate(context.Background(), CreateTemplateInput{Name: "   "}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("blank template error = %v", err)
	}
	if _, err := service.GetTemplateByID(context.Background(), 0); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid template id error = %v", err)
	}
}

func TestMigratedTemplateServiceSlotsCloneAndPositionWrites(t *testing.T) {
	repo := &migratedTemplateRepo{
		template: &model.Template{ID: 2, Name: "Weekly", Slots: []*model.TemplateSlot{{ID: 1}, {ID: 2}}},
		created:  &model.Template{ID: 3, Name: "Weekly (copy)"},
		slot:     &model.TemplateSlot{ID: 4, TemplateID: 2, Weekdays: []int{1, 2}, StartTime: "09:00", EndTime: "10:00"},
		position: &model.TemplateSlotPosition{ID: 5, SlotID: 4, PositionID: 3, RequiredHeadcount: 1, AttendanceResponsible: true},
	}
	service := NewTemplateService(repo, migratedPositionLookup{})
	if cloned, err := service.CloneTemplate(context.Background(), 2); err != nil || cloned.ID != 3 {
		t.Fatalf("CloneTemplate() = %#v, %v", cloned, err)
	}
	if slot, err := service.CreateTemplateSlot(context.Background(), CreateTemplateSlotInput{TemplateID: 2, Weekdays: []int{2, 1, 2}, StartTime: "09:00", EndTime: "10:00"}); err != nil || slot.ID != 4 {
		t.Fatalf("CreateTemplateSlot() = %#v, %v", slot, err)
	}
	if slot, err := service.UpdateTemplateSlot(context.Background(), UpdateTemplateSlotInput{TemplateID: 2, SlotID: 4, Weekdays: []int{1}, StartTime: "10:00", EndTime: "11:00"}); err != nil || slot.ID != 4 {
		t.Fatalf("UpdateTemplateSlot() = %#v, %v", slot, err)
	}
	if err := service.DeleteTemplateSlot(context.Background(), 2, 4); err != nil {
		t.Fatalf("DeleteTemplateSlot() error = %v", err)
	}
	if position, err := service.CreateTemplateSlotPosition(context.Background(), CreateTemplateSlotPositionInput{TemplateID: 2, SlotID: 4, PositionID: 3, RequiredHeadcount: 1, AttendanceResponsible: true}); err != nil || position.ID != 5 {
		t.Fatalf("CreateTemplateSlotPosition() = %#v, %v", position, err)
	}
	if position, err := service.UpdateTemplateSlotPosition(context.Background(), UpdateTemplateSlotPositionInput{TemplateID: 2, SlotID: 4, SlotPositionID: 5, PositionID: 3, RequiredHeadcount: 1}); err != nil || position.ID != 5 {
		t.Fatalf("UpdateTemplateSlotPosition() = %#v, %v", position, err)
	}
	if err := service.DeleteTemplateSlotPosition(context.Background(), 2, 4, 5); err != nil {
		t.Fatalf("DeleteTemplateSlotPosition() error = %v", err)
	}
	if _, err := service.CreateTemplateSlot(context.Background(), CreateTemplateSlotInput{TemplateID: 2, Weekdays: []int{0}, StartTime: "09:00", EndTime: "10:00"}); !errors.Is(err, ErrInvalidWeekday) {
		t.Fatalf("invalid slot weekday error = %v", err)
	}
	if _, err := service.CreateTemplateSlotPosition(context.Background(), CreateTemplateSlotPositionInput{TemplateID: 2, SlotID: 4, PositionID: 3, RequiredHeadcount: 2, AttendanceResponsible: true}); !errors.Is(err, ErrAttendanceResponsibleRequired) {
		t.Fatalf("invalid responsible headcount error = %v", err)
	}
}

type migratedShiftRepo struct {
	shiftChangeRepository
	request     *model.ShiftChangeRequest
	rows        []*model.ShiftChangeRequest
	count       int
	getErr      error
	updateErr   error
	created     *model.ShiftChangeRequest
	createErr   error
	applyResult *repository.ApproveResult
	applyErr    error
}

func (r *migratedShiftRepo) Create(context.Context, repository.CreateShiftChangeRequestParams) (*model.ShiftChangeRequest, error) {
	if r.createErr != nil {
		return nil, r.createErr
	}
	return r.created, nil
}
func (r *migratedShiftRepo) GetByID(context.Context, int64) (*model.ShiftChangeRequest, error) {
	return r.request, r.getErr
}
func (r *migratedShiftRepo) ApplySwap(context.Context, repository.ApplySwapParams) (*repository.ApproveResult, error) {
	return r.applyResult, r.applyErr
}
func (r *migratedShiftRepo) ApplyGive(context.Context, repository.ApplyGiveParams) (*repository.ApproveResult, error) {
	return r.applyResult, r.applyErr
}
func (r *migratedShiftRepo) ListForPublication(context.Context, repository.ListForPublicationParams) ([]*model.ShiftChangeRequest, error) {
	return r.rows, r.getErr
}
func (r *migratedShiftRepo) CountPendingForCounterpart(context.Context, string, time.Time) (int, error) {
	return r.count, r.getErr
}
func (r *migratedShiftRepo) UpdateState(context.Context, repository.UpdateStateParams) error {
	return r.updateErr
}
func (r *migratedShiftRepo) MarkExpired(context.Context, int64, time.Time) error     { return nil }
func (r *migratedShiftRepo) MarkInvalidated(context.Context, int64, time.Time) error { return nil }

func TestMigratedShiftChangeServiceReadAndOwnershipBoundaries(t *testing.T) {
	now := time.Now().UTC().Add(time.Hour)
	counterpart := serviceTestUserB
	request := &model.ShiftChangeRequest{ID: 5, PublicationID: 1, Type: model.ShiftChangeTypeGiveDirect, RequesterUserID: serviceTestUserA, CounterpartUserID: &counterpart, State: model.ShiftChangeStatePending, ExpiresAt: now}
	repo := &migratedShiftRepo{request: request, rows: []*model.ShiftChangeRequest{request}, count: 2}
	pubRepo := &migratedPublicationRepo{publication: serviceTestPublication(model.PublicationStatePublished)}
	service := NewShiftChangeService(repo, pubRepo, nil, "https://app.example.com", serviceTestClock(time.Now().UTC()), nil)
	got, err := service.GetShiftChangeRequest(context.Background(), 5, serviceTestUserA, false)
	if err != nil || got.ID != 5 {
		t.Fatalf("GetShiftChangeRequest() = %#v, %v", got, err)
	}
	rows, err := service.ListShiftChangeRequests(context.Background(), 1, serviceTestUserA, false)
	if err != nil || len(rows) != 1 {
		t.Fatalf("ListShiftChangeRequests() = %#v, %v", rows, err)
	}
	count, err := service.CountPendingForViewer(context.Background(), serviceTestUserB)
	if err != nil || count != 2 {
		t.Fatalf("CountPendingForViewer() = %d, %v", count, err)
	}
	if _, err := service.GetShiftChangeRequest(context.Background(), 5, "019535d9-3df7-79fb-b466-fa907fa17f91", false); !errors.Is(err, ErrShiftChangeNotFound) {
		t.Fatalf("private request error = %v", err)
	}
	if _, err := service.CountPendingForViewer(context.Background(), "bad"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid shift viewer error = %v", err)
	}
}

func TestMigratedShiftChangeServiceCreateDecideApproveAndMembers(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	pub := serviceTestPublication(model.PublicationStatePublished)
	pub.PlannedActiveFrom = now.Add(-24 * time.Hour)
	pub.PlannedActiveUntil = now.Add(7 * 24 * time.Hour)
	assignment := &model.Assignment{ID: 20, PublicationID: 1, UserID: serviceTestUserA, SlotID: 4, Weekday: 1, PositionID: 3}
	request := &model.ShiftChangeRequest{ID: 5, PublicationID: 1, Type: model.ShiftChangeTypeGivePool, RequesterUserID: serviceTestUserA, RequesterAssignmentID: 20, OccurrenceDate: now.AddDate(0, 0, 3), State: model.ShiftChangeStatePending, ExpiresAt: now.Add(24 * time.Hour)}
	repo := &migratedShiftRepo{request: request, rows: []*model.ShiftChangeRequest{request}, count: 1, created: request, applyResult: &repository.ApproveResult{}}
	pubRepo := &migratedPublicationRepo{
		publication:       pub,
		assignment:        assignment,
		publicationShifts: []*model.PublicationShift{{ID: 30, SlotID: 4, TemplateID: 2, Weekday: 1, StartTime: "09:00", EndTime: "10:00", PositionID: 3, PositionName: "Nurse", RequiredHeadcount: 1}},
		publicationAssignments: []*model.AssignmentParticipant{
			{AssignmentID: 20, UserID: serviceTestUserA, Name: "Alice"},
			{AssignmentID: 21, UserID: serviceTestUserB, Name: "Bob"},
		},
		qualified: true,
	}
	service := NewShiftChangeService(repo, pubRepo, nil, "https://app.example.com", serviceTestClock(now), nil)
	created, err := service.CreateShiftChangeRequest(context.Background(), CreateShiftChangeInput{
		PublicationID: 1, RequesterUserID: serviceTestUserA, Type: model.ShiftChangeTypeGivePool,
		RequesterAssignmentID: 20, OccurrenceDate: request.OccurrenceDate,
	})
	if err != nil || created.ID != 5 {
		t.Fatalf("CreateShiftChangeRequest() = %#v, %v", created, err)
	}
	if err := service.ApproveShiftChangeRequest(context.Background(), 5, serviceTestUserB); err != nil {
		t.Fatalf("ApproveShiftChangeRequest() error = %v", err)
	}
	request.State = model.ShiftChangeStateApproved
	if err := service.CancelShiftChangeRequest(context.Background(), 5, serviceTestUserA); !errors.Is(err, ErrShiftChangeNotPending) {
		t.Fatalf("cancel after approve error = %v, want ErrShiftChangeNotPending", err)
	}

	request.Type = model.ShiftChangeTypeGiveDirect
	counterpart := serviceTestUserB
	request.CounterpartUserID = &counterpart
	request.State = model.ShiftChangeStatePending
	if err := service.RejectShiftChangeRequest(context.Background(), 5, serviceTestUserB); err != nil {
		t.Fatalf("RejectShiftChangeRequest() error = %v", err)
	}
	members, err := service.ListPublicationMembers(context.Background(), 1)
	if err != nil || len(members) != 2 || members[0].UserID != serviceTestUserB {
		t.Fatalf("ListPublicationMembers() = %#v, %v", members, err)
	}
}

func TestMigratedShiftChangeServiceRejectsInvalidCreateAndActions(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	pub := serviceTestPublication(model.PublicationStateDraft)
	repo := &migratedShiftRepo{}
	pubRepo := &migratedPublicationRepo{publication: pub}
	service := NewShiftChangeService(repo, pubRepo, nil, "", serviceTestClock(now), nil)
	if _, err := service.CreateShiftChangeRequest(context.Background(), CreateShiftChangeInput{PublicationID: 0}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid shift-change create error = %v", err)
	}
	if err := service.CancelShiftChangeRequest(context.Background(), 0, serviceTestUserA); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid shift-change cancel error = %v", err)
	}
	if err := service.RejectShiftChangeRequest(context.Background(), 0, serviceTestUserA); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid shift-change reject error = %v", err)
	}
	if err := service.ApproveShiftChangeRequest(context.Background(), 0, serviceTestUserA); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid shift-change approve error = %v", err)
	}
	if _, err := service.ListPublicationMembers(context.Background(), 0); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid member list error = %v", err)
	}
}

type migratedLeaveRepo struct {
	leaveRepository
	row       *repository.LeaveWithRequest
	rows      []*repository.LeaveWithRequest
	getErr    error
	created   *model.Leave
	withTxErr error
	insertErr error
}

func (r *migratedLeaveRepo) WithTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	if r.withTxErr != nil {
		return r.withTxErr
	}
	return fn(nil)
}
func (r *migratedLeaveRepo) Insert(_ context.Context, _ *sql.Tx, params repository.InsertLeaveParams) (*model.Leave, error) {
	if r.insertErr != nil {
		return nil, r.insertErr
	}
	if r.created != nil {
		return r.created, nil
	}
	return &model.Leave{ID: 8, UserID: params.UserID, PublicationID: params.PublicationID, ShiftChangeRequestID: params.ShiftChangeRequestID, Category: params.Category, Reason: params.Reason, CreatedAt: params.CreatedAt, UpdatedAt: params.UpdatedAt}, nil
}
func (r *migratedLeaveRepo) GetWithRequestByID(context.Context, int64) (*repository.LeaveWithRequest, error) {
	return r.row, r.getErr
}
func (r *migratedLeaveRepo) GetByID(context.Context, int64) (*model.Leave, *model.ShiftChangeRequest, error) {
	if r.row == nil {
		return nil, nil, r.getErr
	}
	return r.row.Leave, r.row.Request, r.getErr
}
func (r *migratedLeaveRepo) ListForUser(context.Context, string, int, int) ([]*repository.LeaveWithRequest, error) {
	return r.rows, r.getErr
}
func (r *migratedLeaveRepo) ListForPublication(context.Context, int64, int, int) ([]*repository.LeaveWithRequest, error) {
	return r.rows, r.getErr
}
func (r *migratedLeaveRepo) ListPool(context.Context, repository.ListLeavePoolParams) ([]*repository.LeaveWithRequest, int, error) {
	return r.rows, len(r.rows), r.getErr
}
func (r *migratedLeaveRepo) ListActiveOccurrenceKeys(context.Context, string, int64) ([]repository.ActiveLeaveOccurrenceKey, error) {
	return nil, r.getErr
}

type migratedLeaveShiftRepo struct {
	leaveShiftChangeRepository
	request   *model.ShiftChangeRequest
	createErr error
	setErr    error
}

func (r migratedLeaveShiftRepo) CreateTx(context.Context, *sql.Tx, repository.CreateShiftChangeRequestParams) (*model.ShiftChangeRequest, error) {
	if r.createErr != nil {
		return nil, r.createErr
	}
	return r.request, nil
}
func (r migratedLeaveShiftRepo) SetLeaveIDTx(context.Context, *sql.Tx, int64, int64) (*model.ShiftChangeRequest, error) {
	if r.setErr != nil {
		return nil, r.setErr
	}
	return r.request, nil
}

func TestMigratedLeaveServiceReadPathsAndValidation(t *testing.T) {
	now := time.Now().UTC().Add(time.Hour)
	request := &model.ShiftChangeRequest{ID: 7, PublicationID: 1, RequesterUserID: serviceTestUserA, Type: model.ShiftChangeTypeGivePool, State: model.ShiftChangeStatePending, ExpiresAt: now}
	row := &repository.LeaveWithRequest{Leave: &model.Leave{ID: 8, UserID: serviceTestUserA, PublicationID: 1, ShiftChangeRequestID: 7}, Request: request, RequesterName: "Alice"}
	leaveRepo := &migratedLeaveRepo{row: row, rows: []*repository.LeaveWithRequest{row}}
	shiftRepo := &migratedShiftRepo{}
	shiftService := NewShiftChangeService(shiftRepo, &migratedPublicationRepo{}, nil, "", serviceTestClock(time.Now().UTC()), nil)
	service := NewLeaveService(leaveRepo, migratedLeaveShiftRepo{}, shiftService, &migratedPublicationRepo{}, serviceTestClock(time.Now().UTC()))
	got, err := service.GetByID(context.Background(), 8, serviceTestUserA, false)
	if err != nil || got.Leave.ID != 8 || got.RequesterName != "Alice" {
		t.Fatalf("GetByID() = %#v, %v", got, err)
	}
	list, err := service.ListForUser(context.Background(), serviceTestUserA, ListLeavesInput{})
	if err != nil || len(list) != 1 {
		t.Fatalf("ListForUser() = %#v, %v", list, err)
	}
	pool, err := service.ListPool(context.Background(), serviceTestUserB, false, ListLeavePoolInput{})
	if err != nil || pool.TotalCount != 1 {
		t.Fatalf("ListPool() = %#v, %v", pool, err)
	}
	if _, err := service.ListForUser(context.Background(), "bad", ListLeavesInput{}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid leave viewer error = %v", err)
	}
	if _, err := service.GetByID(context.Background(), 8, serviceTestUserB, false); err != nil {
		t.Fatalf("give-pool detail should be viewable: %v", err)
	}
}

func TestMigratedLeaveServiceCreateAndPreview(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	pub := serviceTestPublication(model.PublicationStateActive)
	pub.PlannedActiveFrom = now.Add(-7 * 24 * time.Hour)
	pub.PlannedActiveUntil = now.Add(7 * 24 * time.Hour)
	assignment := &model.AssignmentParticipant{AssignmentID: 20, SlotID: 4, Weekday: 1, PositionID: 3, UserID: serviceTestUserA, Name: "Alice"}
	shift := &model.PublicationShift{ID: 30, SlotID: 4, TemplateID: 2, Weekday: 1, StartTime: "09:00", EndTime: "10:00", PositionID: 3, PositionName: "Nurse", RequiredHeadcount: 1}
	request := &model.ShiftChangeRequest{ID: 7, PublicationID: 1, Type: model.ShiftChangeTypeGivePool, RequesterUserID: serviceTestUserA, RequesterAssignmentID: 20, OccurrenceDate: now.AddDate(0, 0, 3), State: model.ShiftChangeStatePending, ExpiresAt: now.Add(24 * time.Hour)}
	pubRepo := &migratedPublicationRepo{
		publication:            pub,
		publicationAssignments: []*model.AssignmentParticipant{assignment},
		publicationShifts:      []*model.PublicationShift{shift},
		boardEmployees:         []*model.AssignmentBoardEmployee{{UserID: serviceTestUserA, Name: "Alice", PositionIDs: []int64{3}}, {UserID: serviceTestUserB, Name: "Bob", PositionIDs: []int64{3}}},
		assignment:             &model.Assignment{ID: 20, PublicationID: 1, UserID: serviceTestUserA, SlotID: 4, Weekday: 1, PositionID: 3},
	}
	shiftRepo := &migratedShiftRepo{request: request, created: request}
	shiftService := NewShiftChangeService(shiftRepo, pubRepo, nil, "", serviceTestClock(now), nil)
	leaveRow := &repository.LeaveWithRequest{Leave: &model.Leave{ID: 8, UserID: serviceTestUserA, PublicationID: 1, ShiftChangeRequestID: 7}, Request: request}
	leaveRepo := &migratedLeaveRepo{row: leaveRow, rows: []*repository.LeaveWithRequest{leaveRow}}
	leave := NewLeaveService(leaveRepo, migratedLeaveShiftRepo{request: request}, shiftService, pubRepo, serviceTestClock(now))
	created, err := leave.Create(context.Background(), CreateLeaveInput{UserID: serviceTestUserA, AssignmentID: 20, OccurrenceDate: request.OccurrenceDate, Type: model.ShiftChangeTypeGivePool, Category: model.LeaveCategorySick, Reason: "ill"})
	if err != nil || created.Leave.ID != 8 || created.Request.ID != 7 {
		t.Fatalf("Create() = %#v, %v", created, err)
	}
	if err := leave.Cancel(context.Background(), 8, serviceTestUserA); err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}
	if listed, err := leave.ListForPublication(context.Background(), 1, ListLeavesInput{}); err != nil || len(listed) != 1 {
		t.Fatalf("ListForPublication() = %#v, %v", listed, err)
	}
	preview, err := leave.PreviewOccurrences(context.Background(), serviceTestUserA, now.AddDate(0, 0, -4), now.AddDate(0, 0, 6))
	if err != nil || len(preview) != 1 || len(preview[0].DirectCandidates) != 1 || preview[0].DirectCandidates[0].UserID != serviceTestUserB {
		t.Fatalf("PreviewOccurrences() = %#v, %v", preview, err)
	}
	if _, err := leave.Create(context.Background(), CreateLeaveInput{UserID: "bad", AssignmentID: 20, OccurrenceDate: request.OccurrenceDate, Type: model.ShiftChangeTypeGivePool, Category: model.LeaveCategorySick}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid leave create error = %v", err)
	}
	pub.State = model.PublicationStateDraft
	if _, err := leave.PreviewOccurrences(context.Background(), serviceTestUserA, now, now.AddDate(0, 0, 1)); err != nil {
		t.Fatalf("inactive preview should be empty, got %v", err)
	}
}

type migratedAttendanceRepo struct {
	attendanceRepository
	candidateRefs []model.AttendanceShiftRef
	dateRefs      []model.AttendanceShiftRef
	roster        []*model.AttendanceRosterRow
	orphans       []*model.AttendanceRecord
	overtimes     []*model.AttendanceOvertimeRecord
	arrival       *model.AttendanceRecord
	overtime      *model.AttendanceOvertimeRecord
	err           error
}

func (r migratedAttendanceRepo) ListLeaderCandidateShifts(context.Context, int64, string, time.Time, time.Time) ([]model.AttendanceShiftRef, error) {
	return r.candidateRefs, r.err
}
func (r migratedAttendanceRepo) ListPublicationShiftRefsForDate(context.Context, int64, time.Time) ([]model.AttendanceShiftRef, error) {
	return r.dateRefs, r.err
}
func (r migratedAttendanceRepo) ListShiftRoster(context.Context, int64, int64, int, time.Time) ([]*model.AttendanceRosterRow, error) {
	return r.roster, r.err
}
func (r migratedAttendanceRepo) ListOrphanArrivalRecords(context.Context, int64, int64, int, time.Time) ([]*model.AttendanceRecord, error) {
	return r.orphans, r.err
}
func (r migratedAttendanceRepo) ListOvertimeRecords(context.Context, int64, int64, int, time.Time) ([]*model.AttendanceOvertimeRecord, error) {
	return r.overtimes, r.err
}
func (r migratedAttendanceRepo) InsertLeaderArrival(context.Context, repository.UpsertAttendanceArrivalParams) (*model.AttendanceRecord, error) {
	return r.arrival, r.err
}
func (r migratedAttendanceRepo) UpsertAdminArrival(context.Context, repository.UpsertAttendanceArrivalParams) (*model.AttendanceRecord, error) {
	return r.arrival, r.err
}
func (r migratedAttendanceRepo) DeleteArrival(context.Context, int64, int64) (*model.AttendanceRecord, error) {
	return r.arrival, r.err
}
func (r migratedAttendanceRepo) CreateOvertime(context.Context, repository.CreateOvertimeRecordParams) (*model.AttendanceOvertimeRecord, error) {
	return r.overtime, r.err
}
func (r migratedAttendanceRepo) GetOvertime(context.Context, int64, int64) (*model.AttendanceOvertimeRecord, error) {
	return r.overtime, r.err
}
func (r migratedAttendanceRepo) UpdateOvertime(context.Context, repository.UpdateOvertimeRecordParams) (*model.AttendanceOvertimeRecord, error) {
	return r.overtime, r.err
}
func (r migratedAttendanceRepo) DeleteOvertime(context.Context, int64, int64) (*model.AttendanceOvertimeRecord, error) {
	return r.overtime, r.err
}

func TestMigratedAttendanceServiceLeaderAndAdminWorkflows(t *testing.T) {
	now := time.Date(2026, 5, 1, 9, 30, 0, 0, time.UTC)
	pub := serviceTestPublication(model.PublicationStateActive)
	pub.PlannedActiveFrom = now.Add(-24 * time.Hour)
	pub.PlannedActiveUntil = now.Add(24 * time.Hour)
	slot := &model.TemplateSlot{ID: 4, TemplateID: 2, Weekdays: []int{5}, StartTime: "09:00", EndTime: "10:00"}
	arrival := &model.AttendanceRecord{ID: 50, PublicationID: 1, AssignmentID: 20, OccurrenceDate: now, UserID: serviceTestUserA, ArrivedAt: now, RecordedAt: now}
	overtime := &model.AttendanceOvertimeRecord{ID: 60, PublicationID: 1, SlotID: 4, Weekday: 5, OccurrenceDate: now, UserID: serviceTestUserA, Hours: 1.5, Note: "handover"}
	repo := migratedAttendanceRepo{
		candidateRefs: []model.AttendanceShiftRef{{SlotID: 4, Weekday: 5, StartTime: "09:00", EndTime: "10:00", OccurrenceDate: now}},
		dateRefs:      []model.AttendanceShiftRef{{SlotID: 4, Weekday: 5, StartTime: "09:00", EndTime: "10:00", OccurrenceDate: now}},
		roster:        []*model.AttendanceRosterRow{{AssignmentID: 20, SlotID: 4, Weekday: 5, PositionID: 3, PositionName: "Nurse", AttendanceResponsible: true, UserID: serviceTestUserA, UserName: "Alice", UserEmail: "alice@example.com"}},
		arrival:       arrival,
		overtime:      overtime,
	}
	pubRepo := &migratedPublicationRepo{
		publication:        pub,
		slot:               slot,
		slotPositions:      []*model.TemplateSlotPosition{{PositionID: 3, RequiredHeadcount: 1, AttendanceResponsible: true}},
		adminUser:          &model.User{ID: serviceTestUserA, Name: "Alice", Email: "alice@example.com", Status: model.UserStatusActive},
		updatedPublication: func() *model.Publication { updated := *pub; updated.OvertimeEntryWindowHours = 12; return &updated }(),
	}
	service := NewAttendanceService(repo, pubRepo, serviceTestClock(now))
	current, err := service.ListCurrentAttendance(context.Background(), serviceTestUserA)
	if err != nil || len(current.Shifts) != 1 || !current.Shifts[0].ArrivalWindowOpen {
		t.Fatalf("ListCurrentAttendance() = %#v, %v", current, err)
	}
	arrivalDetail, err := service.RecordLeaderArrival(context.Background(), RecordLeaderArrivalInput{ActorUserID: serviceTestUserA, PublicationID: 1, SlotID: 4, AssignmentID: 20, OccurrenceDate: now, UserID: serviceTestUserA})
	if err != nil || arrivalDetail.SlotID != 4 {
		t.Fatalf("RecordLeaderArrival() = %#v, %v", arrivalDetail, err)
	}
	overtimeResult, err := service.RecordLeaderOvertime(context.Background(), RecordOvertimeInput{ActorUserID: serviceTestUserA, PublicationID: 1, SlotID: 4, OccurrenceDate: now, UserID: serviceTestUserA, Hours: 1.5, Note: " handover "})
	if err != nil || overtimeResult.ID != 60 || overtimeResult.Note != "handover" {
		t.Fatalf("RecordLeaderOvertime() = %#v, %v", overtimeResult, err)
	}
	adminDay, err := service.ListAdminAttendance(context.Background(), ListAdminAttendanceInput{PublicationID: 1, OccurrenceDate: now})
	if err != nil || len(adminDay.Shifts) != 1 || adminDay.Shifts[0].RosterCount != 1 {
		t.Fatalf("ListAdminAttendance() = %#v, %v", adminDay, err)
	}
	if detail, err := service.GetAdminShiftAttendance(context.Background(), GetAdminShiftAttendanceInput{PublicationID: 1, SlotID: 4, OccurrenceDate: now}); err != nil || detail.SlotID != 4 {
		t.Fatalf("GetAdminShiftAttendance() = %#v, %v", detail, err)
	}
	if detail, err := service.AdminUpsertArrival(context.Background(), AdminUpsertArrivalInput{ActorUserID: serviceTestUserB, PublicationID: 1, SlotID: 4, AssignmentID: 20, OccurrenceDate: now, UserID: serviceTestUserA, ArrivedAt: now}); err != nil || detail.SlotID != 4 {
		t.Fatalf("AdminUpsertArrival() = %#v, %v", detail, err)
	}
	if err := service.AdminClearArrival(context.Background(), AdminClearArrivalInput{ActorUserID: serviceTestUserB, PublicationID: 1, RecordID: 50}); err != nil {
		t.Fatalf("AdminClearArrival() error = %v", err)
	}
	if record, err := service.AdminCreateOvertime(context.Background(), RecordOvertimeInput{ActorUserID: serviceTestUserB, PublicationID: 1, SlotID: 4, OccurrenceDate: now, UserID: serviceTestUserA, Hours: 1.5, Note: "handover"}); err != nil || record.ID != 60 {
		t.Fatalf("AdminCreateOvertime() = %#v, %v", record, err)
	}
	if record, err := service.AdminUpdateOvertime(context.Background(), AdminUpdateOvertimeInput{ActorUserID: serviceTestUserB, PublicationID: 1, RecordID: 60, Hours: 2, Note: "extended"}); err != nil || record.ID != 60 {
		t.Fatalf("AdminUpdateOvertime() = %#v, %v", record, err)
	}
	if err := service.AdminDeleteOvertime(context.Background(), AdminClearArrivalInput{ActorUserID: serviceTestUserB, PublicationID: 1, RecordID: 60}); err != nil {
		t.Fatalf("AdminDeleteOvertime() error = %v", err)
	}
	if updated, err := service.UpdateAttendanceSettings(context.Background(), UpdateAttendanceSettingsInput{ActorUserID: serviceTestUserB, PublicationID: 1, OvertimeEntryWindowHours: 12}); err != nil || updated.OvertimeEntryWindowHours != 12 {
		t.Fatalf("UpdateAttendanceSettings() = %#v, %v", updated, err)
	}
}

func TestMigratedAttendanceServiceRejectsStaleAndUnauthorizedWrites(t *testing.T) {
	now := time.Date(2026, 5, 1, 9, 30, 0, 0, time.UTC)
	pub := serviceTestPublication(model.PublicationStateActive)
	pub.PlannedActiveFrom = now.Add(-24 * time.Hour)
	pub.PlannedActiveUntil = now.Add(24 * time.Hour)
	pubRepo := &migratedPublicationRepo{publication: pub, slot: &model.TemplateSlot{ID: 4, TemplateID: 2, Weekdays: []int{5}, StartTime: "09:00", EndTime: "10:00"}, slotPositions: []*model.TemplateSlotPosition{{PositionID: 3, RequiredHeadcount: 1, AttendanceResponsible: true}}}
	repo := migratedAttendanceRepo{roster: []*model.AttendanceRosterRow{{AssignmentID: 20, PositionID: 3, PositionName: "Nurse", AttendanceResponsible: true, UserID: serviceTestUserA}}, candidateRefs: []model.AttendanceShiftRef{{SlotID: 4, OccurrenceDate: now}}}
	service := NewAttendanceService(repo, pubRepo, serviceTestClock(now))
	if _, err := service.RecordLeaderArrival(context.Background(), RecordLeaderArrivalInput{ActorUserID: serviceTestUserB, PublicationID: 1, SlotID: 4, AssignmentID: 20, OccurrenceDate: now, UserID: serviceTestUserA}); !errors.Is(err, ErrAttendanceNotLeader) {
		t.Fatalf("non-leader arrival error = %v", err)
	}
	if _, err := service.RecordLeaderArrival(context.Background(), RecordLeaderArrivalInput{ActorUserID: serviceTestUserA, PublicationID: 1, SlotID: 4, AssignmentID: 999, OccurrenceDate: now, UserID: serviceTestUserA}); !errors.Is(err, ErrAttendanceRosterStale) {
		t.Fatalf("stale arrival error = %v", err)
	}
	if _, err := service.AdminCreateOvertime(context.Background(), RecordOvertimeInput{ActorUserID: serviceTestUserA, PublicationID: 1, SlotID: 4, OccurrenceDate: now, UserID: serviceTestUserA, Hours: 0, Note: ""}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid admin overtime error = %v", err)
	}
}

func TestMigratedAttendanceServiceEmptyAndRejectingPaths(t *testing.T) {
	now := time.Now().UTC()
	pubRepo := &migratedPublicationRepo{publication: nil}
	service := NewAttendanceService(migratedAttendanceRepo{}, pubRepo, serviceTestClock(now))
	result, err := service.ListCurrentAttendance(context.Background(), serviceTestUserA)
	if err != nil || result == nil || len(result.Shifts) != 0 {
		t.Fatalf("ListCurrentAttendance() = %#v, %v", result, err)
	}
	if _, err := service.ListCurrentAttendance(context.Background(), "bad"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid attendance viewer error = %v", err)
	}
	if _, err := service.RecordLeaderArrival(context.Background(), RecordLeaderArrivalInput{ActorUserID: "bad"}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid arrival error = %v", err)
	}
	if _, err := service.RecordLeaderOvertime(context.Background(), RecordOvertimeInput{ActorUserID: serviceTestUserA, PublicationID: 1, SlotID: 1, OccurrenceDate: now, UserID: serviceTestUserB, Hours: 0}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid overtime error = %v", err)
	}
	admin, err := service.ListAdminAttendance(context.Background(), ListAdminAttendanceInput{PublicationID: 1, OccurrenceDate: now})
	if err != nil || admin == nil || len(admin.Shifts) != 0 {
		t.Fatalf("ListAdminAttendance() = %#v, %v", admin, err)
	}
}
