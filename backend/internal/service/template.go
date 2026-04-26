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

const (
	maxTemplateDescriptionLength = 500
	maxTemplateNameLength        = 100
	templateCloneSuffix          = " (copy)"
	timeLayoutHourMinute         = "15:04"
)

var (
	ErrInvalidHeadcount             = model.ErrInvalidHeadcount
	ErrInvalidShiftTime             = model.ErrInvalidShiftTime
	ErrInvalidWeekday               = model.ErrInvalidWeekday
	ErrTemplateLocked               = model.ErrTemplateLocked
	ErrTemplateNotFound             = model.ErrTemplateNotFound
	ErrTemplateSlotOverlap          = model.ErrTemplateSlotOverlap
	ErrTemplateSlotNotFound         = model.ErrTemplateSlotNotFound
	ErrTemplateSlotPositionNotFound = model.ErrTemplateSlotPositionNotFound
)

type templateRepository interface {
	ListPaginated(ctx context.Context, params repository.ListTemplatesParams) ([]*model.Template, int, error)
	GetByID(ctx context.Context, id int64) (*model.Template, error)
	Create(ctx context.Context, params repository.CreateTemplateParams) (*model.Template, error)
	Update(ctx context.Context, params repository.UpdateTemplateParams) (*model.Template, error)
	Delete(ctx context.Context, id int64) error
	Clone(ctx context.Context, id int64, name string) (*model.Template, error)
	CreateSlot(ctx context.Context, params repository.CreateTemplateSlotParams) (*model.TemplateSlot, error)
	UpdateSlot(ctx context.Context, params repository.UpdateTemplateSlotParams) (*model.TemplateSlot, error)
	DeleteSlot(ctx context.Context, templateID, slotID int64) error
	CreateSlotPosition(ctx context.Context, params repository.CreateTemplateSlotPositionParams) (*model.TemplateSlotPosition, error)
	UpdateSlotPosition(ctx context.Context, params repository.UpdateTemplateSlotPositionParams) (*model.TemplateSlotPosition, error)
	DeleteSlotPosition(ctx context.Context, templateID, slotID, slotPositionID int64) error
}

type positionLookupRepository interface {
	GetByID(ctx context.Context, id int64) (*model.Position, error)
}

type TemplateService struct {
	templateRepo templateRepository
	positionRepo positionLookupRepository
}

type ListTemplatesInput struct {
	Page     int
	PageSize int
}

type ListTemplatesResult struct {
	Templates  []*model.Template
	Page       int
	PageSize   int
	Total      int
	TotalPages int
}

type CreateTemplateInput struct {
	Name        string
	Description string
}

type UpdateTemplateInput struct {
	ID          int64
	Name        string
	Description string
}

type CreateTemplateSlotInput struct {
	TemplateID int64
	Weekdays   []int
	StartTime  string
	EndTime    string
}

type UpdateTemplateSlotInput struct {
	TemplateID int64
	SlotID     int64
	Weekdays   []int
	StartTime  string
	EndTime    string
}

type CreateTemplateSlotPositionInput struct {
	TemplateID        int64
	SlotID            int64
	PositionID        int64
	RequiredHeadcount int
}

type UpdateTemplateSlotPositionInput struct {
	TemplateID        int64
	SlotID            int64
	SlotPositionID    int64
	PositionID        int64
	RequiredHeadcount int
}

func NewTemplateService(templateRepo templateRepository, positionRepo positionLookupRepository) *TemplateService {
	return &TemplateService{
		templateRepo: templateRepo,
		positionRepo: positionRepo,
	}
}

func (s *TemplateService) ListTemplates(ctx context.Context, input ListTemplatesInput) (*ListTemplatesResult, error) {
	page, pageSize, err := normalizePagination(input.Page, input.PageSize)
	if err != nil {
		return nil, err
	}

	offset := (page - 1) * pageSize
	templates, total, err := s.templateRepo.ListPaginated(ctx, repository.ListTemplatesParams{
		Offset: offset,
		Limit:  pageSize,
	})
	if err != nil {
		return nil, mapTemplateRepositoryError(err)
	}

	totalPages := 0
	if total > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}

	return &ListTemplatesResult{
		Templates:  templates,
		Page:       page,
		PageSize:   pageSize,
		Total:      total,
		TotalPages: totalPages,
	}, nil
}

