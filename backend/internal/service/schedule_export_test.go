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
		for cell, want := range map[string]string{
			"A1": "Time",
			"B1": "Position",
			"C1": "Monday",
			"D1": "Tuesday",
			"E1": "Wednesday",
			"F1": "Thursday",
			"G1": "Friday",
			"H1": "Saturday",
			"I1": "Sunday",
		} {
			assertCellValue(t, file, "Roster", cell, want)
		}
		assertCellValue(t, file, "Roster", "A2", "09:00-12:00")
		for cell, want := range map[string]string{
			"B2": "Front Desk",
			"C2": "Alice",
			"B3": "Front Desk",
			"C3": "Bob",
			"B4": "Front Desk",
			"C4": "",
			"A6": "13:00-17:00",
			"B6": "Cashier",
			"D6": "",
			"B7": "Cashier",
			"D7": "",
		} {
			assertCellValue(t, file, "Roster", cell, want)
		}
		for _, cell := range []string{"C2", "C3", "C4", "D6", "D7"} {
			value, err := file.GetCellValue("Roster", cell)
			if err != nil {
				t.Fatalf("read %s: %v", cell, err)
			}
			if strings.Contains(value, "alice@example.com") || strings.Contains(value, "bob@example.com") || strings.Contains(value, "Empty") {
				t.Fatalf("expected exported seat cell %s to omit emails and vacancy labels, got %q", cell, value)
			}
		}
	})

	t.Run("renders zh labels and leaves vacancies blank", func(t *testing.T) {
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
							PositionID:        7,
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
		assertCellValue(t, file, "排班表", "A1", "时间")
		assertCellValue(t, file, "排班表", "B1", "岗位")
		assertCellValue(t, file, "排班表", "C1", "星期一")
		assertCellValue(t, file, "排班表", "B2", "前台")
		assertCellValue(t, file, "排班表", "C2", "")
	})

	t.Run("renders one row per required seat and spacer rows", func(t *testing.T) {
		t.Parallel()

		workbook, err := renderScheduleExportXLSX(scheduleExportModel{
			Publication: &model.Publication{Name: "春季排班", State: model.PublicationStateAssigning},
			Language:    scheduleExportLanguageZH,
			ExportedAt:  time.Date(2026, 5, 4, 15, 30, 0, 0, time.UTC),
			Rows: []scheduleExportTimeRow{
				{
					StartTime: "09:00",
					EndTime:   "10:00",
					Cells: map[int][]scheduleExportPositionBlock{
						1: {
							{
								PositionID:        1,
								PositionName:      "前台负责人",
								RequiredHeadcount: 1,
								AssigneeNames:     []string{"Alice"},
							},
							{
								PositionID:        2,
								PositionName:      "前台助理",
								RequiredHeadcount: 2,
								AssigneeNames:     []string{"Bob", "Carol"},
							},
						},
					},
				},
				{
					StartTime: "10:00",
					EndTime:   "12:00",
					Cells: map[int][]scheduleExportPositionBlock{
						1: {{
							PositionID:        1,
							PositionName:      "前台负责人",
							RequiredHeadcount: 1,
							AssigneeNames:     []string{"Dave"},
						}},
					},
				},
			},
		})
		if err != nil {
			t.Fatalf("renderScheduleExportXLSX returned error: %v", err)
		}

		file := openScheduleExportWorkbook(t, workbook)
		for cell, want := range map[string]string{
			"A2": "09:00-10:00",
			"B2": "前台负责人",
			"C2": "Alice",
			"B3": "前台助理",
			"C3": "Bob",
			"B4": "前台助理",
			"C4": "Carol",
			"A5": "",
			"B5": "",
			"C5": "",
			"A6": "10:00-12:00",
			"B6": "前台负责人",
			"C6": "Dave",
		} {
			assertCellValue(t, file, "排班表", cell, want)
		}
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
		cell, err := file.GetCellValue("Roster", "C2")
		if err != nil {
			t.Fatalf("read C2: %v", err)
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
