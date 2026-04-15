package handler

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/model"
)

func TestTemplateDetailResponseIncludesEmptyShiftsArray(t *testing.T) {
	t.Parallel()

	payload, err := json.Marshal(templateDetailResponse{
		Template: newTemplateResponse(&model.Template{
			ID:          1,
			Name:        "Empty Template",
			Description: "No shifts yet",
			Shifts:      []*model.TemplateShift{},
		}),
	})
	if err != nil {
		t.Fatalf("marshal template detail response: %v", err)
	}

	if string(payload) == "" {
		t.Fatal("expected non-empty payload")
	}
	shifts, ok := getTemplateField(payload, "shifts")
	if !ok {
		t.Fatalf("expected detail response to include shifts field, got %s", payload)
	}

	shiftsSlice, ok := shifts.([]any)
	if !ok {
		t.Fatalf("expected shifts to marshal as an array, got %T", shifts)
	}
	if len(shiftsSlice) != 0 {
		t.Fatalf("expected shifts array to be empty, got %v", shiftsSlice)
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
