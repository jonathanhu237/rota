package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jonathanhu237/rota/api/internal/rota/model"
	"github.com/jonathanhu237/rota/api/internal/rota/repository"
)

func TestMigratedTemplateServiceUpdateDeleteSuccessAndRejection(t *testing.T) {
	repo := &migratedTemplateRepo{
		template: &model.Template{ID: 2, Name: "Weekly", Description: "old"},
		updated:  &model.Template{ID: 2, Name: "Updated", Description: "new"},
	}
	service := NewTemplateService(repo, migratedPositionLookup{})

	updated, err := service.UpdateTemplate(context.Background(), UpdateTemplateInput{ID: 2, Name: " Updated ", Description: " new "})
	if err != nil || updated.Name != "Updated" {
		t.Fatalf("UpdateTemplate() = %#v, %v", updated, err)
	}
	if err := service.DeleteTemplate(context.Background(), 2); err != nil {
		t.Fatalf("DeleteTemplate() error = %v", err)
	}

	for name, call := range map[string]func() error{
		"update invalid id": func() error {
			_, err := service.UpdateTemplate(context.Background(), UpdateTemplateInput{Name: "name"})
			return err
		},
		"update blank name": func() error {
			_, err := service.UpdateTemplate(context.Background(), UpdateTemplateInput{ID: 2, Name: " "})
			return err
		},
		"delete invalid id": func() error { return service.DeleteTemplate(context.Background(), 0) },
	} {
		t.Run(name, func(t *testing.T) {
			if err := call(); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("error = %v, want ErrInvalidInput", err)
			}
		})
	}
}

func TestMigratedPublicationServiceRejectsEveryAssignmentLifecycleWriteBoundary(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	repo := &migratedPublicationRepo{publication: serviceTestPublication(model.PublicationStateDraft)}
	service := NewPublicationService(repo, serviceTestClock(now))

	calls := map[string]func() error{
		"update": func() error {
			_, err := service.UpdatePublication(context.Background(), UpdatePublicationInput{})
			return err
		},
		"end": func() error {
			_, err := service.EndPublication(context.Background(), 0)
			return err
		},
		"create assignment": func() error {
			_, err := service.CreateAssignment(context.Background(), CreateAssignmentInput{PublicationID: 0})
			return err
		},
		"delete assignment": func() error {
			return service.DeleteAssignment(context.Background(), DeleteAssignmentInput{PublicationID: 0, AssignmentID: 1})
		},
		"auto assign": func() error {
			_, err := service.AutoAssignPublication(context.Background(), 0)
			return err
		},
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			if err := call(); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("error = %v, want ErrInvalidInput", err)
			}
		})
	}

	if _, err := service.GetCurrentRoster(context.Background()); err != nil {
		t.Fatalf("GetCurrentRoster() empty publication error = %v", err)
	}
	repo.getErr = repository.ErrPublicationNotFound
	if _, err := service.GetCurrentRoster(context.Background()); !errors.Is(err, ErrPublicationNotFound) {
		t.Fatalf("GetCurrentRoster() repository error = %v, want ErrPublicationNotFound", err)
	}
}

func TestMigratedPublicationServiceAvailabilityReplacementSuccessAndRejection(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	pub := serviceTestPublication(model.PublicationStateCollecting)
	repo := &migratedPublicationRepo{
		publication:         pub,
		adminUser:           &model.User{ID: serviceTestUserA, Name: "Alice", Status: model.UserStatusActive},
		adminPositions:      []*model.Position{{ID: 3, Name: "Nurse"}},
		replaceAvailability: &repository.ReplaceAdminAvailabilitySubmissionsResult{},
	}
	service := NewPublicationService(repo, serviceTestClock(now))
	result, err := service.ReplaceAdminAvailability(context.Background(), ReplaceAdminAvailabilityInput{
		PublicationID: 1,
		UserID:        serviceTestUserA,
		Submissions:   []model.SlotRef{{SlotID: 4, Weekday: 1}, {SlotID: 4, Weekday: 1}},
	})
	if err != nil || result == nil || result.User.ID != serviceTestUserA {
		t.Fatalf("ReplaceAdminAvailability() = %#v, %v", result, err)
	}
	if _, err := service.ReplaceAdminAvailability(context.Background(), ReplaceAdminAvailabilityInput{
		PublicationID: 1, UserID: serviceTestUserA, Submissions: []model.SlotRef{{SlotID: 4, Weekday: 0}},
	}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid availability replacement error = %v, want ErrInvalidInput", err)
	}
}

func TestMigratedAttendanceServiceRejectsEveryAdminWriteBoundary(t *testing.T) {
	now := time.Date(2026, 5, 1, 9, 30, 0, 0, time.UTC)
	service := NewAttendanceService(migratedAttendanceRepo{}, &migratedPublicationRepo{}, serviceTestClock(now))
	calls := map[string]func() error{
		"admin upsert arrival": func() error {
			_, err := service.AdminUpsertArrival(context.Background(), AdminUpsertArrivalInput{})
			return err
		},
		"admin clear arrival": func() error {
			return service.AdminClearArrival(context.Background(), AdminClearArrivalInput{})
		},
		"admin create overtime": func() error {
			_, err := service.AdminCreateOvertime(context.Background(), RecordOvertimeInput{})
			return err
		},
		"admin update overtime": func() error {
			_, err := service.AdminUpdateOvertime(context.Background(), AdminUpdateOvertimeInput{})
			return err
		},
		"admin delete overtime": func() error {
			return service.AdminDeleteOvertime(context.Background(), AdminClearArrivalInput{})
		},
		"settings": func() error {
			_, err := service.UpdateAttendanceSettings(context.Background(), UpdateAttendanceSettingsInput{})
			return err
		},
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			if err := call(); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("error = %v, want ErrInvalidInput", err)
			}
		})
	}
}

func TestMigratedLeaveAndShiftChangeReadWriteBoundaries(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	pubRepo := &migratedPublicationRepo{publication: serviceTestPublication(model.PublicationStatePublished)}
	shiftService := NewShiftChangeService(&migratedShiftRepo{}, pubRepo, nil, "https://app.example.com", serviceTestClock(now), nil)
	leaveService := NewLeaveService(&migratedLeaveRepo{}, migratedLeaveShiftRepo{}, shiftService, pubRepo, serviceTestClock(now))

	calls := map[string]func() error{
		"leave cancel": func() error { return leaveService.Cancel(context.Background(), 0, serviceTestUserA) },
		"leave get": func() error {
			_, err := leaveService.GetByID(context.Background(), 0, serviceTestUserA, false)
			return err
		},
		"leave publication list": func() error {
			_, err := leaveService.ListForPublication(context.Background(), 0, ListLeavesInput{})
			return err
		},
		"shift get": func() error {
			_, err := shiftService.GetShiftChangeRequest(context.Background(), 0, serviceTestUserA, false)
			return err
		},
		"shift list": func() error {
			_, err := shiftService.ListShiftChangeRequests(context.Background(), 0, serviceTestUserA, false)
			return err
		},
		"shift unread": func() error {
			_, err := shiftService.CountPendingForViewer(context.Background(), "bad")
			return err
		},
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			if err := call(); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("error = %v, want ErrInvalidInput", err)
			}
		})
	}
}
