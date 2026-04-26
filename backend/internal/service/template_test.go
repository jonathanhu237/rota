package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jonathanhu237/rota/backend/internal/audit"
	"github.com/jonathanhu237/rota/backend/internal/audit/audittest"
	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/jonathanhu237/rota/backend/internal/repository"
)

type templateRepositoryMock struct {
	listPaginatedFunc      func(ctx context.Context, params repository.ListTemplatesParams) ([]*model.Template, int, error)
	getByIDFunc            func(ctx context.Context, id int64) (*model.Template, error)
	createFunc             func(ctx context.Context, params repository.CreateTemplateParams) (*model.Template, error)
	updateFunc             func(ctx context.Context, params repository.UpdateTemplateParams) (*model.Template, error)
	deleteFunc             func(ctx context.Context, id int64) error
	cloneFunc              func(ctx context.Context, id int64, name string) (*model.Template, error)
	createSlotFunc         func(ctx context.Context, params repository.CreateTemplateSlotParams) (*model.TemplateSlot, error)
	updateSlotFunc         func(ctx context.Context, params repository.UpdateTemplateSlotParams) (*model.TemplateSlot, error)
	deleteSlotFunc         func(ctx context.Context, templateID, slotID int64) error
	createSlotPositionFunc func(ctx context.Context, params repository.CreateTemplateSlotPositionParams) (*model.TemplateSlotPosition, error)
	updateSlotPositionFunc func(ctx context.Context, params repository.UpdateTemplateSlotPositionParams) (*model.TemplateSlotPosition, error)
	deleteSlotPositionFunc func(ctx context.Context, templateID, slotID, slotPositionID int64) error
}

func (m *templateRepositoryMock) ListPaginated(ctx context.Context, params repository.ListTemplatesParams) ([]*model.Template, int, error) {
	return m.listPaginatedFunc(ctx, params)
}

func (m *templateRepositoryMock) GetByID(ctx context.Context, id int64) (*model.Template, error) {
	if m.getByIDFunc == nil {
		return nil, repository.ErrTemplateNotFound
	}
	return m.getByIDFunc(ctx, id)
}

func (m *templateRepositoryMock) Create(ctx context.Context, params repository.CreateTemplateParams) (*model.Template, error) {
	return m.createFunc(ctx, params)
}

func (m *templateRepositoryMock) Update(ctx context.Context, params repository.UpdateTemplateParams) (*model.Template, error) {
	return m.updateFunc(ctx, params)
}

func (m *templateRepositoryMock) Delete(ctx context.Context, id int64) error {
	return m.deleteFunc(ctx, id)
}

func (m *templateRepositoryMock) Clone(ctx context.Context, id int64, name string) (*model.Template, error) {
	return m.cloneFunc(ctx, id, name)
}

func (m *templateRepositoryMock) CreateSlot(ctx context.Context, params repository.CreateTemplateSlotParams) (*model.TemplateSlot, error) {
	return m.createSlotFunc(ctx, params)
}

func (m *templateRepositoryMock) UpdateSlot(ctx context.Context, params repository.UpdateTemplateSlotParams) (*model.TemplateSlot, error) {
	return m.updateSlotFunc(ctx, params)
}

func (m *templateRepositoryMock) DeleteSlot(ctx context.Context, templateID, slotID int64) error {
	return m.deleteSlotFunc(ctx, templateID, slotID)
}

func (m *templateRepositoryMock) CreateSlotPosition(ctx context.Context, params repository.CreateTemplateSlotPositionParams) (*model.TemplateSlotPosition, error) {
	return m.createSlotPositionFunc(ctx, params)
}

func (m *templateRepositoryMock) UpdateSlotPosition(ctx context.Context, params repository.UpdateTemplateSlotPositionParams) (*model.TemplateSlotPosition, error) {
	return m.updateSlotPositionFunc(ctx, params)
}

func (m *templateRepositoryMock) DeleteSlotPosition(ctx context.Context, templateID, slotID, slotPositionID int64) error {
	return m.deleteSlotPositionFunc(ctx, templateID, slotID, slotPositionID)
}

type positionLookupRepositoryMock struct {
	getByIDFunc func(ctx context.Context, id int64) (*model.Position, error)
}

func (m *positionLookupRepositoryMock) GetByID(ctx context.Context, id int64) (*model.Position, error) {
	return m.getByIDFunc(ctx, id)
}

