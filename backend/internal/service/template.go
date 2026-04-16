package service

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

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
	ErrInvalidHeadcount      = model.ErrInvalidHeadcount
	ErrInvalidShiftTime      = model.ErrInvalidShiftTime
	ErrInvalidWeekday        = model.ErrInvalidWeekday
	ErrTemplateLocked        = model.ErrTemplateLocked
	ErrTemplateNotFound      = model.ErrTemplateNotFound
	ErrTemplateShiftNotFound = model.ErrTemplateShiftNotFound
)

type templateRepository interface {
	ListPaginated(ctx context.Context, params repository.ListTemplatesParams) ([]*model.Template, int, error)
	GetByID(ctx context.Context, id int64) (*model.Template, error)
	Create(ctx context.Context, params repository.CreateTemplateParams) (*model.Template, error)
	Update(ctx context.Context, params repository.UpdateTemplateParams) (*model.Template, error)
	Delete(ctx context.Context, id int64) error
	Clone(ctx context.Context, id int64, name string) (*model.Template, error)
	CreateShift(ctx context.Context, params repository.CreateTemplateShiftParams) (*model.TemplateShift, error)
	UpdateShift(ctx context.Context, params repository.UpdateTemplateShiftParams) (*model.TemplateShift, error)
	DeleteShift(ctx context.Context, templateID, shiftID int64) error
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

type CreateTemplateShiftInput struct {
	TemplateID        int64
	Weekday           int
	StartTime         string
	EndTime           string
	PositionID        int64
	RequiredHeadcount int
}

type UpdateTemplateShiftInput struct {
	TemplateID        int64
	ShiftID           int64
	Weekday           int
	StartTime         string
	EndTime           string
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

	sortTemplateShifts(template.Shifts)
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

	template, err := s.templateRepo.Update(ctx, repository.UpdateTemplateParams{
		ID:          input.ID,
		Name:        name,
		Description: description,
	})
	if err != nil {
		return nil, mapTemplateRepositoryError(err)
	}

	return template, nil
}

func (s *TemplateService) DeleteTemplate(ctx context.Context, id int64) error {
	if id <= 0 {
		return ErrInvalidInput
	}

	if err := s.templateRepo.Delete(ctx, id); err != nil {
		return mapTemplateRepositoryError(err)
	}

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

	sortTemplateShifts(clone.Shifts)
	return clone, nil
}

func (s *TemplateService) CreateTemplateShift(ctx context.Context, input CreateTemplateShiftInput) (*model.TemplateShift, error) {
	if input.TemplateID <= 0 || input.PositionID <= 0 {
		return nil, ErrInvalidInput
	}

	normalizedShiftInput, err := normalizeShiftInput(
		input.Weekday,
		input.StartTime,
		input.EndTime,
		input.RequiredHeadcount,
	)
	if err != nil {
		return nil, err
	}

	if err := s.ensurePositionExists(ctx, input.PositionID); err != nil {
		return nil, err
	}

	shift, err := s.templateRepo.CreateShift(ctx, repository.CreateTemplateShiftParams{
		TemplateID:        input.TemplateID,
		Weekday:           normalizedShiftInput.Weekday,
		StartTime:         normalizedShiftInput.StartTime,
		EndTime:           normalizedShiftInput.EndTime,
		PositionID:        input.PositionID,
		RequiredHeadcount: normalizedShiftInput.RequiredHeadcount,
	})
	if err != nil {
		return nil, mapTemplateRepositoryError(err)
	}

	return shift, nil
}

func (s *TemplateService) UpdateTemplateShift(ctx context.Context, input UpdateTemplateShiftInput) (*model.TemplateShift, error) {
	if input.TemplateID <= 0 || input.ShiftID <= 0 || input.PositionID <= 0 {
		return nil, ErrInvalidInput
	}

	normalizedShiftInput, err := normalizeShiftInput(
		input.Weekday,
		input.StartTime,
		input.EndTime,
		input.RequiredHeadcount,
	)
	if err != nil {
		return nil, err
	}

	if err := s.ensurePositionExists(ctx, input.PositionID); err != nil {
		return nil, err
	}

	shift, err := s.templateRepo.UpdateShift(ctx, repository.UpdateTemplateShiftParams{
		TemplateID:        input.TemplateID,
		ShiftID:           input.ShiftID,
		Weekday:           normalizedShiftInput.Weekday,
		StartTime:         normalizedShiftInput.StartTime,
		EndTime:           normalizedShiftInput.EndTime,
		PositionID:        input.PositionID,
		RequiredHeadcount: normalizedShiftInput.RequiredHeadcount,
	})
	if err != nil {
		return nil, mapTemplateRepositoryError(err)
	}

	return shift, nil
}

func (s *TemplateService) DeleteTemplateShift(ctx context.Context, templateID, shiftID int64) error {
	if templateID <= 0 || shiftID <= 0 {
		return ErrInvalidInput
	}

	if err := s.templateRepo.DeleteShift(ctx, templateID, shiftID); err != nil {
		return mapTemplateRepositoryError(err)
	}

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

type normalizedTemplateShiftInput struct {
	Weekday           int
	StartTime         string
	EndTime           string
	RequiredHeadcount int
}

func normalizeShiftInput(weekday int, startTime, endTime string, requiredHeadcount int) (*normalizedTemplateShiftInput, error) {
	if weekday < 1 || weekday > 7 {
		return nil, ErrInvalidWeekday
	}
	if requiredHeadcount <= 0 {
		return nil, ErrInvalidHeadcount
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

	return &normalizedTemplateShiftInput{
		Weekday:           weekday,
		StartTime:         parsedStartTime.Format(timeLayoutHourMinute),
		EndTime:           parsedEndTime.Format(timeLayoutHourMinute),
		RequiredHeadcount: requiredHeadcount,
	}, nil
}

func sortTemplateShifts(shifts []*model.TemplateShift) {
	sort.Slice(shifts, func(i, j int) bool {
		if shifts[i].Weekday != shifts[j].Weekday {
			return shifts[i].Weekday < shifts[j].Weekday
		}
		if shifts[i].StartTime != shifts[j].StartTime {
			return shifts[i].StartTime < shifts[j].StartTime
		}

		return shifts[i].ID < shifts[j].ID
	})
}

func mapTemplateRepositoryError(err error) error {
	switch {
	case errors.Is(err, repository.ErrTemplateLocked):
		return ErrTemplateLocked
	case errors.Is(err, repository.ErrTemplateNotFound):
		return ErrTemplateNotFound
	case errors.Is(err, repository.ErrTemplateShiftNotFound):
		return ErrTemplateShiftNotFound
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
