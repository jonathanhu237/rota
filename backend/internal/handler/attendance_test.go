package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/jonathanhu237/rota/backend/internal/service"
)

type stubAttendanceService struct {
	listCurrentFunc        func(ctx context.Context, userID int64) (*service.LeaderAttendanceResult, error)
	recordArrivalFunc      func(ctx context.Context, input service.RecordLeaderArrivalInput) (*service.AttendanceShiftDetail, error)
	recordOvertimeFunc     func(ctx context.Context, input service.RecordOvertimeInput) (*model.AttendanceOvertimeRecord, error)
	listAdminFunc          func(ctx context.Context, input service.ListAdminAttendanceInput) (*service.AdminAttendanceDayResult, error)
	getAdminShiftFunc      func(ctx context.Context, input service.GetAdminShiftAttendanceInput) (*service.AttendanceShiftDetail, error)
	adminUpsertArrivalFunc func(ctx context.Context, input service.AdminUpsertArrivalInput) (*service.AttendanceShiftDetail, error)
	adminClearArrivalFunc  func(ctx context.Context, input service.AdminClearArrivalInput) error
	adminCreateOTFunc      func(ctx context.Context, input service.RecordOvertimeInput) (*model.AttendanceOvertimeRecord, error)
	adminUpdateOTFunc      func(ctx context.Context, input service.AdminUpdateOvertimeInput) (*model.AttendanceOvertimeRecord, error)
	adminDeleteOTFunc      func(ctx context.Context, input service.AdminClearArrivalInput) error
	updateSettingsFunc     func(ctx context.Context, input service.UpdateAttendanceSettingsInput) (*model.Publication, error)
}

func (s *stubAttendanceService) ListCurrentAttendance(ctx context.Context, userID int64) (*service.LeaderAttendanceResult, error) {
	return s.listCurrentFunc(ctx, userID)
}

func (s *stubAttendanceService) RecordLeaderArrival(ctx context.Context, input service.RecordLeaderArrivalInput) (*service.AttendanceShiftDetail, error) {
	return s.recordArrivalFunc(ctx, input)
}

func (s *stubAttendanceService) RecordLeaderOvertime(ctx context.Context, input service.RecordOvertimeInput) (*model.AttendanceOvertimeRecord, error) {
	return s.recordOvertimeFunc(ctx, input)
}

func (s *stubAttendanceService) ListAdminAttendance(ctx context.Context, input service.ListAdminAttendanceInput) (*service.AdminAttendanceDayResult, error) {
	return s.listAdminFunc(ctx, input)
}

func (s *stubAttendanceService) GetAdminShiftAttendance(ctx context.Context, input service.GetAdminShiftAttendanceInput) (*service.AttendanceShiftDetail, error) {
	return s.getAdminShiftFunc(ctx, input)
}

func (s *stubAttendanceService) AdminUpsertArrival(ctx context.Context, input service.AdminUpsertArrivalInput) (*service.AttendanceShiftDetail, error) {
	return s.adminUpsertArrivalFunc(ctx, input)
}

func (s *stubAttendanceService) AdminClearArrival(ctx context.Context, input service.AdminClearArrivalInput) error {
	return s.adminClearArrivalFunc(ctx, input)
}

func (s *stubAttendanceService) AdminCreateOvertime(ctx context.Context, input service.RecordOvertimeInput) (*model.AttendanceOvertimeRecord, error) {
	return s.adminCreateOTFunc(ctx, input)
}

func (s *stubAttendanceService) AdminUpdateOvertime(ctx context.Context, input service.AdminUpdateOvertimeInput) (*model.AttendanceOvertimeRecord, error) {
	return s.adminUpdateOTFunc(ctx, input)
}

func (s *stubAttendanceService) AdminDeleteOvertime(ctx context.Context, input service.AdminClearArrivalInput) error {
	return s.adminDeleteOTFunc(ctx, input)
}

func (s *stubAttendanceService) UpdateAttendanceSettings(ctx context.Context, input service.UpdateAttendanceSettingsInput) (*model.Publication, error) {
	return s.updateSettingsFunc(ctx, input)
}

