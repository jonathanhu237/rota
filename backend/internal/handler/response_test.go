package handler

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/jonathanhu237/rota/backend/internal/service"
)

func TestTemplateDetailResponseIncludesEmptySlotsArray(t *testing.T) {
	t.Parallel()

	payload, err := json.Marshal(templateDetailResponse{
		Template: newTemplateResponse(&model.Template{
			ID:          1,
			Name:        "Empty Template",
			Description: "No slots yet",
			Slots:       []*model.TemplateSlot{},
		}),
	})
	if err != nil {
		t.Fatalf("marshal template detail response: %v", err)
	}

	if string(payload) == "" {
		t.Fatal("expected non-empty payload")
	}
	slots, ok := getTemplateField(payload, "slots")
	if !ok {
		t.Fatalf("expected detail response to include slots field, got %s", payload)
	}

	slotsSlice, ok := slots.([]any)
	if !ok {
		t.Fatalf("expected slots to marshal as an array, got %T", slots)
	}
	if len(slotsSlice) != 0 {
		t.Fatalf("expected slots array to be empty, got %v", slotsSlice)
	}
}

func getTemplateField(payload []byte, field string) (any, bool) {
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return nil, false
	}

	templateValue, ok := decoded["template"]
	if !ok {
		return nil, false
	}

	templateMap, ok := templateValue.(map[string]any)
	if !ok {
		return nil, false
	}

	value, ok := templateMap[field]
	return value, ok
}

func TestCurrentPublicationResponseIncludesNullPublication(t *testing.T) {
	t.Parallel()

	payload, err := json.Marshal(currentPublicationResponse{})
	if err != nil {
		t.Fatalf("marshal current publication response: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal current publication response: %v", err)
	}

	value, ok := decoded["publication"]
	if !ok {
		t.Fatalf("expected publication field, got %s", payload)
	}
	if value != nil {
		t.Fatalf("expected null publication, got %#v", value)
	}
}

func TestPublicationResponseIncludesFrontendFields(t *testing.T) {
	t.Parallel()

	publication := &model.Publication{
		ID:                9,
		TemplateID:        3,
		TemplateName:      "Weekday Template",
		Name:              "April schedule",
		State:             model.PublicationStateCollecting,
		SubmissionStartAt: time.Date(2026, 4, 16, 9, 0, 0, 0, time.UTC),
		SubmissionEndAt:   time.Date(2026, 4, 16, 17, 0, 0, 0, time.UTC),
		PlannedActiveFrom: time.Date(2026, 4, 17, 9, 0, 0, 0, time.UTC),
		ActivatedAt:       ptrTime(time.Date(2026, 4, 17, 9, 5, 0, 0, time.UTC)),
		EndedAt:           ptrTime(time.Date(2026, 4, 20, 9, 0, 0, 0, time.UTC)),
		CreatedAt:         time.Date(2026, 4, 15, 8, 0, 0, 0, time.UTC),
		UpdatedAt:         time.Date(2026, 4, 16, 8, 0, 0, 0, time.UTC),
	}

	payload, err := json.Marshal(publicationDetailResponse{
		Publication: newPublicationResponse(publication),
	})
	if err != nil {
		t.Fatalf("marshal publication detail response: %v", err)
	}

	fields := []string{
		"id",
		"template_id",
		"template_name",
		"name",
		"state",
		"submission_start_at",
		"submission_end_at",
		"planned_active_from",
		"activated_at",
		"ended_at",
		"created_at",
		"updated_at",
	}
	for _, field := range fields {
		if _, ok := getPublicationField(payload, field); !ok {
			t.Fatalf("expected publication response to include %q, got %s", field, payload)
		}
	}
}

func TestRosterResponseOmitsAssignmentEmail(t *testing.T) {
	t.Parallel()

	payload, err := json.Marshal(newRosterResponse(&service.RosterResult{
		Publication: &model.Publication{
			ID:                9,
			TemplateID:        3,
			TemplateName:      "Weekday Template",
			Name:              "April schedule",
			State:             model.PublicationStateActive,
			SubmissionStartAt: time.Date(2026, 4, 16, 9, 0, 0, 0, time.UTC),
			SubmissionEndAt:   time.Date(2026, 4, 16, 17, 0, 0, 0, time.UTC),
			PlannedActiveFrom: time.Date(2026, 4, 17, 9, 0, 0, 0, time.UTC),
			ActivatedAt:       ptrTime(time.Date(2026, 4, 17, 9, 5, 0, 0, time.UTC)),
			CreatedAt:         time.Date(2026, 4, 15, 8, 0, 0, 0, time.UTC),
			UpdatedAt:         time.Date(2026, 4, 16, 8, 0, 0, 0, time.UTC),
		},
		Weekdays: []*service.RosterWeekdayResult{
			{
				Weekday: 1,
				Slots: []*service.RosterSlotResult{
					{
						Slot: &model.TemplateSlot{
							ID:         11,
							TemplateID: 3,
							Weekday:    1,
							StartTime:  "09:00",
							EndTime:    "12:00",
						},
						Positions: []*service.RosterPositionResult{
							{
								Position: &model.Position{
									ID:   101,
									Name: "Front Desk",
								},
								RequiredHeadcount: 2,
								Assignments: []*model.AssignmentParticipant{
									{
										AssignmentID: 1,
										SlotID:       11,
										PositionID:   101,
										UserID:       7,
										Name:         "Alice",
										Email:        "alice@example.com",
									},
								},
							},
						},
					},
				},
			},
		},
	}))
	if err != nil {
		t.Fatalf("marshal roster response: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal roster response: %v", err)
	}

	weekdays, ok := decoded["weekdays"].([]any)
	if !ok || len(weekdays) != 1 {
		t.Fatalf("expected one weekday, got %v", decoded["weekdays"])
	}

	weekday, ok := weekdays[0].(map[string]any)
	if !ok {
		t.Fatalf("expected weekday object, got %T", weekdays[0])
	}
	slots, ok := weekday["slots"].([]any)
	if !ok || len(slots) != 1 {
		t.Fatalf("expected one slot, got %v", weekday["slots"])
	}

	slot, ok := slots[0].(map[string]any)
	if !ok {
		t.Fatalf("expected slot object, got %T", slots[0])
	}
	positions, ok := slot["positions"].([]any)
	if !ok || len(positions) != 1 {
		t.Fatalf("expected one position, got %v", slot["positions"])
	}

	position, ok := positions[0].(map[string]any)
	if !ok {
		t.Fatalf("expected position object, got %T", positions[0])
	}
	assignments, ok := position["assignments"].([]any)
	if !ok || len(assignments) != 1 {
		t.Fatalf("expected one assignment, got %v", position["assignments"])
	}

	assignment, ok := assignments[0].(map[string]any)
	if !ok {
		t.Fatalf("expected assignment object, got %T", assignments[0])
	}
	if _, exists := assignment["email"]; exists {
		t.Fatalf("expected roster assignment email to be omitted, got %s", payload)
	}
	if assignment["name"] != "Alice" {
		t.Fatalf("expected assignment name Alice, got %v", assignment["name"])
	}
}

func getPublicationField(payload []byte, field string) (any, bool) {
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return nil, false
	}

	publicationValue, ok := decoded["publication"]
	if !ok {
		return nil, false
	}

	publicationMap, ok := publicationValue.(map[string]any)
	if !ok {
		return nil, false
	}

	value, ok := publicationMap[field]
	return value, ok
}

func ptrTime(value time.Time) *time.Time {
	return &value
}