func TestTemplateServiceListTemplates(t *testing.T) {
	t.Run("success with pagination", func(t *testing.T) {
		t.Parallel()

		var receivedParams repository.ListTemplatesParams
		service := NewTemplateService(
			&templateRepositoryMock{
				listPaginatedFunc: func(ctx context.Context, params repository.ListTemplatesParams) ([]*model.Template, int, error) {
					receivedParams = params
					return []*model.Template{{ID: 1}, {ID: 2}}, 12, nil
				},
			},
			&positionLookupRepositoryMock{},
		)

		result, err := service.ListTemplates(context.Background(), ListTemplatesInput{
			Page:     2,
			PageSize: 5,
		})
		if err != nil {
			t.Fatalf("ListTemplates returned error: %v", err)
		}
		if receivedParams.Offset != 5 || receivedParams.Limit != 5 {
			t.Fatalf("expected offset=5 limit=5, got offset=%d limit=%d", receivedParams.Offset, receivedParams.Limit)
		}
		if result.Page != 2 || result.PageSize != 5 || result.Total != 12 || result.TotalPages != 3 {
			t.Fatalf("unexpected pagination result: %+v", result)
		}
	})
}

func TestTemplateServiceCreateTemplate(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		t.Parallel()

		service := NewTemplateService(
			&templateRepositoryMock{
				createFunc: func(ctx context.Context, params repository.CreateTemplateParams) (*model.Template, error) {
					if params.Name != "Weekday Template" {
						t.Fatalf("expected trimmed name, got %q", params.Name)
					}
					if params.Description != "Covers the core shifts" {
						t.Fatalf("expected trimmed description, got %q", params.Description)
					}

					return &model.Template{
						ID:          1,
						Name:        params.Name,
						Description: params.Description,
					}, nil
				},
			},
			&positionLookupRepositoryMock{},
		)

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())

		template, err := service.CreateTemplate(ctx, CreateTemplateInput{
			Name:        " Weekday Template ",
			Description: " Covers the core shifts ",
		})
		if err != nil {
			t.Fatalf("CreateTemplate returned error: %v", err)
		}
		if template.Name != "Weekday Template" || template.Description != "Covers the core shifts" {
			t.Fatalf("unexpected template: %+v", template)
		}

		event := stub.FindByAction(audit.ActionTemplateCreate)
		if event == nil {
			t.Fatalf("expected %q audit event, got %v", audit.ActionTemplateCreate, stub.Actions())
		}
		if event.TargetType != audit.TargetTypeTemplate {
			t.Fatalf("unexpected target type: %q", event.TargetType)
		}
		if event.TargetID == nil || *event.TargetID != 1 {
			t.Fatalf("unexpected target id: %v", event.TargetID)
		}
		if event.Metadata["name"] != "Weekday Template" {
			t.Fatalf("expected metadata name=%q, got %+v", "Weekday Template", event.Metadata)
		}
	})

	t.Run("empty name returns ErrInvalidInput", func(t *testing.T) {
		t.Parallel()

		service := NewTemplateService(&templateRepositoryMock{}, &positionLookupRepositoryMock{})

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())

		_, err := service.CreateTemplate(ctx, CreateTemplateInput{
			Name:        "   ",
			Description: "",
		})
		if !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("expected ErrInvalidInput, got %v", err)
		}
		if len(stub.Events()) != 0 {
			t.Fatalf("expected no audit events on error, got %v", stub.Actions())
		}
	})

	t.Run("overlong description returns ErrInvalidInput", func(t *testing.T) {
		t.Parallel()

		service := NewTemplateService(&templateRepositoryMock{}, &positionLookupRepositoryMock{})

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())

		_, err := service.CreateTemplate(ctx, CreateTemplateInput{
			Name:        "Weekday",
			Description: strings.Repeat("a", 501),
		})
		if !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("expected ErrInvalidInput, got %v", err)
		}
		if len(stub.Events()) != 0 {
			t.Fatalf("expected no audit events on error, got %v", stub.Actions())
		}
	})

	t.Run("allows a 100-rune Chinese name", func(t *testing.T) {
		t.Parallel()

		chineseName := strings.Repeat("排", 100)
		service := NewTemplateService(
			&templateRepositoryMock{
				createFunc: func(ctx context.Context, params repository.CreateTemplateParams) (*model.Template, error) {
					if params.Name != chineseName {
						t.Fatalf("expected name %q, got %q", chineseName, params.Name)
					}
					return &model.Template{
						ID:          1,
						Name:        params.Name,
						Description: params.Description,
					}, nil
				},
			},
			&positionLookupRepositoryMock{},
		)

		template, err := service.CreateTemplate(context.Background(), CreateTemplateInput{
			Name:        chineseName,
			Description: "中文模板",
		})
		if err != nil {
			t.Fatalf("CreateTemplate returned error: %v", err)
		}
		if template.Name != chineseName {
			t.Fatalf("unexpected template name: %q", template.Name)
		}
	})
}

