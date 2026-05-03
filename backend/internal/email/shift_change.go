package email

import (
	"fmt"
	"strings"
	"time"
)

// ShiftChangeType mirrors the service-level enum. We duplicate the constants
// here to avoid an import cycle between the email package and the model
// package.
type ShiftChangeType string

const (
	ShiftChangeTypeSwap       ShiftChangeType = "swap"
	ShiftChangeTypeGiveDirect ShiftChangeType = "give_direct"
	ShiftChangeTypeGivePool   ShiftChangeType = "give_pool"
)

// ShiftChangeOutcome distinguishes the resolution messages.
type ShiftChangeOutcome string

const (
	ShiftChangeOutcomeApproved    ShiftChangeOutcome = "approved"
	ShiftChangeOutcomeRejected    ShiftChangeOutcome = "rejected"
	ShiftChangeOutcomeClaimed     ShiftChangeOutcome = "claimed"
	ShiftChangeOutcomeCancelled   ShiftChangeOutcome = "cancelled"
	ShiftChangeOutcomeInvalidated ShiftChangeOutcome = "invalidated"
)

// ShiftRef describes a single slot-position assignment in human-readable
// form for email bodies. Callers assemble this from slot-based scheduling
// data plus the display position name.
type ShiftRef struct {
	Weekday        string // "Mon", "Tue", …
	StartTime      string // "09:00"
	EndTime        string // "12:00"
	PositionName   string
	OccurrenceDate *time.Time
}

// ShiftChangeRequestReceivedData drives the "someone is asking you to swap"
// or "someone wants to give you a shift" email.
type ShiftChangeRequestReceivedData struct {
	To               string
	RecipientName    string
	RequesterName    string
	Type             ShiftChangeType
	RequesterShift   ShiftRef
	CounterpartShift *ShiftRef // non-nil only for swap
	BaseURL          string
	Language         string
	Branding         Branding
}

// ShiftChangeResolvedData drives the "your request was approved/rejected/
// claimed/cancelled" email to the requester.
type ShiftChangeResolvedData struct {
	To               string
	RecipientName    string
	Outcome          ShiftChangeOutcome
	Type             ShiftChangeType
	ResponderName    string // the counterpart / claimer; may be empty for cancelled
	RequesterShift   ShiftRef
	CounterpartShift *ShiftRef
	BaseURL          string
	Language         string
	Branding         Branding
}

// BuildShiftChangeRequestReceivedMessage builds the email sent to the
// counterpart when a swap or give_direct request is created.
func BuildShiftChangeRequestReceivedMessage(data ShiftChangeRequestReceivedData) Message {
	return renderShiftChangeRequestReceivedMessage(data)
}

// BuildShiftChangeResolvedMessage builds the email sent to the requester
// when their request is approved, rejected, claimed, or cancelled.
func BuildShiftChangeResolvedMessage(data ShiftChangeResolvedData) Message {
	return renderShiftChangeResolvedMessage(data)
}

func formatShiftRefValue(s ShiftRef) string {
	return fmt.Sprintf("%s %s–%s %s", s.Weekday, s.StartTime, s.EndTime, s.PositionName)
}

func formatShiftRef(s *ShiftRef) string {
	if s == nil {
		return "(unknown)"
	}
	return formatShiftRefValue(*s)
}

func isZeroShiftRef(s ShiftRef) bool {
	return s.Weekday == "" && s.StartTime == "" && s.EndTime == "" && s.PositionName == "" && s.OccurrenceDate == nil
}

func requestsLink(baseURL string) string {
	return strings.TrimRight(baseURL, "/") + "/requests"
}

func FormatShiftRef(s ShiftRef, language string) string {
	if s.OccurrenceDate != nil && !s.OccurrenceDate.IsZero() {
		if normalizeLanguage(language) == "zh" {
			return fmt.Sprintf(
				"%s（%s）%s-%s %s",
				s.OccurrenceDate.Format("2006-01-02"),
				weekdayZH(s.OccurrenceDate.Weekday()),
				s.StartTime,
				s.EndTime,
				s.PositionName,
			)
		}
		return fmt.Sprintf(
			"%s, %s-%s %s",
			s.OccurrenceDate.Format("Mon, Jan 2, 2006"),
			s.StartTime,
			s.EndTime,
			s.PositionName,
		)
	}
	if normalizeLanguage(language) == "zh" {
		return fmt.Sprintf("%s %s-%s %s", localizedWeekdayLabel(s.Weekday, language), s.StartTime, s.EndTime, s.PositionName)
	}
	return formatShiftRefValue(s)
}

func shiftChangeTypeLabel(changeType ShiftChangeType, language string) string {
	switch changeType {
	case ShiftChangeTypeSwap:
		return localizedString(language, "swap", "换班")
	case ShiftChangeTypeGiveDirect:
		return localizedString(language, "direct give-away", "定向转让")
	case ShiftChangeTypeGivePool:
		return localizedString(language, "pool give-away", "开放认领")
	default:
		return localizedString(language, "shift change", "换班")
	}
}

func shiftChangeOutcomeLabel(outcome ShiftChangeOutcome, language string) string {
	switch outcome {
	case ShiftChangeOutcomeApproved:
		return localizedString(language, "approved", "已批准")
	case ShiftChangeOutcomeRejected:
		return localizedString(language, "rejected", "已拒绝")
	case ShiftChangeOutcomeClaimed:
		return localizedString(language, "claimed", "已认领")
	case ShiftChangeOutcomeCancelled:
		return localizedString(language, "cancelled", "已取消")
	case ShiftChangeOutcomeInvalidated:
		return localizedString(language, "invalidated", "已失效")
	default:
		return localizedString(language, "updated", "已更新")
	}
}

func localizedWeekdayLabel(weekday string, language string) string {
	if normalizeLanguage(language) != "zh" {
		return weekday
	}
	switch weekday {
	case "Mon":
		return "周一"
	case "Tue":
		return "周二"
	case "Wed":
		return "周三"
	case "Thu":
		return "周四"
	case "Fri":
		return "周五"
	case "Sat":
		return "周六"
	case "Sun":
		return "周日"
	default:
		return weekday
	}
}

func weekdayZH(weekday time.Weekday) string {
	switch weekday {
	case time.Monday:
		return "周一"
	case time.Tuesday:
		return "周二"
	case time.Wednesday:
		return "周三"
	case time.Thursday:
		return "周四"
	case time.Friday:
		return "周五"
	case time.Saturday:
		return "周六"
	case time.Sunday:
		return "周日"
	default:
		return ""
	}
}
