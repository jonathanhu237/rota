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
		if loaded.ShiftCount != 0 || len(loaded.Slots) != 0 {
			t.Fatalf("expected no slots on new template, got count=%d len=%d", loaded.ShiftCount, len(loaded.Slots))
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

	t.Run("Clone copies slots and slot positions", func(t *testing.T) {
		db := openIntegrationDB(t)
		repo := NewTemplateRepository(db)

		source := seedTemplate(t, db, templateSeed{
			Name:        "Source Template",
			Description: "Source description",
		})
		frontDesk := seedPosition(t, db, positionSeed{Name: "Front Desk"})
		backOffice := seedPosition(t, db, positionSeed{Name: "Back Office"})

		firstSlotID := seedTemplateSlot(t, db, source.ID, 1, "09:05", "12:30")
		secondSlotID := seedTemplateSlot(t, db, source.ID, 3, "13:15", "17:45")
		firstSlotPositionID := seedTemplateSlotPosition(t, db, firstSlotID, frontDesk.ID, 2)
		secondSlotPositionID := seedTemplateSlotPosition(t, db, secondSlotID, backOffice.ID, 1)

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
		if cloned.ShiftCount != 2 || len(cloned.Slots) != 2 {
			t.Fatalf("expected 2 copied slots, got count=%d len=%d", cloned.ShiftCount, len(cloned.Slots))
		}

		assertTemplateSlotEqual(t, cloned.Slots[0], cloned.ID, 1, "09:05", "12:30")
		assertTemplateSlotEqual(t, cloned.Slots[1], cloned.ID, 3, "13:15", "17:45")

		if len(cloned.Slots[0].Positions) != 1 || len(cloned.Slots[1].Positions) != 1 {
			t.Fatalf("expected 1 copied slot position per slot, got %+v", cloned.Slots)
		}

		assertTemplateSlotPositionEqual(t, cloned.Slots[0].Positions[0], cloned.Slots[0].ID, frontDesk.ID, 2)
		assertTemplateSlotPositionEqual(t, cloned.Slots[1].Positions[0], cloned.Slots[1].ID, backOffice.ID, 1)

		if cloned.Slots[0].ID == firstSlotID || cloned.Slots[1].ID == secondSlotID {
			t.Fatalf("expected cloned slots to receive new IDs")
		}
		if cloned.Slots[0].Positions[0].ID == firstSlotPositionID || cloned.Slots[1].Positions[0].ID == secondSlotPositionID {
			t.Fatalf("expected cloned slot positions to receive new IDs")
		}

		loadedSource, err := repo.GetByID(ctx, source.ID)
		if err != nil {
			t.Fatalf("reload source template: %v", err)
		}
		if loadedSource.ShiftCount != 2 || len(loadedSource.Slots) != 2 {
			t.Fatalf("expected source template to remain unchanged, got count=%d len=%d", loadedSource.ShiftCount, len(loadedSource.Slots))
		}
	})

	t.Run("Locked templates reject writes", func(t *testing.T) {
		db := openIntegrationDB(t)
		repo := NewTemplateRepository(db)

		locked := seedTemplate(t, db, templateSeed{
			Name:     "Locked Template",
			IsLocked: true,
		})
		slotID := seedTemplateSlot(t, db, locked.ID, 2, "08:00", "10:00")

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

		_, err = repo.CreateSlot(ctx, CreateTemplateSlotParams{
			TemplateID: locked.ID,
			Weekdays:   []int{4},
			StartTime:  "11:00",
			EndTime:    "12:00",
		})
		if !errors.Is(err, ErrTemplateLocked) {
			t.Fatalf("expected ErrTemplateLocked from create slot, got %v", err)
		}

		_, err = repo.UpdateSlot(ctx, UpdateTemplateSlotParams{
			TemplateID: locked.ID,
			SlotID:     slotID,
			Weekdays:   []int{5},
			StartTime:  "11:00",
			EndTime:    "12:00",
		})
		if !errors.Is(err, ErrTemplateLocked) {
			t.Fatalf("expected ErrTemplateLocked from update slot, got %v", err)
		}

		if err := repo.DeleteSlot(ctx, locked.ID, slotID); !errors.Is(err, ErrTemplateLocked) {
			t.Fatalf("expected ErrTemplateLocked from delete slot, got %v", err)
		}
	})
}

func containsTemplateID(templates []*model.Template, id int64) bool {
	for _, template := range templates {
		if template.ID == id {
			return true
		}
	}
	return false
}