func TestTemplateServiceGetTemplateByID(t *testing.T) {
	t.Run("sorts slots by start time, end time, and nested position id", func(t *testing.T) {
		t.Parallel()

		service := NewTemplateService(
			&templateRepositoryMock{
				getByIDFunc: func(ctx context.Context, id int64) (*model.Template, error) {
					return &model.Template{
						ID:   id,
						Name: "Weekday",
						Slots: []*model.TemplateSlot{
							{
								ID:        3,
								Weekdays:  []int{3},
								StartTime: "11:00",
								EndTime:   "12:00",
								Positions: []*model.TemplateSlotPosition{{ID: 31, PositionID: 4}, {ID: 32, PositionID: 2}},
							},
							{
								ID:        2,
								Weekdays:  []int{1},
								StartTime: "12:00",
								EndTime:   "13:00",
								Positions: []*model.TemplateSlotPosition{{ID: 21, PositionID: 8}},
							},
							{
								ID:        1,
								Weekdays:  []int{1},
								StartTime: "09:00",
								EndTime:   "10:00",
								Positions: []*model.TemplateSlotPosition{{ID: 12, PositionID: 7}, {ID: 11, PositionID: 3}},
							},
						},
					}, nil
				},
			},
			&positionLookupRepositoryMock{},
		)

		template, err := service.GetTemplateByID(context.Background(), 7)
		if err != nil {
			t.Fatalf("GetTemplateByID returned error: %v", err)
		}

		gotIDs := []int64{
			template.Slots[0].ID,
			template.Slots[1].ID,
			template.Slots[2].ID,
		}
		wantIDs := []int64{1, 3, 2}
		for i := range wantIDs {
			if gotIDs[i] != wantIDs[i] {
				t.Fatalf("expected slot order %v, got %v", wantIDs, gotIDs)
			}
		}
		if template.Slots[0].Positions[0].PositionID != 3 || template.Slots[0].Positions[1].PositionID != 7 {
			t.Fatalf("expected positions to be sorted by position id, got %+v", template.Slots[0].Positions)
		}
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()

		service := NewTemplateService(
			&templateRepositoryMock{
				getByIDFunc: func(ctx context.Context, id int64) (*model.Template, error) {
					return nil, repository.ErrTemplateNotFound
				},
			},
			&positionLookupRepositoryMock{},
		)

		_, err := service.GetTemplateByID(context.Background(), 7)
		if !errors.Is(err, ErrTemplateNotFound) {
			t.Fatalf("expected ErrTemplateNotFound, got %v", err)
		}
	})
}

func TestTemplateServiceUpdateTemplate(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		t.Parallel()

		service := NewTemplateService(
			&templateRepositoryMock{
				getByIDFunc: func(ctx context.Context, id int64) (*model.Template, error) {
					return &model.Template{
						ID:          id,
						Name:        "Old Name",
						Description: "Old description",
						IsLocked:    false,
					}, nil
				},
				updateFunc: func(ctx context.Context, params repository.UpdateTemplateParams) (*model.Template, error) {
					if params.ID != 5 || params.Name != "Weekend" || params.Description != "Updated details" {
						t.Fatalf("unexpected update params: %+v", params)
					}
					return &model.Template{
						ID:          params.ID,
						Name:        params.Name,
						Description: params.Description,
					}, nil
				},
			},
			&positionLookupRepositoryMock{},
		)

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())

		template, err := service.UpdateTemplate(ctx, UpdateTemplateInput{
			ID:          5,
			Name:        " Weekend ",
			Description: " Updated details ",
		})
		if err != nil {
			t.Fatalf("UpdateTemplate returned error: %v", err)
		}
		if template.Name != "Weekend" || template.Description != "Updated details" {
			t.Fatalf("unexpected template: %+v", template)
		}

		event := stub.FindByAction(audit.ActionTemplateUpdate)
		if event == nil {
			t.Fatalf("expected %q audit event, got %v", audit.ActionTemplateUpdate, stub.Actions())
		}
		if event.TargetType != audit.TargetTypeTemplate {
			t.Fatalf("unexpected target type: %q", event.TargetType)
		}
		if event.TargetID == nil || *event.TargetID != 5 {
			t.Fatalf("unexpected target id: %v", event.TargetID)
		}
		if _, ok := event.Metadata["name"]; !ok {
			t.Fatalf("expected name change in metadata, got %+v", event.Metadata)
		}
		if _, ok := event.Metadata["description"]; !ok {
			t.Fatalf("expected description change in metadata, got %+v", event.Metadata)
		}
	})

	t.Run("locked template rejects without writes", func(t *testing.T) {
		t.Parallel()

		updateCalled := false
		service := NewTemplateService(
			&templateRepositoryMock{
				getByIDFunc: func(ctx context.Context, id int64) (*model.Template, error) {
					return &model.Template{ID: id, IsLocked: true}, nil
				},
				updateFunc: func(ctx context.Context, params repository.UpdateTemplateParams) (*model.Template, error) {
					updateCalled = true
					return nil, repository.ErrTemplateLocked
				},
			},
			&positionLookupRepositoryMock{},
		)

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())

		_, err := service.UpdateTemplate(ctx, UpdateTemplateInput{
			ID:          3,
			Name:        "Locked",
			Description: "",
		})
		if !errors.Is(err, ErrTemplateLocked) {
			t.Fatalf("expected ErrTemplateLocked, got %v", err)
		}
		if !updateCalled {
			t.Fatal("expected update to be attempted so repository lock guard can reject it")
		}
		if len(stub.Events()) != 0 {
			t.Fatalf("expected no audit events on error, got %v", stub.Actions())
		}
	})
}