func (s *TemplateService) CreateTemplate(ctx context.Context, input CreateTemplateInput) (*model.Template, error) {
	name, description, err := normalizeTemplateInput(input.Name, input.Description)
	if err != nil {
		return nil, err
	}

	template, err := s.templateRepo.Create(ctx, repository.CreateTemplateParams{
		Name:        name,
		Description: description,
	})
	if err != nil {
		return nil, mapTemplateRepositoryError(err)
	}

	targetID := template.ID
	audit.Record(ctx, audit.Event{
		Action:     audit.ActionTemplateCreate,
		TargetType: audit.TargetTypeTemplate,
		TargetID:   &targetID,
		Metadata: map[string]any{
			"name": template.Name,
		},
	})

	return template, nil
}

func (s *TemplateService) GetTemplateByID(ctx context.Context, id int64) (*model.Template, error) {
	if id <= 0 {
		return nil, ErrInvalidInput
	}

	template, err := s.templateRepo.GetByID(ctx, id)
	if err != nil {
		return nil, mapTemplateRepositoryError(err)
	}

	sortTemplateSlots(template.Slots)
	return template, nil
}

func (s *TemplateService) UpdateTemplate(ctx context.Context, input UpdateTemplateInput) (*model.Template, error) {
	if input.ID <= 0 {
		return nil, ErrInvalidInput
	}

	name, description, err := normalizeTemplateInput(input.Name, input.Description)
	if err != nil {
		return nil, err
	}

	// Best-effort capture of the previous state so the audit event can report
	// changed fields. If the lookup fails we still allow the update to proceed
	// and emit an event with a partial diff.
	var previous *model.Template
	if prev, prevErr := s.templateRepo.GetByID(ctx, input.ID); prevErr == nil {
		previous = prev
	}

	template, err := s.templateRepo.Update(ctx, repository.UpdateTemplateParams{
		ID:          input.ID,
		Name:        name,
		Description: description,
	})
	if err != nil {
		return nil, mapTemplateRepositoryError(err)
	}

	changes := map[string]any{}
	if previous != nil {
		if previous.Name != template.Name {
			changes["name"] = map[string]any{"from": previous.Name, "to": template.Name}
		}
		if previous.Description != template.Description {
			changes["description"] = map[string]any{"from": previous.Description, "to": template.Description}
		}
	}

	targetID := template.ID
	audit.Record(ctx, audit.Event{
		Action:     audit.ActionTemplateUpdate,
		TargetType: audit.TargetTypeTemplate,
		TargetID:   &targetID,
		Metadata:   changes,
	})

	return template, nil
}

func (s *TemplateService) DeleteTemplate(ctx context.Context, id int64) error {
	if id <= 0 {
		return ErrInvalidInput
	}

	// Best-effort load of the row before deletion so the audit metadata can
	// include the template name. We tolerate lookup errors because the
	// repository Delete call is the authoritative guard for existence/locking.
	var existingName string
	if existing, lookupErr := s.templateRepo.GetByID(ctx, id); lookupErr == nil {
		existingName = existing.Name
	}

	if err := s.templateRepo.Delete(ctx, id); err != nil {
		return mapTemplateRepositoryError(err)
	}

	metadata := map[string]any{}
	if existingName != "" {
		metadata["name"] = existingName
	}

	targetID := id
	audit.Record(ctx, audit.Event{
		Action:     audit.ActionTemplateDelete,
		TargetType: audit.TargetTypeTemplate,
		TargetID:   &targetID,
		Metadata:   metadata,
	})

	return nil
}