func TestAttendanceHandlerCurrent(t *testing.T) {
	t.Run("returns leader attendance shifts", func(t *testing.T) {
		t.Parallel()

		handler := NewAttendanceHandler(&stubAttendanceService{
			listCurrentFunc: func(ctx context.Context, userID int64) (*service.LeaderAttendanceResult, error) {
				if userID != 1 {
					t.Fatalf("expected user id 1, got %d", userID)
				}
				return &service.LeaderAttendanceResult{
					Publication: samplePublication(),
					Shifts:      []*service.AttendanceShiftDetail{sampleAttendanceShift()},
				}, nil
			},
		})

		recorder := httptest.NewRecorder()
		req := requestWithUser(httptest.NewRequest(http.MethodGet, "/attendance/current", nil), sampleUser())

		handler.Current(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", recorder.Code)
		}
		response := decodeJSONResponse[currentAttendanceResponse](t, recorder)
		if len(response.Shifts) != 1 || len(response.Shifts[0].Roster) != 2 {
			t.Fatalf("unexpected current attendance response: %+v", response)
		}
	})

	t.Run("requires current user context", func(t *testing.T) {
		t.Parallel()

		handler := NewAttendanceHandler(&stubAttendanceService{})
		recorder := httptest.NewRecorder()

		handler.Current(recorder, httptest.NewRequest(http.MethodGet, "/attendance/current", nil))

		assertErrorResponse(t, recorder, http.StatusInternalServerError, "INTERNAL_ERROR")
	})
}