func TestTemplateServiceDeleteTemplate(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		t.Parallel()

		var deletedID int64
		service := NewTemplateService(
			&templateRepositoryMock{
				getByIDFunc: func(ctx context.Context, id int64) (*model.Template, error) {
					return &model.Template{ID: id, Name: "Weekday Template", IsLocked: false}, nil
				},
				deleteFunc: func(ctx context.Context, id int64) error {
					deletedID = id
					return nil
				},
			},
			&positionLookupRepositoryMock{},
		)

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())

		if err := service.DeleteTemplate(ctx, 4); err != nil {
			t.Fatalf("DeleteTemplate returned error: %v", err)
		}
		if deletedID != 4 {
			t.Fatalf("expected delete ID 4, got %d", deletedID)
		}

		event := stub.FindByAction(audit.ActionTemplateDelete)
		if event == nil {
			t.Fatalf("expected %q audit event, got %v", audit.ActionTemplateDelete, stub.Actions())
		}
		if event.TargetType != audit.TargetTypeTemplate {
			t.Fatalf("unexpected target type: %q", event.TargetType)
		}
		if event.TargetID == nil || *event.TargetID != 4 {
			t.Fatalf("unexpected target id: %v", event.TargetID)
		}
		if event.Metadata["name"] != "Weekday Template" {
			t.Fatalf("expected metadata name=%q, got %+v", "Weekday Template", event.Metadata)
		}
	})

	t.Run("locked template rejects", func(t *testing.T) {
		t.Parallel()

		deleteCalled := false
		service := NewTemplateService(
			&templateRepositoryMock{
				getByIDFunc: func(ctx context.Context, id int64) (*model.Template, error) {
					return &model.Template{ID: id, Name: "Locked Template", IsLocked: true}, nil
				},
				deleteFunc: func(ctx context.Context, id int64) error {
					deleteCalled = true
					return repository.ErrTemplateLocked
				},
			},
			&positionLookupRepositoryMock{},
		)

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())

		err := service.DeleteTemplate(ctx, 4)
		if !errors.Is(err, ErrTemplateLocked) {
			t.Fatalf("expected ErrTemplateLocked, got %v", err)
		}
		if !deleteCalled {
			t.Fatal("expected delete to be attempted so repository lock guard can reject it")
		}
		if len(stub.Events()) != 0 {
			t.Fatalf("expected no audit events on error, got %v", stub.Actions())
		}
	})
}

