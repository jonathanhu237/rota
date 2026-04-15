package model

import (
	"errors"
	"time"
)

var (
	ErrInvalidPublicationWindow = errors.New("invalid publication window")
	ErrPublicationAlreadyExists = errors.New("publication already exists")
	ErrPublicationNotFound      = errors.New("publication not found")
	ErrPublicationNotDeletable  = errors.New("publication not deletable")
	ErrPublicationNotCollecting = errors.New("publication not collecting")
	ErrNotQualified             = errors.New("not qualified")
)

type PublicationState string

const (
	PublicationStateDraft      PublicationState = "DRAFT"
	PublicationStateCollecting PublicationState = "COLLECTING"
	PublicationStateAssigning  PublicationState = "ASSIGNING"
	PublicationStateActive     PublicationState = "ACTIVE"
	PublicationStateEnded      PublicationState = "ENDED"
)

type Publication struct {
	ID                int64
	TemplateID        int64
	TemplateName      string
	Name              string
	State             PublicationState
	SubmissionStartAt time.Time
	SubmissionEndAt   time.Time
	PlannedActiveFrom time.Time
	ActivatedAt       *time.Time
	EndedAt           *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type AvailabilitySubmission struct {
	ID              int64
	PublicationID   int64
	UserID          int64
	TemplateShiftID int64
	CreatedAt       time.Time
}

func ResolvePublicationState(publication *Publication, now time.Time) PublicationState {
	if publication == nil {
		return PublicationStateDraft
	}

	switch publication.State {
	case PublicationStateActive, PublicationStateEnded:
		return publication.State
	case "":
		return PublicationStateDraft
	}

	if !now.Before(publication.SubmissionEndAt) {
		return PublicationStateAssigning
	}
	if !now.Before(publication.SubmissionStartAt) {
		return PublicationStateCollecting
	}

	return PublicationStateDraft
}
