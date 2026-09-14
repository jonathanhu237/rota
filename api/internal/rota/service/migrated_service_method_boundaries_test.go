package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jonathanhu237/rota/api/internal/rota/model"
	"github.com/jonathanhu237/rota/api/internal/rota/repository"
)

// Every public Rota service method has a success-path assertion in the
// migrated service tests and an explicit rejection-path assertion here. Keep
// this matrix close to the service boundary so adding a method without its
// input guard is visible during review.
func TestMigratedServiceMethodsRejectInvalidInputs(t *testing.T) {
	ctx := context.Background()
	clock := serviceTestClock(time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC))

	positionService := NewPositionService(&positionServiceFakeRepo{})
	userPositionService := NewUserPositionService(&userPositionServiceFakeRepo{})
	templateService := NewTemplateService(&migratedTemplateRepo{}, migratedPositionLookup{})
	publicationRepo := &migratedPublicationRepo{}
	publicationService := NewPublicationService(publicationRepo, clock)
	shiftChangeService := NewShiftChangeService(&migratedShiftRepo{}, publicationRepo, nil, "", clock, nil)
	leaveService := NewLeaveService(&migratedLeaveRepo{}, migratedLeaveShiftRepo{}, shiftChangeService, publicationRepo, clock)
	attendanceService := NewAttendanceService(migratedAttendanceRepo{}, publicationRepo, clock)

	cases := map[string]func() error{
		"position list": func() error {
			_, err := positionService.ListPositions(ctx, ListPositionsInput{Page: -1})
			return err
		},
		"position get": func() error {
			_, err := positionService.GetPositionByID(ctx, 0)
			return err
		},
		"position create": func() error {
			_, err := positionService.CreatePosition(ctx, CreatePositionInput{Name: " "})
			return err
		},
		"position update": func() error {
			_, err := positionService.UpdatePosition(ctx, UpdatePositionInput{ID: 0, Name: "Nurse"})
			return err
		},
		"position delete": func() error { return positionService.DeletePosition(ctx, 0) },

		"user positions list": func() error {
			_, err := userPositionService.ListUserPositions(ctx, "not-a-uuid")
			return err
		},
		"user positions replace": func() error {
			return userPositionService.ReplaceUserPositions(ctx, ReplaceUserPositionsInput{UserID: "not-a-uuid"})
		},

		"template list": func() error {
			_, err := templateService.ListTemplates(ctx, ListTemplatesInput{Page: -1})
			return err
		},
		"template create": func() error {
			_, err := templateService.CreateTemplate(ctx, CreateTemplateInput{Name: " "})
			return err
		},
		"template get": func() error {
			_, err := templateService.GetTemplateByID(ctx, 0)
			return err
		},
		"template update": func() error {
			_, err := templateService.UpdateTemplate(ctx, UpdateTemplateInput{ID: 0})
			return err
		},
		"template delete": func() error { return templateService.DeleteTemplate(ctx, 0) },
		"template clone": func() error {
			_, err := templateService.CloneTemplate(ctx, 0)
			return err
		},
		"template slot create": func() error {
			_, err := templateService.CreateTemplateSlot(ctx, CreateTemplateSlotInput{TemplateID: 0})
			return err
		},
		"template slot update": func() error {
			_, err := templateService.UpdateTemplateSlot(ctx, UpdateTemplateSlotInput{TemplateID: 0, SlotID: 1})
			return err
		},
		"template slot delete": func() error { return templateService.DeleteTemplateSlot(ctx, 0, 1) },
		"template slot position create": func() error {
			_, err := templateService.CreateTemplateSlotPosition(ctx, CreateTemplateSlotPositionInput{TemplateID: 0, SlotID: 1, PositionID: 1})
			return err
		},
		"template slot position update": func() error {
			_, err := templateService.UpdateTemplateSlotPosition(ctx, UpdateTemplateSlotPositionInput{TemplateID: 0, SlotID: 1, SlotPositionID: 1, PositionID: 1})
			return err
		},
		"template slot position delete": func() error {
			return templateService.DeleteTemplateSlotPosition(ctx, 0, 1, 1)
		},

		"publication list": func() error {
			_, err := publicationService.ListPublications(ctx, ListPublicationsInput{Page: -1})
			return err
		},
		"publication create": func() error {
			_, err := publicationService.CreatePublication(ctx, CreatePublicationInput{TemplateID: 0})
			return err
		},
		"publication update": func() error {
			_, err := publicationService.UpdatePublication(ctx, UpdatePublicationInput{ID: 0})
			return err
		},
		"publication get": func() error {
			_, err := publicationService.GetPublicationByID(ctx, 0)
			return err
		},
		"current publication": func() error {
			publicationRepo.getErr = repository.ErrPublicationNotFound
			_, err := publicationService.GetCurrentPublication(ctx)
			publicationRepo.getErr = nil
			return err
		},
		"publication delete": func() error { return publicationService.DeletePublication(ctx, 0) },
		"availability slots list": func() error {
			_, err := publicationService.ListAvailabilitySubmissionSlots(ctx, 0, serviceTestUserA)
			return err
		},
		"availability submission create": func() error {
			_, err := publicationService.CreateAvailabilitySubmission(ctx, CreateAvailabilitySubmissionInput{PublicationID: 0})
			return err
		},
		"availability submission delete": func() error {
			return publicationService.DeleteAvailabilitySubmission(ctx, DeleteAvailabilitySubmissionInput{PublicationID: 0})
		},
		"qualified shifts list": func() error {
			_, err := publicationService.ListQualifiedPublicationSlotPositions(ctx, 0, serviceTestUserA)
			return err
		},
		"admin availability list": func() error {
			_, err := publicationService.ListAdminAvailability(ctx, ListAdminAvailabilityInput{PublicationID: 0})
			return err
		},
		"admin availability detail": func() error {
			_, err := publicationService.GetAdminAvailabilityDetail(ctx, GetAdminAvailabilityDetailInput{PublicationID: 0, UserID: serviceTestUserA})
			return err
		},
		"admin availability replace": func() error {
			_, err := publicationService.ReplaceAdminAvailability(ctx, ReplaceAdminAvailabilityInput{PublicationID: 0, UserID: serviceTestUserA})
			return err
		},
		"assignment create": func() error {
			_, err := publicationService.CreateAssignment(ctx, CreateAssignmentInput{PublicationID: 0})
			return err
		},
		"assignment delete": func() error {
			return publicationService.DeleteAssignment(ctx, DeleteAssignmentInput{PublicationID: 0, AssignmentID: 1})
		},
		"publication activate": func() error {
			_, err := publicationService.ActivatePublication(ctx, 0)
			return err
		},
		"publication publish": func() error {
			_, err := publicationService.PublishPublication(ctx, 0)
			return err
		},
		"publication end": func() error {
			_, err := publicationService.EndPublication(ctx, 0)
			return err
		},
		"assignment board": func() error {
			_, err := publicationService.GetAssignmentBoard(ctx, 0)
			return err
		},
		"publication roster": func() error {
			_, err := publicationService.GetPublicationRoster(ctx, 0, nil)
			return err
		},
		"current roster": func() error {
			publicationRepo.getErr = repository.ErrPublicationNotFound
			_, err := publicationService.GetCurrentRoster(ctx)
			publicationRepo.getErr = nil
			return err
		},
		"automatic assignment": func() error {
			_, err := publicationService.AutoAssignPublication(ctx, 0)
			return err
		},
		"schedule export": func() error {
			_, err := publicationService.ExportScheduleXLSX(ctx, 0, &model.User{ID: serviceTestUserA}, ExportScheduleOptions{})
			return err
		},

		"shift change create": func() error {
			_, err := shiftChangeService.CreateShiftChangeRequest(ctx, CreateShiftChangeInput{PublicationID: 0})
			return err
		},
		"shift change get": func() error {
			_, err := shiftChangeService.GetShiftChangeRequest(ctx, 0, serviceTestUserA, false)
			return err
		},
		"shift change list": func() error {
			_, err := shiftChangeService.ListShiftChangeRequests(ctx, 0, serviceTestUserA, false)
			return err
		},
		"shift change count": func() error {
			_, err := shiftChangeService.CountPendingForViewer(ctx, "not-a-uuid")
			return err
		},
		"shift change cancel": func() error {
			return shiftChangeService.CancelShiftChangeRequest(ctx, 0, serviceTestUserA)
		},
		"shift change reject": func() error {
			return shiftChangeService.RejectShiftChangeRequest(ctx, 0, serviceTestUserA)
		},
		"shift change approve": func() error {
			return shiftChangeService.ApproveShiftChangeRequest(ctx, 0, serviceTestUserA)
		},
		"publication members": func() error {
			_, err := shiftChangeService.ListPublicationMembers(ctx, 0)
			return err
		},

		"leave create": func() error {
			_, err := leaveService.Create(ctx, CreateLeaveInput{UserID: "not-a-uuid"})
			return err
		},
		"leave cancel": func() error { return leaveService.Cancel(ctx, 0, serviceTestUserA) },
		"leave get": func() error {
			_, err := leaveService.GetByID(ctx, 0, serviceTestUserA, false)
			return err
		},
		"leave pool list": func() error {
			_, err := leaveService.ListPool(ctx, "not-a-uuid", false, ListLeavePoolInput{})
			return err
		},
		"leave user list": func() error {
			_, err := leaveService.ListForUser(ctx, "not-a-uuid", ListLeavesInput{})
			return err
		},
		"leave publication list": func() error {
			_, err := leaveService.ListForPublication(ctx, 0, ListLeavesInput{})
			return err
		},
		"leave occurrence preview": func() error {
			_, err := leaveService.PreviewOccurrences(ctx, "not-a-uuid", time.Time{}, time.Time{})
			return err
		},

		"attendance current list": func() error {
			_, err := attendanceService.ListCurrentAttendance(ctx, "not-a-uuid")
			return err
		},
		"attendance leader arrival": func() error {
			_, err := attendanceService.RecordLeaderArrival(ctx, RecordLeaderArrivalInput{ActorUserID: "not-a-uuid"})
			return err
		},
		"attendance leader overtime": func() error {
			_, err := attendanceService.RecordLeaderOvertime(ctx, RecordOvertimeInput{ActorUserID: "not-a-uuid"})
			return err
		},
		"attendance admin list": func() error {
			_, err := attendanceService.ListAdminAttendance(ctx, ListAdminAttendanceInput{PublicationID: 0})
			return err
		},
		"attendance admin detail": func() error {
			_, err := attendanceService.GetAdminShiftAttendance(ctx, GetAdminShiftAttendanceInput{PublicationID: 0})
			return err
		},
		"attendance admin upsert": func() error {
			_, err := attendanceService.AdminUpsertArrival(ctx, AdminUpsertArrivalInput{ActorUserID: "not-a-uuid"})
			return err
		},
		"attendance admin clear": func() error {
			return attendanceService.AdminClearArrival(ctx, AdminClearArrivalInput{ActorUserID: "not-a-uuid"})
		},
		"attendance admin create overtime": func() error {
			_, err := attendanceService.AdminCreateOvertime(ctx, RecordOvertimeInput{ActorUserID: "not-a-uuid"})
			return err
		},
		"attendance admin update overtime": func() error {
			_, err := attendanceService.AdminUpdateOvertime(ctx, AdminUpdateOvertimeInput{ActorUserID: "not-a-uuid"})
			return err
		},
		"attendance admin delete overtime": func() error {
			return attendanceService.AdminDeleteOvertime(ctx, AdminClearArrivalInput{ActorUserID: "not-a-uuid"})
		},
		"attendance settings": func() error {
			_, err := attendanceService.UpdateAttendanceSettings(ctx, UpdateAttendanceSettingsInput{ActorUserID: "not-a-uuid"})
			return err
		},
	}

	for name, call := range cases {
		t.Run(name, func(t *testing.T) {
			if err := call(); !errors.Is(err, ErrInvalidInput) && !errors.Is(err, ErrPublicationNotFound) {
				t.Fatalf("error = %v, want an input or missing-current rejection", err)
			}
		})
	}
}
