package email

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestLoggerEmailerSend(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	emailer := NewLoggerEmailer(&output)

	err := emailer.Send(context.Background(), Message{
		To:       "worker@example.com",
		Subject:  "Test subject",
		Body:     "Line one\nLine two",
		HTMLBody: "<strong>Line one</strong>",
	})
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}

	written := output.String()
	for _, want := range []string{
		"=== EMAIL (dev) ===",
		"To: worker@example.com",
		"Subject: Test subject",
		"HTML: yes",
		"Line one",
		"Line two",
	} {
		if !strings.Contains(written, want) {
			t.Fatalf("expected output to contain %q, got %q", want, written)
		}
	}
	if strings.Contains(written, "<strong>Line one</strong>") {
		t.Fatalf("logger output must not include HTML body: %q", written)
	}
}

func TestBuildInvitationMessage(t *testing.T) {
	t.Parallel()

	msg := BuildInvitationMessage(TemplateData{
		To:         "worker@example.com",
		Name:       "Worker",
		BaseURL:    "http://localhost:5173",
		Token:      "setup-token",
		Language:   "en",
		Expiration: 72 * time.Hour,
	})

	if msg.To != "worker@example.com" {
		t.Fatalf("expected recipient to match, got %q", msg.To)
	}
	if msg.Kind != KindInvitation {
		t.Fatalf("unexpected kind: %q", msg.Kind)
	}
	if msg.Subject != "[Rota] Invitation to Rota" {
		t.Fatalf("unexpected subject: %q", msg.Subject)
	}
	for _, want := range []string{
		"Hi Worker,",
		"http://localhost:5173/setup-password?token=setup-token",
		"72 hours",
	} {
		if !strings.Contains(msg.Body, want) {
			t.Fatalf("expected body to contain %q, got %q", want, msg.Body)
		}
	}
	for _, want := range []string{
		"<!doctype html>",
		"Set password",
		"http://localhost:5173/setup-password?token=setup-token",
	} {
		if !strings.Contains(msg.HTMLBody, want) {
			t.Fatalf("expected HTML body to contain %q, got %q", want, msg.HTMLBody)
		}
	}
}

func TestBuildPasswordResetMessage(t *testing.T) {
	t.Parallel()

	msg := BuildPasswordResetMessage(TemplateData{
		To:         "worker@example.com",
		Name:       "Worker",
		BaseURL:    "https://app.example.com/base/",
		Token:      "reset-token",
		Language:   "en",
		Expiration: time.Hour,
	})

	if msg.Kind != KindPasswordReset {
		t.Fatalf("unexpected kind: %q", msg.Kind)
	}
	if msg.Subject != "[Rota] Reset your Rota password" {
		t.Fatalf("unexpected subject: %q", msg.Subject)
	}
	for _, want := range []string{
		"https://app.example.com/base/setup-password?token=reset-token",
		"1 hour",
		"If this was not you",
	} {
		if !strings.Contains(msg.Body, want) {
			t.Fatalf("expected body to contain %q, got %q", want, msg.Body)
		}
	}
	if !strings.Contains(msg.HTMLBody, "Reset password") || !strings.Contains(msg.HTMLBody, "https://app.example.com/base/setup-password?token=reset-token") {
		t.Fatalf("expected HTML reset CTA and fallback URL, got %q", msg.HTMLBody)
	}
}

func TestBuildEmailChangeConfirmMessage(t *testing.T) {
	t.Parallel()

	msg := BuildEmailChangeConfirmMessage(TemplateData{
		To:         "alice2@example.com",
		Name:       "Alice",
		BaseURL:    "https://app.example.com",
		Token:      "email-token",
		Language:   "en",
		Expiration: 24 * time.Hour,
	})

	if msg.To != "alice2@example.com" {
		t.Fatalf("expected recipient to match, got %q", msg.To)
	}
	if msg.Kind != KindEmailChangeConfirm {
		t.Fatalf("unexpected kind: %q", msg.Kind)
	}
	if msg.Subject != "[Rota] Confirm your Rota email change" {
		t.Fatalf("unexpected subject: %q", msg.Subject)
	}
	for _, want := range []string{
		"Hi Alice,",
		"https://app.example.com/auth/confirm-email-change?token=email-token",
		"24 hours",
	} {
		if !strings.Contains(msg.Body, want) {
			t.Fatalf("expected body to contain %q, got %q", want, msg.Body)
		}
	}
	if !strings.Contains(msg.HTMLBody, "Confirm email change") || !strings.Contains(msg.HTMLBody, "https://app.example.com/auth/confirm-email-change?token=email-token") {
		t.Fatalf("expected HTML confirmation CTA and fallback URL, got %q", msg.HTMLBody)
	}
}

func TestBuildEmailChangeNoticeMessage(t *testing.T) {
	t.Parallel()

	msg := BuildEmailChangeNoticeMessage(TemplateData{
		To:              "alice@example.com",
		Name:            "Alice",
		Language:        "en",
		NewEmailPartial: PartialMaskEmail("alice2@example.com"),
	})

	if msg.To != "alice@example.com" {
		t.Fatalf("expected recipient to match, got %q", msg.To)
	}
	if msg.Kind != KindEmailChangeNotice {
		t.Fatalf("unexpected kind: %q", msg.Kind)
	}
	if msg.Subject != "[Rota] Rota email change requested" {
		t.Fatalf("unexpected subject: %q", msg.Subject)
	}
	for _, want := range []string{
		"Hi Alice,",
		"a***@example.com",
		"change your password",
	} {
		if !strings.Contains(msg.Body, want) {
			t.Fatalf("expected body to contain %q, got %q", want, msg.Body)
		}
	}
	if strings.Contains(msg.Body, "/auth/confirm-email-change") ||
		strings.Contains(msg.Body, "?token=") ||
		strings.Contains(msg.HTMLBody, "/auth/confirm-email-change") ||
		strings.Contains(msg.HTMLBody, "?token=") {
		t.Fatalf("notice bodies must not contain an actionable confirmation link: text=%q html=%q", msg.Body, msg.HTMLBody)
	}
}

