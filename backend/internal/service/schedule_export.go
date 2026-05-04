package service

import (
	"bytes"
	"context"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/jonathanhu237/rota/backend/internal/repository"
	"github.com/xuri/excelize/v2"
)

type ExportScheduleOptions struct {
	Language string
}

type scheduleExportLanguage string

const (
	scheduleExportLanguageEN scheduleExportLanguage = "en"
	scheduleExportLanguageZH scheduleExportLanguage = "zh"

	scheduleExportBodyRowHeight   = 28.0
	scheduleExportSpacerRowHeight = 18.0
)

type scheduleExportModel struct {
	Publication *model.Publication
	Language    scheduleExportLanguage
	ExportedAt  time.Time
	Rows        []scheduleExportTimeRow
}

type scheduleExportTimeRow struct {
	StartTime string
	EndTime   string
	Cells     map[int][]scheduleExportPositionBlock
}

type scheduleExportPositionBlock struct {
	SlotID            int64
	PositionID        int64
	PositionName      string
	RequiredHeadcount int
	AssigneeNames     []string
}

type scheduleExportSeatRow struct {
	PositionID     int64
	PositionName   string
	AssigneesByDay map[int]string
}

type scheduleExportTimeKey struct {
	StartTime string
	EndTime   string
}

type scheduleExportTranslations struct {
	SheetName      string
	TimeHeader     string
	PositionHeader string
	WeekdayHeaders [7]string
}

func (s *PublicationService) ExportScheduleXLSX(
	ctx context.Context,
	publicationID int64,
	viewer *model.User,
	opts ExportScheduleOptions,
) ([]byte, error) {
	if publicationID <= 0 || viewer == nil || viewer.ID <= 0 {
		return nil, ErrInvalidInput
	}

	language, err := resolveScheduleExportLanguage(opts.Language, viewer)
	if err != nil {
		return nil, err
	}

	now := s.clock.Now()
	publication, err := s.publicationRepo.GetByID(ctx, publicationID)
	if err != nil {
		return nil, mapPublicationRepositoryError(err)
	}

	effectiveState := model.ResolvePublicationState(publication, now)
	if !canExportSchedule(viewer, effectiveState) {
		return nil, ErrPublicationNotActive
	}

	boardView, err := s.publicationRepo.GetAssignmentBoardView(ctx, publicationID)
	if err != nil {
		return nil, mapPublicationRepositoryError(err)
	}

	return renderScheduleExportXLSX(scheduleExportModel{
		Publication: publicationWithEffectiveState(publication, now),
		Language:    language,
		ExportedAt:  now,
		Rows:        buildScheduleExportRows(boardView),
	})
}

func canExportSchedule(viewer *model.User, state model.PublicationState) bool {
	if viewer != nil && viewer.IsAdmin {
		return state == model.PublicationStateAssigning ||
			state == model.PublicationStatePublished ||
			state == model.PublicationStateActive
	}

	return state == model.PublicationStatePublished ||
		state == model.PublicationStateActive
}

func resolveScheduleExportLanguage(raw string, viewer *model.User) (scheduleExportLanguage, error) {
	language := strings.TrimSpace(strings.ToLower(raw))
	if language == "" && viewer != nil && viewer.LanguagePreference != nil {
		language = string(*viewer.LanguagePreference)
	}
	if language == "" {
		language = string(scheduleExportLanguageEN)
	}

	switch language {
	case string(scheduleExportLanguageEN):
		return scheduleExportLanguageEN, nil
	case string(scheduleExportLanguageZH):
		return scheduleExportLanguageZH, nil
	default:
		return "", ErrInvalidInput
	}
}

func buildScheduleExportRows(
	boardView map[repository.AssignmentBoardSlotKey]*repository.AssignmentBoardSlotView,
) []scheduleExportTimeRow {
	rowsByTime := make(map[scheduleExportTimeKey]*scheduleExportTimeRow)
	for slotKey, slotView := range boardView {
		if slotView == nil || slotView.Slot == nil {
			continue
		}

		timeKey := scheduleExportTimeKey{
			StartTime: slotView.Slot.StartTime,
			EndTime:   slotView.Slot.EndTime,
		}
		row := rowsByTime[timeKey]
		if row == nil {
			row = &scheduleExportTimeRow{
				StartTime: timeKey.StartTime,
				EndTime:   timeKey.EndTime,
				Cells:     make(map[int][]scheduleExportPositionBlock),
			}
			rowsByTime[timeKey] = row
		}

		positionIDs := make([]int64, 0, len(slotView.Positions))
		for positionID := range slotView.Positions {
			positionIDs = append(positionIDs, positionID)
		}
		sort.Slice(positionIDs, func(i, j int) bool {
			return positionIDs[i] < positionIDs[j]
		})

		for _, positionID := range positionIDs {
			positionView := slotView.Positions[positionID]
			if positionView == nil || positionView.Position == nil {
				continue
			}

			assignments := cloneAssignmentParticipants(positionView.Assignments)
			names := make([]string, 0, len(assignments))
			for _, assignment := range assignments {
				if assignment == nil {
					continue
				}
				names = append(names, assignment.Name)
			}

			row.Cells[slotKey.Weekday] = append(row.Cells[slotKey.Weekday], scheduleExportPositionBlock{
				SlotID:            slotKey.SlotID,
				PositionID:        positionView.Position.ID,
				PositionName:      positionView.Position.Name,
				RequiredHeadcount: positionView.RequiredHeadcount,
				AssigneeNames:     names,
			})
		}
	}

	rows := make([]scheduleExportTimeRow, 0, len(rowsByTime))
	for _, row := range rowsByTime {
		for weekday := range row.Cells {
			sort.Slice(row.Cells[weekday], func(i, j int) bool {
				left := row.Cells[weekday][i]
				right := row.Cells[weekday][j]
				switch {
				case left.SlotID != right.SlotID:
					return left.SlotID < right.SlotID
				case left.PositionID != right.PositionID:
					return left.PositionID < right.PositionID
				default:
					return left.PositionName < right.PositionName
				}
			})
		}
		rows = append(rows, *row)
	}

	sort.Slice(rows, func(i, j int) bool {
		switch {
		case rows[i].StartTime != rows[j].StartTime:
			return rows[i].StartTime < rows[j].StartTime
		default:
			return rows[i].EndTime < rows[j].EndTime
		}
	})

	return rows
}