func TestTemplateServiceCloneTemplate(t *testing.T) {
	t.Run("clones unlocked template", func(t *testing.T) {
		t.Parallel()

		service := NewTemplateService(
			&templateRepositoryMock{
				cloneFunc: func(ctx context.Context, id int64, name string) (*model.Template, error) {
					if id != 9 {
						t.Fatalf("expected clone ID 9, got %d", id)
					}
					if name != "Weekday Template (copy)" {
						t.Fatalf("unexpected clone name %q", name)
					}
					return &model.Template{
						ID:       10,
						Name:     name,
						IsLocked: false,
						Slots: []*model.TemplateSlot{
							{ID: 1, Weekdays: []int{1}, Positions: []*model.TemplateSlotPosition{{ID: 2, PositionID: 5}}},
						},
					}, nil
				},
				getByIDFunc: func(ctx context.Context, id int64) (*model.Template, error) {
					return &model.Template{ID: id, Name: "Weekday Template", IsLocked: false}, nil
				},
			},
			&positionLookupRepositoryMock{},
		)

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())

		clone, err := service.CloneTemplate(ctx, 9)
		if err != nil {
			t.Fatalf("CloneTemplate returned error: %v", err)
		}
		if clone.IsLocked {
			t.Fatal("expected clone to be unlocked")
		}
		if len(clone.Slots) != 1 || len(clone.Slots[0].Positions) != 1 {
			t.Fatalf("expected slots to be cloned, got %+v", clone.Slots)
		}

		event := stub.FindByAction(audit.ActionTemplateClone)
		if event == nil {
			t.Fatalf("expected %q audit event, got %v", audit.ActionTemplateClone, stub.Actions())
		}
		if event.TargetType != audit.TargetTypeTemplate {
			t.Fatalf("unexpected target type: %q", event.TargetType)
		}
		if event.TargetID == nil || *event.TargetID != 10 {
			t.Fatalf("unexpected target id: %v", event.TargetID)
		}
		if event.Metadata["source_template_id"] != int64(9) {
			t.Fatalf("expected source_template_id=9, got %+v", event.Metadata)
		}
		if event.Metadata["name"] != "Weekday Template (copy)" {
			t.Fatalf("expected metadata name=%q, got %+v", "Weekday Template (copy)", event.Metadata)
		}
	})

	t.Run("clones locked template", func(t *testing.T) {
		t.Parallel()

		service := NewTemplateService(
			&templateRepositoryMock{
				cloneFunc: func(ctx context.Context, id int64, name string) (*model.Template, error) {
					return &model.Template{ID: 11, Name: name, IsLocked: false}, nil
				},
				getByIDFunc: func(ctx context.Context, id int64) (*model.Template, error) {
					return &model.Template{ID: id, Name: "Locked Template", IsLocked: true}, nil
				},
			},
			&positionLookupRepositoryMock{},
		)

		clone, err := service.CloneTemplate(context.Background(), 7)
		if err != nil {
			t.Fatalf("CloneTemplate returned error: %v", err)
		}
		if clone.IsLocked {
			t.Fatal("expected locked source to clone into an unlocked template")
		}
	})

	t.Run("truncates clone name to the template name limit", func(t *testing.T) {
		t.Parallel()

		sourceName := strings.Repeat("a", maxTemplateNameLength)
		expectedCloneName := sourceName[:maxTemplateNameLength-len(templateCloneSuffix)] + templateCloneSuffix

		service := NewTemplateService(
			&templateRepositoryMock{
				cloneFunc: func(ctx context.Context, id int64, name string) (*model.Template, error) {
					if len(name) > maxTemplateNameLength {
						t.Fatalf("expected clone name length <= %d, got %d", maxTemplateNameLength, len(name))
					}
					if name != expectedCloneName {
						t.Fatalf("expected clone name %q, got %q", expectedCloneName, name)
					}

					return &model.Template{ID: 12, Name: name, IsLocked: false}, nil
				},
				getByIDFunc: func(ctx context.Context, id int64) (*model.Template, error) {
					return &model.Template{ID: id, Name: sourceName, IsLocked: false}, nil
				},
			},
			&positionLookupRepositoryMock{},
		)

		clone, err := service.CloneTemplate(context.Background(), 12)
		if err != nil {
			t.Fatalf("CloneTemplate returned error: %v", err)
		}
		if clone.Name != expectedCloneName {
			t.Fatalf("expected clone name %q, got %q", expectedCloneName, clone.Name)
		}
	})

	t.Run("truncates clone name by rune count for CJK text", func(t *testing.T) {
		t.Parallel()

		sourceName := strings.Repeat("排", maxTemplateNameLength)
		expectedCloneName := strings.Repeat(
			"排",
			maxTemplateNameLength-len([]rune(templateCloneSuffix)),
		) + templateCloneSuffix

		service := NewTemplateService(
			&templateRepositoryMock{
				cloneFunc: func(ctx context.Context, id int64, name string) (*model.Template, error) {
					if len([]rune(name)) != maxTemplateNameLength {
						t.Fatalf("expected clone name rune length %d, got %d", maxTemplateNameLength, len([]rune(name)))
					}
					if name != expectedCloneName {
						t.Fatalf("expected clone name %q, got %q", expectedCloneName, name)
					}

					return &model.Template{ID: 13, Name: name, IsLocked: false}, nil
				},
				getByIDFunc: func(ctx context.Context, id int64) (*model.Template, error) {
					return &model.Template{ID: id, Name: sourceName, IsLocked: false}, nil
				},
			},
			&positionLookupRepositoryMock{},
		)

		clone, err := service.CloneTemplate(context.Background(), 13)
		if err != nil {
			t.Fatalf("CloneTemplate returned error: %v", err)
		}
		if clone.Name != expectedCloneName {
			t.Fatalf("expected clone name %q, got %q", expectedCloneName, clone.Name)
		}
	})
}

