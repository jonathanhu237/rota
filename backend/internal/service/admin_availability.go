package service

import (
	"context"
	"sort"

	"github.com/jonathanhu237/rota/backend/internal/audit"
	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/jonathanhu237/rota/backend/internal/repository"
)

type ListAdminAvailabilityInput struct {
	PublicationID int64
	Page          int
	PageSize      int
	Search        string
}

type AdminAvailabilityBoardResult struct {
	Publication *model.Publication
	Employees   []*AdminAvailabilityEmployee
	Page        int
	PageSize    int
	Total       int
	TotalPages  int
}

type AdminAvailabilityEmployee struct {
	UserID         int64
	Name           string
	Email          string
	Positions      []*model.Position
	SubmittedCount int
}

type GetAdminAvailabilityDetailInput struct {
	PublicationID int64
	UserID        int64
}

type AdminAvailabilityDetailResult struct {
	Publication *model.Publication
	User        *model.User
	Positions   []*model.Position
	Slots       []*AdminAvailabilitySlot
	Submissions []model.SlotRef
	Cells       []AdminAvailabilityCell
}

type AdminAvailabilitySlot struct {
	Slot      *model.TemplateSlot
	Positions []AdminAvailabilitySlotPosition
}

type AdminAvailabilitySlotPosition struct {
	Position          *model.Position
	RequiredHeadcount int
}

type AdminAvailabilityCell struct {
	SlotID    int64
	Weekday   int
	Eligible  bool
	Submitted bool
}

type ReplaceAdminAvailabilityInput struct {
	PublicationID int64
	UserID        int64
	Submissions   []model.SlotRef
}

func (s *PublicationService) ListAdminAvailability(
	ctx context.Context,
	input ListAdminAvailabilityInput,
) (*AdminAvailabilityBoardResult, error) {
	if input.PublicationID <= 0 {
		return nil, ErrInvalidInput
	}

	page, pageSize, err := normalizePagination(input.Page, input.PageSize)
	if err != nil {
		return nil, err
	}

	publication, err := s.publicationRepo.GetByID(ctx, input.PublicationID)
	if err != nil {
		return nil, mapPublicationRepositoryError(err)
	}

	employees, total, err := s.publicationRepo.ListAdminAvailabilityEmployees(ctx, repository.ListAdminAvailabilityEmployeesParams{
		PublicationID: input.PublicationID,
		Offset:        (page - 1) * pageSize,
		Limit:         pageSize,
		Search:        input.Search,
	})
	if err != nil {
		return nil, mapPublicationRepositoryError(err)
	}

	resultEmployees := make([]*AdminAvailabilityEmployee, 0, len(employees))
	for _, employee := range employees {
		if employee == nil {
			continue
		}
		resultEmployees = append(resultEmployees, &AdminAvailabilityEmployee{
			UserID:         employee.UserID,
			Name:           employee.Name,
			Email:          employee.Email,
			Positions:      clonePositions(employee.Positions),
			SubmittedCount: employee.SubmittedCount,
		})
	}

	totalPages := 0
	if total > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}

	return &AdminAvailabilityBoardResult{
		Publication: publicationWithEffectiveState(publication, s.clock.Now()),
		Employees:   resultEmployees,
		Page:        page,
		PageSize:    pageSize,
		Total:       total,
		TotalPages:  totalPages,
	}, nil
}

func (s *PublicationService) GetAdminAvailabilityDetail(
	ctx context.Context,
	input GetAdminAvailabilityDetailInput,
) (*AdminAvailabilityDetailResult, error) {
	if input.PublicationID <= 0 || input.UserID <= 0 {
		return nil, ErrInvalidInput
	}

	publication, user, positions, slots, submissions, cells, err := s.loadAdminAvailabilityContext(
		ctx,
		input.PublicationID,
		input.UserID,
	)
	if err != nil {
		return nil, err
	}

	return &AdminAvailabilityDetailResult{
		Publication: publication,
		User:        user,
		Positions:   positions,
		Slots:       slots,
		Submissions: submissions,
		Cells:       cells,
	}, nil
}