func TestAccountEmailBranding(t *testing.T) {
	t.Parallel()

	branding := Branding{
		ProductName:      "排班系统",
		OrganizationName: "Acme",
	}

	invitation := BuildInvitationMessage(TemplateData{
		To:         "worker@example.com",
		Name:       "Worker",
		BaseURL:    "https://app.example.com",
		Token:      "setup-token",
		Language:   "en",
		Expiration: 72 * time.Hour,
		Branding:   branding,
	})
	assertContainsAll(t, invitation.Subject, "[排班系统]", "Invitation to 排班系统")
	assertContainsAll(t, invitation.Body, "administrator from Acme", "invited you to 排班系统", "notification from Acme")
	assertContainsAll(t, invitation.HTMLBody, "排班系统", "Acme")

	passwordReset := BuildPasswordResetMessage(TemplateData{
		To:         "worker@example.com",
		Name:       "Worker",
		BaseURL:    "https://app.example.com",
		Token:      "reset-token",
		Language:   "en",
		Expiration: time.Hour,
		Branding:   branding,
	})
	assertContainsAll(t, passwordReset.Subject, "[排班系统]", "Reset your 排班系统 password")
	assertContainsAll(t, passwordReset.Body, "your 排班系统 account", "notification from Acme")
	assertContainsNone(t, passwordReset.Body, "invited you", "administrator from Acme")

	notice := BuildEmailChangeNoticeMessage(TemplateData{
		To:              "worker@example.com",
		Name:            "Worker",
		Language:        "zh",
		NewEmailPartial: "w***@example.com",
		Branding:        branding,
	})
	assertContainsAll(t, notice.Subject, "[排班系统]", "排班系统 邮箱变更请求")
	assertContainsAll(t, notice.Body, "你的 排班系统 账号", "来自Acme的排班系统自动通知邮件")
}

func TestAccountEmailBrandingOmitsBlankOrganization(t *testing.T) {
	t.Parallel()

	msg := BuildInvitationMessage(TemplateData{
		To:         "worker@example.com",
		Name:       "Worker",
		BaseURL:    "https://app.example.com",
		Token:      "setup-token",
		Language:   "en",
		Expiration: 72 * time.Hour,
		Branding:   Branding{ProductName: "OpsHub", OrganizationName: "  "},
	})

	assertContainsAll(t, msg.Subject, "[OpsHub]", "Invitation to OpsHub")
	assertContainsAll(t, msg.Body, "invited you to OpsHub", "This is an automated OpsHub notification.")
	assertContainsNone(t, msg.Body, "from  ", "from .", "administrator from")
}

func TestShiftChangeEmailBranding(t *testing.T) {
	t.Parallel()

	date := time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC)
	branding := Branding{ProductName: "OpsHub", OrganizationName: "Acme"}

	request := BuildShiftChangeRequestReceivedMessage(ShiftChangeRequestReceivedData{
		To:            "bob@example.com",
		RecipientName: "Bob",
		RequesterName: "Alice",
		Type:          ShiftChangeTypeSwap,
		RequesterShift: ShiftRef{
			Weekday:        "Mon",
			StartTime:      "09:00",
			EndTime:        "12:00",
			PositionName:   "Front Desk",
			OccurrenceDate: &date,
		},
		BaseURL:  "https://app.example.com",
		Language: "en",
		Branding: branding,
	})
	assertContainsAll(t, request.Subject, "[OpsHub]", "New shift change request")
	assertContainsAll(t, request.Body, "This is an automated OpsHub notification from Acme.")
	assertContainsNone(t, request.Body, "Rota")

	resolved := BuildShiftChangeResolvedMessage(ShiftChangeResolvedData{
		To:            "alice@example.com",
		RecipientName: "Alice",
		Outcome:       ShiftChangeOutcomeApproved,
		Type:          ShiftChangeTypeSwap,
		ResponderName: "Bob",
		RequesterShift: ShiftRef{
			Weekday:        "Mon",
			StartTime:      "09:00",
			EndTime:        "12:00",
			PositionName:   "Front Desk",
			OccurrenceDate: &date,
		},
		BaseURL:  "https://app.example.com",
		Language: "zh",
		Branding: Branding{ProductName: "排班系统", OrganizationName: "Acme"},
	})
	assertContainsAll(t, resolved.Subject, "[排班系统]", "换班申请已批准")
	assertContainsAll(t, resolved.Body, "来自Acme的排班系统自动通知邮件")
	assertContainsNone(t, resolved.Body, "Rota")
}

func TestParseAcceptLanguage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		header string
		want   string
	}{
		{header: "zh-CN,zh;q=0.9,en;q=0.8", want: "zh"},
		{header: "en-US,en;q=0.9", want: "en"},
		{header: "fr-FR,fr;q=0.9", want: "en"},
		{header: "en;q=0.2,zh;q=0.9", want: "zh"},
		{header: "", want: "en"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.header, func(t *testing.T) {
			t.Parallel()

			if got := ParseAcceptLanguage(tt.header); got != tt.want {
				t.Fatalf("ParseAcceptLanguage(%q) = %q, want %q", tt.header, got, tt.want)
			}
		})
	}
}

