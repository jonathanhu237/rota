package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/email"
	"github.com/jonathanhu237/rota/backend/internal/model"
)

func TestInvitationEmailBrandingSelection(t *testing.T) {
	t.Parallel()

	var sent email.Message
	helper := newSetupFlowHelper(SetupFlowConfig{
		OutboxRepo: &emailerMock{sendFunc: func(ctx context.Context, msg email.Message) error {
			sent = msg
			return nil
		}},
		AppBaseURL:         "https://app.example.com",
		InvitationTokenTTL: 72 * time.Hour,
		BrandingProvider: brandingProviderMock{
			branding: &model.Branding{
				ProductName:      "排班系统",
				OrganizationName: "Acme",
				Version:          2,
			},
		},
	})

	if err := helper.enqueueInvitationTx(
		context.Background(),
		nil,
		&model.User{ID: 1, Email: "worker@example.com", Name: "Worker"},
		"token",
	); err != nil {
		t.Fatalf("enqueueInvitationTx returned error: %v", err)
	}

	if sent.Subject != "[排班系统] Invitation to 排班系统" ||
		!strings.Contains(sent.Body, "administrator from Acme") ||
		!strings.Contains(sent.Body, "This is an automated 排班系统 notification from Acme.") {
		t.Fatalf("unexpected branded invitation: %+v", sent)
	}
}

func TestInvitationBrandingLookupFailurePreventsEnqueue(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("branding unavailable")
	enqueueCalled := false
	helper := newSetupFlowHelper(SetupFlowConfig{
		OutboxRepo: &emailerMock{sendFunc: func(ctx context.Context, msg email.Message) error {
			enqueueCalled = true
			return nil
		}},
		AppBaseURL:         "https://app.example.com",
		InvitationTokenTTL: 72 * time.Hour,
		BrandingProvider:   brandingProviderMock{err: expectedErr},
	})

	err := helper.enqueueInvitationTx(
		context.Background(),
		nil,
		&model.User{ID: 1, Email: "worker@example.com", Name: "Worker"},
		"token",
	)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected branding error, got %v", err)
	}
	if enqueueCalled {
		t.Fatalf("outbox enqueue should not be called when branding lookup fails")
	}
}

func TestAccountEmailBrandingLookupFailurePreventsEnqueue(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("branding unavailable")
	user := &model.User{ID: 1, Email: "worker@example.com", Name: "Worker"}

	tests := []struct {
		name    string
		enqueue func(*setupFlowHelper) error
	}{
		{
			name: "password reset",
			enqueue: func(helper *setupFlowHelper) error {
				return helper.enqueuePasswordResetTx(context.Background(), nil, user, "token")
			},
		},
		{
			name: "email change confirm",
			enqueue: func(helper *setupFlowHelper) error {
				return helper.enqueueEmailChangeConfirmTx(context.Background(), nil, user, "new@example.com", "token")
			},
		},
		{
			name: "email change notice",
			enqueue: func(helper *setupFlowHelper) error {
				return helper.enqueueEmailChangeNoticeTx(context.Background(), nil, user, "new@example.com")
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			enqueueCalled := false
			helper := newSetupFlowHelper(SetupFlowConfig{
				OutboxRepo: &emailerMock{sendFunc: func(ctx context.Context, msg email.Message) error {
					enqueueCalled = true
					return nil
				}},
				AppBaseURL:            "https://app.example.com",
				PasswordResetTokenTTL: time.Hour,
				BrandingProvider:      brandingProviderMock{err: expectedErr},
			})

			err := tt.enqueue(helper)
			if !errors.Is(err, expectedErr) {
				t.Fatalf("expected branding error, got %v", err)
			}
			if enqueueCalled {
				t.Fatalf("outbox enqueue should not be called when branding lookup fails")
			}
		})
	}
}

