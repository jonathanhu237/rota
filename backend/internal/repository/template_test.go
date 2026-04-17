//go:build integration

package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/jonathanhu237/rota/backend/internal/model"
)

func TestTemplateRepositoryIntegration(t *testing.T) {
	ctx := context.Background()

	t.Run("Create GetByID Update ListPaginated and Delete round-trip", func(t *testing.T) {
		db := openIntegrationDB(t)
		repo := NewTemplateRepository(db)

		created, err := repo.Create(ctx, CreateTemplateParams{
			Name:        "Weekday Roster",
			Description: "Initial description",
		})
		if err != nil {
			t.Fatalf("create template: %v", err)
		}
		if created.IsLocked {
			t.Fatalf("expected unlocked template, got locked")
		}
		if created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
			t.Fatalf("expected timestamps to be populated: %+v", created)
		}

		loaded, err := repo.GetByID(ctx, created.ID)
		if err != nil {
			t.Fatalf("get template: %v", err)
		}
		if loaded.ID != created.ID || loaded.Name != created.Name || loaded.Description != created.Description {
			t.Fatalf("unexpected loaded template: %+v", loaded)
		}
		if loaded.ShiftCount != 0 || len(loaded.Shifts) != 0 {
			t.Fatalf("expected no shifts on new template, got count=%d len=%d", loaded.ShiftCount, len(loaded.Shifts))
		}

		updated, err := repo.Update(ctx, UpdateTemplateParams{
			ID:          created.ID,
			Name:        "Weekday Roster Updated",
			Description: "Updated description",
		})
		if err != nil {
			t.Fatalf("update template: %v", err)
		}
		if updated.Name != "Weekday Roster Updated" || updated.Description != "Updated description" {
			t.Fatalf("unexpected updated template: %+v", updated)
		}

		other := seedTemplate(t, db, templateSeed{Name: "Secondary Template"})

		templates, total, err := repo.ListPaginated(ctx, ListTemplatesParams{Offset: 0, Limit: 10})
		if err != nil {
			t.Fatalf("list templates: %v", err)
		}
		if total != 2 {
			t.Fatalf("expected total 2, got %d", total)
		}
		if len(templates) != 2 {
			t.Fatalf("expected 2 templates, got %d", len(templates))
		}
		if !containsTemplateID(templates, updated.ID) {
			t.Fatalf("expected list to contain updated template %d, got %+v", updated.ID, templates)
		}
		if !containsTemplateID(templates, other.ID) {
			t.Fatalf("expected list to contain seeded template %d, got %+v", other.ID, templates)
		}

		if err := repo.Delete(ctx, updated.ID); err != nil {
			t.Fatalf("delete template: %v", err)
		}

		_, err = repo.GetByID(ctx, updated.ID)
		if !errors.Is(err, ErrTemplateNotFound) {
			t.Fatalf("expected ErrTemplateNotFound after delete, got %v", err)
		}
	})

	t.Run("Clone copies shifts", func(t *testing.T) {
		db := openIntegrationDB(t)
		repo := NewTemplateRepository(db)

		source := seedTemplate(t, db, templateSeed{
			Name:        "Source Template",
			Description: "Source description",
		})
		frontDesk := seedPosition(t, db, positionSeed{Name: "Front Desk"})
		backOffice := seedPosition(t, db, positionSeed{Name: "Back Office"})

		firstShift := seedTemplateShift(t, db, templateShiftSeed{
			TemplateID:        source.ID,
			Weekday:           1,
			StartTime:         "09:05",
			EndTime:           "12:30",
			PositionID:        frontDesk.ID,
			RequiredHeadcount: 2,
		})
		secondShift := seedTemplateShift(t, db, templateShiftSeed{
			TemplateID:        source.ID,
			Weekday:           3,
			StartTime:         "13:15",
			EndTime:           "17:45",
			PositionID:        backOffice.ID,
			RequiredHeadcount: 1,
		})

		cloned, err := repo.Clone(ctx, source.ID, "Cloned Template")
		if err != nil {
			t.Fatalf("clone template: %v", err)
		}
		if cloned.ID == source.ID {
			t.Fatalf("expected a new template ID, got %d", cloned.ID)
		}
		if cloned.Name != "Cloned Template" || cloned.Description != source.Description {
			t.Fatalf("unexpected cloned template: %+v", cloned)
		}
		if cloned.IsLocked {
			t.Fatalf("expected cloned template to be unlocked")
		}
		if cloned.ShiftCount != 2 || len(cloned.Shifts) != 2 {
			t.Fatalf("expected 2 copied shifts, got count=%d len=%d", cloned.ShiftCount, len(cloned.Shifts))
		}

		assertShiftEqual(t, cloned.Shifts[0], cloned.ID, 1, "09:05", "12:30", frontDesk.ID, 2)
		assertShiftEqual(t, cloned.Shifts[1], cloned.ID, 3, "13:15", "17:45", backOffice.ID, 1)

		if cloned.Shifts[0].ID == firstShift.ID || cloned.Shifts[1].ID == secondShift.ID {
			t.Fatalf("expected cloned shifts to receive new IDs")
		}

		loadedSource, err := repo.GetByID(ctx, source.ID)
		if err != nil {
			t.Fatalf("reload source template: %v", err)
		}
		if loadedSource.ShiftCount != 2 || len(loadedSource.Shifts) != 2 {
			t.Fatalf("expected source template to remain unchanged, got count=%d len=%d", loadedSource.ShiftCount, len(loadedSource.Shifts))
		}
	})

	t.Run("Locked templates reject writes", func(t *testing.T) {
		db := openIntegrationDB(t)
		repo := NewTemplateRepository(db)

		locked := seedTemplate(t, db, templateSeed{
			Name:     "Locked Template",
			IsLocked: true,
		})
		position := seedPosition(t, db, positionSeed{Name: "Locked Position"})
		shift := seedTemplateShift(t, db, templateShiftSeed{
			TemplateID:        locked.ID,
			Weekday:           2,
			StartTime:         "08:00",
			EndTime:           "10:00",
			PositionID:        position.ID,
			RequiredHeadcount: 1,
		})

		_, err := repo.Update(ctx, UpdateTemplateParams{
			ID:          locked.ID,
			Name:        "Unlocked?",
			Description: "Nope",
		})
		if !errors.Is(err, ErrTemplateLocked) {
			t.Fatalf("expected ErrTemplateLocked from update, got %v", err)
		}

		if err := repo.Delete(ctx, locked.ID); !errors.Is(err, ErrTemplateLocked) {
			t.Fatalf("expected ErrTemplateLocked from delete, got %v", err)
		}

		_, err = repo.CreateShift(ctx, CreateTemplateShiftParams{
			TemplateID:        locked.ID,
			Weekday:           4,
			StartTime:         "11:00",
			EndTime:           "12:00",
			PositionID:        position.ID,
			RequiredHeadcount: 1,
		})
		if !errors.Is(err, ErrTemplateLocked) {
			t.Fatalf("expected ErrTemplateLocked from create shift, got %v", err)
		}

		_, err = repo.UpdateShift(ctx, UpdateTemplateShiftParams{
			TemplateID:        locked.ID,
			ShiftID:           shift.ID,
			Weekday:           5,
			StartTime:         "11:00",
			EndTime:           "12:00",
			PositionID:        position.ID,
			RequiredHeadcount: 1,
		})
		if !errors.Is(err, ErrTemplateLocked) {
			t.Fatalf("expected ErrTemplateLocked from update shift, got %v", err)
		}

		if err := repo.DeleteShift(ctx, locked.ID, shift.ID); !errors.Is(err, ErrTemplateLocked) {
			t.Fatalf("expected ErrTemplateLocked from delete shift, got %v", err)
		}
	})

	t.Run("Shift CRUD and TO_CHAR formatting on read", func(t *testing.T) {
		db := openIntegrationDB(t)
		repo := NewTemplateRepository(db)

		template := seedTemplate(t, db, templateSeed{
			Name:        "Shift Template",
			Description: "Shift CRUD",
		})
		position := seedPosition(t, db, positionSeed{Name: "Shift Position"})

		createdShift, err := repo.CreateShift(ctx, CreateTemplateShiftParams{
			TemplateID:        template.ID,
			Weekday:           2,
			StartTime:         "9:05",
			EndTime:           "17:45",
			PositionID:        position.ID,
			RequiredHeadcount: 3,
		})
		if err != nil {
			t.Fatalf("create shift: %v", err)
		}
		assertShiftEqual(t, createdShift, template.ID, 2, "09:05", "17:45", position.ID, 3)

		loaded, err := repo.GetByID(ctx, template.ID)
		if err != nil {
			t.Fatalf("get template with shift: %v", err)
		}
		if loaded.ShiftCount != 1 || len(loaded.Shifts) != 1 {
			t.Fatalf("expected one shift on template, got count=%d len=%d", loaded.ShiftCount, len(loaded.Shifts))
		}
		assertShiftEqual(t, loaded.Shifts[0], template.ID, 2, "09:05", "17:45", position.ID, 3)

		updatedShift, err := repo.UpdateShift(ctx, UpdateTemplateShiftParams{
			TemplateID:        template.ID,
			ShiftID:           createdShift.ID,
			Weekday:           4,
			StartTime:         "10:07",
			EndTime:           "18:30",
			PositionID:        position.ID,
			RequiredHeadcount: 4,
		})
		if err != nil {
			t.Fatalf("update shift: %v", err)
		}
		assertShiftEqual(t, updatedShift, template.ID, 4, "10:07", "18:30", position.ID, 4)

		loaded, err = repo.GetByID(ctx, template.ID)
		if err != nil {
			t.Fatalf("reload template after shift update: %v", err)
		}
		if loaded.ShiftCount != 1 || len(loaded.Shifts) != 1 {
			t.Fatalf("expected one shift after update, got count=%d len=%d", loaded.ShiftCount, len(loaded.Shifts))
		}
		assertShiftEqual(t, loaded.Shifts[0], template.ID, 4, "10:07", "18:30", position.ID, 4)

		if err := repo.DeleteShift(ctx, template.ID, createdShift.ID); err != nil {
			t.Fatalf("delete shift: %v", err)
		}

		loaded, err = repo.GetByID(ctx, template.ID)
		if err != nil {
			t.Fatalf("reload template after shift delete: %v", err)
		}
		if loaded.ShiftCount != 0 || len(loaded.Shifts) != 0 {
			t.Fatalf("expected no shifts after delete, got count=%d len=%d", loaded.ShiftCount, len(loaded.Shifts))
		}
	})
}