func TestTemplateRenderingAllKindsAndLanguages(t *testing.T) {
	t.Parallel()

	date := time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC)
	counterpartDate := time.Date(2026, 5, 5, 0, 0, 0, 0, time.UTC)
	for _, language := range []string{"en", "zh"} {
		language := language
		t.Run(language, func(t *testing.T) {
			t.Parallel()

			fixture := templateExpectationsForLanguage(language, date, counterpartDate)
			for _, expectation := range fixture.messages {
				expectation := expectation
				t.Run(expectation.name, func(t *testing.T) {
					t.Parallel()

					msg := expectation.message
					if !strings.HasPrefix(msg.Subject, "[Rota] ") {
						t.Fatalf("%s subject missing prefix: %q", msg.Kind, msg.Subject)
					}
					if msg.Body == "" || msg.HTMLBody == "" {
						t.Fatalf("%s should render text and HTML bodies", msg.Kind)
					}

					assertContainsAll(t, msg.Subject, expectation.subjectContains...)
					assertContainsAll(t, msg.Body, append([]string{fixture.footer}, expectation.textContains...)...)
					assertContainsAll(t, msg.HTMLBody, append([]string{"<!doctype html>", fixture.footer}, expectation.htmlContains...)...)
					assertContainsNone(t, msg.Body, expectation.forbidden...)
					assertContainsNone(t, msg.HTMLBody, expectation.forbidden...)
					if expectation.token != "" {
						assertTokenOnlyAppearsInURL(t, msg.Body, expectation.token, expectation.tokenURL)
						assertTokenOnlyAppearsInURL(t, msg.HTMLBody, expectation.token, expectation.tokenURL)
					}
				})
			}
		})
	}
}

func TestShiftChangeOutcomeLabelsAreLocalized(t *testing.T) {
	t.Parallel()

	date := time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC)
	outcomes := []struct {
		outcome ShiftChangeOutcome
		en      string
		zh      string
	}{
		{outcome: ShiftChangeOutcomeApproved, en: "approved", zh: "已批准"},
		{outcome: ShiftChangeOutcomeRejected, en: "rejected", zh: "已拒绝"},
		{outcome: ShiftChangeOutcomeClaimed, en: "claimed", zh: "已认领"},
		{outcome: ShiftChangeOutcomeCancelled, en: "cancelled", zh: "已取消"},
		{outcome: ShiftChangeOutcomeInvalidated, en: "invalidated", zh: "已失效"},
	}

	for _, tt := range outcomes {
		tt := tt
		t.Run(string(tt.outcome), func(t *testing.T) {
			t.Parallel()

			for _, language := range []string{"en", "zh"} {
				language := language
				t.Run(language, func(t *testing.T) {
					t.Parallel()

					want := tt.en
					if language == "zh" {
						want = tt.zh
					}
					msg := BuildShiftChangeResolvedMessage(ShiftChangeResolvedData{
						To:            "a@example.com",
						RecipientName: "Alice",
						Outcome:       tt.outcome,
						Type:          ShiftChangeTypeSwap,
						ResponderName: "Bob",
						RequesterShift: ShiftRef{
							Weekday:        "Mon",
							StartTime:      "09:00",
							EndTime:        "12:00",
							PositionName:   "Front Desk",
							OccurrenceDate: &date,
						},
						BaseURL:  "https://app.example.com",
						Language: language,
					})

					assertContainsAll(t, msg.Subject, want)
					assertContainsAll(t, msg.Body, want)
					assertContainsAll(t, msg.HTMLBody, want)
				})
			}
		})
	}
}

type templateLanguageExpectations struct {
	footer   string
	messages []templateMessageExpectation
}

type templateMessageExpectation struct {
	name            string
	message         Message
	subjectContains []string
	textContains    []string
	htmlContains    []string
	forbidden       []string
	token           string
	tokenURL        string
}