func TestPasswordResetEmailBrandingSelection(t *testing.T) {
	t.Parallel()

	var sent email.Message
	helper := newSetupFlowHelper(SetupFlowConfig{
		OutboxRepo: &emailerMock{sendFunc: func(ctx context.Context, msg email.Message) error {
			sent = msg
			return nil
		}},
		AppBaseURL:            "https://app.example.com",
		PasswordResetTokenTTL: time.Hour,
		BrandingProvider: brandingProviderMock{
			branding: &model.Branding{
				ProductName:      "OpsHub",
				OrganizationName: "Acme",
				Version:          2,
			},
		},
	})

	if err := helper.enqueuePasswordResetTx(
		context.Background(),
		nil,
		&model.User{ID: 1, Email: "worker@example.com", Name: "Worker"},
		"token",
	); err != nil {
		t.Fatalf("enqueuePasswordResetTx returned error: %v", err)
	}

	if sent.Subject != "[OpsHub] Reset your OpsHub password" ||
		!strings.Contains(sent.Body, "your OpsHub account") ||
		!strings.Contains(sent.Body, "This is an automated OpsHub notification from Acme.") {
		t.Fatalf("unexpected branded password reset: %+v", sent)
	}
}

func TestEmailChangeEmailBrandingSelection(t *testing.T) {
	t.Parallel()

	var messages []email.Message
	helper := newSetupFlowHelper(SetupFlowConfig{
		OutboxRepo: &emailerMock{sendFunc: func(ctx context.Context, msg email.Message) error {
			messages = append(messages, msg)
			return nil
		}},
		AppBaseURL: "https://app.example.com",
		BrandingProvider: brandingProviderMock{
			branding: &model.Branding{
				ProductName:      "OpsHub",
				OrganizationName: "Acme",
				Version:          2,
			},
		},
	})
	user := &model.User{ID: 1, Email: "worker@example.com", Name: "Worker"}

	if err := helper.enqueueEmailChangeConfirmTx(context.Background(), nil, user, "new@example.com", "token"); err != nil {
		t.Fatalf("enqueueEmailChangeConfirmTx returned error: %v", err)
	}
	if err := helper.enqueueEmailChangeNoticeTx(context.Background(), nil, user, "new@example.com"); err != nil {
		t.Fatalf("enqueueEmailChangeNoticeTx returned error: %v", err)
	}

	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	for _, msg := range messages {
		if !strings.HasPrefix(msg.Subject, "[OpsHub] ") ||
			!strings.Contains(msg.Body, "OpsHub account") ||
			!strings.Contains(msg.Body, "notification from Acme") {
			t.Fatalf("unexpected branded email-change message: %+v", msg)
		}
	}
}

func TestShiftChangeEmailBrandingSelection(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
	pub, sc := buildShiftChangeFixture(now)
	emailer := &emailStub{}
	svc := NewShiftChangeService(
		sc,
		pub,
		emailer,
		"https://rota.example.com",
		fixedClock{now: now},
		nil,
		WithShiftChangeBrandingProvider(brandingProviderMock{
			branding: &model.Branding{ProductName: "OpsHub", OrganizationName: "Acme", Version: 2},
		}),
	)

	counterpartUserID := int64(8)
	counterpartAssignmentID := int64(101)
	if _, err := svc.CreateShiftChangeRequest(context.Background(), CreateShiftChangeInput{
		PublicationID:             1,
		RequesterUserID:           7,
		Type:                      model.ShiftChangeTypeSwap,
		RequesterAssignmentID:     100,
		OccurrenceDate:            mondayOccurrence(now),
		CounterpartUserID:         &counterpartUserID,
		CounterpartAssignmentID:   &counterpartAssignmentID,
		CounterpartOccurrenceDate: timePtr(wednesdayOccurrence(now)),
	}); err != nil {
		t.Fatalf("CreateShiftChangeRequest returned error: %v", err)
	}

	messages := emailer.messages()
	if len(messages) != 1 {
		t.Fatalf("expected 1 email, got %d", len(messages))
	}
	if !strings.HasPrefix(messages[0].Subject, "[OpsHub] ") ||
		!strings.Contains(messages[0].Body, "notification from Acme") {
		t.Fatalf("unexpected branded shift-change message: %+v", messages[0])
	}
}

