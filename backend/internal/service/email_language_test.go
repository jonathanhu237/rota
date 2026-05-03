package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/email"
	"github.com/jonathanhu237/rota/backend/internal/model"
)

func TestInvitationEmailLanguageSelection(t *testing.T) {
	t.Parallel()

	zh := model.LanguagePreferenceZH
	en := model.LanguagePreferenceEN
	tests := []struct {
		name        string
		invitee     *model.User
		ctx         context.Context
		wantSubject string
	}{
		{
			name:        "invitee preference wins",
			invitee:     &model.User{ID: 1, Email: "invitee@example.com", Name: "Invitee", LanguagePreference: &en},
			ctx:         WithEmailActorLanguage(context.Background(), string(zh)),
			wantSubject: "[Rota] Invitation to Rota",
		},
		{
			name:        "actor preference after invitee",
			invitee:     &model.User{ID: 1, Email: "invitee@example.com", Name: "Invitee"},
			ctx:         WithEmailActorLanguage(context.Background(), string(zh)),
			wantSubject: "[Rota] Rota 邀请",
		},
		{
			name:        "request fallback after persisted preferences",
			invitee:     &model.User{ID: 1, Email: "invitee@example.com", Name: "Invitee"},
			ctx:         WithEmailRequestLanguage(context.Background(), "zh-CN,zh;q=0.9,en;q=0.8"),
			wantSubject: "[Rota] Rota 邀请",
		},
		{
			name:        "unsupported falls back to English",
			invitee:     &model.User{ID: 1, Email: "invitee@example.com", Name: "Invitee"},
			ctx:         WithEmailRequestLanguage(context.Background(), "fr-FR,fr;q=0.9"),
			wantSubject: "[Rota] Invitation to Rota",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var sent email.Message
			helper := newSetupFlowHelper(SetupFlowConfig{
				OutboxRepo: &emailerMock{sendFunc: func(ctx context.Context, msg email.Message) error {
					sent = msg
					return nil
				}},
				AppBaseURL:         "https://app.example.com",
				InvitationTokenTTL: 72 * time.Hour,
			})

			if err := helper.enqueueInvitationTx(tt.ctx, nil, tt.invitee, "token"); err != nil {
				t.Fatalf("enqueueInvitationTx returned error: %v", err)
			}
			if sent.Kind != email.KindInvitation || sent.Subject != tt.wantSubject || sent.HTMLBody == "" {
				t.Fatalf("unexpected invitation message: %+v", sent)
			}
		})
	}
}

func TestPasswordResetEmailLanguageSelection(t *testing.T) {
	t.Parallel()

	var sent email.Message
	helper := newSetupFlowHelper(SetupFlowConfig{
		OutboxRepo: &emailerMock{sendFunc: func(ctx context.Context, msg email.Message) error {
			sent = msg
			return nil
		}},
		AppBaseURL:            "https://app.example.com",
		PasswordResetTokenTTL: time.Hour,
	})

	user := &model.User{ID: 2, Email: "worker@example.com", Name: "Worker"}
	ctx := WithEmailRequestLanguage(context.Background(), "zh-CN,zh;q=0.9")
	if err := helper.enqueuePasswordResetTx(ctx, nil, user, "token"); err != nil {
		t.Fatalf("enqueuePasswordResetTx returned error: %v", err)
	}
	if sent.Kind != email.KindPasswordReset || !strings.Contains(sent.Subject, "重置") || !strings.Contains(sent.Body, "重置") {
		t.Fatalf("expected Chinese password reset message, got %+v", sent)
	}
}

func TestEmailChangeEmailLanguageSelection(t *testing.T) {
	t.Parallel()

	en := model.LanguagePreferenceEN
	var messages []email.Message
	helper := newSetupFlowHelper(SetupFlowConfig{
		OutboxRepo: &emailerMock{sendFunc: func(ctx context.Context, msg email.Message) error {
			messages = append(messages, msg)
			return nil
		}},
		AppBaseURL: "https://app.example.com",
	})

	user := &model.User{ID: 3, Email: "old@example.com", Name: "Worker", LanguagePreference: &en}
	ctx := WithEmailRequestLanguage(context.Background(), "zh-CN,zh;q=0.9")
	if err := helper.enqueueEmailChangeConfirmTx(ctx, nil, user, "new@example.com", "token"); err != nil {
		t.Fatalf("enqueueEmailChangeConfirmTx returned error: %v", err)
	}
	if err := helper.enqueueEmailChangeNoticeTx(ctx, nil, user, "new@example.com"); err != nil {
		t.Fatalf("enqueueEmailChangeNoticeTx returned error: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("messages length = %d, want 2", len(messages))
	}
	for _, msg := range messages {
		if strings.Contains(msg.Subject, "邮箱") || strings.Contains(msg.Body, "你好") {
			t.Fatalf("persisted English preference should beat request language, got %+v", msg)
		}
	}
}

func TestShiftChangeResolvedEmailLanguageResolution(t *testing.T) {
	t.Parallel()

	en := model.LanguagePreferenceEN
	if got := resolveRequestEmailLanguage(
		WithEmailRequestLanguage(context.Background(), "zh-CN,zh;q=0.9"),
		&model.User{LanguagePreference: &en},
	); got != "en" {
		t.Fatalf("recipient preference should win, got %q", got)
	}
	if got := resolveRequestEmailLanguage(
		WithEmailRequestLanguage(context.Background(), "zh-CN,zh;q=0.9"),
		&model.User{},
	); got != "zh" {
		t.Fatalf("request language should be used when recipient has none, got %q", got)
	}
	if got := resolveSystemEmailLanguage(&model.User{}); got != "en" {
		t.Fatalf("system emails should fall back to English, got %q", got)
	}
}

func TestPublicationInvalidationEmailLanguageResolution(t *testing.T) {
	t.Parallel()

	zh := model.LanguagePreferenceZH
	if got := resolveSystemEmailLanguage(&model.User{LanguagePreference: &zh}); got != "zh" {
		t.Fatalf("system invalidation should use recipient preference, got %q", got)
	}
	if got := resolveSystemEmailLanguage(&model.User{}); got != "en" {
		t.Fatalf("system invalidation should fall back to English, got %q", got)
	}
}
