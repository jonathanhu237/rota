package service

import (
	"bytes"
	"context"
	"fmt"
	"sort"
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

type scheduleExportTimeKey struct {
	StartTime string
	EndTime   string
}

type scheduleExportTranslations struct {
	SheetName       string
	TimeHeader      string
	WeekdayHeaders  [7]string
	StatusLabel     string
	ExportedAtLabel string
	VacancyLabel    string
	StateLabels     map[model.PublicationState]string
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

	titleStyle, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 14},
	})
	if err != nil {
		return nil, err
	}
	metadataStyle, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Color: "4B5563"},
	})
	if err != nil {
		return nil, err
	}
	headerStyle, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"E5EEF8"}, Pattern: 1},
		Border:    scheduleExportBorder(),
	})
	if err != nil {
		return nil, err
	}
	cellStyle, err := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Vertical: "top", WrapText: true},
		Border:    scheduleExportBorder(),
	})
	if err != nil {
		return nil, err
	}

	publicationName := ""
	state := model.PublicationStateDraft
	if exportModel.Publication != nil {
		publicationName = exportModel.Publication.Name
		state = exportModel.Publication.State
	}
	if err := setCell(f, sheet, "A1", publicationName); err != nil {
		return nil, err
	}
	if err := f.SetCellStyle(sheet, "A1", "A1", titleStyle); err != nil {
		return nil, err
	}
	if err := setCell(f, sheet, "A2", labels.StatusLabel+": "+labels.StateLabels[state]); err != nil {
		return nil, err
	}
	if err := setCell(f, sheet, "A3", labels.ExportedAtLabel+": "+formatScheduleExportTime(exportModel.ExportedAt, exportModel.Language)); err != nil {
		return nil, err
	}
	if err := f.SetCellStyle(sheet, "A2", "A3", metadataStyle); err != nil {
		return nil, err
	}

	headerRow := 5
	if err := setCell(f, sheet, "A5", labels.TimeHeader); err != nil {
		return nil, err
	}
	for weekday := 1; weekday <= 7; weekday++ {
		cell, err := excelize.CoordinatesToCellName(weekday+1, headerRow)
		if err != nil {
			return nil, err
		}
		if err := setCell(f, sheet, cell, labels.WeekdayHeaders[weekday-1]); err != nil {
			return nil, err
		}
	}
	if err := f.SetCellStyle(sheet, "A5", "H5", headerStyle); err != nil {
		return nil, err
	}

	for i, row := range exportModel.Rows {
		xlsxRow := headerRow + 1 + i
		timeCell, err := excelize.CoordinatesToCellName(1, xlsxRow)
		if err != nil {
			return nil, err
		}
		if err := setCell(f, sheet, timeCell, row.StartTime+"-"+row.EndTime); err != nil {
			return nil, err
		}

		for weekday := 1; weekday <= 7; weekday++ {
			cell, err := excelize.CoordinatesToCellName(weekday+1, xlsxRow)
			if err != nil {
				return nil, err
			}
			if err := setCell(f, sheet, cell, scheduleExportCellValue(row.Cells[weekday], labels.VacancyLabel)); err != nil {
				return nil, err
			}
		}
		if err := f.SetRowHeight(sheet, xlsxRow, 48); err != nil {
			return nil, err
		}
	}

	lastRow := headerRow
	if len(exportModel.Rows) > 0 {
		lastRow = headerRow + len(exportModel.Rows)
	}
	lastCell, err := excelize.CoordinatesToCellName(8, lastRow)
	if err != nil {
		return nil, err
	}
	if len(exportModel.Rows) > 0 {
		if err := f.SetCellStyle(sheet, "A6", lastCell, cellStyle); err != nil {
			return nil, err
		}
	}
	if err := f.SetColWidth(sheet, "A", "A", 14); err != nil {
		return nil, err
	}
	if err := f.SetColWidth(sheet, "B", "H", 24); err != nil {
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

func scheduleExportCellValue(blocks []scheduleExportPositionBlock, vacancyLabel string) string {
	if len(blocks) == 0 {
		return ""
	}

	values := make([]string, 0, len(blocks))
	for _, block := range blocks {
		lines := []string{fmt.Sprintf("%s (%d)", block.PositionName, block.RequiredHeadcount)}
		lines = append(lines, block.AssigneeNames...)
		vacancies := block.RequiredHeadcount - len(block.AssigneeNames)
		for i := 0; i < vacancies; i++ {
			lines = append(lines, vacancyLabel)
		}
		values = append(values, strings.Join(lines, "\n"))
	}

	return strings.Join(values, "\n\n")
}

func scheduleExportLabels(language scheduleExportLanguage) scheduleExportTranslations {
	if language == scheduleExportLanguageZH {
		return scheduleExportTranslations{
			SheetName:       "排班表",
			TimeHeader:      "时间",
			WeekdayHeaders:  [7]string{"周一", "周二", "周三", "周四", "周五", "周六", "周日"},
			StatusLabel:     "状态",
			ExportedAtLabel: "导出时间",
			VacancyLabel:    "空缺",
			StateLabels: map[model.PublicationState]string{
				model.PublicationStateDraft:      "草稿",
				model.PublicationStateCollecting: "收集中",
				model.PublicationStateAssigning:  "排班中",
				model.PublicationStatePublished:  "已发布",
				model.PublicationStateActive:     "生效中",
				model.PublicationStateEnded:      "已结束",
			},
		}
	}

	return scheduleExportTranslations{
		SheetName:       "Roster",
		TimeHeader:      "Time",
		WeekdayHeaders:  [7]string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"},
		StatusLabel:     "Status",
		ExportedAtLabel: "Exported at",
		VacancyLabel:    "Empty",
		StateLabels: map[model.PublicationState]string{
			model.PublicationStateDraft:      "Draft",
			model.PublicationStateCollecting: "Collecting",
			model.PublicationStateAssigning:  "Assigning",
			model.PublicationStatePublished:  "Published",
			model.PublicationStateActive:     "Active",
			model.PublicationStateEnded:      "Ended",
		},
	}
}

func formatScheduleExportTime(t time.Time, language scheduleExportLanguage) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04")
}

func scheduleExportBorder() []excelize.Border {
	return []excelize.Border{
		{Type: "left", Color: "CBD5E1", Style: 1},
		{Type: "top", Color: "CBD5E1", Style: 1},
		{Type: "right", Color: "CBD5E1", Style: 1},
		{Type: "bottom", Color: "CBD5E1", Style: 1},
	}
}