func (s *PublicationService) ReplaceAdminAvailability(
	ctx context.Context,
	input ReplaceAdminAvailabilityInput,
) (*AdminAvailabilityDetailResult, error) {
	if input.PublicationID <= 0 || input.UserID <= 0 {
		return nil, ErrInvalidInput
	}

	normalized, err := normalizeAdminAvailabilityTarget(input.Submissions)
	if err != nil {
		return nil, err
	}

	now := s.clock.Now()
	replaceResult, err := s.publicationRepo.ReplaceAdminAvailabilitySubmissions(ctx, repository.ReplaceAdminAvailabilitySubmissionsParams{
		PublicationID: input.PublicationID,
		UserID:        input.UserID,
		Submissions:   normalized,
		CreatedAt:     now,
		Now:           now,
	})
	if err != nil {
		return nil, mapPublicationRepositoryError(err)
	}

	detail, err := s.GetAdminAvailabilityDetail(ctx, GetAdminAvailabilityDetailInput{
		PublicationID: input.PublicationID,
		UserID:        input.UserID,
	})
	if err != nil {
		return nil, err
	}

	recordAdminAvailabilityAudit(ctx, audit.ActionAvailabilityAdminDelete, input.PublicationID, input.UserID, replaceResult.Removed)
	recordAdminAvailabilityAudit(ctx, audit.ActionAvailabilityAdminCreate, input.PublicationID, input.UserID, replaceResult.Added)

	return detail, nil
}

func (s *PublicationService) loadAdminAvailabilityContext(
	ctx context.Context,
	publicationID, userID int64,
) (
	*model.Publication,
	*model.User,
	[]*model.Position,
	[]*AdminAvailabilitySlot,
	[]model.SlotRef,
	[]AdminAvailabilityCell,
	error,
) {
	publication, err := s.publicationRepo.GetByID(ctx, publicationID)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, mapPublicationRepositoryError(err)
	}
	effectivePublication := publicationWithEffectiveState(publication, s.clock.Now())

	user, positions, err := s.publicationRepo.GetAdminAvailabilityUser(ctx, publicationID, userID)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, mapPublicationRepositoryError(err)
	}

	shifts, err := s.publicationRepo.ListPublicationShifts(ctx, publicationID)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, mapPublicationRepositoryError(err)
	}
	slots := adminAvailabilitySlotsFromShifts(shifts)

	submissions, err := s.publicationRepo.ListSubmissionSlots(ctx, publicationID, userID)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, mapPublicationRepositoryError(err)
	}

	cells := adminAvailabilityCells(slots, positions, submissions)
	return effectivePublication, cloneUser(user), clonePositions(positions), slots, submissions, cells, nil
}

func normalizeAdminAvailabilityTarget(
	submissions []model.SlotRef,
) ([]model.SlotRef, error) {
	seen := make(map[model.SlotRef]struct{}, len(submissions))
	normalized := make([]model.SlotRef, 0, len(submissions))
	for _, submission := range submissions {
		if submission.SlotID <= 0 || submission.Weekday < 1 || submission.Weekday > 7 {
			return nil, ErrInvalidInput
		}
		if _, ok := seen[submission]; ok {
			continue
		}
		seen[submission] = struct{}{}
		normalized = append(normalized, submission)
	}

	sortSlotRefsForService(normalized)
	return normalized, nil
}