func (s *TemplateService) CloneTemplate(ctx context.Context, id int64) (*model.Template, error) {
	if id <= 0 {
		return nil, ErrInvalidInput
	}

	template, err := s.templateRepo.GetByID(ctx, id)
	if err != nil {
		return nil, mapTemplateRepositoryError(err)
	}

	clone, err := s.templateRepo.Clone(ctx, id, buildTemplateCloneName(template.Name))
	if err != nil {
		return nil, mapTemplateRepositoryError(err)
	}

	sortTemplateSlots(clone.Slots)

	targetID := clone.ID
	audit.Record(ctx, audit.Event{
		Action:     audit.ActionTemplateClone,
		TargetType: audit.TargetTypeTemplate,
		TargetID:   &targetID,
		Metadata: map[string]any{
			"source_template_id": id,
			"name":               clone.Name,
		},
	})

	return clone, nil
}

func (s *TemplateService) CreateTemplateSlot(ctx context.Context, input CreateTemplateSlotInput) (*model.TemplateSlot, error) {
	if input.TemplateID <= 0 {
		return nil, ErrInvalidInput
	}

	normalizedSlotInput, err := normalizeSlotInput(input.Weekdays, input.StartTime, input.EndTime)
	if err != nil {
		return nil, err
	}

	slot, err := s.templateRepo.CreateSlot(ctx, repository.CreateTemplateSlotParams{
		TemplateID: input.TemplateID,
		Weekdays:   normalizedSlotInput.Weekdays,
		StartTime:  normalizedSlotInput.StartTime,
		EndTime:    normalizedSlotInput.EndTime,
	})
	if err != nil {
		return nil, mapTemplateRepositoryError(err)
	}

	return slot, nil
}

func (s *TemplateService) UpdateTemplateSlot(ctx context.Context, input UpdateTemplateSlotInput) (*model.TemplateSlot, error) {
	if input.TemplateID <= 0 || input.SlotID <= 0 {
		return nil, ErrInvalidInput
	}

	normalizedSlotInput, err := normalizeSlotInput(input.Weekdays, input.StartTime, input.EndTime)
	if err != nil {
		return nil, err
	}

	slot, err := s.templateRepo.UpdateSlot(ctx, repository.UpdateTemplateSlotParams{
		TemplateID: input.TemplateID,
		SlotID:     input.SlotID,
		Weekdays:   normalizedSlotInput.Weekdays,
		StartTime:  normalizedSlotInput.StartTime,
		EndTime:    normalizedSlotInput.EndTime,
	})
	if err != nil {
		return nil, mapTemplateRepositoryError(err)
	}

	return slot, nil
}

func (s *TemplateService) DeleteTemplateSlot(ctx context.Context, templateID, slotID int64) error {
	if templateID <= 0 || slotID <= 0 {
		return ErrInvalidInput
	}

	if err := s.templateRepo.DeleteSlot(ctx, templateID, slotID); err != nil {
		return mapTemplateRepositoryError(err)
	}

	return nil
}

func (s *TemplateService) CreateTemplateSlotPosition(
	ctx context.Context,
	input CreateTemplateSlotPositionInput,
) (*model.TemplateSlotPosition, error) {
	if input.TemplateID <= 0 || input.SlotID <= 0 || input.PositionID <= 0 {
		return nil, ErrInvalidInput
	}

	requiredHeadcount, err := normalizeRequiredHeadcount(input.RequiredHeadcount)
	if err != nil {
		return nil, err
	}

	if err := s.ensurePositionExists(ctx, input.PositionID); err != nil {
		return nil, err
	}

	slotPosition, err := s.templateRepo.CreateSlotPosition(ctx, repository.CreateTemplateSlotPositionParams{
		TemplateID:        input.TemplateID,
		SlotID:            input.SlotID,
		PositionID:        input.PositionID,
		RequiredHeadcount: requiredHeadcount,
	})
	if err != nil {
		return nil, mapTemplateRepositoryError(err)
	}

	targetID := slotPosition.ID
	audit.Record(ctx, audit.Event{
		Action:     audit.ActionSlotPositionCreate,
		TargetType: audit.TargetTypeSlotPosition,
		TargetID:   &targetID,
		Metadata: map[string]any{
			"template_id":        input.TemplateID,
			"slot_id":            slotPosition.SlotID,
			"position_id":        slotPosition.PositionID,
			"required_headcount": slotPosition.RequiredHeadcount,
		},
	})

	return slotPosition, nil
}