func TestShiftChangeBrandingLookupFailurePreventsEnqueue(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("branding unavailable")
	now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
	pub, sc := buildShiftChangeFixture(now)
	emailer := &emailStub{}
	svc := NewShiftChangeService(
		sc,
		pub,
		emailer,
		"https://rota.example.com",
		fixedClock{now: now},
		nil,
		WithShiftChangeBrandingProvider(brandingProviderMock{err: expectedErr}),
	)

	counterpartUserID := int64(8)
	counterpartAssignmentID := int64(101)
	_, err := svc.CreateShiftChangeRequest(context.Background(), CreateShiftChangeInput{
		PublicationID:             1,
		RequesterUserID:           7,
		Type:                      model.ShiftChangeTypeSwap,
		RequesterAssignmentID:     100,
		OccurrenceDate:            mondayOccurrence(now),
		CounterpartUserID:         &counterpartUserID,
		CounterpartAssignmentID:   &counterpartAssignmentID,
		CounterpartOccurrenceDate: timePtr(wednesdayOccurrence(now)),
	})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected branding error, got %v", err)
	}
	if messages := emailer.messages(); len(messages) != 0 {
		t.Fatalf("expected no emails, got %+v", messages)
	}
}

func TestShiftChangeResolutionBrandingLookupFailurePreventsEnqueue(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("branding unavailable")
	now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
	pub, sc := buildShiftChangeFixture(now)
	emailer := &emailStub{}
	svc := NewShiftChangeService(
		sc,
		pub,
		emailer,
		"https://rota.example.com",
		fixedClock{now: now},
		nil,
		WithShiftChangeBrandingProvider(brandingProviderMock{err: expectedErr}),
	)
	req := &model.ShiftChangeRequest{
		ID:                    1,
		PublicationID:         1,
		Type:                  model.ShiftChangeTypeGivePool,
		RequesterUserID:       7,
		RequesterAssignmentID: 100,
		OccurrenceDate:        mondayOccurrence(now),
		State:                 model.ShiftChangeStatePending,
	}

	err := svc.enqueueRequestResolvedTx(
		context.Background(),
		nil,
		req,
		email.ShiftChangeOutcomeInvalidated,
		7,
	)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected branding error, got %v", err)
	}
	if messages := emailer.messages(); len(messages) != 0 {
		t.Fatalf("expected no emails, got %+v", messages)
	}
}

func TestPublicationInvalidationBrandingLookupFailurePreventsEnqueue(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("branding unavailable")
	now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
	repo := newPublicationRepositoryStatefulMock()
	emailer := &emailStub{}
	svc := NewPublicationService(
		repo,
		fixedClock{now: now},
		WithPublicationShiftChangeNotifications(nil, emailer, "https://rota.example.com", nil),
		WithPublicationBrandingProvider(brandingProviderMock{err: expectedErr}),
	)
	req := &model.ShiftChangeRequest{
		ID:                    55,
		PublicationID:         1,
		Type:                  model.ShiftChangeTypeGivePool,
		RequesterUserID:       7,
		RequesterAssignmentID: 100,
		OccurrenceDate:        mondayOccurrence(now),
		State:                 model.ShiftChangeStatePending,
	}

	err := svc.enqueueShiftChangeRequestInvalidatedTx(
		context.Background(),
		nil,
		req,
		100,
		nil,
	)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected branding error, got %v", err)
	}
	if messages := emailer.messages(); len(messages) != 0 {
		t.Fatalf("expected no emails, got %+v", messages)
	}
}

type brandingProviderMock struct {
	branding *model.Branding
	err      error
}

func (m brandingProviderMock) GetBranding(ctx context.Context) (*model.Branding, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.branding != nil {
		return m.branding, nil
	}
	return &model.Branding{ProductName: "Rota", Version: 1}, nil
}