func templateExpectationsForLanguage(language string, date, counterpartDate time.Time) templateLanguageExpectations {
	setupURL := "https://app.example.com/setup-password?token=setup-token"
	resetURL := "https://app.example.com/setup-password?token=reset-token"
	confirmURL := "https://app.example.com/auth/confirm-email-change?token=email-token"
	requestsURL := "https://app.example.com/requests"
	requesterShiftEN := "Mon, May 4, 2026, 09:00-12:00 Front Desk"
	counterpartShiftEN := "Tue, May 5, 2026, 13:00-16:00 Support"
	requesterShiftZH := "2026-05-04（周一）09:00-12:00 Front Desk"
	counterpartShiftZH := "2026-05-05（周二）13:00-16:00 Support"
	commonForbiddenForNotice := []string{
		"?token=",
		"/setup-password",
		"/auth/confirm-email-change",
		"Set password",
		"Reset password",
		"Confirm email change",
		"设置密码",
		"重置密码",
		"确认邮箱变更",
	}

	if language == "zh" {
		return templateLanguageExpectations{
			footer: "这是一封 Rota 自动通知邮件。",
			messages: []templateMessageExpectation{
				{
					name:            "invitation",
					message:         BuildInvitationMessage(TemplateData{To: "a@example.com", Name: "Alice", BaseURL: "https://app.example.com", Token: "setup-token", Language: language, Expiration: 72 * time.Hour}),
					subjectContains: []string{"Rota 邀请"},
					textContains:    []string{"Alice，你好", "设置密码", setupURL, "如果按钮无法打开", "72 小时"},
					htmlContains:    []string{"Alice，你好", "设置密码", setupURL, "如果按钮无法打开", "72 小时"},
					token:           "setup-token",
					tokenURL:        setupURL,
				},
				{
					name:            "password_reset",
					message:         BuildPasswordResetMessage(TemplateData{To: "a@example.com", Name: "Alice", BaseURL: "https://app.example.com", Token: "reset-token", Language: language, Expiration: time.Hour}),
					subjectContains: []string{"重置 Rota 密码"},
					textContains:    []string{"Alice，你好", "重置密码", resetURL, "如果按钮无法打开", "1 小时"},
					htmlContains:    []string{"Alice，你好", "重置密码", resetURL, "如果按钮无法打开", "1 小时"},
					token:           "reset-token",
					tokenURL:        resetURL,
				},
				{
					name:            "email_change_confirm",
					message:         BuildEmailChangeConfirmMessage(TemplateData{To: "new@example.com", Name: "Alice", BaseURL: "https://app.example.com", Token: "email-token", Language: language, Expiration: 24 * time.Hour}),
					subjectContains: []string{"确认 Rota 邮箱变更"},
					textContains:    []string{"Alice，你好", "确认邮箱变更", confirmURL, "如果按钮无法打开", "24 小时"},
					htmlContains:    []string{"Alice，你好", "确认邮箱变更", confirmURL, "如果按钮无法打开", "24 小时"},
					token:           "email-token",
					tokenURL:        confirmURL,
				},
				{
					name:            "email_change_notice",
					message:         BuildEmailChangeNoticeMessage(TemplateData{To: "a@example.com", Name: "Alice", Language: language, NewEmailPartial: "n***@example.com"}),
					subjectContains: []string{"Rota 邮箱变更请求"},
					textContains:    []string{"Alice，你好", "邮箱变更请求", "n***@example.com", "立即修改密码"},
					htmlContains:    []string{"Alice，你好", "邮箱变更请求", "n***@example.com", "立即修改密码"},
					forbidden:       commonForbiddenForNotice,
				},
				{
					name: "shift_change_request_received",
					message: BuildShiftChangeRequestReceivedMessage(ShiftChangeRequestReceivedData{
						To:            "b@example.com",
						RecipientName: "Bob",
						RequesterName: "Alice",
						Type:          ShiftChangeTypeSwap,
						RequesterShift: ShiftRef{
							Weekday:        "Mon",
							StartTime:      "09:00",
							EndTime:        "12:00",
							PositionName:   "Front Desk",
							OccurrenceDate: &date,
						},
						CounterpartShift: &ShiftRef{
							Weekday:        "Tue",
							StartTime:      "13:00",
							EndTime:        "16:00",
							PositionName:   "Support",
							OccurrenceDate: &counterpartDate,
						},
						BaseURL:  "https://app.example.com",
						Language: language,
					}),
					subjectContains: []string{"新的换班申请"},
					textContains:    []string{"Bob，你好", "Alice", "换班", "对方班次", requesterShiftZH, "你的班次", counterpartShiftZH, "查看申请", requestsURL, "如果按钮无法打开"},
					htmlContains:    []string{"Bob，你好", "Alice", "换班", "对方班次", requesterShiftZH, "你的班次", counterpartShiftZH, "查看申请", requestsURL, "如果按钮无法打开"},
				},
				{
					name: "shift_change_resolved",
					message: BuildShiftChangeResolvedMessage(ShiftChangeResolvedData{
						To:            "a@example.com",
						RecipientName: "Alice",
						Outcome:       ShiftChangeOutcomeApproved,
						Type:          ShiftChangeTypeSwap,
						ResponderName: "Bob",
						RequesterShift: ShiftRef{
							Weekday:        "Mon",
							StartTime:      "09:00",
							EndTime:        "12:00",
							PositionName:   "Front Desk",
							OccurrenceDate: &date,
						},
						BaseURL:  "https://app.example.com",
						Language: language,
					}),
					subjectContains: []string{"换班申请已批准"},
					textContains:    []string{"Alice，你好", "换班", "Bob", "已批准", "你的班次", requesterShiftZH, "查看申请", requestsURL, "如果按钮无法打开"},
					htmlContains:    []string{"Alice，你好", "换班", "Bob", "已批准", "你的班次", requesterShiftZH, "查看申请", requestsURL, "如果按钮无法打开"},
				},
			},
		}
	}

	return templateLanguageExpectations{
		footer: "This is an automated Rota notification.",
		messages: []templateMessageExpectation{
			{
				name:            "invitation",
				message:         BuildInvitationMessage(TemplateData{To: "a@example.com", Name: "Alice", BaseURL: "https://app.example.com", Token: "setup-token", Language: language, Expiration: 72 * time.Hour}),
				subjectContains: []string{"Invitation to Rota"},
				textContains:    []string{"Hi Alice", "Set password", setupURL, "If the button does not work", "72 hours"},
				htmlContains:    []string{"Hi Alice", "Set password", setupURL, "If the button does not work", "72 hours"},
				token:           "setup-token",
				tokenURL:        setupURL,
			},
			{
				name:            "password_reset",
				message:         BuildPasswordResetMessage(TemplateData{To: "a@example.com", Name: "Alice", BaseURL: "https://app.example.com", Token: "reset-token", Language: language, Expiration: time.Hour}),
				subjectContains: []string{"Reset your Rota password"},
				textContains:    []string{"Hi Alice", "Reset password", resetURL, "If the button does not work", "1 hour"},
				htmlContains:    []string{"Hi Alice", "Reset password", resetURL, "If the button does not work", "1 hour"},
				token:           "reset-token",
				tokenURL:        resetURL,
			},
			{
				name:            "email_change_confirm",
				message:         BuildEmailChangeConfirmMessage(TemplateData{To: "new@example.com", Name: "Alice", BaseURL: "https://app.example.com", Token: "email-token", Language: language, Expiration: 24 * time.Hour}),
				subjectContains: []string{"Confirm your Rota email change"},
				textContains:    []string{"Hi Alice", "Confirm email change", confirmURL, "If the button does not work", "24 hours"},
				htmlContains:    []string{"Hi Alice", "Confirm email change", confirmURL, "If the button does not work", "24 hours"},
				token:           "email-token",
				tokenURL:        confirmURL,
			},
			{
				name:            "email_change_notice",
				message:         BuildEmailChangeNoticeMessage(TemplateData{To: "a@example.com", Name: "Alice", Language: language, NewEmailPartial: "n***@example.com"}),
				subjectContains: []string{"Rota email change requested"},
				textContains:    []string{"Hi Alice", "email-change request", "n***@example.com", "change your password"},
				htmlContains:    []string{"Hi Alice", "email-change request", "n***@example.com", "change your password"},
				forbidden:       commonForbiddenForNotice,
			},
			{
				name: "shift_change_request_received",
				message: BuildShiftChangeRequestReceivedMessage(ShiftChangeRequestReceivedData{
					To:            "b@example.com",
					RecipientName: "Bob",
					RequesterName: "Alice",
					Type:          ShiftChangeTypeSwap,
					RequesterShift: ShiftRef{
						Weekday:        "Mon",
						StartTime:      "09:00",
						EndTime:        "12:00",
						PositionName:   "Front Desk",
						OccurrenceDate: &date,
					},
					CounterpartShift: &ShiftRef{
						Weekday:        "Tue",
						StartTime:      "13:00",
						EndTime:        "16:00",
						PositionName:   "Support",
						OccurrenceDate: &counterpartDate,
					},
					BaseURL:  "https://app.example.com",
					Language: language,
				}),
				subjectContains: []string{"New shift change request"},
				textContains:    []string{"Hi Bob", "Alice", "swap", "Their shift", requesterShiftEN, "Your shift", counterpartShiftEN, "View request", requestsURL, "If the button does not work"},
				htmlContains:    []string{"Hi Bob", "Alice", "swap", "Their shift", requesterShiftEN, "Your shift", counterpartShiftEN, "View request", requestsURL, "If the button does not work"},
			},
			{
				name: "shift_change_resolved",
				message: BuildShiftChangeResolvedMessage(ShiftChangeResolvedData{
					To:            "a@example.com",
					RecipientName: "Alice",
					Outcome:       ShiftChangeOutcomeApproved,
					Type:          ShiftChangeTypeSwap,
					ResponderName: "Bob",
					RequesterShift: ShiftRef{
						Weekday:        "Mon",
						StartTime:      "09:00",
						EndTime:        "12:00",
						PositionName:   "Front Desk",
						OccurrenceDate: &date,
					},
					BaseURL:  "https://app.example.com",
					Language: language,
				}),
				subjectContains: []string{"Shift change request approved"},
				textContains:    []string{"Hi Alice", "swap", "Bob", "approved", "Your shift", requesterShiftEN, "View request", requestsURL, "If the button does not work"},
				htmlContains:    []string{"Hi Alice", "swap", "Bob", "approved", "Your shift", requesterShiftEN, "View request", requestsURL, "If the button does not work"},
			},
		},
	}
}