func (s *TemplateService) UpdateTemplateSlotPosition(
	ctx context.Context,
	input UpdateTemplateSlotPositionInput,
) (*model.TemplateSlotPosition, error) {
	if input.TemplateID <= 0 || input.SlotID <= 0 || input.SlotPositionID <= 0 || input.PositionID <= 0 {
		return nil, ErrInvalidInput
	}

	requiredHeadcount, err := normalizeRequiredHeadcount(input.RequiredHeadcount)
	if err != nil {
		return nil, err
	}

	if err := s.ensurePositionExists(ctx, input.PositionID); err != nil {
		return nil, err
	}

	var previous *model.TemplateSlotPosition
	if prev, prevErr := s.templateRepo.GetByID(ctx, input.TemplateID); prevErr == nil {
		for _, slot := range prev.Slots {
			if slot.ID != input.SlotID {
				continue
			}
			for _, candidate := range slot.Positions {
				if candidate.ID == input.SlotPositionID {
					previous = candidate
					break
				}
			}
		}
	}

	slotPosition, err := s.templateRepo.UpdateSlotPosition(ctx, repository.UpdateTemplateSlotPositionParams{
		TemplateID:        input.TemplateID,
		SlotID:            input.SlotID,
		SlotPositionID:    input.SlotPositionID,
		PositionID:        input.PositionID,
		RequiredHeadcount: requiredHeadcount,
	})
	if err != nil {
		return nil, mapTemplateRepositoryError(err)
	}

	changes := map[string]any{
		"template_id": input.TemplateID,
		"slot_id":     input.SlotID,
	}
	if previous != nil {
		if previous.PositionID != slotPosition.PositionID {
			changes["position_id"] = map[string]any{"from": previous.PositionID, "to": slotPosition.PositionID}
		}
		if previous.RequiredHeadcount != slotPosition.RequiredHeadcount {
			changes["required_headcount"] = map[string]any{
				"from": previous.RequiredHeadcount,
				"to":   slotPosition.RequiredHeadcount,
			}
		}
	}

	targetID := slotPosition.ID
	audit.Record(ctx, audit.Event{
		Action:     audit.ActionSlotPositionUpdate,
		TargetType: audit.TargetTypeSlotPosition,
		TargetID:   &targetID,
		Metadata:   changes,
	})

	return slotPosition, nil
}

func (s *TemplateService) DeleteTemplateSlotPosition(
	ctx context.Context,
	templateID, slotID, slotPositionID int64,
) error {
	if templateID <= 0 || slotID <= 0 || slotPositionID <= 0 {
		return ErrInvalidInput
	}

	if err := s.templateRepo.DeleteSlotPosition(ctx, templateID, slotID, slotPositionID); err != nil {
		return mapTemplateRepositoryError(err)
	}

	targetID := slotPositionID
	audit.Record(ctx, audit.Event{
		Action:     audit.ActionSlotPositionDelete,
		TargetType: audit.TargetTypeSlotPosition,
		TargetID:   &targetID,
		Metadata: map[string]any{
			"template_id": templateID,
			"slot_id":     slotID,
		},
	})

	return nil
}

func (s *TemplateService) ensurePositionExists(ctx context.Context, positionID int64) error {
	_, err := s.positionRepo.GetByID(ctx, positionID)
	switch {
	case errors.Is(err, repository.ErrPositionNotFound):
		return ErrPositionNotFound
	case err != nil:
		return err
	default:
		return nil
	}
}

func normalizeTemplateInput(name, description string) (string, string, error) {
	normalizedName := strings.TrimSpace(name)
	if normalizedName == "" || utf8.RuneCountInString(normalizedName) > maxTemplateNameLength {
		return "", "", ErrInvalidInput
	}

	normalizedDescription := strings.TrimSpace(description)
	if utf8.RuneCountInString(normalizedDescription) > maxTemplateDescriptionLength {
		return "", "", ErrInvalidInput
	}

	return normalizedName, normalizedDescription, nil
}