func renderScheduleExportXLSX(exportModel scheduleExportModel) ([]byte, error) {
	labels := scheduleExportLabels(exportModel.Language)
	f := excelize.NewFile()
	defer f.Close()

	sheet := labels.SheetName
	defaultSheet := f.GetSheetName(0)
	if defaultSheet == "" {
		defaultSheet = "Sheet1"
	}
	if err := f.SetSheetName(defaultSheet, sheet); err != nil {
		return nil, err
	}

	headerStyle, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "111827", Size: 12},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"D9EAF7"}, Pattern: 1},
		Border:    scheduleExportTableBorder("B7C7D9"),
	})
	if err != nil {
		return nil, err
	}
	timeStyle, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Color: "111827", Size: 12},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"EAF4F8"}, Pattern: 1},
		Border:    scheduleExportTableBorder("B7C7D9"),
	})
	if err != nil {
		return nil, err
	}
	bodyStyle, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Color: "111827", Size: 12},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"FFFFFF"}, Pattern: 1},
		Border:    scheduleExportTableBorder("D6DEE8"),
	})
	if err != nil {
		return nil, err
	}
	bodyAltStyle, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Color: "111827", Size: 12},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"F7FAFC"}, Pattern: 1},
		Border:    scheduleExportTableBorder("D6DEE8"),
	})
	if err != nil {
		return nil, err
	}
	spacerStyle, err := f.NewStyle(&excelize.Style{
		Fill:   excelize.Fill{Type: "pattern", Color: []string{"FFFFFF"}, Pattern: 1},
		Border: scheduleExportTableBorder("E5E7EB"),
	})
	if err != nil {
		return nil, err
	}

	headerRow := 1
	if err := setCell(f, sheet, "A1", labels.TimeHeader); err != nil {
		return nil, err
	}
	if err := setCell(f, sheet, "B1", labels.PositionHeader); err != nil {
		return nil, err
	}
	for weekday := 1; weekday <= 7; weekday++ {
		cell, err := excelize.CoordinatesToCellName(weekday+2, headerRow)
		if err != nil {
			return nil, err
		}
		if err := setCell(f, sheet, cell, labels.WeekdayHeaders[weekday-1]); err != nil {
			return nil, err
		}
	}
	if err := f.SetCellStyle(sheet, "A1", "I1", headerStyle); err != nil {
		return nil, err
	}
	if err := f.SetRowHeight(sheet, headerRow, 34); err != nil {
		return nil, err
	}

	xlsxRow := headerRow + 1
	for timeIndex, row := range exportModel.Rows {
		seatRows := buildScheduleExportSeatRows(row)
		if len(seatRows) == 0 {
			seatRows = []scheduleExportSeatRow{{
				AssigneesByDay: make(map[int]string),
			}}
		}

		blockStartRow := xlsxRow
		for seatIndex, seatRow := range seatRows {
			timeCell, err := excelize.CoordinatesToCellName(1, xlsxRow)
			if err != nil {
				return nil, err
			}
			if seatIndex == 0 {
				if err := setCell(f, sheet, timeCell, row.StartTime+"-"+row.EndTime); err != nil {
					return nil, err
				}
			}

			positionCell, err := excelize.CoordinatesToCellName(2, xlsxRow)
			if err != nil {
				return nil, err
			}
			if err := setCell(f, sheet, positionCell, seatRow.PositionName); err != nil {
				return nil, err
			}

			for weekday := 1; weekday <= 7; weekday++ {
				cell, err := excelize.CoordinatesToCellName(weekday+2, xlsxRow)
				if err != nil {
					return nil, err
				}
				if err := setCell(f, sheet, cell, seatRow.AssigneesByDay[weekday]); err != nil {
					return nil, err
				}
			}

			rowStyle := bodyStyle
			if seatIndex%2 == 1 {
				rowStyle = bodyAltStyle
			}
			lastCell, err := excelize.CoordinatesToCellName(9, xlsxRow)
			if err != nil {
				return nil, err
			}
			if err := f.SetCellStyle(sheet, positionCell, lastCell, rowStyle); err != nil {
				return nil, err
			}
			if err := f.SetCellStyle(sheet, timeCell, timeCell, timeStyle); err != nil {
				return nil, err
			}
			if err := f.SetRowHeight(sheet, xlsxRow, scheduleExportBodyRowHeight); err != nil {
				return nil, err
			}
			xlsxRow++
		}

		if len(seatRows) > 1 {
			startCell, err := excelize.CoordinatesToCellName(1, blockStartRow)
			if err != nil {
				return nil, err
			}
			endCell, err := excelize.CoordinatesToCellName(1, xlsxRow-1)
			if err != nil {
				return nil, err
			}
			if err := f.MergeCell(sheet, startCell, endCell); err != nil {
				return nil, err
			}
			if err := f.SetCellStyle(sheet, startCell, endCell, timeStyle); err != nil {
				return nil, err
			}
		}

		if timeIndex < len(exportModel.Rows)-1 {
			lastSpacerCell, err := excelize.CoordinatesToCellName(9, xlsxRow)
			if err != nil {
				return nil, err
			}
			if err := f.SetCellStyle(sheet, "A"+strconv.Itoa(xlsxRow), lastSpacerCell, spacerStyle); err != nil {
				return nil, err
			}
			if err := f.SetRowHeight(sheet, xlsxRow, scheduleExportSpacerRowHeight); err != nil {
				return nil, err
			}
			xlsxRow++
		}
	}

	if err := f.SetColWidth(sheet, "A", "A", 18); err != nil {
		return nil, err
	}
	if err := f.SetColWidth(sheet, "B", "B", 16); err != nil {
		return nil, err
	}
	if err := f.SetColWidth(sheet, "C", "I", 18); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func setCell(f *excelize.File, sheet, cell string, value any) error {
	return f.SetCellValue(sheet, cell, value)
}