func TestAttendanceHandlerLeaderWrites(t *testing.T) {
	t.Run("records leader arrival", func(t *testing.T) {
		t.Parallel()

		handler := NewAttendanceHandler(&stubAttendanceService{
			recordArrivalFunc: func(ctx context.Context, input service.RecordLeaderArrivalInput) (*service.AttendanceShiftDetail, error) {
				if input.ActorUserID != 1 || input.PublicationID != 9 || input.AssignmentID != 1002 {
					t.Fatalf("unexpected arrival input: %+v", input)
				}
				if input.ArrivedAt == nil {
					t.Fatalf("expected arrived_at from request")
				}
				return sampleAttendanceShift(), nil
			},
		})
		arrivedAt := time.Date(2026, 4, 20, 9, 5, 0, 0, time.UTC)
		recorder := httptest.NewRecorder()
		req := requestWithUser(jsonRequest(t, http.MethodPost, "/attendance/arrivals", map[string]any{
			"publication_id":  9,
			"slot_id":         21,
			"assignment_id":   1002,
			"occurrence_date": "2026-04-20",
			"user_id":         2,
			"arrived_at":      arrivedAt,
		}), sampleUser())

		handler.RecordLeaderArrival(recorder, req)

		if recorder.Code != http.StatusCreated {
			t.Fatalf("expected status 201, got %d", recorder.Code)
		}
		response := decodeJSONResponse[attendanceShiftDetailResponse](t, recorder)
		if response.Shift.SlotID != 21 {
			t.Fatalf("unexpected response: %+v", response)
		}
	})

	t.Run("rejects invalid arrival date", func(t *testing.T) {
		t.Parallel()

		handler := NewAttendanceHandler(&stubAttendanceService{})
		recorder := httptest.NewRecorder()
		req := requestWithUser(jsonRequest(t, http.MethodPost, "/attendance/arrivals", map[string]any{
			"publication_id":  9,
			"slot_id":         21,
			"assignment_id":   1002,
			"occurrence_date": "bad",
			"user_id":         2,
		}), sampleUser())

		handler.RecordLeaderArrival(recorder, req)

		assertErrorResponse(t, recorder, http.StatusBadRequest, "INVALID_REQUEST")
	})

	t.Run("maps attendance write errors", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name   string
			err    error
			status int
			code   string
		}{
			{name: "non leader", err: service.ErrAttendanceNotLeader, status: http.StatusForbidden, code: "ATTENDANCE_NOT_LEADER"},
			{name: "already recorded", err: service.ErrAttendanceAlreadyRecorded, status: http.StatusConflict, code: "ATTENDANCE_ALREADY_RECORDED"},
			{name: "closed window", err: service.ErrAttendanceWindowClosed, status: http.StatusConflict, code: "ATTENDANCE_WINDOW_CLOSED"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				handler := NewAttendanceHandler(&stubAttendanceService{
					recordArrivalFunc: func(ctx context.Context, input service.RecordLeaderArrivalInput) (*service.AttendanceShiftDetail, error) {
						return nil, tc.err
					},
				})
				recorder := httptest.NewRecorder()
				req := requestWithUser(jsonRequest(t, http.MethodPost, "/attendance/arrivals", map[string]any{
					"publication_id":  9,
					"slot_id":         21,
					"assignment_id":   1002,
					"occurrence_date": "2026-04-20",
					"user_id":         2,
				}), sampleUser())

				handler.RecordLeaderArrival(recorder, req)

				assertErrorResponse(t, recorder, tc.status, tc.code)
			})
		}
	})

	t.Run("records leader overtime", func(t *testing.T) {
		t.Parallel()

		handler := NewAttendanceHandler(&stubAttendanceService{
			recordOvertimeFunc: func(ctx context.Context, input service.RecordOvertimeInput) (*model.AttendanceOvertimeRecord, error) {
				if input.ActorUserID != 1 || input.Hours != 1.5 || input.Note != "cleanup" {
					t.Fatalf("unexpected overtime input: %+v", input)
				}
				return sampleAttendanceOvertime(), nil
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithUser(jsonRequest(t, http.MethodPost, "/attendance/overtime", map[string]any{
			"publication_id":  9,
			"slot_id":         21,
			"occurrence_date": "2026-04-20",
			"user_id":         2,
			"hours":           1.5,
			"note":            "cleanup",
		}), sampleUser())

		handler.RecordLeaderOvertime(recorder, req)

		if recorder.Code != http.StatusCreated {
			t.Fatalf("expected status 201, got %d", recorder.Code)
		}
		response := decodeJSONResponse[attendanceOvertimeDetailResponse](t, recorder)
		if response.Overtime.ID != 77 {
			t.Fatalf("unexpected response: %+v", response)
		}
	})
}

func TestAttendanceHandlerAdmin(t *testing.T) {
	t.Run("lists admin attendance day", func(t *testing.T) {
		t.Parallel()

		handler := NewAttendanceHandler(&stubAttendanceService{
			listAdminFunc: func(ctx context.Context, input service.ListAdminAttendanceInput) (*service.AdminAttendanceDayResult, error) {
				if input.PublicationID != 9 || input.OccurrenceDate.Format("2006-01-02") != "2026-04-20" {
					t.Fatalf("unexpected list input: %+v", input)
				}
				return &service.AdminAttendanceDayResult{
					Publication: samplePublication(),
					Date:        input.OccurrenceDate,
					Shifts: []*service.AttendanceShiftSummary{
						{
							SlotID:         21,
							Weekday:        1,
							OccurrenceDate: input.OccurrenceDate,
							ScheduledStart: sampleAttendanceStart(),
							ScheduledEnd:   sampleAttendanceEnd(),
							RosterCount:    2,
							PendingCount:   1,
						},
					},
				}, nil
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithPathValues(httptest.NewRequest(http.MethodGet, "/publications/9/attendance?date=2026-04-20", nil), map[string]string{"id": "9"})

		handler.ListAdmin(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", recorder.Code)
		}
		response := decodeJSONResponse[adminAttendanceDayResponse](t, recorder)
		if response.Date != "2026-04-20" || len(response.Shifts) != 1 {
			t.Fatalf("unexpected response: %+v", response)
		}
	})

	t.Run("rejects invalid admin date", func(t *testing.T) {
		t.Parallel()

		handler := NewAttendanceHandler(&stubAttendanceService{})
		recorder := httptest.NewRecorder()
		req := requestWithPathValues(httptest.NewRequest(http.MethodGet, "/publications/9/attendance?date=bad", nil), map[string]string{"id": "9"})

		handler.ListAdmin(recorder, req)

		assertErrorResponse(t, recorder, http.StatusBadRequest, "INVALID_REQUEST")
	})

	t.Run("upserts admin arrival", func(t *testing.T) {
		t.Parallel()

		handler := NewAttendanceHandler(&stubAttendanceService{
			adminUpsertArrivalFunc: func(ctx context.Context, input service.AdminUpsertArrivalInput) (*service.AttendanceShiftDetail, error) {
				if input.ActorUserID != 1 || input.PublicationID != 9 || input.SlotID != 21 {
					t.Fatalf("unexpected admin upsert input: %+v", input)
				}
				return sampleAttendanceShift(), nil
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithUser(requestWithPathValues(jsonRequest(t, http.MethodPut, "/publications/9/attendance/arrivals", map[string]any{
			"slot_id":         21,
			"assignment_id":   1002,
			"occurrence_date": "2026-04-20",
			"user_id":         2,
			"arrived_at":      sampleAttendanceStart(),
		}), map[string]string{"id": "9"}), sampleUser())

		handler.AdminUpsertArrival(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", recorder.Code)
		}
	})

	t.Run("clears admin arrival", func(t *testing.T) {
		t.Parallel()

		handler := NewAttendanceHandler(&stubAttendanceService{
			adminClearArrivalFunc: func(ctx context.Context, input service.AdminClearArrivalInput) error {
				if input.ActorUserID != 1 || input.PublicationID != 9 || input.RecordID != 88 {
					t.Fatalf("unexpected clear input: %+v", input)
				}
				return nil
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithUser(
			requestWithPathValues(
				httptest.NewRequest(http.MethodDelete, "/publications/9/attendance/arrivals/88", nil),
				map[string]string{"id": "9", "record_id": "88"},
			),
			sampleUser(),
		)

		handler.AdminClearArrival(recorder, req)

		if recorder.Code != http.StatusNoContent {
			t.Fatalf("expected status 204, got %d", recorder.Code)
		}
	})

	t.Run("updates attendance settings", func(t *testing.T) {
		t.Parallel()

		handler := NewAttendanceHandler(&stubAttendanceService{
			updateSettingsFunc: func(ctx context.Context, input service.UpdateAttendanceSettingsInput) (*model.Publication, error) {
				if input.ActorUserID != 1 || input.PublicationID != 9 || input.OvertimeEntryWindowHours != 12.5 {
					t.Fatalf("unexpected settings input: %+v", input)
				}
				publication := samplePublication()
				publication.OvertimeEntryWindowHours = 12.5
				return publication, nil
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithUser(requestWithPathValues(jsonRequest(t, http.MethodPatch, "/publications/9/attendance/settings", map[string]any{
			"overtime_entry_window_hours": 12.5,
		}), map[string]string{"id": "9"}), sampleUser())

		handler.UpdateSettings(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", recorder.Code)
		}
		response := decodeJSONResponse[attendanceSettingsResponse](t, recorder)
		if response.Publication == nil || response.Publication.OvertimeEntryWindowHours != 12.5 {
			t.Fatalf("unexpected response: %+v", response)
		}
	})
}

func sampleAttendanceShift() *service.AttendanceShiftDetail {
	leaderRecord := &model.AttendanceRecord{
		ID:             66,
		PublicationID:  9,
		AssignmentID:   1001,
		OccurrenceDate: sampleAttendanceDate(),
		UserID:         1,
		UserName:       "Leader",
		UserEmail:      "leader@example.com",
		ArrivedAt:      sampleAttendanceStart(),
		RecordedAt:     sampleAttendanceStart(),
		UpdatedAt:      sampleAttendanceStart(),
	}
	return &service.AttendanceShiftDetail{
		Publication:        samplePublication(),
		SlotID:             21,
		Weekday:            1,
		StartTime:          "09:00",
		EndTime:            "12:00",
		OccurrenceDate:     sampleAttendanceDate(),
		ScheduledStart:     sampleAttendanceStart(),
		ScheduledEnd:       sampleAttendanceEnd(),
		ArrivalWindowOpen:  true,
		OvertimeWindowOpen: true,
		Roster: []*service.AttendanceRosterEntry{
			{
				AssignmentID:          1001,
				PositionID:            7,
				PositionName:          "负责人",
				AttendanceResponsible: true,
				UserID:                1,
				UserName:              "Leader",
				UserEmail:             "leader@example.com",
				Status:                model.AttendanceStatusPresent,
				Record:                leaderRecord,
			},
			{
				AssignmentID: 1002,
				PositionID:   8,
				PositionName: "Front Desk",
				UserID:       2,
				UserName:     "Worker",
				UserEmail:    "worker@example.com",
				Status:       model.AttendanceStatusPending,
			},
		},
		OvertimeRecords: []*model.AttendanceOvertimeRecord{sampleAttendanceOvertime()},
	}
}

func sampleAttendanceOvertime() *model.AttendanceOvertimeRecord {
	return &model.AttendanceOvertimeRecord{
		ID:             77,
		PublicationID:  9,
		SlotID:         21,
		Weekday:        1,
		OccurrenceDate: sampleAttendanceDate(),
		UserID:         2,
		UserName:       "Worker",
		UserEmail:      "worker@example.com",
		Hours:          1.5,
		Note:           "cleanup",
		RecordedAt:     sampleAttendanceEnd(),
		UpdatedAt:      sampleAttendanceEnd(),
	}
}

func sampleAttendanceDate() time.Time {
	return time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC)
}

func sampleAttendanceStart() time.Time {
	return time.Date(2026, 4, 20, 9, 0, 0, 0, time.UTC)
}

func sampleAttendanceEnd() time.Time {
	return time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC)
}