type normalizedTemplateSlotInput struct {
	Weekdays  []int
	StartTime string
	EndTime   string
}

func normalizeSlotInput(weekdays []int, startTime, endTime string) (*normalizedTemplateSlotInput, error) {
	normalizedWeekdays, err := normalizeWeekdays(weekdays)
	if err != nil {
		return nil, ErrInvalidWeekday
	}

	parsedStartTime, err := time.Parse(timeLayoutHourMinute, startTime)
	if err != nil {
		return nil, ErrInvalidShiftTime
	}
	parsedEndTime, err := time.Parse(timeLayoutHourMinute, endTime)
	if err != nil {
		return nil, ErrInvalidShiftTime
	}
	if !parsedEndTime.After(parsedStartTime) {
		return nil, ErrInvalidShiftTime
	}

	return &normalizedTemplateSlotInput{
		Weekdays:  normalizedWeekdays,
		StartTime: parsedStartTime.Format(timeLayoutHourMinute),
		EndTime:   parsedEndTime.Format(timeLayoutHourMinute),
	}, nil
}

func normalizeWeekdays(weekdays []int) ([]int, error) {
	if len(weekdays) == 0 {
		return nil, ErrInvalidWeekday
	}

	seen := make(map[int]struct{}, len(weekdays))
	for _, weekday := range weekdays {
		if weekday < 1 || weekday > 7 {
			return nil, ErrInvalidWeekday
		}
		seen[weekday] = struct{}{}
	}

	normalized := make([]int, 0, len(seen))
	for weekday := range seen {
		normalized = append(normalized, weekday)
	}
	sort.Ints(normalized)
	return normalized, nil
}

func normalizeRequiredHeadcount(requiredHeadcount int) (int, error) {
	if requiredHeadcount <= 0 {
		return 0, ErrInvalidHeadcount
	}

	return requiredHeadcount, nil
}

func sortTemplateSlots(slots []*model.TemplateSlot) {
	for _, slot := range slots {
		sortTemplateSlotPositions(slot.Positions)
	}

	sort.Slice(slots, func(i, j int) bool {
		if slots[i].StartTime != slots[j].StartTime {
			return slots[i].StartTime < slots[j].StartTime
		}
		if slots[i].EndTime != slots[j].EndTime {
			return slots[i].EndTime < slots[j].EndTime
		}

		return slots[i].ID < slots[j].ID
	})
}

func sortTemplateSlotPositions(slotPositions []*model.TemplateSlotPosition) {
	sort.Slice(slotPositions, func(i, j int) bool {
		if slotPositions[i].PositionID != slotPositions[j].PositionID {
			return slotPositions[i].PositionID < slotPositions[j].PositionID
		}

		return slotPositions[i].ID < slotPositions[j].ID
	})
}

func mapTemplateRepositoryError(err error) error {
	switch {
	case errors.Is(err, repository.ErrTemplateLocked):
		return ErrTemplateLocked
	case errors.Is(err, repository.ErrTemplateNotFound):
		return ErrTemplateNotFound
	case errors.Is(err, repository.ErrTemplateSlotOverlap):
		return ErrTemplateSlotOverlap
	case errors.Is(err, repository.ErrTemplateSlotNotFound):
		return ErrTemplateSlotNotFound
	case errors.Is(err, repository.ErrTemplateSlotPositionNotFound):
		return ErrTemplateSlotPositionNotFound
	default:
		return err
	}
}

func buildTemplateCloneName(name string) string {
	maxBaseRunes := maxTemplateNameLength - utf8.RuneCountInString(templateCloneSuffix)
	if utf8.RuneCountInString(name) <= maxBaseRunes {
		return name + templateCloneSuffix
	}

	var builder strings.Builder
	builder.Grow(maxTemplateNameLength)

	runeCount := 0
	for _, r := range name {
		if runeCount >= maxBaseRunes {
			break
		}
		builder.WriteRune(r)
		runeCount++
	}

	builder.WriteString(templateCloneSuffix)
	return builder.String()
}