func adminAvailabilitySlotsFromShifts(shifts []*model.PublicationShift) []*AdminAvailabilitySlot {
	slots := make([]*AdminAvailabilitySlot, 0)
	slotByRef := make(map[model.SlotRef]*AdminAvailabilitySlot)
	for _, shift := range shifts {
		if shift == nil {
			continue
		}
		ref := model.SlotRef{SlotID: shift.SlotID, Weekday: shift.Weekday}
		slot := slotByRef[ref]
		if slot == nil {
			slot = &AdminAvailabilitySlot{
				Slot: &model.TemplateSlot{
					ID:         shift.SlotID,
					TemplateID: shift.TemplateID,
					Weekdays:   []int{shift.Weekday},
					StartTime:  shift.StartTime,
					EndTime:    shift.EndTime,
					CreatedAt:  shift.CreatedAt,
					UpdatedAt:  shift.UpdatedAt,
				},
				Positions: make([]AdminAvailabilitySlotPosition, 0, 1),
			}
			slotByRef[ref] = slot
			slots = append(slots, slot)
		}
		slot.Positions = append(slot.Positions, AdminAvailabilitySlotPosition{
			Position: &model.Position{
				ID:   shift.PositionID,
				Name: shift.PositionName,
			},
			RequiredHeadcount: shift.RequiredHeadcount,
		})
	}

	sort.Slice(slots, func(i, j int) bool {
		left := slots[i].Slot
		right := slots[j].Slot
		if left.Weekdays[0] != right.Weekdays[0] {
			return left.Weekdays[0] < right.Weekdays[0]
		}
		if left.StartTime != right.StartTime {
			return left.StartTime < right.StartTime
		}
		return left.ID < right.ID
	})
	for _, slot := range slots {
		sort.Slice(slot.Positions, func(i, j int) bool {
			return slot.Positions[i].Position.ID < slot.Positions[j].Position.ID
		})
	}

	return slots
}

func adminAvailabilityCells(
	slots []*AdminAvailabilitySlot,
	positions []*model.Position,
	submissions []model.SlotRef,
) []AdminAvailabilityCell {
	userPositionIDs := make(map[int64]struct{}, len(positions))
	for _, position := range positions {
		if position != nil {
			userPositionIDs[position.ID] = struct{}{}
		}
	}

	submitted := make(map[model.SlotRef]struct{}, len(submissions))
	for _, submission := range submissions {
		submitted[submission] = struct{}{}
	}

	cells := make([]AdminAvailabilityCell, 0, len(slots))
	for _, slot := range slots {
		if slot == nil || slot.Slot == nil {
			continue
		}
		weekday := publicationSlotWeekdayForService(slot.Slot)
		ref := model.SlotRef{SlotID: slot.Slot.ID, Weekday: weekday}
		eligible := false
		for _, slotPosition := range slot.Positions {
			if slotPosition.Position == nil {
				continue
			}
			if _, ok := userPositionIDs[slotPosition.Position.ID]; ok {
				eligible = true
				break
			}
		}
		_, isSubmitted := submitted[ref]
		cells = append(cells, AdminAvailabilityCell{
			SlotID:    ref.SlotID,
			Weekday:   ref.Weekday,
			Eligible:  eligible,
			Submitted: isSubmitted,
		})
	}

	return cells
}

func recordAdminAvailabilityAudit(ctx context.Context, action string, publicationID, userID int64, slots []model.SlotRef) {
	for _, slot := range slots {
		audit.Record(ctx, audit.Event{
			Action:     action,
			TargetType: audit.TargetTypeAvailabilitySubmission,
			Metadata: map[string]any{
				"publication_id": publicationID,
				"user_id":        userID,
				"slot_id":        slot.SlotID,
				"weekday":        slot.Weekday,
			},
		})
	}
}

func clonePositions(positions []*model.Position) []*model.Position {
	cloned := make([]*model.Position, 0, len(positions))
	for _, position := range positions {
		if position == nil {
			continue
		}
		positionCopy := *position
		cloned = append(cloned, &positionCopy)
	}
	return cloned
}

func cloneUser(user *model.User) *model.User {
	if user == nil {
		return nil
	}
	cloned := *user
	return &cloned
}

func publicationSlotWeekdayForService(slot *model.TemplateSlot) int {
	if slot == nil || len(slot.Weekdays) == 0 {
		return 0
	}
	return slot.Weekdays[0]
}

func sortSlotRefsForService(slots []model.SlotRef) {
	sort.Slice(slots, func(i, j int) bool {
		if slots[i].Weekday != slots[j].Weekday {
			return slots[i].Weekday < slots[j].Weekday
		}
		return slots[i].SlotID < slots[j].SlotID
	})
}
