package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/audit"
	"github.com/jonathanhu237/rota/backend/internal/audit/audittest"
	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/jonathanhu237/rota/backend/internal/repository"
)

type templateRepositoryMock struct {
	listPaginatedFunc func(ctx context.Context, params repository.ListTemplatesParams) ([]*model.Template, int, error)
	getByIDFunc       func(ctx context.Context, id int64) (*model.Template, error)
	createFunc        func(ctx context.Context, params repository.CreateTemplateParams) (*model.Template, error)
	updateFunc        func(ctx context.Context, params repository.UpdateTemplateParams) (*model.Template, error)
	deleteFunc        func(ctx context.Context, id int64) error
	cloneFunc         func(ctx context.Context, id int64, name string) (*model.Template, error)
	createShiftFunc   func(ctx context.Context, params repository.CreateTemplateShiftParams) (*model.TemplateShift, error)
	updateShiftFunc   func(ctx context.Context, params repository.UpdateTemplateShiftParams) (*model.TemplateShift, error)
	deleteShiftFunc   func(ctx context.Context, templateID, shiftID int64) error
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

func (m *templateRepositoryMock) CreateShift(ctx context.Context, params repository.CreateTemplateShiftParams) (*model.TemplateShift, error) {
	return m.createShiftFunc(ctx, params)
}

func (m *templateRepositoryMock) UpdateShift(ctx context.Context, params repository.UpdateTemplateShiftParams) (*model.TemplateShift, error) {
	return m.updateShiftFunc(ctx, params)
}

func (m *templateRepositoryMock) DeleteShift(ctx context.Context, templateID, shiftID int64) error {
	return m.deleteShiftFunc(ctx, templateID, shiftID)
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
	t.Run("sorts shifts by weekday then start time", func(t *testing.T) {
		t.Parallel()

		service := NewTemplateService(
			&templateRepositoryMock{
				getByIDFunc: func(ctx context.Context, id int64) (*model.Template, error) {
					return &model.Template{
						ID:   id,
						Name: "Weekday",
						Shifts: []*model.TemplateShift{
							{ID: 3, Weekday: 3, StartTime: "11:00"},
							{ID: 2, Weekday: 1, StartTime: "12:00"},
							{ID: 1, Weekday: 1, StartTime: "09:00"},
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
			template.Shifts[0].ID,
			template.Shifts[1].ID,
			template.Shifts[2].ID,
		}
		wantIDs := []int64{1, 2, 3}
		for i := range wantIDs {
			if gotIDs[i] != wantIDs[i] {
				t.Fatalf("expected shift order %v, got %v", wantIDs, gotIDs)
			}
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
						Shifts: []*model.TemplateShift{
							{ID: 1, Weekday: 1},
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
		if len(clone.Shifts) != 1 {
			t.Fatalf("expected shifts to be cloned, got %+v", clone.Shifts)
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

func TestTemplateServiceCreateTemplateShift(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		t.Parallel()

		now := time.Now()
		service := NewTemplateService(
			&templateRepositoryMock{
				getByIDFunc: func(ctx context.Context, id int64) (*model.Template, error) {
					return &model.Template{ID: id, IsLocked: false}, nil
				},
				createShiftFunc: func(ctx context.Context, params repository.CreateTemplateShiftParams) (*model.TemplateShift, error) {
					if params.TemplateID != 5 || params.Weekday != 2 || params.StartTime != "09:00" || params.EndTime != "12:00" || params.PositionID != 7 || params.RequiredHeadcount != 3 {
						t.Fatalf("unexpected create shift params: %+v", params)
					}
					return &model.TemplateShift{
						ID:                1,
						TemplateID:        params.TemplateID,
						Weekday:           params.Weekday,
						StartTime:         params.StartTime,
						EndTime:           params.EndTime,
						PositionID:        params.PositionID,
						RequiredHeadcount: params.RequiredHeadcount,
						CreatedAt:         now,
						UpdatedAt:         now,
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

		shift, err := service.CreateTemplateShift(ctx, CreateTemplateShiftInput{
			TemplateID:        5,
			Weekday:           2,
			StartTime:         "09:00",
			EndTime:           "12:00",
			PositionID:        7,
			RequiredHeadcount: 3,
		})
		if err != nil {
			t.Fatalf("CreateTemplateShift returned error: %v", err)
		}
		if shift.Weekday != 2 || shift.StartTime != "09:00" || shift.EndTime != "12:00" {
			t.Fatalf("unexpected shift: %+v", shift)
		}

		event := stub.FindByAction(audit.ActionTemplateShiftCreate)
		if event == nil {
			t.Fatalf("expected %q audit event, got %v", audit.ActionTemplateShiftCreate, stub.Actions())
		}
		if event.TargetType != audit.TargetTypeTemplateShift {
			t.Fatalf("unexpected target type: %q", event.TargetType)
		}
		if event.TargetID == nil || *event.TargetID != 1 {
			t.Fatalf("unexpected target id: %v", event.TargetID)
		}
		if event.Metadata["template_id"] != int64(5) {
			t.Fatalf("expected template_id=5, got %+v", event.Metadata)
		}
		if event.Metadata["weekday"] != 2 {
			t.Fatalf("expected weekday=2, got %+v", event.Metadata)
		}
		if event.Metadata["start_time"] != "09:00" {
			t.Fatalf("expected start_time=09:00, got %+v", event.Metadata)
		}
		if event.Metadata["end_time"] != "12:00" {
			t.Fatalf("expected end_time=12:00, got %+v", event.Metadata)
		}
		if event.Metadata["position_id"] != int64(7) {
			t.Fatalf("expected position_id=7, got %+v", event.Metadata)
		}
		if event.Metadata["required_headcount"] != 3 {
			t.Fatalf("expected required_headcount=3, got %+v", event.Metadata)
		}
	})

	t.Run("invalid weekday returns ErrInvalidWeekday", func(t *testing.T) {
		t.Parallel()

		service := NewTemplateService(&templateRepositoryMock{}, &positionLookupRepositoryMock{})

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())

		_, err := service.CreateTemplateShift(ctx, CreateTemplateShiftInput{
			TemplateID:        1,
			Weekday:           8,
			StartTime:         "09:00",
			EndTime:           "10:00",
			PositionID:        2,
			RequiredHeadcount: 1,
		})
		if !errors.Is(err, ErrInvalidWeekday) {
			t.Fatalf("expected ErrInvalidWeekday, got %v", err)
		}
		if len(stub.Events()) != 0 {
			t.Fatalf("expected no audit events on error, got %v", stub.Actions())
		}
	})

	t.Run("invalid time range returns ErrInvalidShiftTime", func(t *testing.T) {
		t.Parallel()

		service := NewTemplateService(&templateRepositoryMock{}, &positionLookupRepositoryMock{})

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())

		_, err := service.CreateTemplateShift(ctx, CreateTemplateShiftInput{
			TemplateID:        1,
			Weekday:           1,
			StartTime:         "09:00",
			EndTime:           "09:00",
			PositionID:        2,
			RequiredHeadcount: 1,
		})
		if !errors.Is(err, ErrInvalidShiftTime) {
			t.Fatalf("expected ErrInvalidShiftTime, got %v", err)
		}
		if len(stub.Events()) != 0 {
			t.Fatalf("expected no audit events on error, got %v", stub.Actions())
		}
	})

	t.Run("invalid headcount returns ErrInvalidHeadcount", func(t *testing.T) {
		t.Parallel()

		service := NewTemplateService(&templateRepositoryMock{}, &positionLookupRepositoryMock{})

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())

		_, err := service.CreateTemplateShift(ctx, CreateTemplateShiftInput{
			TemplateID:        1,
			Weekday:           1,
			StartTime:         "09:00",
			EndTime:           "10:00",
			PositionID:        2,
			RequiredHeadcount: 0,
		})
		if !errors.Is(err, ErrInvalidHeadcount) {
			t.Fatalf("expected ErrInvalidHeadcount, got %v", err)
		}
		if len(stub.Events()) != 0 {
			t.Fatalf("expected no audit events on error, got %v", stub.Actions())
		}
	})

	t.Run("position not found", func(t *testing.T) {
		t.Parallel()

		service := NewTemplateService(
			&templateRepositoryMock{
				getByIDFunc: func(ctx context.Context, id int64) (*model.Template, error) {
					return &model.Template{ID: id, IsLocked: false}, nil
				},
			},
			&positionLookupRepositoryMock{
				getByIDFunc: func(ctx context.Context, id int64) (*model.Position, error) {
					return nil, repository.ErrPositionNotFound
				},
			},
		)

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())

		_, err := service.CreateTemplateShift(ctx, CreateTemplateShiftInput{
			TemplateID:        1,
			Weekday:           1,
			StartTime:         "09:00",
			EndTime:           "10:00",
			PositionID:        99,
			RequiredHeadcount: 1,
		})
		if !errors.Is(err, ErrPositionNotFound) {
			t.Fatalf("expected ErrPositionNotFound, got %v", err)
		}
		if len(stub.Events()) != 0 {
			t.Fatalf("expected no audit events on error, got %v", stub.Actions())
		}
	})

	t.Run("locked template rejects", func(t *testing.T) {
		t.Parallel()

		createCalled := false
		service := NewTemplateService(
			&templateRepositoryMock{
				createShiftFunc: func(ctx context.Context, params repository.CreateTemplateShiftParams) (*model.TemplateShift, error) {
					createCalled = true
					return nil, repository.ErrTemplateLocked
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

		_, err := service.CreateTemplateShift(ctx, CreateTemplateShiftInput{
			TemplateID:        1,
			Weekday:           1,
			StartTime:         "09:00",
			EndTime:           "10:00",
			PositionID:        2,
			RequiredHeadcount: 1,
		})
		if !errors.Is(err, ErrTemplateLocked) {
			t.Fatalf("expected ErrTemplateLocked, got %v", err)
		}
		if !createCalled {
			t.Fatal("expected create shift to be attempted so repository lock guard can reject it")
		}
		if len(stub.Events()) != 0 {
			t.Fatalf("expected no audit events on error, got %v", stub.Actions())
		}
	})
}

func TestTemplateServiceUpdateTemplateShift(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		t.Parallel()

		service := NewTemplateService(
			&templateRepositoryMock{
				getByIDFunc: func(ctx context.Context, id int64) (*model.Template, error) {
					return &model.Template{
						ID:       id,
						IsLocked: false,
						Shifts: []*model.TemplateShift{
							{
								ID:                8,
								TemplateID:        id,
								Weekday:           1,
								StartTime:         "09:00",
								EndTime:           "12:00",
								PositionID:        3,
								RequiredHeadcount: 1,
							},
						},
					}, nil
				},
				updateShiftFunc: func(ctx context.Context, params repository.UpdateTemplateShiftParams) (*model.TemplateShift, error) {
					if params.TemplateID != 4 || params.ShiftID != 8 || params.Weekday != 5 || params.StartTime != "13:00" || params.EndTime != "16:00" || params.PositionID != 7 || params.RequiredHeadcount != 2 {
						t.Fatalf("unexpected update shift params: %+v", params)
					}
					return &model.TemplateShift{
						ID:                params.ShiftID,
						TemplateID:        params.TemplateID,
						Weekday:           params.Weekday,
						StartTime:         params.StartTime,
						EndTime:           params.EndTime,
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

		shift, err := service.UpdateTemplateShift(ctx, UpdateTemplateShiftInput{
			TemplateID:        4,
			ShiftID:           8,
			Weekday:           5,
			StartTime:         "13:00",
			EndTime:           "16:00",
			PositionID:        7,
			RequiredHeadcount: 2,
		})
		if err != nil {
			t.Fatalf("UpdateTemplateShift returned error: %v", err)
		}
		if shift.ID != 8 {
			t.Fatalf("expected updated shift ID 8, got %d", shift.ID)
		}

		event := stub.FindByAction(audit.ActionTemplateShiftUpdate)
		if event == nil {
			t.Fatalf("expected %q audit event, got %v", audit.ActionTemplateShiftUpdate, stub.Actions())
		}
		if event.TargetType != audit.TargetTypeTemplateShift {
			t.Fatalf("unexpected target type: %q", event.TargetType)
		}
		if event.TargetID == nil || *event.TargetID != 8 {
			t.Fatalf("unexpected target id: %v", event.TargetID)
		}
		if event.Metadata["template_id"] != int64(4) {
			t.Fatalf("expected template_id=4, got %+v", event.Metadata)
		}
		if _, ok := event.Metadata["weekday"]; !ok {
			t.Fatalf("expected weekday change in metadata, got %+v", event.Metadata)
		}
		if _, ok := event.Metadata["start_time"]; !ok {
			t.Fatalf("expected start_time change in metadata, got %+v", event.Metadata)
		}
		if _, ok := event.Metadata["end_time"]; !ok {
			t.Fatalf("expected end_time change in metadata, got %+v", event.Metadata)
		}
		if _, ok := event.Metadata["position_id"]; !ok {
			t.Fatalf("expected position_id change in metadata, got %+v", event.Metadata)
		}
		if _, ok := event.Metadata["required_headcount"]; !ok {
			t.Fatalf("expected required_headcount change in metadata, got %+v", event.Metadata)
		}
	})

	t.Run("locked template rejects", func(t *testing.T) {
		t.Parallel()

		updateCalled := false
		service := NewTemplateService(
			&templateRepositoryMock{
				updateShiftFunc: func(ctx context.Context, params repository.UpdateTemplateShiftParams) (*model.TemplateShift, error) {
					updateCalled = true
					return nil, repository.ErrTemplateLocked
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

		_, err := service.UpdateTemplateShift(ctx, UpdateTemplateShiftInput{
			TemplateID:        2,
			ShiftID:           9,
			Weekday:           1,
			StartTime:         "09:00",
			EndTime:           "10:00",
			PositionID:        3,
			RequiredHeadcount: 1,
		})
		if !errors.Is(err, ErrTemplateLocked) {
			t.Fatalf("expected ErrTemplateLocked, got %v", err)
		}
		if !updateCalled {
			t.Fatal("expected update shift to be attempted so repository lock guard can reject it")
		}
		if len(stub.Events()) != 0 {
			t.Fatalf("expected no audit events on error, got %v", stub.Actions())
		}
	})

	t.Run("shift not in template", func(t *testing.T) {
		t.Parallel()

		service := NewTemplateService(
			&templateRepositoryMock{
				getByIDFunc: func(ctx context.Context, id int64) (*model.Template, error) {
					return &model.Template{ID: id, IsLocked: false}, nil
				},
				updateShiftFunc: func(ctx context.Context, params repository.UpdateTemplateShiftParams) (*model.TemplateShift, error) {
					return nil, repository.ErrTemplateShiftNotFound
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

		_, err := service.UpdateTemplateShift(ctx, UpdateTemplateShiftInput{
			TemplateID:        2,
			ShiftID:           99,
			Weekday:           1,
			StartTime:         "09:00",
			EndTime:           "10:00",
			PositionID:        3,
			RequiredHeadcount: 1,
		})
		if !errors.Is(err, ErrTemplateShiftNotFound) {
			t.Fatalf("expected ErrTemplateShiftNotFound, got %v", err)
		}
		if len(stub.Events()) != 0 {
			t.Fatalf("expected no audit events on error, got %v", stub.Actions())
		}
	})
}

func TestTemplateServiceDeleteTemplateShift(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		t.Parallel()

		var gotTemplateID int64
		var gotShiftID int64
		service := NewTemplateService(
			&templateRepositoryMock{
				getByIDFunc: func(ctx context.Context, id int64) (*model.Template, error) {
					return &model.Template{ID: id, IsLocked: false}, nil
				},
				deleteShiftFunc: func(ctx context.Context, templateID, shiftID int64) error {
					gotTemplateID = templateID
					gotShiftID = shiftID
					return nil
				},
			},
			&positionLookupRepositoryMock{},
		)

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())

		if err := service.DeleteTemplateShift(ctx, 6, 12); err != nil {
			t.Fatalf("DeleteTemplateShift returned error: %v", err)
		}
		if gotTemplateID != 6 || gotShiftID != 12 {
			t.Fatalf("expected delete shift 12 from template 6, got shift %d template %d", gotShiftID, gotTemplateID)
		}

		event := stub.FindByAction(audit.ActionTemplateShiftDelete)
		if event == nil {
			t.Fatalf("expected %q audit event, got %v", audit.ActionTemplateShiftDelete, stub.Actions())
		}
		if event.TargetType != audit.TargetTypeTemplateShift {
			t.Fatalf("unexpected target type: %q", event.TargetType)
		}
		if event.TargetID == nil || *event.TargetID != 12 {
			t.Fatalf("unexpected target id: %v", event.TargetID)
		}
		if event.Metadata["template_id"] != int64(6) {
			t.Fatalf("expected template_id=6, got %+v", event.Metadata)
		}
	})

	t.Run("locked template rejects", func(t *testing.T) {
		t.Parallel()

		deleteCalled := false
		service := NewTemplateService(
			&templateRepositoryMock{
				deleteShiftFunc: func(ctx context.Context, templateID, shiftID int64) error {
					deleteCalled = true
					return repository.ErrTemplateLocked
				},
			},
			&positionLookupRepositoryMock{},
		)

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())

		err := service.DeleteTemplateShift(ctx, 6, 12)
		if !errors.Is(err, ErrTemplateLocked) {
			t.Fatalf("expected ErrTemplateLocked, got %v", err)
		}
		if !deleteCalled {
			t.Fatal("expected delete shift to be attempted so repository lock guard can reject it")
		}
		if len(stub.Events()) != 0 {
			t.Fatalf("expected no audit events on error, got %v", stub.Actions())
		}
	})
}