func TestTemplateServiceCreateTemplateSlot(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		t.Parallel()

		service := NewTemplateService(
			&templateRepositoryMock{
				createSlotFunc: func(ctx context.Context, params repository.CreateTemplateSlotParams) (*model.TemplateSlot, error) {
					if params.TemplateID != 5 || !intSlicesEqual(params.Weekdays, []int{2, 4}) || params.StartTime != "09:00" || params.EndTime != "12:00" {
						t.Fatalf("unexpected create slot params: %+v", params)
					}
					return &model.TemplateSlot{
						ID:         1,
						TemplateID: params.TemplateID,
						Weekdays:   append([]int(nil), params.Weekdays...),
						StartTime:  params.StartTime,
						EndTime:    params.EndTime,
					}, nil
				},
			},
			&positionLookupRepositoryMock{},
		)

		slot, err := service.CreateTemplateSlot(context.Background(), CreateTemplateSlotInput{
			TemplateID: 5,
			Weekdays:   []int{4, 2, 2},
			StartTime:  "09:00",
			EndTime:    "12:00",
		})
		if err != nil {
			t.Fatalf("CreateTemplateSlot returned error: %v", err)
		}
		if slot.ID != 1 || !intSlicesEqual(slot.Weekdays, []int{2, 4}) {
			t.Fatalf("unexpected slot: %+v", slot)
		}
	})

	t.Run("invalid time range returns ErrInvalidShiftTime", func(t *testing.T) {
		t.Parallel()

		service := NewTemplateService(&templateRepositoryMock{}, &positionLookupRepositoryMock{})

		_, err := service.CreateTemplateSlot(context.Background(), CreateTemplateSlotInput{
			TemplateID: 1,
			Weekdays:   []int{1},
			StartTime:  "09:00",
			EndTime:    "09:00",
		})
		if !errors.Is(err, ErrInvalidShiftTime) {
			t.Fatalf("expected ErrInvalidShiftTime, got %v", err)
		}
	})
}

func TestTemplateServiceUpdateTemplateSlot(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		t.Parallel()

		service := NewTemplateService(
			&templateRepositoryMock{
				updateSlotFunc: func(ctx context.Context, params repository.UpdateTemplateSlotParams) (*model.TemplateSlot, error) {
					if params.TemplateID != 4 || params.SlotID != 8 || !intSlicesEqual(params.Weekdays, []int{1, 5}) || params.StartTime != "13:00" || params.EndTime != "16:00" {
						t.Fatalf("unexpected update slot params: %+v", params)
					}
					return &model.TemplateSlot{
						ID:         params.SlotID,
						TemplateID: params.TemplateID,
						Weekdays:   append([]int(nil), params.Weekdays...),
						StartTime:  params.StartTime,
						EndTime:    params.EndTime,
					}, nil
				},
			},
			&positionLookupRepositoryMock{},
		)

		slot, err := service.UpdateTemplateSlot(context.Background(), UpdateTemplateSlotInput{
			TemplateID: 4,
			SlotID:     8,
			Weekdays:   []int{5, 1},
			StartTime:  "13:00",
			EndTime:    "16:00",
		})
		if err != nil {
			t.Fatalf("UpdateTemplateSlot returned error: %v", err)
		}
		if slot.ID != 8 || !intSlicesEqual(slot.Weekdays, []int{1, 5}) {
			t.Fatalf("unexpected slot: %+v", slot)
		}
	})

	t.Run("slot not in template", func(t *testing.T) {
		t.Parallel()

		service := NewTemplateService(
			&templateRepositoryMock{
				updateSlotFunc: func(ctx context.Context, params repository.UpdateTemplateSlotParams) (*model.TemplateSlot, error) {
					return nil, repository.ErrTemplateSlotNotFound
				},
			},
			&positionLookupRepositoryMock{},
		)

		_, err := service.UpdateTemplateSlot(context.Background(), UpdateTemplateSlotInput{
			TemplateID: 2,
			SlotID:     99,
			Weekdays:   []int{1},
			StartTime:  "09:00",
			EndTime:    "10:00",
		})
		if !errors.Is(err, ErrTemplateSlotNotFound) {
			t.Fatalf("expected ErrTemplateSlotNotFound, got %v", err)
		}
	})
}

