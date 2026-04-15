package handler

import (
	"encoding/json"
	"testing"

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
