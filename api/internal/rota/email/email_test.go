package email

import (
	"strings"
	"testing"
	"time"
)

func TestShiftChangeRequestEmailRendersBusinessContentInBothLanguages(t *testing.T) {
	t.Parallel()

	date := time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC)
	counterpartDate := date.AddDate(0, 0, 1)
	for _, language := range []string{"en", "zh"} {
		language := language
		t.Run(language, func(t *testing.T) {
			t.Parallel()
			message := BuildShiftChangeRequestReceivedMessage(ShiftChangeRequestReceivedData{
				To:            "bob@example.com",
				RecipientName: "Bob & trusted",
				RequesterName: "Alice",
				Type:          ShiftChangeTypeSwap,
				RequesterShift: ShiftRef{
					StartTime: "09:00", EndTime: "12:00", PositionName: "Front Desk", OccurrenceDate: &date,
				},
				CounterpartShift: &ShiftRef{
					StartTime: "13:00", EndTime: "16:00", PositionName: "Support", OccurrenceDate: &counterpartDate,
				},
				BaseURL:  "https://app.example.com/",
				Language: language,
				Branding: Branding{ProductName: "OpsHub", OrganizationName: "Acme"},
			})

			if message.Kind != KindShiftChangeRequestReceived || message.To != "bob@example.com" {
				t.Fatalf("unexpected message identity: %#v", message)
			}
			if message.Subject == "" || message.Body == "" || message.HTMLBody == "" {
				t.Fatalf("business notification did not render: %#v", message)
			}
			for _, body := range []string{message.Body, message.HTMLBody} {
				if !strings.Contains(body, "https://app.example.com/rota/requests") {
					t.Fatalf("notification missing canonical requests link: %q", body)
				}
				if strings.Contains(body, "<trusted>") {
					t.Fatalf("HTML renderer leaked raw recipient markup: %q", body)
				}
			}
			if !strings.Contains(message.HTMLBody, "Bob &amp; trusted") {
				t.Fatalf("HTML notification did not escape recipient name: %q", message.HTMLBody)
			}
			if language == "zh" {
				if !strings.Contains(message.Body, "周一") || !strings.Contains(message.Body, "来自Acme") {
					t.Fatalf("Chinese notification missing localized business content: %q", message.Body)
				}
			} else if !strings.Contains(message.Body, "Mon, May 4, 2026") || !strings.Contains(message.Body, "from Acme") {
				t.Fatalf("English notification missing localized business content: %q", message.Body)
			}
		})
	}
}

func TestLeaveCoverageEmailUsesLeaveDetailRoute(t *testing.T) {
	t.Parallel()

	leaveID := int64(42)
	date := time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC)
	shift := ShiftRef{StartTime: "09:00", EndTime: "12:00", PositionName: "Front Desk", OccurrenceDate: &date}
	request := BuildShiftChangeRequestReceivedMessage(ShiftChangeRequestReceivedData{
		To: "bob@example.com", RecipientName: "Bob", RequesterName: "Alice", Type: ShiftChangeTypeGiveDirect,
		LeaveID: &leaveID, RequesterShift: shift, BaseURL: "https://app.example.com", Language: "en",
	})
	resolved := BuildShiftChangeResolvedMessage(ShiftChangeResolvedData{
		To: "alice@example.com", RecipientName: "Alice", Outcome: ShiftChangeOutcomeClaimed, Type: ShiftChangeTypeGivePool,
		LeaveID: &leaveID, ResponderName: "Bob", RequesterShift: shift, BaseURL: "https://app.example.com", Language: "en",
	})

	for name, message := range map[string]Message{"request": request, "resolved": resolved} {
		t.Run(name, func(t *testing.T) {
			if !strings.Contains(message.Body, "https://app.example.com/rota/leaves/42") ||
				!strings.Contains(message.HTMLBody, "https://app.example.com/rota/leaves/42") {
				t.Fatalf("%s message missing leave detail link: %#v", name, message)
			}
			if strings.Contains(message.Body, "https://app.example.com/rota/requests") {
				t.Fatalf("%s leave message points to generic requests page: %q", name, message.Body)
			}
		})
	}
}

func TestShiftChangeOutcomeAndLanguageAreLocalized(t *testing.T) {
	t.Parallel()

	date := time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC)
	for _, outcome := range []ShiftChangeOutcome{
		ShiftChangeOutcomeApproved,
		ShiftChangeOutcomeRejected,
		ShiftChangeOutcomeClaimed,
		ShiftChangeOutcomeCancelled,
		ShiftChangeOutcomeInvalidated,
	} {
		outcome := outcome
		t.Run(string(outcome), func(t *testing.T) {
			for _, language := range []string{"en", "zh"} {
				message := BuildShiftChangeResolvedMessage(ShiftChangeResolvedData{
					To: "alice@example.com", RecipientName: "Alice", Outcome: outcome, Type: ShiftChangeTypeSwap,
					ResponderName: "Bob", RequesterShift: ShiftRef{StartTime: "09:00", EndTime: "12:00", PositionName: "Desk", OccurrenceDate: &date},
					BaseURL: "https://app.example.com", Language: language,
				})
				if message.Subject == "" || message.Body == "" || message.HTMLBody == "" {
					t.Fatalf("empty %s %s message", language, outcome)
				}
			}
		})
	}
}

func TestParseAcceptLanguagePrefersSupportedHighestQuality(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"zh-CN,zh;q=0.9,en;q=0.8": "zh",
		"en-US,en;q=0.9":          "en",
		"fr-FR,fr;q=0.9":          "en",
		"en;q=0.2,zh;q=0.9":       "zh",
		"":                        "en",
	}
	for header, want := range tests {
		if got := ParseAcceptLanguage(header); got != want {
			t.Fatalf("ParseAcceptLanguage(%q) = %q, want %q", header, got, want)
		}
	}
}

func TestFormatShiftRefLocalizesOccurrenceDate(t *testing.T) {
	t.Parallel()

	date := time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC)
	ref := ShiftRef{StartTime: "09:00", EndTime: "12:00", PositionName: "Front Desk Assistant", OccurrenceDate: &date}
	if got := FormatShiftRef(ref, "en"); got != "Mon, May 4, 2026, 09:00-12:00 Front Desk Assistant" {
		t.Fatalf("English shift summary = %q", got)
	}
	if got := FormatShiftRef(ref, "zh"); got != "2026-05-04（周一）09:00-12:00 Front Desk Assistant" {
		t.Fatalf("Chinese shift summary = %q", got)
	}
}
