package service

import (
	"time"

	"github.com/jonathanhu237/rota/backend/internal/email"
	"github.com/jonathanhu237/rota/backend/internal/model"
)

func buildPublicationShiftIndex(
	shifts []*model.PublicationShift,
) map[slotPositionKey]*model.PublicationShift {
	index := make(map[slotPositionKey]*model.PublicationShift, len(shifts))
	for _, shift := range shifts {
		if shift == nil {
			continue
		}
		index[slotPositionKey{
			SlotID:     shift.SlotID,
			Weekday:    shift.Weekday,
			PositionID: shift.PositionID,
		}] = shift
	}
	return index
}

func findPublicationShiftBySlotPosition(
	shifts []*model.PublicationShift,
	slotID int64,
	positionID int64,
) *model.PublicationShift {
	for _, shift := range shifts {
		if shift != nil && shift.SlotID == slotID && shift.PositionID == positionID {
			return shift
		}
	}
	return nil
}

func findPublicationShiftForAssignment(
	shiftIndex map[slotPositionKey]*model.PublicationShift,
	assignment *model.Assignment,
) *model.PublicationShift {
	if assignment == nil {
		return nil
	}
	return shiftIndex[slotPositionKey{
		SlotID:     assignment.SlotID,
		Weekday:    assignment.Weekday,
		PositionID: assignment.PositionID,
	}]
}

func findPublicationShiftForParticipant(
	shiftIndex map[slotPositionKey]*model.PublicationShift,
	assignment *model.AssignmentParticipant,
) *model.PublicationShift {
	if assignment == nil {
		return nil
	}
	return shiftIndex[slotPositionKey{
		SlotID:     assignment.SlotID,
		Weekday:    assignment.Weekday,
		PositionID: assignment.PositionID,
	}]
}

func collectUserShifts(
	assignments []*model.AssignmentParticipant,
	shiftIndex map[slotPositionKey]*model.PublicationShift,
	userID int64,
	excludeAssignmentIDs []int64,
	addShifts []*model.PublicationShift,
) []*model.PublicationShift {
	excluded := make(map[int64]struct{}, len(excludeAssignmentIDs))
	for _, id := range excludeAssignmentIDs {
		excluded[id] = struct{}{}
	}

	final := make([]*model.PublicationShift, 0)
	for _, assignment := range assignments {
		if assignment.UserID != userID {
			continue
		}
		if _, skip := excluded[assignment.AssignmentID]; skip {
			continue
		}
		if shift := findPublicationShiftForParticipant(shiftIndex, assignment); shift != nil {
			final = append(final, shift)
		}
	}

	for _, shift := range addShifts {
		if shift != nil {
			final = append(final, shift)
		}
	}

	return final
}

func hasOverlapInSet(shifts []*model.PublicationShift) bool {
	for i := 0; i < len(shifts); i++ {
		for j := i + 1; j < len(shifts); j++ {
			if shifts[i].Weekday != shifts[j].Weekday {
				continue
			}
			if shifts[i].StartTime < shifts[j].EndTime && shifts[j].StartTime < shifts[i].EndTime {
				return true
			}
		}
	}
	return false
}

func toShiftRef(shift *model.PublicationShift) email.ShiftRef {
	return toShiftRefWithOccurrence(shift, nil)
}

func toShiftRefWithOccurrence(shift *model.PublicationShift, occurrenceDate *time.Time) email.ShiftRef {
	if shift == nil {
		return email.ShiftRef{}
	}

	return email.ShiftRef{
		Weekday:        weekdayLabel(shift.Weekday),
		StartTime:      shift.StartTime,
		EndTime:        shift.EndTime,
		PositionName:   shift.PositionName,
		OccurrenceDate: occurrenceDate,
	}
}