func TestTemplateServiceDeleteTemplateSlot(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		t.Parallel()

		var gotTemplateID int64
		var gotSlotID int64
		service := NewTemplateService(
			&templateRepositoryMock{
				deleteSlotFunc: func(ctx context.Context, templateID, slotID int64) error {
					gotTemplateID = templateID
					gotSlotID = slotID
					return nil
				},
			},
			&positionLookupRepositoryMock{},
		)

		if err := service.DeleteTemplateSlot(context.Background(), 6, 12); err != nil {
			t.Fatalf("DeleteTemplateSlot returned error: %v", err)
		}
		if gotTemplateID != 6 || gotSlotID != 12 {
			t.Fatalf("expected delete slot 12 from template 6, got slot %d template %d", gotSlotID, gotTemplateID)
		}
	})

	t.Run("locked template rejects", func(t *testing.T) {
		t.Parallel()

		service := NewTemplateService(
			&templateRepositoryMock{
				deleteSlotFunc: func(ctx context.Context, templateID, slotID int64) error {
					return repository.ErrTemplateLocked
				},
			},
			&positionLookupRepositoryMock{},
		)

		err := service.DeleteTemplateSlot(context.Background(), 6, 12)
		if !errors.Is(err, ErrTemplateLocked) {
			t.Fatalf("expected ErrTemplateLocked, got %v", err)
		}
	})
}

func TestTemplateServiceCreateTemplateSlotPosition(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		t.Parallel()

		service := NewTemplateService(
			&templateRepositoryMock{
				createSlotPositionFunc: func(ctx context.Context, params repository.CreateTemplateSlotPositionParams) (*model.TemplateSlotPosition, error) {
					if params.TemplateID != 5 || params.SlotID != 9 || params.PositionID != 7 || params.RequiredHeadcount != 3 {
						t.Fatalf("unexpected create slot position params: %+v", params)
					}
					return &model.TemplateSlotPosition{
						ID:                1,
						SlotID:            params.SlotID,
						PositionID:        params.PositionID,
						RequiredHeadcount: params.RequiredHeadcount,
					}, nil
				},
			},
			&positionLookupRepositoryMock{
				getByIDFunc: func(ctx context.Context, id int64) (*model.Position, error) {
					return &model.Position{ID: id}, nil
				},
			},
		)

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())

		slotPosition, err := service.CreateTemplateSlotPosition(ctx, CreateTemplateSlotPositionInput{
			TemplateID:        5,
			SlotID:            9,
			PositionID:        7,
			RequiredHeadcount: 3,
		})
		if err != nil {
			t.Fatalf("CreateTemplateSlotPosition returned error: %v", err)
		}
		if slotPosition.ID != 1 || slotPosition.SlotID != 9 {
			t.Fatalf("unexpected slot position: %+v", slotPosition)
		}

		event := stub.FindByAction(audit.ActionSlotPositionCreate)
		if event == nil {
			t.Fatalf("expected %q audit event, got %v", audit.ActionSlotPositionCreate, stub.Actions())
		}
		if event.Metadata["slot_id"] != int64(9) || event.Metadata["position_id"] != int64(7) {
			t.Fatalf("unexpected audit metadata: %+v", event.Metadata)
		}
	})

	t.Run("invalid headcount returns ErrInvalidHeadcount", func(t *testing.T) {
		t.Parallel()

		service := NewTemplateService(&templateRepositoryMock{}, &positionLookupRepositoryMock{})

		_, err := service.CreateTemplateSlotPosition(context.Background(), CreateTemplateSlotPositionInput{
			TemplateID:        1,
			SlotID:            2,
			PositionID:        3,
			RequiredHeadcount: 0,
		})
		if !errors.Is(err, ErrInvalidHeadcount) {
			t.Fatalf("expected ErrInvalidHeadcount, got %v", err)
		}
	})
}

