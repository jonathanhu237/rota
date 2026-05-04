package service

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/jonathanhu237/rota/backend/internal/repository"
	"github.com/xuri/excelize/v2"
)

func TestRenderScheduleExportXLSX(t *testing.T) {
	t.Run("renders localized workbook content without emails", func(t *testing.T) {
		t.Parallel()

		board := map[repository.AssignmentBoardSlotKey]*repository.AssignmentBoardSlotView{
			{SlotID: 22, Weekday: 2}: {
				Slot: &model.TemplateSlot{
					ID:        22,
					StartTime: "13:00",
					EndTime:   "17:00",
				},
				Positions: map[int64]*repository.AssignmentBoardPositionView{
					102: {
						Position:          &model.Position{ID: 102, Name: "Cashier"},
						RequiredHeadcount: 2,
					},
				},
			},
			{SlotID: 21, Weekday: 1}: {
				Slot: &model.TemplateSlot{
					ID:        21,
					StartTime: "09:00",
					EndTime:   "12:00",
				},
				Positions: map[int64]*repository.AssignmentBoardPositionView{
					101: {
						Position:          &model.Position{ID: 101, Name: "Front Desk"},
						RequiredHeadcount: 3,
						Assignments: []*model.AssignmentParticipant{
							{UserID: 8, Name: "Bob", Email: "bob@example.com"},
							{UserID: 7, Name: "Alice", Email: "alice@example.com"},
						},
					},
				},
			},
		}

		workbook, err := renderScheduleExportXLSX(scheduleExportModel{
			Publication: &model.Publication{Name: "Spring Rota", State: model.PublicationStatePublished},
			Language:    scheduleExportLanguageEN,
			ExportedAt:  time.Date(2026, 5, 4, 15, 30, 0, 0, time.UTC),
			Rows:        buildScheduleExportRows(board),
		})
		if err != nil {
			t.Fatalf("renderScheduleExportXLSX returned error: %v", err)
		}

		file := openScheduleExportWorkbook(t, workbook)
		sheets := file.GetSheetList()
		if len(sheets) != 1 || sheets[0] != "Roster" {
			t.Fatalf("expected one Roster sheet, got %+v", sheets)
		}
		assertCellValue(t, file, "Roster", "A1", "Spring Rota")
		assertCellValue(t, file, "Roster", "A2", "Status: Published")
		assertCellValue(t, file, "Roster", "A3", "Exported at: 2026-05-04 15:30")
		for cell, want := range map[string]string{
			"A5": "Time",
			"B5": "Mon",
			"C5": "Tue",
			"D5": "Wed",
			"E5": "Thu",
			"F5": "Fri",
			"G5": "Sat",
			"H5": "Sun",
		} {
			assertCellValue(t, file, "Roster", cell, want)
		}
		assertCellValue(t, file, "Roster", "A6", "09:00-12:00")
		assertCellValue(t, file, "Roster", "A7", "13:00-17:00")

		mondayCell, err := file.GetCellValue("Roster", "B6")
		if err != nil {
			t.Fatalf("read B6: %v", err)
		}
		for _, want := range []string{"Front Desk (3)", "Alice", "Bob", "Empty"} {
			if !strings.Contains(mondayCell, want) {
				t.Fatalf("expected B6 to contain %q, got %q", want, mondayCell)
			}
		}
		if strings.Contains(mondayCell, "alice@example.com") || strings.Contains(mondayCell, "bob@example.com") {
			t.Fatalf("expected emails to be omitted, got %q", mondayCell)
		}
		lines := strings.Split(mondayCell, "\n")
		if countLines(lines, "Alice") != 1 || countLines(lines, "Bob") != 1 || countLines(lines, "Empty") != 1 {
			t.Fatalf("expected Alice, Bob, and one vacancy on separate lines, got %q", mondayCell)
		}

		tuesdayCell, err := file.GetCellValue("Roster", "C7")
		if err != nil {
			t.Fatalf("read C7: %v", err)
		}
		if !strings.Contains(tuesdayCell, "Cashier (2)") || countLines(strings.Split(tuesdayCell, "\n"), "Empty") != 2 {
			t.Fatalf("expected empty scheduled position with two vacancies, got %q", tuesdayCell)
		}
	})

	t.Run("renders zh labels and vacancy text", func(t *testing.T) {
		t.Parallel()

		workbook, err := renderScheduleExportXLSX(scheduleExportModel{
			Publication: &model.Publication{Name: "春季排班", State: model.PublicationStateAssigning},
			Language:    scheduleExportLanguageZH,
			ExportedAt:  time.Date(2026, 5, 4, 15, 30, 0, 0, time.UTC),
			Rows: []scheduleExportTimeRow{
				{
					StartTime: "09:00",
					EndTime:   "12:00",
					Cells: map[int][]scheduleExportPositionBlock{
						1: {{
							PositionName:      "前台",
							RequiredHeadcount: 1,
						}},
					},
				},
			},
		})
		if err != nil {
			t.Fatalf("renderScheduleExportXLSX returned error: %v", err)
		}

		file := openScheduleExportWorkbook(t, workbook)
		sheets := file.GetSheetList()
		if len(sheets) != 1 || sheets[0] != "排班表" {
			t.Fatalf("expected one 排班表 sheet, got %+v", sheets)
		}
		assertCellValue(t, file, "排班表", "A2", "状态: 排班中")
		assertCellValue(t, file, "排班表", "A3", "导出时间: 2026-05-04 15:30")
		assertCellValue(t, file, "排班表", "A5", "时间")
		assertCellValue(t, file, "排班表", "B5", "周一")
		assertCellValue(t, file, "排班表", "B6", "前台 (1)\n空缺")
	})
}