func buildScheduleExportSeatRows(row scheduleExportTimeRow) []scheduleExportSeatRow {
	blocksByPosition := make(map[int64]map[int]scheduleExportPositionBlock)
	positionNames := make(map[int64]string)
	maxSeatsByPosition := make(map[int64]int)

	for weekday, blocks := range row.Cells {
		for _, block := range blocks {
			if block.PositionID == 0 {
				continue
			}
			if blocksByPosition[block.PositionID] == nil {
				blocksByPosition[block.PositionID] = make(map[int]scheduleExportPositionBlock)
			}
			blocksByPosition[block.PositionID][weekday] = block
			positionNames[block.PositionID] = block.PositionName
			maxSeatsByPosition[block.PositionID] = max(maxSeatsByPosition[block.PositionID], max(block.RequiredHeadcount, len(block.AssigneeNames)))
		}
	}

	positionIDs := make([]int64, 0, len(blocksByPosition))
	for positionID := range blocksByPosition {
		positionIDs = append(positionIDs, positionID)
	}
	sort.Slice(positionIDs, func(i, j int) bool { return positionIDs[i] < positionIDs[j] })

	rows := make([]scheduleExportSeatRow, 0)
	for _, positionID := range positionIDs {
		seatCount := maxSeatsByPosition[positionID]
		if seatCount < 1 {
			seatCount = 1
		}
		for seatIndex := 0; seatIndex < seatCount; seatIndex++ {
			seatRow := scheduleExportSeatRow{
				PositionID:     positionID,
				PositionName:   positionNames[positionID],
				AssigneesByDay: make(map[int]string),
			}
			for weekday := 1; weekday <= 7; weekday++ {
				block, ok := blocksByPosition[positionID][weekday]
				if !ok || seatIndex >= len(block.AssigneeNames) {
					continue
				}
				seatRow.AssigneesByDay[weekday] = block.AssigneeNames[seatIndex]
			}
			rows = append(rows, seatRow)
		}
	}

	return rows
}

func scheduleExportLabels(language scheduleExportLanguage) scheduleExportTranslations {
	if language == scheduleExportLanguageZH {
		return scheduleExportTranslations{
			SheetName:      "排班表",
			TimeHeader:     "时间",
			PositionHeader: "岗位",
			WeekdayHeaders: [7]string{"星期一", "星期二", "星期三", "星期四", "星期五", "星期六", "星期日"},
		}
	}

	return scheduleExportTranslations{
		SheetName:      "Roster",
		TimeHeader:     "Time",
		PositionHeader: "Position",
		WeekdayHeaders: [7]string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"},
	}
}

func scheduleExportTableBorder(color string) []excelize.Border {
	return []excelize.Border{
		{Type: "left", Color: color, Style: 1},
		{Type: "top", Color: color, Style: 1},
		{Type: "right", Color: color, Style: 1},
		{Type: "bottom", Color: color, Style: 1},
	}
}
