package service

import (
	"errors"
	"testing"
)

func TestNormalizeTemplateAndSlotInputs(t *testing.T) {
	name, description, err := normalizeTemplateInput("  Weekly rota ", "  ward coverage  ")
	if err != nil || name != "Weekly rota" || description != "ward coverage" {
		t.Fatalf("normalizeTemplateInput() = %q, %q, %v", name, description, err)
	}

	slot, err := normalizeSlotInput([]int{3, 1, 3}, "09:00", "10:30")
	if err != nil {
		t.Fatalf("normalizeSlotInput() error = %v", err)
	}
	if len(slot.Weekdays) != 2 || slot.Weekdays[0] != 1 || slot.Weekdays[1] != 3 {
		t.Fatalf("normalized weekdays = %#v, want [1 3]", slot.Weekdays)
	}
	if slot.StartTime != "09:00" || slot.EndTime != "10:30" {
		t.Fatalf("normalized times = %q-%q", slot.StartTime, slot.EndTime)
	}
}

func TestNormalizeTemplateAndSlotInputsRejectInvalidValues(t *testing.T) {
	for _, test := range []struct {
		name string
		call func() error
		want error
	}{
		{name: "blank template name", call: func() error {
			_, _, err := normalizeTemplateInput(" ", "description")
			return err
		}, want: ErrInvalidInput},
		{name: "description too long", call: func() error {
			_, _, err := normalizeTemplateInput("valid", string(make([]byte, maxTemplateDescriptionLength+1)))
			return err
		}, want: ErrInvalidInput},
		{name: "empty weekdays", call: func() error {
			_, err := normalizeSlotInput(nil, "09:00", "10:00")
			return err
		}, want: ErrInvalidWeekday},
		{name: "weekday out of range", call: func() error {
			_, err := normalizeSlotInput([]int{8}, "09:00", "10:00")
			return err
		}, want: ErrInvalidWeekday},
		{name: "bad start time", call: func() error {
			_, err := normalizeSlotInput([]int{1}, "09:60", "10:00")
			return err
		}, want: ErrInvalidShiftTime},
		{name: "end before start", call: func() error {
			_, err := normalizeSlotInput([]int{1}, "10:00", "09:00")
			return err
		}, want: ErrInvalidShiftTime},
		{name: "zero headcount", call: func() error {
			_, err := normalizeRequiredHeadcount(0)
			return err
		}, want: ErrInvalidHeadcount},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := test.call(); !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
		})
	}
}