func TestPublicationServiceExportScheduleXLSX(t *testing.T) {
	t.Run("allows admin to export assigning publication", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		repo := newPublicationRepositoryStatefulMock()
		repo.publications[1] = assigningPublication(now)
		service := NewPublicationService(repo, fixedClock{now: now})

		workbook, err := service.ExportScheduleXLSX(context.Background(), 1, &model.User{ID: 1, IsAdmin: true}, ExportScheduleOptions{
			Language: "zh",
		})
		if err != nil {
			t.Fatalf("ExportScheduleXLSX returned error: %v", err)
		}
		file := openScheduleExportWorkbook(t, workbook)
		if sheets := file.GetSheetList(); len(sheets) != 1 || sheets[0] != "排班表" {
			t.Fatalf("expected zh workbook, got sheets %+v", sheets)
		}
	})

	t.Run("allows employee to export published publication", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		repo := newPublicationRepositoryStatefulMock()
		repo.publications[1] = publishedPublication(now)
		service := NewPublicationService(repo, fixedClock{now: now})

		workbook, err := service.ExportScheduleXLSX(context.Background(), 1, &model.User{ID: 7}, ExportScheduleOptions{
			Language: "en",
		})
		if err != nil {
			t.Fatalf("ExportScheduleXLSX returned error: %v", err)
		}
		file := openScheduleExportWorkbook(t, workbook)
		if sheets := file.GetSheetList(); len(sheets) != 1 || sheets[0] != "Roster" {
			t.Fatalf("expected en workbook, got sheets %+v", sheets)
		}
	})

	t.Run("rejects employee assigning export", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		repo := newPublicationRepositoryStatefulMock()
		repo.publications[1] = assigningPublication(now)
		service := NewPublicationService(repo, fixedClock{now: now})

		_, err := service.ExportScheduleXLSX(context.Background(), 1, &model.User{ID: 7}, ExportScheduleOptions{})
		if !errors.Is(err, ErrPublicationNotActive) {
			t.Fatalf("expected ErrPublicationNotActive, got %v", err)
		}
	})

	t.Run("rejects draft export", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		repo := newPublicationRepositoryStatefulMock()
		repo.publications[1] = draftPublication(now)
		service := NewPublicationService(repo, fixedClock{now: now})

		_, err := service.ExportScheduleXLSX(context.Background(), 1, &model.User{ID: 1, IsAdmin: true}, ExportScheduleOptions{})
		if !errors.Is(err, ErrPublicationNotActive) {
			t.Fatalf("expected ErrPublicationNotActive, got %v", err)
		}
	})

	t.Run("rejects unsupported language", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		repo := newPublicationRepositoryStatefulMock()
		repo.publications[1] = publishedPublication(now)
		service := NewPublicationService(repo, fixedClock{now: now})

		_, err := service.ExportScheduleXLSX(context.Background(), 1, &model.User{ID: 7}, ExportScheduleOptions{
			Language: "fr",
		})
		if !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("expected ErrInvalidInput, got %v", err)
		}
	})

	t.Run("exports baseline assignment despite occurrence override", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		repo := newPublicationRepositoryStatefulMock()
		publication := publishedPublication(now)
		repo.publications[1] = publication
		repo.assignments[assignmentKey(1, 7, 21)] = &model.Assignment{
			ID:            1,
			PublicationID: 1,
			UserID:        7,
			SlotID:        21,
			Weekday:       1,
			PositionID:    101,
			CreatedAt:     now.Add(-30 * time.Minute),
		}
		weekStart, err := resolveRosterWeekStart(publication, nil, now)
		if err != nil {
			t.Fatalf("resolveRosterWeekStart returned error: %v", err)
		}
		repo.assignmentOverrides[1] = &model.AssignmentOverride{
			ID:             1,
			AssignmentID:   1,
			OccurrenceDate: occurrenceDateForWeekday(weekStart, 1),
			UserID:         8,
			CreatedAt:      now,
		}
		service := NewPublicationService(repo, fixedClock{now: now})

		workbook, err := service.ExportScheduleXLSX(context.Background(), 1, &model.User{ID: 1, IsAdmin: true}, ExportScheduleOptions{
			Language: "en",
		})
		if err != nil {
			t.Fatalf("ExportScheduleXLSX returned error: %v", err)
		}
		file := openScheduleExportWorkbook(t, workbook)
		cell, err := file.GetCellValue("Roster", "B6")
		if err != nil {
			t.Fatalf("read B6: %v", err)
		}
		if !strings.Contains(cell, "Alice") {
			t.Fatalf("expected baseline assignee Alice, got %q", cell)
		}
		if strings.Contains(cell, "Bob") || strings.Contains(cell, "alice@example.com") {
			t.Fatalf("expected override user and emails to be omitted, got %q", cell)
		}
	})
}

func openScheduleExportWorkbook(t testing.TB, data []byte) *excelize.File {
	t.Helper()

	file, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("open workbook: %v", err)
	}
	t.Cleanup(func() {
		_ = file.Close()
	})
	return file
}

func assertCellValue(t testing.TB, file *excelize.File, sheet, cell, want string) {
	t.Helper()

	got, err := file.GetCellValue(sheet, cell)
	if err != nil {
		t.Fatalf("read %s!%s: %v", sheet, cell, err)
	}
	if got != want {
		t.Fatalf("expected %s!%s = %q, got %q", sheet, cell, want, got)
	}
}

func countLines(lines []string, want string) int {
	count := 0
	for _, line := range lines {
		if line == want {
			count++
		}
	}
	return count
}