func assertContainsAll(t *testing.T, value string, wants ...string) {
	t.Helper()

	for _, want := range wants {
		if !strings.Contains(value, want) {
			t.Fatalf("expected value to contain %q, got %q", want, value)
		}
	}
}

func assertContainsNone(t *testing.T, value string, forbidden ...string) {
	t.Helper()

	for _, want := range forbidden {
		if strings.Contains(value, want) {
			t.Fatalf("expected value not to contain %q, got %q", want, value)
		}
	}
}

func assertTokenOnlyAppearsInURL(t *testing.T, value, token, url string) {
	t.Helper()

	tokenCount := strings.Count(value, token)
	urlCount := strings.Count(value, url)
	if tokenCount == 0 {
		t.Fatalf("expected token %q to appear in URL %q, got %q", token, url, value)
	}
	if tokenCount != urlCount {
		t.Fatalf("token %q should only appear inside URL %q: token count=%d url count=%d body=%q", token, url, tokenCount, urlCount, value)
	}
}

func TestShiftSummaryLocalizesOccurrenceDate(t *testing.T) {
	t.Parallel()

	date := time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC)
	ref := ShiftRef{
		Weekday:        "Mon",
		StartTime:      "09:00",
		EndTime:        "12:00",
		PositionName:   "Front Desk Assistant",
		OccurrenceDate: &date,
	}

	if got := FormatShiftRef(ref, "en"); got != "Mon, May 4, 2026, 09:00-12:00 Front Desk Assistant" {
		t.Fatalf("English shift summary = %q", got)
	}
	if got := FormatShiftRef(ref, "zh"); got != "2026-05-04（周一）09:00-12:00 Front Desk Assistant" {
		t.Fatalf("Chinese shift summary = %q", got)
	}
}

