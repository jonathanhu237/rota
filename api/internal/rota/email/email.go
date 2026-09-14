package email

import "strings"

// Message is the rendered, business notification payload handed to the
// template-owned mail-task/outbox adapter. Transport, account mail, and
// credential-bearing templates belong to the Temvia authentication package.
type Message struct {
	Kind             string
	To               string
	Name             string
	Language         string
	SystemName       string
	OrganizationName string
	Subject          string
	Body             string
	HTMLBody         string
}

const (
	KindShiftChangeRequestReceived = "shift_change_request_received"
	KindShiftChangeResolved        = "shift_change_resolved"
)

func NormalizeLanguage(language string) string {
	normalized := strings.ToLower(strings.TrimSpace(language))
	switch {
	case normalized == "zh" || strings.HasPrefix(normalized, "zh-"):
		return "zh"
	case normalized == "en" || strings.HasPrefix(normalized, "en-"):
		return "en"
	default:
		return "en"
	}
}

func normalizeLanguage(language string) string {
	return NormalizeLanguage(language)
}