func assertShiftEqual(
	t *testing.T,
	got *model.TemplateShift,
	wantTemplateID int64,
	wantWeekday int,
	wantStart string,
	wantEnd string,
	wantPositionID int64,
	wantRequiredHeadcount int,
) {
	t.Helper()

	if got.TemplateID != wantTemplateID {
		t.Fatalf("expected template ID %d, got %d", wantTemplateID, got.TemplateID)
	}
	if got.Weekday != wantWeekday {
		t.Fatalf("expected weekday %d, got %d", wantWeekday, got.Weekday)
	}
	if got.StartTime != wantStart {
		t.Fatalf("expected start time %q, got %q", wantStart, got.StartTime)
	}
	if got.EndTime != wantEnd {
		t.Fatalf("expected end time %q, got %q", wantEnd, got.EndTime)
	}
	if got.PositionID != wantPositionID {
		t.Fatalf("expected position ID %d, got %d", wantPositionID, got.PositionID)
	}
	if got.RequiredHeadcount != wantRequiredHeadcount {
		t.Fatalf("expected headcount %d, got %d", wantRequiredHeadcount, got.RequiredHeadcount)
	}
	if got.CreatedAt.IsZero() || got.UpdatedAt.IsZero() {
		t.Fatalf("expected shift timestamps to be populated: %+v", got)
	}
}

func containsTemplateID(templates []*model.Template, id int64) bool {
	for _, template := range templates {
		if template.ID == id {
			return true
		}
	}
	return false
}