func TestPartialMaskEmail(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "standard", in: "alice@example.com", want: "a***@example.com"},
		{name: "single rune", in: "a@example.com", want: "a***@example.com"},
		{name: "invalid", in: "not-an-email", want: "***"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := PartialMaskEmail(tt.in); got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestNewSMTPEmailerRejectsPlainAuthWithoutTLS(t *testing.T) {
	t.Parallel()

	_, err := NewSMTPEmailer(withTLSMode(SMTPConfig{
		Host:     "localhost",
		Port:     1025,
		User:     "mailer",
		Password: "secret",
		From:     "Rota <noreply@example.com>",
	}, "none"))
	if err == nil {
		t.Fatal("expected error for SMTP user in none TLS mode")
	}
}

func TestNewSMTPEmailerAcceptsSupportedTLSModes(t *testing.T) {
	t.Parallel()

	for _, mode := range []string{"starttls", "implicit"} {
		mode := mode
		t.Run(mode, func(t *testing.T) {
			t.Parallel()

			_, err := NewSMTPEmailer(withTLSMode(SMTPConfig{
				Host: "localhost",
				Port: 587,
				From: "Rota <noreply@example.com>",
			}, mode))
			if err != nil {
				t.Fatalf("NewSMTPEmailer returned error for mode %q: %v", mode, err)
			}
		})
	}
}

func TestSMTPEmailerSendRequiresSTARTTLSByDefault(t *testing.T) {
	t.Parallel()

	server := newSMTPTestServer(t, smtpTestServerOptions{})
	defer server.Close()

	emailer, err := NewSMTPEmailer(SMTPConfig{
		Host: "localhost",
		Port: server.Port(),
		From: "Rota <noreply@example.com>",
	})
	if err != nil {
		t.Fatalf("NewSMTPEmailer returned error: %v", err)
	}

	err = emailer.Send(context.Background(), Message{
		To:      "worker@example.com",
		Subject: "Invitation",
		Body:    "Please finish setup.",
	})
	if err == nil {
		t.Fatal("expected Send to fail when STARTTLS is unavailable")
	}
}

func TestSMTPEmailerSendWithSTARTTLSSucceeds(t *testing.T) {
	t.Parallel()

	server := newSMTPTestServer(t, smtpTestServerOptions{advertiseStartTLS: true})
	defer server.Close()

	emailer, err := NewSMTPEmailer(withTLSMode(SMTPConfig{
		Host: "localhost",
		Port: server.Port(),
		From: "Rota <noreply@example.com>",
	}, "starttls"))
	if err != nil {
		t.Fatalf("NewSMTPEmailer returned error: %v", err)
	}
	trustServerCertificate(t, emailer, server.Certificate())

	err = emailer.Send(context.Background(), Message{
		To:      "worker@example.com",
		Subject: "Invitation",
		Body:    "Please finish setup.",
	})
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	if !server.StartTLSUsed() {
		t.Fatal("expected SMTP session to upgrade with STARTTLS")
	}
	if body := server.LastMessage(); !strings.Contains(body, "Please finish setup.") {
		t.Fatalf("expected delivered message body, got %q", body)
	}
}

func TestSMTPEmailerSendWithImplicitTLSSucceeds(t *testing.T) {
	t.Parallel()

	server := newSMTPTestServer(t, smtpTestServerOptions{implicitTLS: true})
	defer server.Close()

	emailer, err := NewSMTPEmailer(withTLSMode(SMTPConfig{
		Host: "localhost",
		Port: server.Port(),
		From: "Rota <noreply@example.com>",
	}, "implicit"))
	if err != nil {
		t.Fatalf("NewSMTPEmailer returned error: %v", err)
	}
	trustServerCertificate(t, emailer, server.Certificate())

	err = emailer.Send(context.Background(), Message{
		To:      "worker@example.com",
		Subject: "Reset",
		Body:    "Reset instructions.",
	})
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	if body := server.LastMessage(); !strings.Contains(body, "Reset instructions.") {
		t.Fatalf("expected delivered message body, got %q", body)
	}
}

func TestSMTPEmailerSendWithoutTLSDeliversWithoutAuth(t *testing.T) {
	t.Parallel()

	server := newSMTPTestServer(t, smtpTestServerOptions{})
	defer server.Close()

	emailer, err := NewSMTPEmailer(withTLSMode(SMTPConfig{
		Host: "localhost",
		Port: server.Port(),
		From: "Rota <noreply@example.com>",
	}, "none"))
	if err != nil {
		t.Fatalf("NewSMTPEmailer returned error: %v", err)
	}

	err = emailer.Send(context.Background(), Message{
		To:      "worker@example.com",
		Subject: "Local dev",
		Body:    "Mailpit relay",
	})
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	if server.StartTLSUsed() {
		t.Fatal("expected none mode to skip STARTTLS")
	}
	if body := server.LastMessage(); !strings.Contains(body, "Mailpit relay") {
		t.Fatalf("expected delivered message body, got %q", body)
	}
}

func TestSMTPEmailerSendHonorsContextTimeoutDuringSMTPExchange(t *testing.T) {
	t.Parallel()

	server := newSMTPTestServer(t, smtpTestServerOptions{hangAfterData: true})
	defer server.Close()

	emailer, err := NewSMTPEmailer(withTLSMode(SMTPConfig{
		Host: "localhost",
		Port: server.Port(),
		From: "Rota <noreply@example.com>",
	}, "none"))
	if err != nil {
		t.Fatalf("NewSMTPEmailer returned error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	startedAt := time.Now()
	err = emailer.Send(ctx, Message{
		To:      "worker@example.com",
		Subject: "Timeout",
		Body:    "This send should time out.",
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context deadline error, got %v", err)
	}
	if elapsed := time.Since(startedAt); elapsed > time.Second {
		t.Fatalf("Send ignored context deadline for too long: elapsed=%s", elapsed)
	}
}

func TestSMTPEmailerSendHonorsContextTimeoutWaitingForGreeting(t *testing.T) {
	t.Parallel()

	server := newSMTPTestServer(t, smtpTestServerOptions{hangBeforeGreeting: true})
	defer server.Close()

	emailer, err := NewSMTPEmailer(withTLSMode(SMTPConfig{
		Host: "localhost",
		Port: server.Port(),
		From: "Rota <noreply@example.com>",
	}, "none"))
	if err != nil {
		t.Fatalf("NewSMTPEmailer returned error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	startedAt := time.Now()
	err = emailer.Send(ctx, Message{
		To:      "worker@example.com",
		Subject: "Timeout",
		Body:    "This send should time out before greeting.",
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context deadline error, got %v", err)
	}
	if elapsed := time.Since(startedAt); elapsed > time.Second {
		t.Fatalf("Send ignored context deadline for too long: elapsed=%s", elapsed)
	}
}

func TestSMTPEmailerRenderPayloadMultipartAlternative(t *testing.T) {
	t.Parallel()

	emailer := &SMTPEmailer{fromLine: "Rota <noreply@example.com>"}
	payload, err := emailer.renderPayload(Message{
		To:       "worker@example.com",
		Subject:  "测试 subject",
		Body:     "Plain body",
		HTMLBody: "<p>HTML body</p>",
	})
	if err != nil {
		t.Fatalf("renderPayload returned error: %v", err)
	}

	body := string(payload)
	for _, want := range []string{
		"MIME-Version: 1.0",
		"Content-Type: multipart/alternative",
		"Subject: =?utf-8?",
		"Content-Type: text/plain; charset=UTF-8",
		"Content-Type: text/html; charset=UTF-8",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected payload to contain %q, got %q", want, body)
		}
	}
	plainIndex := strings.Index(body, "Content-Type: text/plain; charset=UTF-8")
	htmlIndex := strings.Index(body, "Content-Type: text/html; charset=UTF-8")
	if plainIndex < 0 || htmlIndex < 0 || plainIndex > htmlIndex {
		t.Fatalf("expected text/plain part before text/html part, got %q", body)
	}
}

func TestSMTPEmailerRenderPayloadPlainTextWithoutHTML(t *testing.T) {
	t.Parallel()

	emailer := &SMTPEmailer{fromLine: "Rota <noreply@example.com>"}
	payload, err := emailer.renderPayload(Message{
		To:      "worker@example.com",
		Subject: "Plain",
		Body:    "Only text",
	})
	if err != nil {
		t.Fatalf("renderPayload returned error: %v", err)
	}

	body := string(payload)
	if !strings.Contains(body, "Content-Type: text/plain; charset=UTF-8") {
		t.Fatalf("expected plain text content type, got %q", body)
	}
	if strings.Contains(body, "text/html") || strings.Contains(body, "multipart/alternative") {
		t.Fatalf("plain message must not include HTML or multipart payload: %q", body)
	}
}

type smtpTestServerOptions struct {
	advertiseStartTLS  bool
	implicitTLS        bool
	hangBeforeGreeting bool
	hangAfterData      bool
}

type smtpTestServer struct {
	t           *testing.T
	listener    net.Listener
	tlsConfig   *tls.Config
	certificate *x509.Certificate

	advertiseStartTLS  bool
	hangBeforeGreeting bool
	hangAfterData      bool

	done     chan struct{}
	stop     chan struct{}
	stopOnce sync.Once

	mu           sync.Mutex
	startTLSUsed bool
	messages     []string
}

func newSMTPTestServer(t *testing.T, options smtpTestServerOptions) *smtpTestServer {
	t.Helper()

	tlsConfig, certificate := mustGenerateServerTLSConfig(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen returned error: %v", err)
	}
	if options.implicitTLS {
		listener = tls.NewListener(listener, tlsConfig)
	}

	server := &smtpTestServer{
		t:                  t,
		listener:           listener,
		tlsConfig:          tlsConfig,
		certificate:        certificate,
		advertiseStartTLS:  options.advertiseStartTLS,
		hangBeforeGreeting: options.hangBeforeGreeting,
		hangAfterData:      options.hangAfterData,
		done:               make(chan struct{}),
		stop:               make(chan struct{}),
	}

	go server.serve()

	return server
}

func (s *smtpTestServer) serve() {
	defer close(s.done)

	conn, err := s.listener.Accept()
	if err != nil {
		return
	}
	defer conn.Close()

	if s.hangBeforeGreeting {
		<-s.stop
		return
	}

	if err := s.handleConnection(conn); err != nil && !errors.Is(err, io.EOF) {
		s.t.Errorf("SMTP test server error: %v", err)
	}
}

func (s *smtpTestServer) handleConnection(conn net.Conn) error {
	reader := newSMTPTestReader(conn)
	writer := newSMTPTestWriter(conn)
	if err := writer.line("220 localhost ESMTP ready"); err != nil {
		return err
	}

	for {
		line, err := reader.line()
		if err != nil {
			return err
		}

		command := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(command, "EHLO "), strings.HasPrefix(command, "HELO "):
			if err := writer.greetingCapabilities(s.advertiseStartTLS); err != nil {
				return err
			}
		case command == "STARTTLS":
			if !s.advertiseStartTLS {
				if err := writer.line("454 TLS not available"); err != nil {
					return err
				}
				continue
			}

			if err := writer.line("220 Ready to start TLS"); err != nil {
				return err
			}

			tlsConn := tls.Server(conn, s.tlsConfig)
			if err := tlsConn.Handshake(); err != nil {
				return err
			}

			s.recordStartTLS()
			conn = tlsConn
			reader = newSMTPTestReader(conn)
			writer = newSMTPTestWriter(conn)
			s.advertiseStartTLS = false
		case strings.HasPrefix(command, "MAIL FROM:"):
			if err := writer.line("250 OK"); err != nil {
				return err
			}
		case strings.HasPrefix(command, "RCPT TO:"):
			if err := writer.line("250 OK"); err != nil {
				return err
			}
		case command == "DATA":
			if err := writer.line("354 End data with <CR><LF>.<CR><LF>"); err != nil {
				return err
			}
			message, err := reader.data()
			if err != nil {
				return err
			}
			s.recordMessage(message)
			if s.hangAfterData {
				<-s.stop
				return nil
			}
			if err := writer.line("250 Message accepted"); err != nil {
				return err
			}
		case command == "QUIT":
			return writer.line("221 Bye")
		default:
			return fmt.Errorf("unexpected SMTP command: %q", line)
		}
	}
}

func (s *smtpTestServer) Close() {
	s.t.Helper()

	_ = s.listener.Close()
	s.stopOnce.Do(func() {
		close(s.stop)
	})
	select {
	case <-s.done:
	case <-time.After(2 * time.Second):
		s.t.Fatal("timed out waiting for SMTP test server shutdown")
	}
}

func (s *smtpTestServer) Port() int {
	s.t.Helper()

	return s.listener.Addr().(*net.TCPAddr).Port
}

func (s *smtpTestServer) Certificate() *x509.Certificate {
	s.t.Helper()

	return s.certificate
}

func (s *smtpTestServer) StartTLSUsed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.startTLSUsed
}

func (s *smtpTestServer) LastMessage() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.messages) == 0 {
		return ""
	}

	return s.messages[len(s.messages)-1]
}

func (s *smtpTestServer) recordStartTLS() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.startTLSUsed = true
}

func (s *smtpTestServer) recordMessage(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.messages = append(s.messages, message)
}

type smtpTestReader struct {
	conn   net.Conn
	buffer []byte
}

func newSMTPTestReader(conn net.Conn) *smtpTestReader {
	return &smtpTestReader{conn: conn}
}

func (r *smtpTestReader) line() (string, error) {
	if err := r.conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		return "", err
	}

	for {
		index := bytes.Index(r.buffer, []byte("\r\n"))
		if index >= 0 {
			line := string(r.buffer[:index])
			r.buffer = r.buffer[index+2:]
			return line, nil
		}

		chunk := make([]byte, 1024)
		n, err := r.conn.Read(chunk)
		if err != nil {
			return "", err
		}
		r.buffer = append(r.buffer, chunk[:n]...)
	}
}