func TestTemplateServiceUpdateTemplateSlotPosition(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		t.Parallel()

		service := NewTemplateService(
			&templateRepositoryMock{
				getByIDFunc: func(ctx context.Context, id int64) (*model.Template, error) {
					return &model.Template{
						ID: id,
						Slots: []*model.TemplateSlot{
							{
								ID: 4,
								Positions: []*model.TemplateSlotPosition{
									{ID: 8, PositionID: 3, RequiredHeadcount: 1},
								},
							},
						},
					}, nil
				},
				updateSlotPositionFunc: func(ctx context.Context, params repository.UpdateTemplateSlotPositionParams) (*model.TemplateSlotPosition, error) {
					if params.TemplateID != 4 || params.SlotID != 4 || params.SlotPositionID != 8 || params.PositionID != 7 || params.RequiredHeadcount != 2 {
						t.Fatalf("unexpected update slot position params: %+v", params)
					}
					return &model.TemplateSlotPosition{
						ID:                params.SlotPositionID,
						SlotID:            params.SlotID,
						PositionID:        params.PositionID,
						RequiredHeadcount: params.RequiredHeadcount,
					}, nil
				},
			},
			&positionLookupRepositoryMock{
				getByIDFunc: func(ctx context.Context, id int64) (*model.Position, error) {
					return &model.Position{ID: id}, nil
				},
			},
		)

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())

		slotPosition, err := service.UpdateTemplateSlotPosition(ctx, UpdateTemplateSlotPositionInput{
			TemplateID:        4,
			SlotID:            4,
			SlotPositionID:    8,
			PositionID:        7,
			RequiredHeadcount: 2,
		})
		if err != nil {
			t.Fatalf("UpdateTemplateSlotPosition returned error: %v", err)
		}
		if slotPosition.ID != 8 || slotPosition.PositionID != 7 {
			t.Fatalf("unexpected slot position: %+v", slotPosition)
		}

		event := stub.FindByAction(audit.ActionSlotPositionUpdate)
		if event == nil {
			t.Fatalf("expected %q audit event, got %v", audit.ActionSlotPositionUpdate, stub.Actions())
		}
		if event.Metadata["slot_id"] != int64(4) {
			t.Fatalf("unexpected audit metadata: %+v", event.Metadata)
		}
	})

	t.Run("slot position not found", func(t *testing.T) {
		t.Parallel()

		service := NewTemplateService(
			&templateRepositoryMock{
				updateSlotPositionFunc: func(ctx context.Context, params repository.UpdateTemplateSlotPositionParams) (*model.TemplateSlotPosition, error) {
					return nil, repository.ErrTemplateSlotPositionNotFound
				},
			},
			&positionLookupRepositoryMock{
				getByIDFunc: func(ctx context.Context, id int64) (*model.Position, error) {
					return &model.Position{ID: id}, nil
				},
			},
		)

		_, err := service.UpdateTemplateSlotPosition(context.Background(), UpdateTemplateSlotPositionInput{
			TemplateID:        2,
			SlotID:            4,
			SlotPositionID:    99,
			PositionID:        3,
			RequiredHeadcount: 1,
		})
		if !errors.Is(err, ErrTemplateSlotPositionNotFound) {
			t.Fatalf("expected ErrTemplateSlotPositionNotFound, got %v", err)
		}
	})
}

func TestTemplateServiceDeleteTemplateSlotPosition(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		t.Parallel()

		service := NewTemplateService(
			&templateRepositoryMock{
				deleteSlotPositionFunc: func(ctx context.Context, templateID, slotID, slotPositionID int64) error {
					if templateID != 6 || slotID != 4 || slotPositionID != 12 {
						t.Fatalf("unexpected delete slot position target: template=%d slot=%d slotPosition=%d", templateID, slotID, slotPositionID)
					}
					return nil
				},
			},
			&positionLookupRepositoryMock{},
		)

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())

		if err := service.DeleteTemplateSlotPosition(ctx, 6, 4, 12); err != nil {
			t.Fatalf("DeleteTemplateSlotPosition returned error: %v", err)
		}

		event := stub.FindByAction(audit.ActionSlotPositionDelete)
		if event == nil {
			t.Fatalf("expected %q audit event, got %v", audit.ActionSlotPositionDelete, stub.Actions())
		}
		if event.Metadata["slot_id"] != int64(4) {
			t.Fatalf("unexpected audit metadata: %+v", event.Metadata)
		}
	})

	t.Run("locked template rejects", func(t *testing.T) {
		t.Parallel()

		service := NewTemplateService(
			&templateRepositoryMock{
				deleteSlotPositionFunc: func(ctx context.Context, templateID, slotID, slotPositionID int64) error {
					return repository.ErrTemplateLocked
				},
			},
			&positionLookupRepositoryMock{},
		)

		err := service.DeleteTemplateSlotPosition(context.Background(), 6, 4, 12)
		if !errors.Is(err, ErrTemplateLocked) {
			t.Fatalf("expected ErrTemplateLocked, got %v", err)
		}
	})
}

func intSlicesEqual(left, right []int) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