func (r *smtpTestReader) data() (string, error) {
	var lines []string
	for {
		line, err := r.line()
		if err != nil {
			return "", err
		}
		if line == "." {
			return strings.Join(lines, "\r\n"), nil
		}
		lines = append(lines, line)
	}
}

type smtpTestWriter struct {
	conn net.Conn
}

func newSMTPTestWriter(conn net.Conn) *smtpTestWriter {
	return &smtpTestWriter{conn: conn}
}

func (w *smtpTestWriter) line(value string) error {
	if err := w.conn.SetWriteDeadline(time.Now().Add(2 * time.Second)); err != nil {
		return err
	}
	_, err := w.conn.Write([]byte(value + "\r\n"))
	return err
}

func (w *smtpTestWriter) greetingCapabilities(includeStartTLS bool) error {
	lines := []string{"250-localhost"}
	if includeStartTLS {
		lines = append(lines, "250-STARTTLS")
	}
	lines = append(lines, "250 OK")

	for _, line := range lines {
		if err := w.line(line); err != nil {
			return err
		}
	}
	return nil
}

func mustGenerateServerTLSConfig(t *testing.T) (*tls.Config, *x509.Certificate) {
	t.Helper()

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey returned error: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "localhost",
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:              []string{"localhost"},
		BasicConstraintsValid: true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, template, template, privateKey.Public(), privateKey)
	if err != nil {
		t.Fatalf("CreateCertificate returned error: %v", err)
	}

	certificate, err := x509.ParseCertificate(derBytes)
	if err != nil {
		t.Fatalf("ParseCertificate returned error: %v", err)
	}

	pemCertificate := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	pemKey, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		t.Fatalf("MarshalECPrivateKey returned error: %v", err)
	}
	pemPrivateKey := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: pemKey})

	serverCertificate, err := tls.X509KeyPair(pemCertificate, pemPrivateKey)
	if err != nil {
		t.Fatalf("X509KeyPair returned error: %v", err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{serverCertificate},
		MinVersion:   tls.VersionTLS12,
	}, certificate
}

func withTLSMode(config SMTPConfig, mode string) SMTPConfig {
	config.TLSMode = mode
	return config
}

func trustServerCertificate(t *testing.T, emailer *SMTPEmailer, certificate *x509.Certificate) {
	t.Helper()

	rootCAs := x509.NewCertPool()
	rootCAs.AddCert(certificate)

	emailer.tlsConfig = &tls.Config{
		MinVersion: tls.VersionTLS12,
		RootCAs:    rootCAs,
		ServerName: "localhost",
	}
}
